#!/usr/bin/env python3
"""Centag 流水线模式自动化测试 v2 - 向导式配置 + 详细报告"""

import subprocess
import time
import json
import os
import sys
import argparse
import platform
from datetime import datetime

def load_config():
    """加载配置，优先级：环境变量 > 配置文件 > 代码默认值"""
    # 1. 代码默认值
    defaults = {
        "token": "",
        "base_url": "http://localhost:20060",
        "input": "python的特点",
        "backend": "ppmodel",
        "timeout": "30",
    }
    
    # 2. 配置文件 (可选的 .env 文件)
    config_paths = [
        os.path.expanduser("~/.centag/test-config.env"),
        os.path.join(os.path.dirname(__file__), "..", "secrets", ".env"),
        ".test-env",
    ]
    for config_path in config_paths:
        if os.path.exists(config_path):
            try:
                with open(config_path, "r") as f:
                    for line in f:
                        line = line.strip()
                        if line and not line.startswith("#") and "=" in line:
                            key, value = line.split("=", 1)
                            key = key.strip()
                            value = value.strip().strip('"').strip("'")
                            # 映射配置文件键到配置键
                            if key == "LLM_PROXY_ADMIN_API_KEY":
                                defaults["token"] = value
                            elif key == "LLM_PROXY_SERVER_PORT":
                                defaults["base_url"] = f"http://localhost:{value}"
                            elif key == "TEST_BACKEND":
                                defaults["backend"] = value
                            elif key == "TEST_TIMEOUT":
                                defaults["timeout"] = value
            except Exception:
                pass
            break  # 只读取第一个存在的配置文件
    
    # 3. 环境变量 (最高优先级)
    env_mappings = {
        "CENTAG_TEST_TOKEN": "token",
        "CENTAG_BASE_URL": "base_url",
        "CENTAG_TEST_INPUT": "input",
        "CENTAG_TEST_BACKEND": "backend",
        "CENTAG_TEST_TIMEOUT": "timeout",
    }
    for env_key, config_key in env_mappings.items():
        env_value = os.environ.get(env_key)
        if env_value:
            defaults[config_key] = env_value
    
    # 类型转换
    defaults["timeout"] = int(defaults.get("timeout", 30))
    
    return defaults

# 加载配置
DEFAULTS = load_config()

# 预设测试问题
PRESET_QUESTIONS = [
    "python的特点",
    "解释一下什么是机器学习",
    "写一首关于春天的诗",
    "1+1等于几",
    "用英文介绍一下中国",
    "如何学习编程",
]

# 虚拟模型名映射 (使用 pipeline_ 格式)
DEFAULT_VIRTUAL_MODELS = {
    "#d": "pipeline_direct-backend",
    "#s": "pipeline_smart-scheduling",
    "#f": "pipeline_fallback",
    "#o": "pipeline_optimize",
    "#a": "pipeline_audit",
    "#m": "pipeline_model-matching",
    "#r": "pipeline_router",
    "#ag": "pipeline_aggregator",
    "#l": "pipeline_translate",
    "#ch": "pipeline_cache-hit",  # cache hit
    "#cm": "pipeline_cache-mode", # cache mode
}

# 这些模式的尾节点为 processor（非 generator），响应 content 可能为空，
# 此时以 usage.total_tokens > 0 作为 LLM 调用成功的判定标准
CONTENT_OPTIONAL_MODES = {"#mem0"}

def get_pipeline_configs(base_url, token):
    """从数据库获取当前所有流水线配置"""
    try:
        # 尝试通过 API 获取流水线配置
        cmd = [
            "curl", "-s", "-X", "GET",
            f"{base_url}/api/v1/pipelines",
            "-H", f"Authorization: Bearer {token}",
            "-H", "Content-Type: application/json"
        ]
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
        
        if result.returncode == 0 and result.stdout:
            try:
                data = json.loads(result.stdout)
                if isinstance(data, dict) and 'data' in data:
                    return data['data']
                elif isinstance(data, dict) and 'pipelines' in data:
                    return data['pipelines']
                elif isinstance(data, list):
                    return data
                else:
                    return []
            except json.JSONDecodeError:
                return []
        else:
            return []
    except Exception as e:
        return []

def build_dynamic_virtual_models(base_url, token):
    """根据数据库中的实际配置构建虚拟模型映射"""
    pipelines = get_pipeline_configs(base_url, token)
    virtual_models = {}
    
    for pipeline in pipelines:
        pipeline_id = pipeline.get('id')
        shortcut_code = pipeline.get('shortcut_code')
        
        if shortcut_code and shortcut_code.strip():
            # 使用数据库中的实际快捷码
            virtual_models[shortcut_code.strip()] = f"pipeline_{pipeline_id}"
    
    # 添加默认映射作为后备
    default_models = DEFAULT_VIRTUAL_MODELS.copy()
    
    # 合并：数据库配置优先
    default_models.update(virtual_models)
    return default_models

