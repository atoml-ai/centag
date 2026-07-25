#!/usr/bin/env bash
# Centag pprof 采样与泄漏初判助手
#
# 依赖：curl；分析阶段可选 go（go tool pprof）。
# 前提：Centag 已开 pprof（./start.sh debug 默认开，或 CENTAG_PPROF=true）。
#
# 用法：
#   ./scripts/pprof-sample.sh once                 # 采一次并打印摘要
#   ./scripts/pprof-sample.sh watch               # 按间隔持续采样（Ctrl+C 结束并自动对比）
#   ./scripts/pprof-sample.sh analyze <dir>       # 分析已有采样目录
#   ./scripts/pprof-sample.sh diff <a.pb.gz> <b.pb.gz>
#
# 环境变量：
#   PPROF_ADDR     默认 http://127.0.0.1:6060
#   SAMPLE_DIR     采样输出根目录（默认 ~/.centag/var/pprof-samples）
#   INTERVAL_SEC   watch 间隔秒（默认 60）
#   DURATION_SEC   watch 最长秒数（默认 0=无限，直到 Ctrl+C）
#   PROCESS_GREP   额外匹配进程名（默认 centag|opencode）
#   SKIP_GC        设为 1 则结束时不做 ?gc=1 采样

set -euo pipefail

PPROF_ADDR="${PPROF_ADDR:-http://127.0.0.1:6060}"
SAMPLE_ROOT="${SAMPLE_DIR:-${CENTAG_INSTALL_ROOT:-$HOME/.centag}/var/pprof-samples}"
INTERVAL_SEC="${INTERVAL_SEC:-60}"
DURATION_SEC="${DURATION_SEC:-0}"
PROCESS_GREP="${PROCESS_GREP:-centag|opencode}"
SKIP_GC="${SKIP_GC:-0}"

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
CYAN=$'\033[0;36m'
NC=$'\033[0m'

log()  { printf '%s\n' "$*"; }
info() { printf "${CYAN}%s${NC}\n" "$*"; }
warn() { printf "${YELLOW}%s${NC}\n" "$*"; }
ok()   { printf "${GREEN}%s${NC}\n" "$*"; }
err()  { printf "${RED}%s${NC}\n" "$*" >&2; }

usage() {
  sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    err "缺少命令: $1"
    exit 1
  }
}

pprof_base() {
  printf '%s' "${PPROF_ADDR%/}"
}

check_pprof() {
  local base url
  base="$(pprof_base)"
  url="${base}/debug/pprof/"
  if ! curl -fsS --connect-timeout 2 --max-time 5 "$url" >/dev/null 2>&1; then
    err "无法访问 pprof: $url"
    err "请先: ./start.sh debug personal  （debug 默认开 pprof）"
    err "或:   CENTAG_PPROF=true ./start.sh run personal"
    exit 1
  fi
}

new_run_dir() {
  local ts dir
  ts="$(date '+%Y%m%d-%H%M%S')"
  dir="${SAMPLE_ROOT}/${ts}"
  mkdir -p "$dir"
  printf '%s' "$dir"
}

# 写 RSS / 进程表
sample_ps() {
  local out="$1"
  {
    echo "# time=$(date '+%Y-%m-%dT%H:%M:%S%z')"
    echo "# grep=${PROCESS_GREP}"
    # RSS 单位 KB（macOS/Linux ps 常见）
    if ps -axo pid=,rss=,vsz=,%mem=,etime=,comm= >/dev/null 2>&1; then
      printf 'PID\tRSS_KB\tVSZ_KB\tMEM%%\tELAPSED\tCOMM\n'
      # shellcheck disable=SC2009
      ps -axo pid=,rss=,vsz=,%mem=,etime=,comm= 2>/dev/null \
        | awk -v re="$PROCESS_GREP" 'BEGIN{IGNORECASE=1} $0 ~ re {printf "%s\t%s\t%s\t%s\t%s\t%s\n",$1,$2,$3,$4,$5,$6}'
    else
      ps aux 2>/dev/null | awk -v re="$PROCESS_GREP" 'BEGIN{IGNORECASE=1} NR==1 || $0 ~ re'
    fi
  } >"$out"
}

