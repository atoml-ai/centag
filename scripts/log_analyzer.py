#!/usr/bin/env python3
"""
Centag 日志分析脚本
分析指定日期的日志文件，生成 Markdown 报告和 JSON 摘要
"""

import argparse
import json
import os
import re
from collections import defaultdict
from datetime import datetime, timedelta


def get_target_date(yesterday: bool) -> str:
    """获取目标分析日期"""
    if yesterday:
        return (datetime.now() - timedelta(days=1)).strftime("%Y-%m-%d")
    return datetime.now().strftime("%Y-%m-%d")


def parse_log_line(line: str) -> dict | None:
    """解析单行日志，支持三种格式"""
    line = line.strip()
    if not line:
        return None

    # 尝试解析 JSON 格式日志
    if line.startswith("{"):
        try:
            data = json.loads(line)
            level = data.get("level", "info").upper()
            timestamp = data.get("timestamp", "")
            message = data.get("message", "")
            caller = data.get("caller", "")

            # 从 caller 提取模块名
            module = "unknown"
            if caller:
                parts = caller.split("/")
                if parts:
                    module = parts[0]

            # 从 timestamp 提取日期
            date = ""
            hour = ""
            if timestamp:
                match = re.match(r"(\d{4}-\d{2}-\d{2})T(\d{2})", timestamp)
                if match:
                    date = match.group(1)
                    hour = match.group(2)

            return {
                "level": level,
                "timestamp": timestamp,
                "message": message,
                "module": module,
                "date": date,
                "hour": hour,
                "raw": line,
            }
        except json.JSONDecodeError:
            pass

    # 尝试解析 tab 分隔格式：2026-04-16T09:03:33.552+0800\tinfo\tlogger/logger.go:128\tMessage
    tab_match = re.match(
        r"(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})\.\d+[+-]\d+\t(\w+)\t([^\t]+)\t(.*)",
        line,
    )
    if tab_match:
        date_str = tab_match.group(1)
        hour = tab_match.group(2).split(":")[0]
        level = tab_match.group(3).upper()
        caller = tab_match.group(4)
        message = tab_match.group(5)

        # 从 caller 提取模块名
        module = "unknown"
        if caller:
            parts = caller.split("/")
            if parts:
                module = parts[0]

        return {
            "level": level,
            "timestamp": f"{date_str}T{tab_match.group(2)}",
            "message": message,
            "module": module,
            "date": date_str,
            "hour": hour,
            "raw": line,
        }

    # 尝试解析简单格式日志：2026/04/09 09:27:56 [Module] Message
    simple_match = re.match(
        r"(\d{4}/\d{2}/\d{2})\s+(\d{2}:\d{2}:\d{2})\s+\[([^\]]+)\]\s+(.*)", line
    )
    if simple_match:
        date_str = simple_match.group(1).replace("/", "-")
        hour = simple_match.group(2).split(":")[0]
        module = simple_match.group(3)
        message = simple_match.group(4)

        # 判断日志级别
        level = "INFO"
        if "error" in message.lower() or "fail" in message.lower():
            level = "ERROR"
        elif "warn" in message.lower():
            level = "WARN"

        return {
            "level": level,
            "timestamp": f"{date_str}T{simple_match.group(2)}",
            "message": message,
            "module": module,
            "date": date_str,
            "hour": hour,
            "raw": line,
        }

    return None


def extract_backend_service(message: str) -> str | None:
    """从日志消息中提取后端服务名称"""
    backend_patterns = [
        (r"\[OpenAI Backend\]", "OpenAI"),
        (r"\[Ollama Backend\]", "Ollama"),
        (r"\[Anthropic Backend\]", "Anthropic"),
        (r"\[OpenAI Protocol\]", "OpenAI Protocol"),
        (r"ollama", "Ollama"),
        (r"openai", "OpenAI"),
        (r"anthropic", "Anthropic"),
        (r"azure", "Azure"),
        (r"gemini", "Gemini"),
    ]

    for pattern, name in backend_patterns:
        if re.search(pattern, message, re.IGNORECASE):
            return name
    return None


def analyze_logs(log_path: str, target_date: str) -> dict:
    """分析日志文件"""
    stats = {
        "total": 0,
        "by_level": defaultdict(int),
        "by_module": defaultdict(int),
        "by_hour": defaultdict(int),
        "by_backend": defaultdict(int),
        "errors": [],
        "warnings": [],
        "date": target_date,
    }

    if not os.path.exists(log_path):
        return stats

    with open(log_path, "r", encoding="utf-8", errors="ignore") as f:
        for line in f:
            parsed = parse_log_line(line)
            if not parsed:
                continue

            # 只统计目标日期的日志
            if parsed["date"] != target_date:
                continue

            stats["total"] += 1
            stats["by_level"][parsed["level"]] += 1
            stats["by_module"][parsed["module"]] += 1
            stats["by_hour"][parsed["hour"]] += 1

            # 提取后端服务
            backend = extract_backend_service(parsed["message"])
            if backend:
                stats["by_backend"][backend] += 1

            # 收集错误和警告
            if parsed["level"] == "ERROR":
                stats["errors"].append(parsed["raw"])
            elif parsed["level"] == "WARN":
                stats["warnings"].append(parsed["raw"])

    return stats