def get_test_modes_from_db(base_url, token):
    """从数据库获取需要测试的模式"""
    pipelines = get_pipeline_configs(base_url, token)
    
    modes = []
    
    for pipeline in pipelines:
        pipeline_id = pipeline.get('id')
        shortcut_code = pipeline.get('shortcut_code', '')
        name = pipeline.get('name', pipeline_id)
        
        if shortcut_code:
            modes.append({
                "code": shortcut_code,
                "name": f"{name} ({shortcut_code})",
                "extra_headers": {}
            })
    
    return modes

REPORT_DIR = os.path.join(os.path.dirname(__file__), "..", "docs", "test-reports")

def print_header():
    """打印欢迎头"""
    print("\n" + "=" * 60)
    print("🚀 Centag 流水线模式自动化测试 v2.0")
    print("=" * 60)
    print()

def select_option(prompt, options, default_idx=0):
    """显示选项列表供用户选择"""
    print(f"\n{prompt}")
    for i, opt in enumerate(options):
        marker = "●" if i == default_idx else "○"
        print(f"  {marker} [{i}] {opt}")
    
    while True:
        choice = input(f"\n请选择 [0-{len(options)-1}] (默认 {default_idx}): ").strip()
        if choice == "":
            return options[default_idx]
        try:
            idx = int(choice)
            if 0 <= idx < len(options):
                return options[idx]
        except:
            pass
        print(f"⚠️  请输入 0-{len(options)-1} 之间的数字")

def input_with_default(prompt, default):
    """带默认值的输入"""
    value = input(f"{prompt} [{default}]: ").strip()
    return value if value else default

def wizard_config():
    """向导式配置"""
    print_header()
    
    # 1. 选择测试服务器
    print("📡 步骤 1/4: 配置测试服务器")
    print("-" * 60)
    
    server_options = [
        f"本地开发环境 ({DEFAULTS['base_url']})",
        "自定义服务器",
    ]
    server_choice = select_option("选择测试服务器:", server_options, 0)
    
    if "本地" in server_choice:
        base_url = DEFAULTS["base_url"]
        token = DEFAULTS["token"]
    else:
        base_url = input_with_default("服务器地址", DEFAULTS["base_url"])
        token = input_with_default("API Token", DEFAULTS["token"])
    
    # 2. 选择测试问题
    print("\n\n📝 步骤 2/4: 选择测试问题")
    print("-" * 60)
    
    question_options = PRESET_QUESTIONS.copy()
    question_options.append("自定义问题")
    question_choice = select_option("选择测试问题:", question_options, 0)
    
    if question_choice == "自定义问题":
        test_input = input_with_default("输入测试问题", DEFAULTS["input"])
    else:
        test_input = question_choice
    
    # 3. 选择测试后端
    print("\n\n🖥️  步骤 3/4: 配置测试后端")
    print("-" * 60)
    
    backend = input_with_default("后端名称 (用于 #d 直连模式)", DEFAULTS["backend"])
    
    # 4. 选择超时时间
    print("\n\n⏱️  步骤 4/4: 设置超时时间")
    print("-" * 60)
    
    timeout_options = ["15秒 (快速测试)", "30秒 (推荐)", "60秒 (完整测试)", "自定义"]
    timeout_choice = select_option("选择请求超时时间:", timeout_options, 1)
    
    if "15秒" in timeout_choice:
        timeout = 15
    elif "30秒" in timeout_choice:
        timeout = 30
    elif "60秒" in timeout_choice:
        timeout = 60
    else:
        timeout = int(input_with_default("超时时间(秒)", "30"))
    
    # 显示配置摘要
    print("\n\n" + "=" * 60)
    print("✅ 测试配置确认")
    print("=" * 60)
    print(f"  服务器: {base_url}")
    print(f"  Token: {token[:20]}...{token[-10:]}")
    print(f"  测试问题: {test_input}")
    print(f"  后端: {backend}")
    print(f"  超时: {timeout}秒")
    print("=" * 60)
    
    return {
        "token": token,
        "base_url": base_url,
        "input": test_input,
        "backend": backend,
        "timeout": timeout,
    }

def check_service(base_url):
    """检查服务是否运行"""
    try:
        resp = subprocess.run(
            ["curl", "-s", f"{base_url}/api/v1/status"],
            capture_output=True, text=True, timeout=5
        )
        return "healthy" in resp.stdout
    except:
        return False