# 从 ps 快照提取最大 centag RSS（KB）
max_centag_rss_kb() {
  local file="$1"
  awk -F'\t' 'BEGIN{IGNORECASE=1; max=0}
    NR>1 && $6 ~ /centag/ {
      if ($2+0 > max) max=$2+0
    }
    END{print max+0}' "$file" 2>/dev/null || echo 0
}

fetch_profile() {
  local name="$1" out="$2" query="${3:-}"
  local url
  url="$(pprof_base)/debug/pprof/${name}${query}"
  if ! curl -fsS --connect-timeout 3 --max-time 120 "$url" -o "$out"; then
    err "拉取失败: $url"
    return 1
  fi
}

# 采一轮：prefix 如 001 / final
sample_round() {
  local dir="$1" prefix="$2" with_gc="${3:-0}"
  local base="${dir}/${prefix}"
  mkdir -p "$dir"

  sample_ps "${base}.ps.txt"
  fetch_profile heap "${base}.heap.pb.gz" || return 1
  fetch_profile goroutine "${base}.goroutine.pb.gz" || return 1
  # 文本摘要（无需 go）
  curl -fsS --max-time 30 "$(pprof_base)/debug/pprof/heap?debug=1" >"${base}.heap.debug1.txt" 2>/dev/null || true
  curl -fsS --max-time 30 "$(pprof_base)/debug/pprof/goroutine?debug=1" >"${base}.goroutine.debug1.txt" 2>/dev/null || true

  if [ "$with_gc" = "1" ] && [ "$SKIP_GC" != "1" ]; then
    info "[${prefix}] 触发 GC 后再采 heap…"
    fetch_profile heap "${base}.heap-after-gc.pb.gz" "?gc=1" || true
    sample_ps "${base}.ps-after-gc.txt"
  fi

  local rss
  rss="$(max_centag_rss_kb "${base}.ps.txt")"
  ok "[${prefix}] 已保存 → ${base}.*  (centag RSS≈${rss} KB)"
  printf '%s\n' "$rss" >"${base}.rss_kb"
}

extract_inuse_space() {
  # 从 heap?debug=1 文本里尽量抠 Inuse_space / # Total
  local f="$1"
  [ -f "$f" ] || { echo "n/a"; return; }
  if grep -E -q 'Inuse_space|inuse_space' "$f" 2>/dev/null; then
    grep -E 'Inuse_space|inuse_space' "$f" | head -3 | tr '\n' ' '
    echo
    return
  fi
  # Go debug=1 常见表头行
  if grep -E -q '^#.*(heap|Total)' "$f" 2>/dev/null; then
    grep -E '^#' "$f" | head -8
    return
  fi
  echo "n/a (see $(basename "$f"))"
}

count_goroutines() {
  local f="$1"
  [ -f "$f" ] || { echo 0; return; }
  # debug=1 第一行常为 "goroutine profile: total N"
  local n
  n="$(awk '/goroutine profile: total/{print $NF; exit}' "$f" 2>/dev/null || true)"
  if [[ "$n" =~ ^[0-9]+$ ]]; then
    echo "$n"
    return
  fi
  # 回退：统计 "goroutine N [" 行
  grep -cE '^goroutine [0-9]+ \[' "$f" 2>/dev/null || echo 0
}

list_prefixes() {
  local dir="$1"
  # 001.heap.pb.gz → 001
  find "$dir" -maxdepth 1 -name '*.heap.pb.gz' ! -name '*after-gc*' \
    | sed 's|.*/||; s|\.heap\.pb\.gz||' \
    | sort
}

run_pprof_top() {
  local file="$1" label="$2"
  if ! command -v go >/dev/null 2>&1; then
    warn "未安装 go，跳过 pprof -top（$label）"
    return 0
  fi
  if [ ! -s "$file" ]; then
    warn "文件为空: $file"
    return 0
  fi
  info "── go tool pprof -top ($label) ──"
  # inuse_space 看当前占用；失败则退回默认
  go tool pprof -top -inuse_space "$file" 2>/dev/null | head -25 \
    || go tool pprof -top "$file" 2>/dev/null | head -25 \
    || warn "pprof -top 失败: $file"
  echo
}

