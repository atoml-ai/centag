#!/usr/bin/env python3
"""
Centag Wizard Test — HTML 报告生成器

用法: python3 wizard-report.py [--output /path/to/report.html]
依赖:
- /tmp/wizard_test_data.json（主流程结果）
- /tmp/wizard_variant_matrix.json（入口变体矩阵，可选）
"""

import os, sys, json
from datetime import datetime, timezone

# --- 工具函数 ---
def read_json(p): return json.load(open(p)) if os.path.exists(p) else None
def env(k, d=""): return os.environ.get(k, d)
def mask_key(k):
    if not k: return "未设置"
    return k[:4] + "****" + k[-4:] if len(k) > 12 else k[:4] + "****"
def esc(s): return str(s).replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
def clip(s, max_len=12000):
    t = str(s or "")
    if len(t) <= max_len:
        return t
    return t[:max_len] + f"\n...(truncated, total={len(t)})"
def to_float(v, d=0.0):
    try: return float(v)
    except Exception: return d
def fallback_variant_curl(m, base_url):
    req_model = m.get("req_model") or env("TEST_BACKEND_MODEL", "gpt-4o-mini")
    body = json.dumps({
        "model": req_model,
        "messages": [{"role": "user", "content": f"{m.get('req_prompt_prefix','')}请严格回复：{m.get('req_tag','')}"}],
        "temperature": 0,
        "max_tokens": 64
    }, ensure_ascii=False)
    mode = (m.get("req_mode_header") or "").strip()
    if mode:
        return f'curl -s --max-time 60 -X POST "{base_url}/v1/chat/completions" -H "Authorization: Bearer $TEST_AUTH_KEY" -H "X-Proxy-Mode: {mode}" -H "Content-Type: application/json" -d \'{body}\''
    return f'curl -s --max-time 60 -X POST "{base_url}/v1/chat/completions" -H "Authorization: Bearer $TEST_AUTH_KEY" -H "Content-Type: application/json" -d \'{body}\''

# --- 读取数据 ---
test_data = read_json("/tmp/wizard_test_data.json") or []
pipeline_updates = read_json("/tmp/wizard_pipeline_update_cmds.json") or []
probe = read_json("/tmp/wizard_probe.json") or {}
matrix_data = read_json("/tmp/wizard_variant_matrix.json") or []
runtime_ctx = read_json("/tmp/wizard_runtime_context.json") or {}
admin_data = read_json("/tmp/admin_e2e_results.json") or {}