def test_mode(mode_info, config, log_file, method='model', virtual_models=None):
    """测试单个模式"""
    code = mode_info["code"]
    name = mode_info["name"]
    headers = mode_info["extra_headers"]
    timeout = config["timeout"]
    
    # 使用虚拟模型名
    if virtual_models is None:
        virtual_models = DEFAULT_VIRTUAL_MODELS
    model = virtual_models.get(code, "pipeline.custom.auto")
    
    print(f"\n{'━' * 60}")
    print(f"测试: {name} ({code}) [方式: {method}]")
    print(f"模型: {model}")
    
    # 某些模式需要更长的超时时间
    mode_timeout = timeout
    # 缓存相关模式：默认的缓存模式快捷码或包含cache的自定义快捷码
    is_cache_mode = code in ["#ch", "#cm"] or any(cache_keyword in code.lower() for cache_keyword in ["cache", "fuck"])
    if code in ["#f", "#a", "#ag", "#l", "#mem0"] or is_cache_mode:  # 降级、审核、聚合、翻译、Mem0、缓存模式需要更长超时
        mode_timeout = max(timeout, 120)  # 至少 120 秒
    
    # 根据指定方式构建请求数据
    request_data = {}
    cmd_headers = ["Content-Type: application/json"]
    
    if method == 'header':
        # 方式1: 使用 X-Proxy-Mode 头
        request_data = {
            "model": "test-model",
            "messages": [{"role": "user", "content": config["input"]}]
        }
        cmd_headers.append(f"X-Proxy-Mode: {code}")
    elif method == 'model':
        # 方式2: 使用 model 字段 (推荐)
        request_data = {
            "model": model,
            "messages": [{"role": "user", "content": config["input"]}]
        }
    elif method == 'content':
        # 方式3: 使用 content 前缀
        request_data = {
            "model": "test-model",
            "messages": [{"role": "user", "content": f"{code} {config['input']}"}]
        }
    
    request_json = json.dumps(request_data, ensure_ascii=False)
    
    # 构建 curl 命令
    cmd = [
        "curl", "-s", "--max-time", str(mode_timeout),
        "-X", "POST",
        f"{config['base_url']}/v1/chat/completions?token={config['token']}",
    ]
    
    for header in cmd_headers:
        cmd.extend(["-H", header])
    
    cmd.extend(["-d", request_json])
    
    # 执行请求
    start_time = time.time()
    try:
        resp = subprocess.run(cmd, capture_output=True, text=True, timeout=mode_timeout + 5)
        response_text = resp.stdout
        raw_response = response_text
    except subprocess.TimeoutExpired:
        response_text = ""
        raw_response = "TIMEOUT"
    end_time = time.time()
    
    duration = int((end_time - start_time) * 1000)
    
    # 分析响应
    parsed_data = None
    try:
        parsed_data = json.loads(response_text)
        if "choices" in parsed_data and len(parsed_data["choices"]) > 0:
            content = parsed_data["choices"][0].get("message", {}).get("content", "")
            if content and content.strip():
                # 改进的判断策略：
                # 1. 首先检查是否是错误响应
                # 2. 然后检查内容长度，过短的内容可能未调用 LLM
                # 3. 最后才比较是否等于输入
                
                input_len = len(config["input"].strip())
                content_len = len(content.strip())
                
                # 判断是否可能未调用 LLM（内容太短或等于输入）
                # 但要考虑正常情况：翻译模式、摘要模式等可能返回较短内容
                is_likely_not_llm = False
                
                # 情况1: 内容完全等于输入（可能是直通或缓存未命中）
                if content.strip() == config["input"]:
                    is_likely_not_llm = True
                # 情况2: 内容长度小于输入长度的 50%（可能是无效响应）
                elif content_len < input_len * 0.5 and content_len < 50:
                    is_likely_not_llm = True
                # 情况3: 内容是常见的无效提示
                elif content.strip().lower() in ['python的特点', 'python', 'input', 'test']:
                    is_likely_not_llm = True
                
                if is_likely_not_llm:
                    # 进一步验证：检查是否包含典型的 LLM 回答特征
                    llm_indicators = [
                        '，', '。', '、',  # 中文标点
                        '\n',  # 换行
                        'Python', 'python', '机器学习', 'AI',  # 常见内容关键词
                        '###', '**', '*',  # Markdown 格式
                    ]
                    
                    has_llm_indicators = any(indicator in content for indicator in llm_indicators)
                    
                    if has_llm_indicators:
                        # 有 LLM 特征，但内容等于输入，可能是缓存问题
                        status = "WARN"
                        detail = f"内容等于输入，可能命中缓存或未调用 LLM (内容长度: {content_len})"
                    else:
                        # 确实没有调用 LLM
                        status = "WARN"
                        detail = "返回原始输入，未调用 LLM"
                else:
                    status = "SUCCESS"
                    detail = content[:200]
            else:
                # 内容为空：对 content-optional 模式（如 #mem0），
                # 检查 usage 指标确认 LLM 是否实际被调用
                if code in CONTENT_OPTIONAL_MODES:
                    usage = parsed_data.get("usage", {})
                    total_tokens = usage.get("total_tokens", 0) if isinstance(usage, dict) else 0
                    if total_tokens > 0:
                        status = "SUCCESS"
                        detail = f"LLM 调用成功（{total_tokens} tokens，尾节点为 processor，content 为空属正常行为）"
                    else:
                        status = "FAIL"
                        detail = "响应内容为空且无 token 消耗，LLM 未被调用"
                else:
                    status = "FAIL"
                    detail = "响应内容为空"
        else:
            status = "FAIL"
            detail = parsed_data.get("error", {}).get("message", "Unknown error") if isinstance(parsed_data.get("error"), dict) else parsed_data.get("error", "Unknown error")
    except json.JSONDecodeError:
        status = "FAIL"
        detail = "响应解析失败或超时"
    except Exception as e:
        status = "FAIL"
        detail = str(e)
    
    # 显示结果
    if status == "SUCCESS":
        print(f"  ✅ 成功 ({duration}ms)")
        print(f"  内容: {detail[:100]}...")
    elif status == "WARN":
        print(f"  ⚠️  警告 ({duration}ms)")
        print(f"  问题: {detail}")
    else:
        print(f"  ❌ 失败 ({duration}ms)")
        print(f"  错误: {detail}")
    
    # 构建 curl 命令用于报告 (根据使用的方式)
    curl_cmd = f"curl -X POST '{config['base_url']}/v1/chat/completions?token={config['token']}'"
    
    if method == 'header':
        # 方式1: 显示 X-Proxy-Mode 头
        curl_cmd += f" \\\n  -H 'Content-Type: application/json' \\\n  -H 'X-Proxy-Mode: {code}'"
    elif method == 'model':
        # 方式2: 只显示 Content-Type
        curl_cmd += f" \\\n  -H 'Content-Type: application/json'"
    elif method == 'content':
        # 方式3: 只显示 Content-Type
        curl_cmd += f" \\\n  -H 'Content-Type: application/json'"
    
    if headers:
        for key, value in headers.items():
            curl_cmd += f" \\\n  -H '{key}: {value}'"
    
    curl_cmd += f" \\\n  -d '{request_json}'"
    
    return {
        "code": code,
        "name": name,
        "status": status,
        "duration": duration,
        "content": detail[:200],
        "error": detail if status == "FAIL" else "",
        "curl_cmd": curl_cmd,
        "request_data": request_data,
        "response_data": parsed_data if parsed_data else {"raw": raw_response[:500]},
        "model": model,
    }