def generate_markdown_report(stats: dict, output_path: str) -> None:
    """生成 Markdown 格式报告"""
    date = stats["date"]
    generated_at = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

    # 检查是否有严重错误
    error_count = stats["by_level"].get("ERROR", 0)
    warning_header = ""
    if error_count > 10:
        warning_header = "## ⚠️ 警告：检测到严重错误\n\n"
        warning_header += f"本日错误数达到 **{error_count}** 条，超过阈值 (10)，请检查系统状态！\n\n"

    # 模块统计 (Top 10)
    top_modules = sorted(stats["by_module"].items(), key=lambda x: x[1], reverse=True)[
        :10
    ]

    # 小时分布
    hours = sorted(stats["by_hour"].items(), key=lambda x: int(x[0]) if x[0].isdigit() else 0)

    # 后端服务统计
    backends = sorted(stats["by_backend"].items(), key=lambda x: x[1], reverse=True)

    md_content = f"""# Centag 日志分析报告

{warning_header}**日期：** {date}
**生成时间：** {generated_at}

## 📊 总览

| 指标 | 数值 |
|------|------|
| 总日志数 | {stats['total']} |
| 错误数 | {stats['by_level'].get('ERROR', 0)} |
| 警告数 | {stats['by_level'].get('WARN', 0)} |
| 信息数 | {stats['by_level'].get('INFO', 0)} |

## 🔧 模块分支统计 (Top 10)

| 排名 | 模块 | 日志数 | 占比 |
|------|------|--------|------|
"""

    for i, (module, count) in enumerate(top_modules, 1):
        percentage = (count / stats["total"] * 100) if stats["total"] > 0 else 0
        md_content += f"| {i} | {module} | {count} | {percentage:.1f}% |\n"

    md_content += "\n## 🕐 按小时分布\n\n| 小时 | 日志数 | 小时 | 日志数 |\n|------|--------|------|--------|\n"

    # 两列显示
    mid = len(hours) // 2
    for i in range(max(mid, len(hours) - mid)):
        left = hours[i] if i < len(hours) else ("", 0)
        right = hours[i + mid] if i + mid < len(hours) else ("", 0)
        md_content += f"| {left[0]}:00 | {left[1]} | {right[0]}:00 | {right[1]} |\n"

    md_content += "\n## 🖥️ 后端服务统计\n\n| 服务 | 请求数 | 占比 |\n|------|--------|------|\n"

    for service, count in backends:
        percentage = (count / stats["total"] * 100) if stats["total"] > 0 else 0
        md_content += f"| {service} | {count} | {percentage:.1f}% |\n"

    # 错误详情
    md_content += "\n## ❌ 错误详情 (Top 20)\n\n```\n"
    for error in stats["errors"][:20]:
        md_content += f"{error}\n"
    if not stats["errors"]:
        md_content += "无错误日志\n"
    md_content += "```\n"

    # 警告详情
    md_content += "\n## ⚠️ 警告详情 (Top 20)\n\n```\n"
    for warning in stats["warnings"][:20]:
        md_content += f"{warning}\n"
    if not stats["warnings"]:
        md_content += "无警告日志\n"
    md_content += "```\n"

    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    with open(output_path, "w", encoding="utf-8") as f:
        f.write(md_content)


def generate_json_summary(stats: dict, output_path: str) -> None:
    """生成 JSON 格式摘要"""
    summary = {
        "date": stats["date"],
        "generated_at": datetime.now().isoformat(),
        "total": stats["total"],
        "by_level": dict(stats["by_level"]),
        "top_modules": dict(
            sorted(stats["by_module"].items(), key=lambda x: x[1], reverse=True)[:10]
        ),
        "by_backend": dict(stats["by_backend"]),
        "error_count": len(stats["errors"]),
        "warning_count": len(stats["warnings"]),
    }

    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(summary, f, indent=2, ensure_ascii=False)


def main():
    parser = argparse.ArgumentParser(description="Centag 日志分析工具")
    parser.add_argument(
        "--yesterday",
        action="store_true",
        help="分析昨天的日志",
    )
    parser.add_argument(
        "--date",
        type=str,
        help="指定分析日期 (YYYY-MM-DD)",
    )
    parser.add_argument(
        "--log-path",
        type=str,
        default=os.path.expanduser("~/aispaces/centag/bin/logs/centag.log"),
        help="日志文件路径",
    )
    parser.add_argument(
        "--output-dir",
        type=str,
        default=os.path.expanduser("~/aispaces/centag/bin/logs/analysis"),
        help="报告输出目录",
    )

    args = parser.parse_args()

    # 确定目标日期
    if args.date:
        target_date = args.date
    else:
        target_date = get_target_date(args.yesterday)

    print(f"开始分析日志...")
    print(f"目标日期：{target_date}")
    print(f"日志文件：{args.log_path}")

    # 分析日志
    stats = analyze_logs(args.log_path, target_date)

    # 生成报告
    md_path = os.path.join(args.output_dir, f"log_analysis_{target_date}.md")
    json_path = os.path.join(args.output_dir, f"log_summary_{target_date}.json")

    generate_markdown_report(stats, md_path)
    generate_json_summary(stats, json_path)

    print(f"\n分析完成!")
    print(f"总日志数：{stats['total']}")
    print(f"错误数：{stats['by_level'].get('ERROR', 0)}")
    print(f"警告数：{stats['by_level'].get('WARN', 0)}")
    print(f"Markdown 报告：{md_path}")
    print(f"JSON 摘要：{json_path}")

    if stats["by_level"].get("ERROR", 0) > 10:
        print("\n⚠️ 警告：错误数超过阈值，请检查系统状态!")


if __name__ == "__main__":
    main()