run_pprof_diff() {
  local base="$1" next="$2"
  if ! command -v go >/dev/null 2>&1; then
    warn "未安装 go，跳过 diff"
    return 0
  fi
  if [ ! -s "$base" ] || [ ! -s "$next" ]; then
    warn "diff 需要两个非空 heap 文件"
    return 0
  fi
  info "── heap diff (后 − 前，inuse_space) ──"
  go tool pprof -top -inuse_space -diff_base="$base" "$next" 2>/dev/null | head -30 \
    || go tool pprof -top -diff_base="$base" "$next" 2>/dev/null | head -30 \
    || warn "pprof diff 失败"
  echo
}

write_report() {
  local dir="$1"
  local report="${dir}/REPORT.md"
  local first last first_rss last_rss first_g last_g n
  # macOS bash 3.2 无 mapfile：用临时文件列前缀
  local plist
  plist="$(mktemp)"
  list_prefixes "$dir" >"$plist"
  n="$(wc -l <"$plist" | tr -d ' ')"
  if [ "${n:-0}" -eq 0 ]; then
    rm -f "$plist"
    err "目录无采样: $dir"
    return 1
  fi
  first="$(head -1 "$plist")"
  last="$(tail -1 "$plist")"
  rm -f "$plist"

  first_rss="$(cat "${dir}/${first}.rss_kb" 2>/dev/null || echo 0)"
  last_rss="$(cat "${dir}/${last}.rss_kb" 2>/dev/null || echo 0)"
  first_g="$(count_goroutines "${dir}/${first}.goroutine.debug1.txt")"
  last_g="$(count_goroutines "${dir}/${last}.goroutine.debug1.txt")"

  local rss_delta=$((last_rss - first_rss))
  local g_delta=$((last_g - first_g))

  {
    echo "# Centag pprof 采样报告"
    echo
    echo "- 目录: \`$dir\`"
    echo "- pprof: \`$PPROF_ADDR\`"
    echo "- 采样轮次: ${n}（${first} → ${last}）"
    echo "- 生成时间: $(date '+%Y-%m-%d %H:%M:%S %z')"
    echo
    echo "## 摘要"
    echo
    echo "| 指标 | 首次 (${first}) | 末次 (${last}) | Δ |"
    echo "|------|-----------------|----------------|---|"
    echo "| centag RSS (KB) | ${first_rss} | ${last_rss} | ${rss_delta} |"
    echo "| goroutine 约数 | ${first_g} | ${last_g} | ${g_delta} |"
    echo
    echo "### 首轮 heap debug 摘要"
    echo '```'
    extract_inuse_space "${dir}/${first}.heap.debug1.txt"
    echo '```'
    echo
    echo "### 末轮 heap debug 摘要"
    echo '```'
    extract_inuse_space "${dir}/${last}.heap.debug1.txt"
    echo '```'
    echo
    echo "## 初判建议（启发式，非结论）"
    echo
    if [ "$rss_delta" -gt 102400 ]; then
      echo "- RSS 增加超过约 **100MB**：优先看 heap diff 大户（SSE/\`[]byte\`/\`string\` 多为整包缓冲峰值）。"
    elif [ "$rss_delta" -gt 20480 ]; then
      echo "- RSS 增加约 **20MB+**：建议在业务停干净后等 2–3 分钟再 \`once\`，或看 \`*-after-gc\`。"
    else
      echo "- RSS 变化不大（<20MB）：更像波动；拉长 watch 或加大负载再比。"
    fi
    if [ "$g_delta" -gt 50 ]; then
      echo "- goroutine 明显增加（Δ>${g_delta}）：检查 MITM 隧道 \`io.Copy\`、未关闭的 HTTP body。"
    fi
    if [ -f "${dir}/${last}.heap-after-gc.pb.gz" ]; then
      echo "- 已采 GC 后 heap：\`go tool pprof -top -inuse_space ${dir}/${last}.heap-after-gc.pb.gz\`"
      echo "  - GC 后仍高：倾向被引用持有；GC 后明显下降：倾向峰值/延迟回收。"
    fi
    echo
    echo "## 常用命令"
    echo
    echo '```bash'
    echo "go tool pprof -http=:6061 ${dir}/${last}.heap.pb.gz"
    echo "go tool pprof -top -inuse_space -diff_base=${dir}/${first}.heap.pb.gz ${dir}/${last}.heap.pb.gz"
    echo "go tool pprof -top ${dir}/${last}.goroutine.pb.gz"
    echo '```'
    echo
    echo "## 采样文件"
    echo
    echo '```'
    ls -la "$dir" | sed '1d'
    echo '```'
  } >"$report"

  ok "报告已写: $report"
  echo
  # 终端再打一版简报
  info "======== 自动分析简报 ========"
  log "轮次: ${first} → ${last}  (共 ${n} 次)"
  log "centag RSS: ${first_rss} KB → ${last_rss} KB  (Δ ${rss_delta} KB)"
  log "goroutines: ${first_g} → ${last_g}  (Δ ${g_delta})"
  if [ "$rss_delta" -gt 102400 ] || [ "$g_delta" -gt 50 ]; then
    warn "→ 倾向「值得深挖」（RSS 大涨或 goroutine 堆积）。看 REPORT.md 与 heap diff。"
  elif [ "$rss_delta" -gt 20480 ]; then
    warn "→ 中等上涨：停负载后再采一次 / 看 after-gc。"
  else
    ok "→ 未见明显持续泄漏迹象（仍建议长任务后复测）。"
  fi
  echo
  run_pprof_top "${dir}/${last}.heap.pb.gz" "末轮 heap"
  if [ "$first" != "$last" ]; then
    run_pprof_diff "${dir}/${first}.heap.pb.gz" "${dir}/${last}.heap.pb.gz"
  fi
  run_pprof_top "${dir}/${last}.goroutine.pb.gz" "末轮 goroutine"
  info "完整报告: $report"
}