def generate_html_report(results, config):
    """生成 HTML 报告"""
    success_count = sum(1 for r in results if r["status"] == "SUCCESS")
    warn_count = sum(1 for r in results if r["status"] == "WARN")
    fail_count = sum(1 for r in results if r["status"] == "FAIL")
    total_count = len(results)
    
    report_file = f"{REPORT_DIR}/pipeline-test-{datetime.now().strftime('%Y%m%d-%H%M%S')}.html"
    
    # 生成表格行
    table_rows = ""
    for idx, r in enumerate(results):
        status_class = "badge-success" if r["status"] == "SUCCESS" else ("badge-warn" if r["status"] == "WARN" else "badge-fail")
        status_text = "✅ 成功" if r["status"] == "SUCCESS" else ("⚠️ 警告" if r["status"] == "WARN" else "❌ 失败")
        note = r["content"] if r["status"] == "SUCCESS" else (r["error"] if r["error"] else r["content"])
        
        curl_cmd_escaped = r.get("curl_cmd", "").replace("'", "\\'").replace("\n", "\\n")
        
        table_rows += f"""
                <tr>
                    <td><strong>{r["name"]}</strong></td>
                    <td><code>{r["code"]}</code></td>
                    <td><span class="badge {status_class}">{status_text}</span></td>
                    <td>{r["duration"]}ms</td>
                    <td class="note-cell">{note[:150]}</td>
                    <td class="detail-cell">
                        <button class="detail-btn" onclick="showDetailModal({idx})">📄 查看详情</button>
                    </td>
                </tr>"""
    
    # 生成问题分析
    issues = ""
    
    p0_issues = [r for r in results if r["status"] == "FAIL"]
    if p0_issues:
        issues += "<h3>🔴 P0 问题（必须修复）</h3>"
        for r in p0_issues:
            cause = ""
            if r["code"] == "#f":
                cause = "降级组配置不完整或后端调用失败，可能缺少 fallback 节点"
            elif r["code"] == "#a":
                cause = "审核节点未正确调用 LLM 或审核结果未正确传递到响应"
            elif r["code"] == "#ag":
                cause = "聚合节点需要多个上游输出但只有一个输入，或聚合逻辑未处理单输入场景"
            elif r["code"] == "#l":
                cause = "翻译节点未正确配置目标语言或翻译插件未正确注册"
            else:
                cause = r["error"]
            
            issues += f"""
            <div class="issue issue-p0">
                <div class="issue-title">{r["code"]} {r["name"]} - 测试失败</div>
                <div class="issue-desc">
                    <strong>错误信息:</strong> {r["error"][:200] if r["error"] else r["content"]}<br>
                    <strong>耗时:</strong> {r["duration"]}ms<br>
                    <strong>可能原因:</strong> {cause}
                </div>
            </div>"""
    
    p1_issues = [r for r in results if r["status"] == "WARN"]
    if p1_issues:
        issues += "<h3>🟡 P1 问题（建议修复）</h3>"
        for r in p1_issues:
            issues += f"""
            <div class="issue issue-p1">
                <div class="issue-title">{r["code"]} {r["name"]} - 未调用 LLM</div>
                <div class="issue-desc">
                    <strong>现象:</strong> 返回原始输入内容<br>
                    <strong>耗时:</strong> {r["duration"]}ms（极短，说明未调用 LLM）<br>
                    <strong>可能原因:</strong> 缓存命中（exact match）或 fallback 路径直接返回输入
                </div>
            </div>"""
    
    # 生成详情数据 (JSON)
    details_json = json.dumps(results, ensure_ascii=False, indent=2)
    
    html = f"""<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Centag 流水线测试报告</title>
    <style>
        * {{ margin: 0; padding: 0; box-sizing: border-box; }}
        body {{ font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background: #f0f2f5; padding: 20px; min-height: 100vh; color: #333; }}
        .container {{ max-width: 1600px; margin: 0 auto; background: white; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.08); padding: 32px; }}
        h1 {{ color: #1a1a1a; margin-bottom: 8px; font-size: 1.75em; font-weight: 600; }}
        h2 {{ color: #1a1a1a; margin: 24px 0 16px 0; font-size: 1.25em; font-weight: 600; border-bottom: 1px solid #e8e8e8; padding-bottom: 8px; }}
        h3 {{ color: #595959; margin: 16px 0 12px 0; font-size: 1.1em; font-weight: 500; }}
        .meta {{ color: #8c8c8c; margin-bottom: 24px; font-size: 0.9em; }}
        .summary {{ display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 32px; }}
        .stat {{ background: #fafafa; padding: 20px; border-radius: 6px; text-align: center; border: 1px solid #e8e8e8; }}
        .stat-number {{ font-size: 2em; font-weight: 600; margin-bottom: 4px; }}
        .stat-label {{ color: #8c8c8c; font-size: 0.85em; }}
        .success {{ color: #52c41a; }}
        .warn {{ color: #faad14; }}
        .fail {{ color: #ff4d4f; }}
        table {{ width: 100%; border-collapse: separate; border-spacing: 0; margin-bottom: 24px; border: 1px solid #e8e8e8; border-radius: 6px; overflow: hidden; font-size: 0.9em; }}
        th, td {{ padding: 12px 16px; text-align: left; border-bottom: 1px solid #f0f0f0; }}
        th {{ background: #fafafa; color: #595959; font-weight: 500; font-size: 0.85em; text-transform: uppercase; letter-spacing: 0.5px; }}
        tr:last-child td {{ border-bottom: none; }}
        tr:hover {{ background: #fafafa; }}
        .badge {{ display: inline-block; padding: 4px 10px; border-radius: 12px; font-size: 0.85em; font-weight: 500; }}
        .badge-success {{ background: #f6ffed; color: #389e0d; border: 1px solid #b7eb8f; }}
        .badge-warn {{ background: #fffbe6; color: #d46b08; border: 1px solid #ffe58f; }}
        .badge-fail {{ background: #fff2f0; color: #cf1322; border: 1px solid #ffccc7; }}
        code {{ background: #f5f5f5; padding: 2px 6px; border-radius: 4px; font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Roboto Mono', Consolas, 'Courier New', monospace; font-size: 0.9em; color: #d63384; }}
        .note-cell {{ max-width: 250px; word-wrap: break-word; color: #595959; }}
        .detail-cell {{ text-align: center; }}
        .detail-btn {{ background: #1890ff; color: white; border: none; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 0.85em; transition: all 0.2s; }}
        .detail-btn:hover {{ background: #40a9ff; transform: scale(1.05); }}
        .detail-btn:active {{ transform: scale(0.95); }}
        .issue {{ background: #fafafa; border-left: 3px solid #1890ff; padding: 16px; margin-bottom: 16px; border-radius: 4px; }}
        .issue-p0 {{ background: #fff2f0; border-left-color: #ff4d4f; }}
        .issue-p1 {{ background: #fffbe6; border-left-color: #faad14; }}
        .issue-title {{ font-weight: 500; margin-bottom: 8px; font-size: 1em; color: #1a1a1a; }}
        .issue-desc {{ color: #595959; line-height: 1.6; font-size: 0.9em; }}
        .issue-desc code {{ background: #f0f0f0; }}
        .footer {{ margin-top: 32px; padding-top: 16px; border-top: 1px solid #e8e8e8; color: #8c8c8c; text-align: center; font-size: 0.85em; }}
        
        /* 模态框样式 */
        .modal {{ display: none; position: fixed; z-index: 1000; left: 0; top: 0; width: 100%; height: 100%; background-color: rgba(0,0,0,0.5); }}
        .modal-content {{ background-color: #fefefe; margin: 5% auto; padding: 0; border-radius: 8px; width: 90%; max-width: 1200px; max-height: 80vh; overflow: hidden; box-shadow: 0 4px 16px rgba(0,0,0,0.2); }}
        .modal-header {{ background: #fafafa; padding: 16px 24px; border-bottom: 1px solid #e8e8e8; display: flex; justify-content: space-between; align-items: center; }}
        .modal-header h2 {{ margin: 0; border: none; padding: 0; font-size: 1.25em; }}
        .close {{ color: #8c8c8c; font-size: 28px; font-weight: bold; cursor: pointer; transition: color 0.2s; }}
        .close:hover {{ color: #1a1a1a; }}
        .modal-body {{ padding: 24px; overflow-y: auto; max-height: calc(80vh - 80px); }}
        .detail-section {{ margin-bottom: 24px; }}
        .detail-section h3 {{ margin-bottom: 12px; color: #1a1a1a; }}
        .detail-box {{ background: #f5f5f5; padding: 16px; border-radius: 6px; border: 1px solid #e8e8e8; overflow-x: auto; position: relative; }}
        .detail-box pre {{ margin: 0; font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace; font-size: 0.85em; line-height: 1.6; white-space: pre-wrap; word-wrap: break-word; padding-right: 70px; }}
        .copy-btn {{ position: absolute; top: 8px; right: 8px; background: #1890ff; color: white; border: none; padding: 6px 14px; border-radius: 4px; cursor: pointer; font-size: 0.85em; transition: all 0.2s; display: flex; align-items: center; gap: 4px; }}
        .copy-btn:hover {{ background: #40a9ff; transform: scale(1.05); }}
        .copy-btn:active {{ transform: scale(0.95); }}
        .copy-btn.copied {{ background: #52c41a; }}
    </style>
</head>
<body>
    <div class="container">
        <h1>📊 Centag 流水线模式测试报告</h1>
        <div class="meta">
            🕐 测试时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')} | 
            🖥️ 服务器: {config['base_url']} | 
            📝 输入: {config['input']}
        </div>
        
        <div class="summary">
            <div class="stat">
                <div class="stat-number success">{success_count}</div>
                <div class="stat-label">✅ 成功</div>
            </div>
            <div class="stat">
                <div class="stat-number warn">{warn_count}</div>
                <div class="stat-label">⚠️ 警告</div>
            </div>
            <div class="stat">
                <div class="stat-number fail">{fail_count}</div>
                <div class="stat-label">❌ 失败</div>
            </div>
            <div class="stat">
                <div class="stat-number" style="color: #667eea;">{total_count}</div>
                <div class="stat-label">📈 总计</div>
            </div>
        </div>
        
        <h2>📋 测试结果详情</h2>
        <table>
            <thead>
                <tr>
                    <th>模式</th>
                    <th>快捷码</th>
                    <th>状态</th>
                    <th>耗时</th>
                    <th>备注/错误</th>
                    <th>详情</th>
                </tr>
            </thead>
            <tbody>
                {table_rows}
            </tbody>
        </table>
        
        <h2>🔍 问题分析</h2>
        {issues}
        
        <div class="footer">
            <p>🤖 报告由自动化测试脚本生成 | Centag Pipeline Test Suite v2.0</p>
        </div>
    </div>
    
    <!-- 详情模态框 -->
    <div id="detailModal" class="modal">
        <div class="modal-content">
            <div class="modal-header">
                <h2 id="modalTitle">测试详情</h2>
                <span class="close" onclick="closeModal()">&times;</span>
            </div>
            <div class="modal-body" id="modalBody">
            </div>
        </div>
    </div>
    
    <script>
        const testData = {details_json};
        
        function showDetailModal(index) {{
            const data = testData[index];
            const modal = document.getElementById('detailModal');
            const title = document.getElementById('modalTitle');
            const body = document.getElementById('modalBody');
            
            title.textContent = `${{data.name}} (${{data.code}}) - 测试详情`;
            
            body.innerHTML = `
                <div class="detail-section">
                    <h3>📊 基本信息</h3>
                    <div class="detail-box">
                        <pre>模式: ${{data.name}} (${{data.code}})
状态: ${{data.status}}
耗时: ${{data.duration}}ms
模型: ${{data.model}}</pre>
                    </div>
                </div>
                
                <div class="detail-section">
                    <h3>📤 请求数据</h3>
                    <div class="detail-box">
                        <button class="copy-btn" onclick="copyContent(this, 'request-${{index}}')">📋 复制</button>
                        <pre id="request-${{index}}">${{JSON.stringify(data.request_data, null, 2)}}</pre>
                    </div>
                </div>
                
                <div class="detail-section">
                    <h3>📥 响应数据</h3>
                    <div class="detail-box">
                        <button class="copy-btn" onclick="copyContent(this, 'response-${{index}}')">📋 复制</button>
                        <pre id="response-${{index}}">${{JSON.stringify(data.response_data, null, 2)}}</pre>
                    </div>
                </div>
                
                <div class="detail-section">
                    <h3>💻 测试命令</h3>
                    <div class="detail-box">
                        <button class="copy-btn" onclick="copyContent(this, 'curl-${{index}}')">📋 复制</button>
                        <pre id="curl-${{index}}">${{data.curl_cmd}}</pre>
                    </div>
                </div>
            `;
            
            modal.style.display = 'block';
        }}
        
        function closeModal() {{
            document.getElementById('detailModal').style.display = 'none';
        }}
        
        function copyContent(button, elementId) {{
            const element = document.getElementById(elementId);
            const text = element.textContent;
            
            navigator.clipboard.writeText(text).then(() => {{
                const originalHTML = button.innerHTML;
                button.innerHTML = '✅ 已复制';
                button.classList.add('copied');
                
                setTimeout(() => {{
                    button.innerHTML = originalHTML;
                    button.classList.remove('copied');
                }}, 2000);
            }}).catch(err => {{
                console.error('复制失败:', err);
                button.innerHTML = '❌ 失败';
                setTimeout(() => {{
                    button.innerHTML = '📋 复制';
                }}, 2000);
            }});
        }}
        
        window.onclick = function(event) {{
            const modal = document.getElementById('detailModal');
            if (event.target == modal) {{
                modal.style.display = 'none';
            }}
        }}
        
        document.addEventListener('keydown', function(event) {{
            if (event.key === 'Escape') {{
                closeModal();
            }}
        }});
    </script>
</body>
</html>"""
    
    os.makedirs(REPORT_DIR, exist_ok=True)
    with open(report_file, 'w', encoding='utf-8') as f:
        f.write(html)
    
    return report_file