# --- 汇总统计 ---
TOTAL = len(test_data)
PASSED = sum(1 for t in test_data if t.get("passed"))
FAILED = TOTAL - PASSED
PASS_RATE = (PASSED * 100 // TOTAL) if TOTAL > 0 else 0
MATRIX_TOTAL = len(matrix_data)
MATRIX_SKIPPED = sum(1 for m in matrix_data if m.get("skipped"))
MATRIX_EFFECTIVE_TOTAL = sum(1 for m in matrix_data if not m.get("skipped"))
MATRIX_PASSED = sum(1 for m in matrix_data if m.get("passed") and not m.get("skipped"))
MATRIX_FAILED = MATRIX_EFFECTIVE_TOTAL - MATRIX_PASSED
MATRIX_PASS_RATE = (MATRIX_PASSED * 100 // MATRIX_EFFECTIVE_TOTAL) if MATRIX_EFFECTIVE_TOTAL > 0 else 0
ADMIN_RESULTS = admin_data.get("results", []) if isinstance(admin_data, dict) else []
ADMIN_TOTAL = len(ADMIN_RESULTS)
ADMIN_PASSED = sum(1 for r in ADMIN_RESULTS if r.get("ok"))
ADMIN_FAILED = ADMIN_TOTAL - ADMIN_PASSED
ADMIN_PASS_RATE = (ADMIN_PASSED * 100 // ADMIN_TOTAL) if ADMIN_TOTAL > 0 else 0
OVERALL_TOTAL = TOTAL + ADMIN_TOTAL
OVERALL_PASSED = PASSED + ADMIN_PASSED
OVERALL_FAILED = OVERALL_TOTAL - OVERALL_PASSED
OVERALL_PASS_RATE = (OVERALL_PASSED * 100 // OVERALL_TOTAL) if OVERALL_TOTAL > 0 else 0

variant_stats = {}
matrix_by_pipeline = {}
if MATRIX_TOTAL > 0:
    for row in matrix_data:
        if row.get("skipped"):
            continue
        p = row.get("pipeline", "unknown")
        matrix_by_pipeline.setdefault(p, []).append(row)
        v = row.get("variant", "unknown")
        s = variant_stats.setdefault(v, {
            "total": 0, "passed": 0, "elapsed_sum": 0.0, "resolved_hits_ok": 0, "finished_match_ok": 0
        })
        s["total"] += 1
        if row.get("passed"):
            s["passed"] += 1
        s["elapsed_sum"] += to_float(row.get("elapsed_s", 0), 0.0)
        if (row.get("log_resolved_hits", 0) or 0) > 0:
            s["resolved_hits_ok"] += 1
        if (row.get("log_finished_pipeline_match", 0) or 0) > 0:
            s["finished_match_ok"] += 1
for p in matrix_by_pipeline:
    matrix_by_pipeline[p] = sorted(matrix_by_pipeline[p], key=lambda x: (str(x.get("variant", "")), int(x.get("try", 0) or 0)))
pipeline_update_by_pipeline = {}
for pu in pipeline_updates:
    pipeline_update_by_pipeline[pu.get("pipeline", "")] = pu

# --- 环境变量 ---
BASE_URL = env("TEST_BASE_URL", "http://localhost:20060")
DEPLOY = env("CENTAG_DEPLOY_TYPE", "personal")
BID = env("TEST_BACKEND_ID", "?")
BMODEL = env("TEST_BACKEND_MODEL", "?")
BTYPE = env("TEST_BACKEND_TYPE", "?")
BURL = env("TEST_BACKEND_BASE_URL", "?")
BKEY = mask_key(env("TEST_BACKEND_KEY", ""))
PIPELINES = env("TEST_PIPELINES", "")
ENTRY_VARIANTS = env("TEST_ENTRY_VARIANTS", "header-full")
REPEAT_PER_VARIANT = env("TEST_REPEAT_PER_VARIANT", "1")
LOG_EVIDENCE_LEVEL = env("TEST_LOG_EVIDENCE_LEVEL", "basic")
CRED = "用户 API Key" if DEPLOY == "team" else "Admin JWT Token"
NOW = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
TS  = datetime.now().strftime("%Y%m%d_%H%M%S")
TEMP_BACKEND_USED = bool(runtime_ctx.get("temp_backend_used", False))
TEMP_BACKEND_SOURCE_ID = runtime_ctx.get("temp_backend_source_id", "")
TEMP_BACKEND_FINAL_ID = runtime_ctx.get("temp_backend_final_id", "")
TEMP_BACKEND_REASON = runtime_ctx.get("temp_backend_reason", "")
TEMP_BACKEND_SIGNAL = runtime_ctx.get("temp_backend_signal", "")
HEADER_OVERRIDE_SUPPORTED = runtime_ctx.get("header_override_supported", "")

# --- 探测状态 ---
ps = str(probe.get("success", "false")).lower()
pst = probe.get("status", "unknown")
perr = probe.get("error", "无")
prt = probe.get("response_time", 0)
pmod = probe.get("models_count", 0)
bhealth = "✅ 通过" if (ps == "true" or pst in ("healthy", "available")) else ("⚠️ 异常: " + perr)
bhcls = "pass" if (ps == "true" or pst in ("healthy", "available")) else "warn"

# --- 读取 CSS ---
script_dir = os.path.dirname(os.path.abspath(__file__))
css_path = os.path.join(script_dir, "wizard-report.css")
CSS = open(css_path).read() if os.path.exists(css_path) else ""

# --- 输出路径 ---
out = f"/tmp/wizard_test_report_{TS}.html"
for i, a in enumerate(sys.argv[1:]):
    if a in ("--output", "-o") and i+2 < len(sys.argv):
        out = sys.argv[i+2]; break
    elif a.startswith("--output="):
        out = a.split("=", 1)[1]; break

# ========================================================================
# 生成报告
# ========================================================================
html = [f"""<!DOCTYPE html><html lang="zh-CN">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Centag 向导测试报告</title><style>{CSS}</style></head>
<body><div class="container">
<h1>🧪 Centag 向导测试报告</h1>
<p class="subtitle">自动化验证流水线与管理功能（按本次测试类型自动展示）</p>

<style>
  .admin-case-row {{ cursor: pointer; }}
  .admin-case-row:hover {{ background: rgba(59, 130, 246, 0.08); }}
  .admin-case-row:focus {{ outline: 2px solid #3b82f6; outline-offset: -2px; }}
  .admin-case-detail-row td {{ background: rgba(15, 23, 42, 0.25); }}
  .pipeline-case-row {{ cursor: pointer; }}
  .pipeline-case-row:hover {{ background: rgba(59, 130, 246, 0.08); }}
  .pipeline-case-row:focus {{ outline: 2px solid #3b82f6; outline-offset: -2px; }}
  .pipeline-case-detail-row td {{ background: rgba(15, 23, 42, 0.25); }}
  .overview-two-col {{
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }}
  @media (max-width: 980px) {{
    .overview-two-col {{
      grid-template-columns: 1fr;
    }}
  }}
</style>

<h2>📋 测试概览与导航</h2>
<div class="overview-two-col">
<div class="info-grid">
  <div class="key">测试时间</div><div class="val">{NOW}</div>
  <div class="key">部署版本</div><div class="val">{DEPLOY} 版</div>
  <div class="key">服务地址</div><div class="val">{esc(BASE_URL)}</div>
  <div class="key">测试凭据</div><div class="val">{CRED}</div>
  <div class="key">后端</div><div class="val">{esc(BID)} / {esc(BMODEL)} / {esc(BTYPE)}</div>
  <div class="key">后端 Base URL</div><div class="val">{esc(BURL)}</div>
  <div class="key">测试流水线</div><div class="val">{esc(PIPELINES)}</div>
  <div class="key">入口变体</div><div class="val">{esc(ENTRY_VARIANTS)} × {esc(REPEAT_PER_VARIANT)}（{esc(LOG_EVIDENCE_LEVEL)}）</div>
  <div class="key">Header 覆盖能力</div><div class="val">{esc(HEADER_OVERRIDE_SUPPORTED or 'unknown')}</div>
  <div class="key">临时后端兜底</div><div class="val">{'启用并切换' if TEMP_BACKEND_USED else '未触发'}</div>
</div>
<div class="info-grid">
  <div class="key">总结果</div><div class="val">✅ {OVERALL_PASSED} / ❌ {OVERALL_FAILED} / 共 {OVERALL_TOTAL}（{OVERALL_PASS_RATE}%）</div>
  <div class="key">流水线结果</div><div class="val">✅ {PASSED} / ❌ {FAILED} / 共 {TOTAL}（{PASS_RATE}%）</div>
  <div class="key">管理结果</div><div class="val">✅ {ADMIN_PASSED} / ❌ {ADMIN_FAILED} / 共 {ADMIN_TOTAL}（{ADMIN_PASS_RATE}%）</div>
  <div class="key">矩阵结果</div><div class="val">✅ {MATRIX_PASSED} / ❌ {MATRIX_FAILED} / 有效 {MATRIX_EFFECTIVE_TOTAL}（{MATRIX_PASS_RATE}%）</div>
  <div class="key">流水线测试专区</div><div class="val"><a href="#pipeline-suite">跳转</a></div>
  <div class="key">管理功能测试专区</div><div class="val"><a href="#admin-suite">跳转</a></div>
  <div class="key">复测指南</div><div class="val"><a href="#rerun-guide">跳转</a></div>
  <div class="key">故障分析</div><div class="val"><a href="#troubleshooting">跳转</a></div>
</div>
</div>
"""]

if TEMP_BACKEND_USED:
    html.append(f"""
<div class="analysis-box warn">
  已触发临时后端兜底：
  source=<code>{esc(TEMP_BACKEND_SOURCE_ID)}</code> →
  final=<code>{esc(TEMP_BACKEND_FINAL_ID)}</code>，
  reason=<code>{esc(TEMP_BACKEND_REASON)}</code>，
  signal=<code>{esc(TEMP_BACKEND_SIGNAL or '-')}</code>。
</div>
""")

html.append('<h2 id="pipeline-suite">🚦 流水线测试专区</h2>')

if TOTAL > 0:
    html.append(f"""

<h2>🔧 后端配置详情</h2>
<h3>启用后端</h3>
<div class="code-block">curl -s --max-time 10 -X PUT "{esc(BASE_URL)}/api/v1/backends/{esc(BID)}" \\
  -H "Authorization: Bearer $JWT" \\
  -H "Content-Type: application/json" \\
  -d '{{"id":"{esc(BID)}","enabled":true,"api_key":"****","type":"{esc(BTYPE)}","base_url":"{esc(BURL)}","auto_fetch_models":true}}'</div>

<h3>后端连通性验证</h3>
<div class="code-block">curl -s --max-time 15 -X POST "{esc(BASE_URL)}/api/v1/backends/{esc(BID)}/probe" \\
  -H "Authorization: Bearer $JWT" \\
  -H "Content-Type: application/json"</div>
<table>
  <tr><th>探测项目</th><th>结果</th></tr>
  <tr><td>连通状态</td><td><span class="badge badge-{bhcls}">{bhealth}</span></td></tr>
  <tr><td>响应时间</td><td>{prt} ms</td></tr>
  <tr><td>可用模型数</td><td>{pmod}</td></tr>
  <tr><td>错误信息</td><td>{esc(perr)}</td></tr>
</table>

""")
else:
    html.append('<div class="analysis-box">本次未执行流水线测试（admin-only 模式）。</div>')

html.append('<h2>🧪 流水线测试详情</h2>')
if TOTAL == 0:
    html.append('<div class="analysis-box">本次未执行流水线测试，无明细可展示。</div>')
else:
    html.append(f'<div class="analysis-box">节点配置检查已并入流水线详情，每条用例都包含节点更新状态（目标 backend=<code>{esc(BID)}</code>, model=<code>{esc(BMODEL)}</code>）。</div>')
    html.append("""
<table class="pipeline-case-table">
  <tr><th>流水线</th><th>状态</th><th>HTTP</th><th>模式</th><th>耗时(s)</th><th>Tokens</th><th>模型</th><th>节点配置</th></tr>
""")
    for idx, t in enumerate(test_data):
        pid = t.get("pipeline", "?")
        mode = t.get("mode", pid)
        passed = t.get("passed", False)
        badge_html = '<span class="badge badge-pass">✅ 通过</span>' if passed else '<span class="badge badge-fail">❌ 失败</span>'
        acls = "analysis-box" if passed else "analysis-box error"
        entries = matrix_by_pipeline.get(pid, [])
        pu = pipeline_update_by_pipeline.get(pid, {})
        node_update_success = bool(pu.get("success"))
        node_update_badge = '<span class="badge badge-pass">✅ 成功</span>' if node_update_success else '<span class="badge badge-fail">❌ 失败</span>'
        node_update_http = pu.get("http_code", "?")
        node_update_note = pu.get("note", "无")
        node_update_cmd = pu.get("put_cmd", "")

        variant_parts = []
        if not entries:
            variant_parts.append('<div class="analysis-box warn">该流水线本次未启用入口变体矩阵，或未采集到矩阵数据。</div>')
        else:
            for m in entries:
                skipped = bool(m.get("skipped"))
                ok = bool(m.get("passed")) and not skipped
                if skipped:
                    badge = '<span class="badge badge-warn">跳过</span>'
                else:
                    badge = '<span class="badge badge-pass">通过</span>' if ok else '<span class="badge badge-fail">失败</span>'
                variant = m.get("variant", "?")
                vtry = m.get("try", "?")
                curl_cmd = m.get("curl_cmd") or fallback_variant_curl(m, BASE_URL)
                variant_parts.append(f"""
<details class="entry-detail">
  <summary>{badge} <code>{esc(variant)}</code> · 第 {esc(vtry)} 轮 · HTTP {esc(m.get('http_code','?'))} · X-Pipeline-Id={esc(m.get('resp_x_pipeline_id',''))}</summary>
  <div class="entry-body">
    <div class="mini-grid">
      <div class="key">请求标识</div><div class="val"><code>{esc(m.get('req_tag',''))}</code></div>
      <div class="key">耗时</div><div class="val">{esc(m.get('elapsed_s', 0))} s</div>
      <div class="key">X-Proxy-Mode</div><div class="val">{esc(m.get('resp_x_proxy_mode',''))}</div>
      <div class="key">X-Pipeline-Id</div><div class="val">{esc(m.get('resp_x_pipeline_id',''))}</div>
      <div class="key">X-Backend-Id</div><div class="val">{esc(m.get('resp_x_backend_id',''))}</div>
      <div class="key">响应 Tokens</div><div class="val">{esc(m.get('tokens', 0))}</div>
      <div class="key">日志时间窗</div><div class="val"><code>{esc(m.get('window_from',''))}</code> ~ <code>{esc(m.get('window_to',''))}</code></div>
      <div class="key">日志命中</div><div class="val">resolved={esc(m.get('log_resolved_hits',0))}, finished总数={esc(m.get('log_finished_hits',0))}, finished匹配={esc(m.get('log_finished_pipeline_match',0))}</div>
      <div class="key">证据强度</div><div class="val"><code>{esc(m.get('log_evidence_strength','basic'))}</code></div>
      <div class="key">主 request_id</div><div class="val"><code>{esc(m.get('log_finished_primary_request_id','') or '-')}</code></div>
      <div class="key">日志 request_id</div><div class="val">resolved: <code>{esc(m.get('log_resolved_request_ids','') or '-')}</code><br/>finished: <code>{esc(m.get('log_finished_request_ids','') or '-')}</code></div>
      <div class="key">判定备注</div><div class="val">{esc(m.get('note',''))}</div>
    </div>
    <div class="section-label">入口请求 curl</div>
    <div class="code-block">{esc(curl_cmd)}</div>
""")
                if m.get("log_resolved_cmd"):
                    variant_parts.append(f"""
    <div class="section-label">日志检索 curl（Resolved）</div>
    <div class="code-block">{esc(m.get('log_resolved_cmd',''))}</div>
""")
                if m.get("log_finished_cmd"):
                    variant_parts.append(f"""
    <div class="section-label">日志检索 curl（Finished）</div>
    <div class="code-block">{esc(m.get('log_finished_cmd',''))}</div>
""")
                if m.get("log_resolved_full") or m.get("log_finished_full"):
                    variant_parts.append(f"""
    <details class="log-detail">
      <summary>完整日志（点击展开）</summary>
      <div class="section-label">Resolved 日志全文</div>
      <div class="code-block">{esc(m.get('log_resolved_full','(未采集或无匹配日志)'))}</div>
      <div class="section-label">Finished 日志全文</div>
      <div class="code-block">{esc(m.get('log_finished_full','(未采集或无匹配日志)'))}</div>
    </details>
""")
                variant_parts.append("""
  </div>
</details>
""")
        variant_html = "".join(variant_parts)

        detail_id = f"pipeline-detail-{idx}"
        evidence_block = f"""
<div class="entry-body">
  <div class="section-label">🧩 节点配置信息</div>
  <div class="mini-grid">
    <div class="key">目标后端</div><div class="val"><code>{esc(BID)}</code></div>
    <div class="key">目标模型</div><div class="val"><code>{esc(BMODEL)}</code></div>
    <div class="key">更新状态</div><div class="val">{node_update_badge}</div>
    <div class="key">更新 HTTP</div><div class="val">{esc(node_update_http)}</div>
    <div class="key">更新备注</div><div class="val">{esc(node_update_note)}</div>
  </div>
  <div class="section-label">节点更新请求 curl</div>
  <div class="code-block">{esc(node_update_cmd or '(无记录)')}</div>
  <div class="section-label">📤 响应内容</div>
  <div class="content-display">{esc(t.get('content','') or '(空)')}</div>
  <div class="section-label">⚙️ 执行命令</div>
  <div class="code-block">{esc(t.get('curl_cmd',''))}</div>
  <div class="section-label">📨 响应头</div>
  <div class="code-block">{esc(t.get('resp_headers_snippet',''))}</div>
  <div class="section-label">📦 响应体</div>
  <div class="code-block">{esc(t.get('resp_snippet',''))}</div>
  <div class="section-label">🧭 入口变体明细</div>
  {variant_html}
  <div class="section-label">🔍 测试结论</div>
  <div class="{acls}">{esc(t.get('analysis',''))}</div>
</div>
"""
        html.append(
            f"<tr class='pipeline-case-row' data-detail-id='{detail_id}' tabindex='0' role='button' aria-expanded='false'>"
            f"<td>{esc(pid)}</td>"
            f"<td>{badge_html}</td>"
            f"<td>{esc(t.get('http_code','?'))}</td>"
            f"<td>{esc(mode)}</td>"
            f"<td>{esc(t.get('duration_s',0))}</td>"
            f"<td>{esc(t.get('tokens',0))}</td>"
            f"<td>{esc(t.get('model_returned','?'))}</td>"
            f"<td>{node_update_badge}</td>"
            f"</tr>"
            f"<tr id='{detail_id}' class='pipeline-case-detail-row' style='display:none;'>"
            f"<td colspan='8'>{evidence_block}</td>"
            f"</tr>"
        )
    html.append("</table>")

# 入口变体矩阵
html.append('<h2>🧭 入口变体矩阵</h2>')
if MATRIX_TOTAL == 0:
    html.append('<div class="analysis-box warn">本次未启用入口变体矩阵，或矩阵结果文件缺失（/tmp/wizard_variant_matrix.json）。</div>')
else:
    html.append(f"""
<div class="matrix-summary">
  <div><strong>总样本</strong>：{MATRIX_TOTAL}</div>
  <div><strong>跳过</strong>：{MATRIX_SKIPPED}</div>
  <div><strong>有效样本</strong>：{MATRIX_EFFECTIVE_TOTAL}</div>
  <div><strong>通过</strong>：<span class="ok">{MATRIX_PASSED}</span></div>
  <div><strong>失败</strong>：<span class="bad">{MATRIX_FAILED}</span></div>
  <div><strong>通过率</strong>：{MATRIX_PASS_RATE}%</div>
</div>
<h3>按入口分组统计</h3>
<table>
  <tr>
    <th>入口变体</th><th>通过/总数</th><th>通过率</th><th>平均耗时(s)</th>
    <th>resolved 命中率</th><th>finished 匹配率</th>
  </tr>
""")
    for variant_name, s in sorted(variant_stats.items()):
        total = s["total"] if s["total"] > 0 else 1
        pass_rate = int((s["passed"] * 100) / total)
        avg_elapsed = s["elapsed_sum"] / total
        resolved_rate = int((s["resolved_hits_ok"] * 100) / total)
        finished_rate = int((s["finished_match_ok"] * 100) / total)
        rate_cls = "ok" if pass_rate >= 95 else ("warn" if pass_rate >= 70 else "bad")
        html.append(
            f"<tr>"
            f"<td><code>{esc(variant_name)}</code></td>"
            f"<td>{s['passed']}/{s['total']}</td>"
            f"<td><span class='rate-pill {rate_cls}'>{pass_rate}%</span></td>"
            f"<td>{avg_elapsed:.2f}</td>"
            f"<td>{resolved_rate}%</td>"
            f"<td>{finished_rate}%</td>"
            f"</tr>"
        )
    html.append("</table>")

    html.append('<div class="analysis-box">入口逐条详情已嵌入到上方“流水线测试详情”对应流水线行中，可直接点击整行展开对照与复测。</div>')

html.append('<h2 id="admin-suite">🛠️ 管理功能测试专区</h2>')
if ADMIN_TOTAL == 0:
    html.append('<div class="analysis-box warn">本次未执行管理功能测试，或结果文件缺失（/tmp/admin_e2e_results.json）。</div>')
else:
    html.append("""
<table class="admin-case-table">
  <tr><th>模块</th><th>用例</th><th>方法</th><th>接口</th><th>HTTP</th><th>断言</th><th>状态</th></tr>
""")
    for idx, row in enumerate(ADMIN_RESULTS):
        ok = bool(row.get("ok"))
        http_ok = bool(row.get("http_ok"))
        assert_ok = bool(row.get("assert_ok"))
        badge = '<span class="badge badge-pass">✅ 通过</span>' if ok else '<span class="badge badge-fail">❌ 失败</span>'
        evidence_block = f"""
  <div class="entry-body">
    <div class="mini-grid">
      <div class="key">预期 HTTP</div><div class="val"><code>{esc(row.get('expected_codes',''))}</code></div>
      <div class="key">断言说明</div><div class="val">{esc(row.get('assert_desc',''))}</div>
      <div class="key">断言表达式</div><div class="val"><code>{esc(row.get('assert_expr',''))}</code></div>
      <div class="key">判定</div><div class="val">HTTP={'✅' if row.get('http_ok') else '❌'}，业务断言={'✅' if row.get('assert_ok') else '❌'}</div>
      <div class="key">测试数据</div><div class="val">{esc(row.get('mock_data','无'))}</div>
      <div class="key">结论依据</div><div class="val">{esc(row.get('note',''))}</div>
    </div>
    <div class="section-label">请求 curl</div>
    <div class="code-block">{esc(row.get('curl_cmd',''))}</div>
    <div class="section-label">请求体（mock 数据）</div>
    <div class="code-block">{esc(clip(row.get('request_payload','(无请求体)'), 4000))}</div>
    <div class="section-label">响应头</div>
    <div class="code-block">{esc(clip(row.get('response_headers',''), 6000))}</div>
    <div class="section-label">响应体</div>
    <div class="code-block">{esc(clip(row.get('response_body',''), 12000))}</div>
  </div>
"""
        detail_id = f"admin-detail-{idx}"
        html.append(
            f"<tr class='admin-case-row' data-detail-id='{detail_id}' tabindex='0' role='button' aria-expanded='false'>"
            f"<td>{esc(row.get('module','-'))}</td>"
            f"<td>{esc(row.get('name','?'))}</td>"
            f"<td>{esc(row.get('method','?'))}</td>"
            f"<td><code>{esc(row.get('path','?'))}</code></td>"
            f"<td>{esc(row.get('http_code','?'))}</td>"
            f"<td>HTTP:{'✅' if http_ok else '❌'} / 业务:{'✅' if assert_ok else '❌'}</td>"
            f"<td>{badge}</td>"
            f"</tr>"
            f"<tr id='{detail_id}' class='admin-case-detail-row' style='display:none;'>"
            f"<td colspan='7'>{evidence_block}</td>"
            f"</tr>"
        )
    html.append("</table>")

# 复测指南
html.append('<h2 id="rerun-guide">📥 复测指南</h2><div class="reproduce-section"><p style="font-size:14px;margin-bottom:12px;color:var(--text-secondary);">以下命令可直接在终端运行复现本次测试：</p>')
if TOTAL > 0:
    for t in test_data:
        pid = t.get("pipeline", "?")
        cmd = t.get("curl_cmd", "")
        if cmd:
            html.append(f'<div class="section-label" style="font-size:13px;">{esc(pid)}</div>')
            html.append(f'<div class="code-block">{esc(cmd)}</div>')
if ADMIN_TOTAL > 0:
    html.append('<div class="section-label" style="font-size:13px;">admin-e2e</div>')
    html.append('<div class="code-block">bash docs/harness/skills/admin-e2e-test.sh</div>')
html.append("</div>")

# 故障总结
html.append('<h2 id="troubleshooting">⚠️ 故障分析总结</h2>')
if OVERALL_FAILED == 0:
    html.append(f'<div class="analysis-box">✅ 全部 {OVERALL_TOTAL} 条测试用例通过！系统运行正常。</div>')
else:
    html.append(f"""<div class="analysis-box error">
  {OVERALL_FAILED}/{OVERALL_TOTAL} 条测试失败。常见原因：
  <ul style="margin-top:8px;padding-left:20px;">
    <li><strong>HTTP 200 + 空响应</strong>：后端错误被吞没。用 curl 直连后端验证 Key。</li>
    <li><strong>HTTP 4xx</strong>：认证/权限问题，检查 JWT、流水线存在性。</li>
    <li><strong>HTTP 5xx</strong>：后端异常或 Centag 内部错误。</li>
    <li><strong>API Key</strong>：余额不足、过期或无模型权限。</li>
  </ul>
</div>""")

if TOTAL > 0 and PASSED == 0 and FAILED > 0:
    html.append(f"""<div class="analysis-box warn">
  <strong>⚠️ 关键警告</strong>：全部流水线返回空响应（content 空 + tokens=0）。
  强烈建议用 curl 直连验证：
  <div class="code-block" style="margin-top:8px;">curl -s -X POST "{esc(BURL)}/chat/completions" \\
  -H "Authorization: Bearer {BKEY}" \\
  -H "Content-Type: application/json" \\
  -d '{{"model":"{esc(BMODEL)}","messages":[{{"role":"user","content":"test"}}]}}'</div>
</div>""")

# 页脚
html.append(f"""<h2>📂 原始数据文件</h2>
<table><tr><th>文件名</th><th>说明</th></tr>
<tr><td><code>/tmp/wizard_test_*.json</code></td><td>每条流水线的完整响应</td></tr>
<tr><td><code>/tmp/wizard_test_data.json</code></td><td>测试数据汇总</td></tr>
<tr><td><code>/tmp/wizard_pipeline_update_cmds.json</code></td><td>流水线更新记录</td></tr>
<tr><td><code>/tmp/wizard_probe.json</code></td><td>后端探测结果</td></tr>
<tr><td><code>/tmp/wizard_variant_matrix.json</code></td><td>入口变体矩阵结果（可选）</td></tr>
<tr><td><code>/tmp/admin_e2e_results.json</code></td><td>管理功能测试结果（可选）</td></tr>
</table>
<div class="footer">Centag Wizard Test Report · 自动生成 · {NOW}</div>
<script>
  (function () {{
    function toggleRow(row) {{
      const detailId = row.getAttribute("data-detail-id");
      if (!detailId) return;
      const detail = document.getElementById(detailId);
      if (!detail) return;
      const expanded = row.getAttribute("aria-expanded") === "true";
      row.setAttribute("aria-expanded", expanded ? "false" : "true");
      detail.style.display = expanded ? "none" : "table-row";
    }}

    document.querySelectorAll(".admin-case-row, .pipeline-case-row").forEach(function (row) {{
      row.addEventListener("click", function () {{ toggleRow(row); }});
      row.addEventListener("keydown", function (e) {{
        if (e.key === "Enter" || e.key === " ") {{
          e.preventDefault();
          toggleRow(row);
        }}
      }});
    }});
  }})();
</script>
</div></body></html>""")

# 写入
with open(out, "w", encoding="utf-8") as f:
    f.write("\n".join(html))

print(f"✅ HTML 报告已生成: {out}")
print(f"   总通过: {OVERALL_PASSED}/{OVERALL_TOTAL} ({OVERALL_PASS_RATE}%), 总失败: {OVERALL_FAILED}/{OVERALL_TOTAL}")
