#!/usr/bin/env bash
# =============================================================
# Centag 部署包统一入口
#
# 三维：形态(form) × 系统(os) × 架构(arch)
#
#   form:  cli | desktop
#   os:    macos | linux | windows | fnos | docker
#   arch:  amd64 | arm64 | host | all   （可选）
#
# 用法:
#   ./start.sh package <form> <os> [arch] [选项...]
#   ./start.sh package list
#
# 示例:
#   ./start.sh package desktop macos
#   ./start.sh package desktop macos arm64 --skip-frontend
#   ./start.sh package cli linux all
#   ./start.sh package cli fnos amd64 --edition personal
#   ./start.sh package cli docker
#
# 默认参数见仓库根目录 packaging.env（主要服务 fnos）。
# =============================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${ROOT}/packaging.env"

if [ -f "$ENV_FILE" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
fi

PACKAGE_APP_NAME="${PACKAGE_APP_NAME:-Centag}"
PACKAGE_APP_ID="${PACKAGE_APP_ID:-centag}"
PACKAGE_ARCH="${PACKAGE_ARCH:-amd64}"
PACKAGE_MODE="${PACKAGE_MODE:-native}"
PACKAGE_EDITION="${PACKAGE_EDITION:-minimal}"
PACKAGE_OUTPUT="${PACKAGE_OUTPUT:-${HOME}/.centag/var/packages}"
IMAGE_PREFIX="${IMAGE_PREFIX:-ghcr.io/marmotcai/}"

export PACKAGE_APP_NAME PACKAGE_APP_ID PACKAGE_ARCH PACKAGE_MODE PACKAGE_EDITION PACKAGE_OUTPUT IMAGE_PREFIX
export PACKAGE_ADMIN_PASSWORD="${PACKAGE_ADMIN_PASSWORD:-}"
export PACKAGE_ADMIN_USERNAME="${PACKAGE_ADMIN_USERNAME:-}"
export PACKAGE_ADMIN_API_KEY="${PACKAGE_ADMIN_API_KEY:-}"
export PACKAGE_API_KEY_STORAGE_SECRET="${PACKAGE_API_KEY_STORAGE_SECRET:-}"

fail() { echo "error: $*" >&2; exit 1; }
log() { echo "==> $*" >&2; }

usage() {
  cat <<EOF
${PACKAGE_APP_NAME} 部署包（形态 × 系统 × 架构）

用法:
  ./start.sh package <form> <os> [arch] [选项...]
  ./start.sh package list

维度:
  form   cli | desktop
  os     macos | linux | windows | fnos | docker
  arch   amd64 | arm64 | host | all   （可省略；desktop 默认 host，cli/linux 默认 all）

含义:
  cli       命令行网关包（可交叉编译）
  desktop   桌面入口包（macOS→dmg+zip，Windows→zip；需本机对应系统，含托盘壳）
  fnos      飞牛 OS（作为一种目标系统，走 .fpk）
  docker    容器离线包（作为一种目标系统）

组合规则:
  desktop + macos|windows   → 本机桌面包（必须与当前 OS 一致）
  desktop + linux|fnos|docker → 不支持（Linux/NAS/容器用 cli）
  cli + linux|macos|windows → CLI tarball（--platforms）
  cli + fnos                → .fpk（edition/mode 见 packaging.env）
  cli + docker              → Docker 离线包

示例:
  ./start.sh package desktop macos
  ./start.sh package desktop macos host --skip-frontend
  ./start.sh package desktop windows
  ./start.sh package cli linux
  ./start.sh package cli linux amd64
  ./start.sh package cli linux all
  ./start.sh package cli macos all          # 交叉编译 macOS CLI（非桌面）
  ./start.sh package cli fnos amd64 --edition personal
  ./start.sh package cli docker
  ./start.sh package list

输出:
  cli/desktop（macos|linux|windows）→ ~/.centag/var/release/<version>/
  fnos / docker                     → ${PACKAGE_OUTPUT} 或各脚本默认
EOF
}

list_targets() {
  cat <<EOF
形态 × 系统矩阵（✓ 支持 / · 不适用）:

            macos     linux     windows   fnos      docker
  cli         ✓         ✓         ✓         ✓         ✓
  desktop     ✓         ·         ✓         ·         ·

常用命令:
  ./start.sh package desktop macos [--skip-frontend]
  ./start.sh package desktop windows
  ./start.sh package cli linux [amd64|arm64|all]
  ./start.sh package cli fnos [amd64|arm64] [--edition personal]
  ./start.sh package cli docker

说明:
  • desktop = 桌面分发形态（替代旧名 tray/dmg 目标）；内含托盘壳 + sidecar
  • cli     = 命令行分发形态
  • fnos / docker 是目标系统，不是第三种形态
  • install.sh 发布集 = desktop(macos)+desktop(windows)+cli(linux)，由 CI/本机分别构建

默认参数文件: ${ENV_FILE}
EOF
}

normalize_os() {
  case "$1" in
    macos|darwin|osx) echo "macos" ;;
    linux) echo "linux" ;;
    windows|win) echo "windows" ;;
    fnos|fn|feiniu) echo "fnos" ;;
    docker|docker-offline|docker_offline) echo "docker" ;;
    *) return 1 ;;
  esac
}