def main():
    """主流程"""
    # 解析命令行参数
    parser = argparse.ArgumentParser(description='Centag 流水线模式测试')
    parser.add_argument('--auto', action='store_true', help='使用自动模式(默认配置)')
    parser.add_argument('--modes', nargs='+', help='指定要测试的模式,如: --modes #d #s #f')
    parser.add_argument('--method', choices=['header', 'model', 'content'], default='model',
                       help='指定流水线的方式: header(X-Proxy-Mode头), model(模型字段), content(内容前缀)。默认: model')
    parser.add_argument('--sync-db', action='store_true', help='从数据库同步当前的快捷码配置')
    args = parser.parse_args()
    
    # 检查是否使用自动模式
    auto_mode = args.auto
    
    if auto_mode:
        # 自动模式: 使用默认配置
        print("🚀 使用自动模式(默认配置)")
        config = DEFAULTS.copy()
    else:
        # 向导模式: 交互式配置
        config = wizard_config()
    
    log_file = f"/tmp/centag-test-{datetime.now().strftime('%Y%m%d-%H%M%S')}.log"
    
    # 根据是否同步数据库决定使用哪种配置
    if args.sync_db:
        print("🔄 从数据库同步流水线配置...")
        VIRTUAL_MODELS = build_dynamic_virtual_models(config['base_url'], config['token'])
        all_modes = get_test_modes_from_db(config['base_url'], config['token'])
        
        # 如果从数据库获取不到模式，则使用默认模式
        if not all_modes:
            print("⚠️ 从数据库获取模式失败，使用默认模式")
            all_modes = [
                {"code": "#d", "name": "直连后端", "extra_headers": {}},
                {"code": "#s", "name": "系统调度", "extra_headers": {}},
                {"code": "#f", "name": "降级模式", "extra_headers": {}},
                {"code": "#o", "name": "优化模式", "extra_headers": {}},
                {"code": "#a", "name": "审核模式", "extra_headers": {}},
                {"code": "#m", "name": "模型匹配", "extra_headers": {}},
                {"code": "#r", "name": "路由模式", "extra_headers": {}},
                {"code": "#ag", "name": "聚合模式", "extra_headers": {}},
                {"code": "#l", "name": "翻译模式", "extra_headers": {}},
                {"code": "#ch", "name": "缓存命中", "extra_headers": {}},
                {"code": "#cm", "name": "缓存模式", "extra_headers": {}},
            ]
    else:
        VIRTUAL_MODELS = DEFAULT_VIRTUAL_MODELS
        all_modes = [
            {"code": "#d", "name": "直连后端", "extra_headers": {}},
            {"code": "#s", "name": "系统调度", "extra_headers": {}},
            {"code": "#f", "name": "降级模式", "extra_headers": {}},
            {"code": "#o", "name": "优化模式", "extra_headers": {}},
            {"code": "#a", "name": "审核模式", "extra_headers": {}},
            {"code": "#m", "name": "模型匹配", "extra_headers": {}},
            {"code": "#r", "name": "路由模式", "extra_headers": {}},
            {"code": "#ag", "name": "聚合模式", "extra_headers": {}},
            {"code": "#l", "name": "翻译模式", "extra_headers": {}},
            {"code": "#ch", "name": "缓存命中", "extra_headers": {}},
            {"code": "#cm", "name": "缓存模式", "extra_headers": {}},
        ]
    
    # 根据参数筛选模式
    if args.modes:
        # 过滤出指定的模式
        modes = [m for m in all_modes if m["code"] in args.modes]
        if not modes:
            print(f"❌ 未找到指定的模式: {args.modes}")
            print(f"可用模式: {', '.join([m['code'] for m in all_modes])}")
            sys.exit(1)
        print(f"🎯 测试指定模式: {', '.join([m['code'] for m in modes])}")
    else:
        # 默认测试所有模式
        modes = all_modes
        print("🎯 测试所有模式")
    
    print("\n\n🚀 开始执行测试...")
    print("=" * 60)
    
    # 检查服务
    if not check_service(config["base_url"]):
        print(f"\n❌ 服务未运行: {config['base_url']}")
        print("请先启动服务后再执行测试")
        sys.exit(1)
    else:
        print(f"✅ 服务已运行: {config['base_url']}")
    
    # 执行测试
    print(f"\n📋 使用方式: {args.method}")
    results = []
    for mode_info in modes:
        result = test_mode(mode_info, config, log_file, method=args.method, virtual_models=VIRTUAL_MODELS)
        results.append(result)
        time.sleep(1)
    
    # 统计
    success_count = sum(1 for r in results if r["status"] == "SUCCESS")
    warn_count = sum(1 for r in results if r["status"] == "WARN")
    fail_count = sum(1 for r in results if r["status"] == "FAIL")
    
    print("\n" + "=" * 60)
    print("✅ 测试完成")
    print("=" * 60)
    print(f"  ✅ 成功: {success_count}")
    print(f"  ⚠️  警告: {warn_count}")
    print(f"  ❌ 失败: {fail_count}")
    print("=" * 60)
    
    # 生成报告
    report_path = generate_html_report(results, config)
    print(f"\n📄 HTML 报告已生成: {report_path}")
    print(f"📝 日志文件: {log_file}")
    
    # 自动打开浏览器 (跨平台支持)
    print(f"\n🌐 正在打开测试报告...")
    try:
        import platform
        system = platform.system()
        if system == "Darwin":  # macOS
            subprocess.run(["open", report_path])
        elif system == "Windows":
            subprocess.run(["start", report_path], shell=True)
        else:  # Linux/Unix
            subprocess.run(["xdg-open", report_path])
    except Exception as e:
        print(f"⚠️  无法自动打开报告: {e}")
        print(f"   请手动打开: {report_path}")

if __name__ == "__main__":
    main()