cmd_once() {
  need_cmd curl
  check_pprof
  local dir
  dir="$(new_run_dir)"
  info "采样目录: $dir"
  sample_round "$dir" "001" 1
  write_report "$dir"
}

cmd_watch() {
  need_cmd curl
  check_pprof
  local dir i=1 started now elapsed prefix
  dir="$(new_run_dir)"
  info "持续采样 → $dir"
  info "间隔 ${INTERVAL_SEC}s；DURATION_SEC=${DURATION_SEC:-0}（0=直到 Ctrl+C）"
  info "建议：另开终端跑 wrap+OpenCode 长任务，结束后回到此终端 Ctrl+C"
  echo

  started="$(date +%s)"
  # Ctrl+C 时做末轮分析
  cleanup_watch() {
    echo
    warn "停止采样，生成对比报告…"
    # 再采一轮带 GC
    prefix="$(printf '%03d' "$i")"
    sample_round "$dir" "$prefix" 1 || true
    write_report "$dir" || true
    exit 0
  }
  trap cleanup_watch INT TERM

  while true; do
    prefix="$(printf '%03d' "$i")"
    sample_round "$dir" "$prefix" 0
    i=$((i + 1))
    now="$(date +%s)"
    elapsed=$((now - started))
    if [ "$DURATION_SEC" -gt 0 ] && [ "$elapsed" -ge "$DURATION_SEC" ]; then
      info "已达 DURATION_SEC=${DURATION_SEC}，结束。"
      prefix="$(printf '%03d' "$i")"
      sample_round "$dir" "$prefix" 1
      write_report "$dir"
      break
    fi
    sleep "$INTERVAL_SEC"
  done
}

cmd_analyze() {
  local dir="${1:-}"
  if [ -z "$dir" ] || [ ! -d "$dir" ]; then
    err "用法: $0 analyze <采样目录>"
    exit 1
  fi
  write_report "$dir"
}

cmd_diff() {
  local a="${1:-}" b="${2:-}"
  if [ -z "$a" ] || [ -z "$b" ]; then
    err "用法: $0 diff <heap_a.pb.gz> <heap_b.pb.gz>"
    exit 1
  fi
  need_cmd go
  run_pprof_diff "$a" "$b"
}

main() {
  local cmd="${1:-}"
  case "$cmd" in
    ""|-h|--help|help) usage 0 ;;
    once) shift; cmd_once "$@" ;;
    watch) shift; cmd_watch "$@" ;;
    analyze) shift; cmd_analyze "$@" ;;
    diff) shift; cmd_diff "$@" ;;
    *)
      err "未知命令: $cmd"
      usage 1
      ;;
  esac
}

main "$@"