normalize_form() {
  case "$1" in
    cli|command|cmdline) echo "cli" ;;
    desktop|desk|gui) echo "desktop" ;;
    # legacy aliases → rewrite hints
    tray|dmg)
      fail "已废弃目标 '$1'：请用形态 desktop，例如: ./start.sh package desktop macos"
      ;;
    github|gh|install-sh)
      fail "已废弃目标 '$1'：github/install.sh 是「desktop(macos)+desktop(windows)+cli(linux)」组合，请分条构建或跑 CI"
      ;;
    *) return 1 ;;
  esac
}

normalize_arch() {
  case "$1" in
    amd64|x86_64|x64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    host|native) echo "host" ;;
    all|any) echo "all" ;;
    *) return 1 ;;
  esac
}

host_goos() {
  if command -v go >/dev/null 2>&1; then
    go env GOOS
    return
  fi
  case "$(uname -s 2>/dev/null || true)" in
    Darwin*) echo darwin ;;
    Linux*) echo linux ;;
    MINGW*|MSYS*|CYGWIN*) echo windows ;;
    *) echo unknown ;;
  esac
}

host_goarch() {
  if command -v go >/dev/null 2>&1; then
    go env GOARCH
    return
  fi
  case "$(uname -m 2>/dev/null || true)" in
    arm64|aarch64) echo arm64 ;;
    *) echo amd64 ;;
  esac
}

# Map form/os/arch → platforms list for build-artifacts.sh
cli_platforms() {
  local os="$1" arch="$2"
  local goos
  case "$os" in
    macos) goos="darwin" ;;
    linux) goos="linux" ;;
    windows) goos="windows" ;;
    *) fail "cli 不支持 os=$os 的 platforms 映射" ;;
  esac

  case "$arch" in
    host) echo "${goos}-$(host_goarch)" ;;
    amd64|arm64) echo "${goos}-${arch}" ;;
    all) echo "${goos}-amd64,${goos}-arm64" ;;
    *) fail "unknown arch: $arch" ;;
  esac
}

require_host_os() {
  local want="$1" # macos|windows|linux
  local have
  have="$(host_goos)"
  case "$want" in
    macos) [[ "$have" == "darwin" ]] || fail "desktop macos 须在 macOS 本机构建（当前 GOOS=$have）" ;;
    windows) [[ "$have" == "windows" ]] || fail "desktop windows 须在 Windows 本机构建（当前 GOOS=$have）" ;;
    linux) [[ "$have" == "linux" ]] || fail "须在 Linux 本机（当前 GOOS=$have）" ;;
  esac
}

run_desktop() {
  local os="$1" arch="$2"
  shift 2 || true
  case "$os" in
    macos)
      require_host_os macos
      [[ "$arch" == "host" || "$arch" == "all" || "$arch" == "$(host_goarch)" ]] \
        || fail "desktop 不交叉编译；本机 arch=$(host_goarch)，请求 arch=$arch"
      exec bash "${ROOT}/scripts/release/package-desktop.sh" "$@"
      ;;
    windows)
      require_host_os windows
      [[ "$arch" == "host" || "$arch" == "all" || "$arch" == "$(host_goarch)" ]] \
        || fail "desktop 不交叉编译；本机 arch=$(host_goarch)，请求 arch=$arch"
      exec bash "${ROOT}/scripts/release/package-desktop.sh" "$@"
      ;;
    linux|fnos|docker)
      fail "desktop 不支持 os=$os（请用: ./start.sh package cli $os ...）"
      ;;
    *)
      fail "unknown os: $os"
      ;;
  esac
}

run_cli() {
  local os="$1" arch="$2"
  shift 2 || true
  case "$os" in
    macos|linux|windows)
      local platforms
      platforms="$(cli_platforms "$os" "$arch")"
      local has_components=false
      local has_platforms=false
      local arg
      for arg in "$@"; do
        case "$arg" in
          --components) has_components=true ;;
          --platforms) has_platforms=true ;;
        esac
      done
      local extra=()
      [ "$has_components" = false ] && extra+=(--components personal)
      [ "$has_platforms" = false ] && extra+=(--platforms "$platforms")
      log "cli ${os}/${arch} → platforms=${platforms}"
      exec bash "${ROOT}/scripts/release/build-artifacts.sh" "${extra[@]}" "$@"
      ;;
    fnos)
      local has_mode=false has_arch=false has_output=false has_prefix=false has_edition=false
      local arg
      for arg in "$@"; do
        case "$arg" in
          --mode) has_mode=true ;;
          --arch) has_arch=true ;;
          --output) has_output=true ;;
          --image-prefix) has_prefix=true ;;
          --edition) has_edition=true ;;
        esac
      done
      local fn_arch="$PACKAGE_ARCH"
      case "$arch" in
        host) fn_arch="$(host_goarch)" ;;
        amd64|arm64) fn_arch="$arch" ;;
        all) fail "fnos 请指定单一 arch: amd64 或 arm64" ;;
      esac
      local extra=()
      [ "$has_mode" = false ] && extra+=(--mode "$PACKAGE_MODE")
      [ "$has_arch" = false ] && extra+=(--arch "$fn_arch")
      [ "$has_output" = false ] && extra+=(--output "$PACKAGE_OUTPUT")
      [ "$has_prefix" = false ] && extra+=(--image-prefix "$IMAGE_PREFIX")
      [ "$has_edition" = false ] && extra+=(--edition "$PACKAGE_EDITION")
      log "cli fnos/${fn_arch} → build-fpk.sh"
      exec bash "${ROOT}/deploy/fnos/build-fpk.sh" "${extra[@]}" "$@"
      ;;
    docker)
      log "cli docker → start.sh docker pack"
      exec bash "${ROOT}/start.sh" docker pack "$@"
      ;;
    *)
      fail "unknown os: $os"
      ;;
  esac
}

# --- argv ------------------------------------------------------------------
arg1="${1:-}"
if [ -z "$arg1" ] || [ "$arg1" = "-h" ] || [ "$arg1" = "--help" ] || [ "$arg1" = "help" ]; then
  usage
  exit 0
fi

if [ "$arg1" = "list" ] || [ "$arg1" = "ls" ]; then
  list_targets
  exit 0
fi

# Legacy single-token shortcuts (emit migration hint then map)
case "$arg1" in
  dmg)
    fail "请改用: ./start.sh package desktop macos [arch] ..."
    ;;
  tray)
    fail "请改用形态 desktop: ./start.sh package desktop macos|windows ..."
    ;;
  github|gh)
    fail "请分别构建: package desktop macos / package desktop windows / package cli linux
完整矩阵由 CI 汇总；本机可: package desktop macos && package cli linux"
    ;;
  desktop|cli)
    ;;
  fnos)
    # allow: package fnos [arch] → package cli fnos [arch]
    shift
    set -- cli fnos "$@"
    arg1=cli
    ;;
  docker-offline|docker_offline)
    shift
    set -- cli docker "$@"
    arg1=cli
    ;;
esac

FORM="$(normalize_form "${1:-}" || true)"
[[ -n "$FORM" ]] || fail "未知形态 '${1:-}'（要 cli|desktop）。见: ./start.sh package list"
shift

OS_RAW="${1:-}"
[[ -n "$OS_RAW" ]] || fail "缺少系统参数 os（macos|linux|windows|fnos|docker）"
OS="$(normalize_os "$OS_RAW" || true)"
[[ -n "$OS" ]] || fail "未知系统 '$OS_RAW'（要 macos|linux|windows|fnos|docker）"
shift

ARCH="host"
if [[ "${1:-}" != "" && "${1:-}" != -* ]]; then
  ARCH="$(normalize_arch "$1" || true)"
  [[ -n "$ARCH" ]] || fail "未知架构 '$1'（要 amd64|arm64|host|all）"
  shift
else
  # defaults by form/os
  case "$FORM-$OS" in
    desktop-*) ARCH="host" ;;
    cli-linux|cli-macos|cli-windows) ARCH="all" ;;
    cli-fnos) ARCH="$PACKAGE_ARCH" ;;
    cli-docker) ARCH="host" ;;
  esac
fi

case "$FORM" in
  desktop) run_desktop "$OS" "$ARCH" "$@" ;;
  cli) run_cli "$OS" "$ARCH" "$@" ;;
  *) fail "internal: bad form $FORM" ;;
esac
