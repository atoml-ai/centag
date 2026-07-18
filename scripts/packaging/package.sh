#!/usr/bin/env bash
# =============================================================
# Centag 第三方系统 / 渠道打包统一入口
#
# 用法:
#   ./scripts/packaging/package.sh list
#   ./scripts/packaging/package.sh fnos [--mode native|docker] [--arch amd64] ...
#   ./scripts/packaging/package.sh docker-offline
#
# 默认参数见仓库根目录 packaging.env；CLI / 环境变量可覆盖。
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
PACKAGE_OUTPUT="${PACKAGE_OUTPUT:-bin/packages}"
IMAGE_PREFIX="${IMAGE_PREFIX:-ghcr.io/marmotcai/}"

export PACKAGE_APP_NAME PACKAGE_APP_ID PACKAGE_ARCH PACKAGE_MODE PACKAGE_EDITION PACKAGE_OUTPUT IMAGE_PREFIX
export PACKAGE_ADMIN_PASSWORD="${PACKAGE_ADMIN_PASSWORD:-}"
export PACKAGE_ADMIN_USERNAME="${PACKAGE_ADMIN_USERNAME:-}"
export PACKAGE_ADMIN_API_KEY="${PACKAGE_ADMIN_API_KEY:-}"

usage() {
  cat <<EOF
${PACKAGE_APP_NAME} 第三方打包工具

用法:
  ./scripts/packaging/package.sh <target> [选项...]
  ./scripts/packaging/package.sh list

已注册目标:
  fnos             飞牛 OS (.fpk) — deploy/fnos/build-fpk.sh
  docker-offline   Docker 离线镜像包 — ./start.sh docker pack

全局默认（packaging.env）:
  PACKAGE_ARCH=${PACKAGE_ARCH}
  PACKAGE_MODE=${PACKAGE_MODE}
  PACKAGE_EDITION=${PACKAGE_EDITION}
  PACKAGE_OUTPUT=${PACKAGE_OUTPUT}
  IMAGE_PREFIX=${IMAGE_PREFIX}

管理员密码来源（fnos）:
  --admin-password > PACKAGE_ADMIN_PASSWORD > config/secrets/.env

示例:
  ./scripts/packaging/package.sh fnos
  ./scripts/packaging/package.sh fnos --edition minimal --arch amd64
  ./scripts/packaging/package.sh fnos --edition personal --arch amd64
  ./scripts/packaging/package.sh fnos --edition team --admin-password '***'
  make package TARGET=fnos PACKAGE_EDITION=personal
  ./start.sh package fnos --edition minimal --arch amd64
EOF
}

list_targets() {
  cat <<EOF
可用打包目标:

  目标             产物形式              实现
  --------------   -------------------   --------------------------------
  fnos             .fpk (native/docker)  deploy/fnos/build-fpk.sh
  docker-offline   离线 Docker 目录包    start.sh docker pack

默认参数文件: ${ENV_FILE}
EOF
}

target="${1:-}"
if [ -z "$target" ] || [ "$target" = "-h" ] || [ "$target" = "--help" ] || [ "$target" = "help" ]; then
  usage
  exit 0
fi
shift

case "$target" in
  list|ls)
    list_targets
    ;;

  fnos)
    # 若调用方未显式传关键参数，注入 packaging.env 默认值
    has_mode=false
    has_arch=false
    has_output=false
    has_prefix=false
    has_edition=false
    for arg in "$@"; do
      case "$arg" in
        --mode) has_mode=true ;;
        --arch) has_arch=true ;;
        --output) has_output=true ;;
        --image-prefix) has_prefix=true ;;
        --edition) has_edition=true ;;
      esac
    done

    extra=()
    [ "$has_mode" = false ] && extra+=(--mode "$PACKAGE_MODE")
    [ "$has_arch" = false ] && extra+=(--arch "$PACKAGE_ARCH")
    [ "$has_output" = false ] && extra+=(--output "$PACKAGE_OUTPUT")
    [ "$has_prefix" = false ] && extra+=(--image-prefix "$IMAGE_PREFIX")
    [ "$has_edition" = false ] && extra+=(--edition "$PACKAGE_EDITION")

    exec bash "${ROOT}/deploy/fnos/build-fpk.sh" "${extra[@]}" "$@"
    ;;

  docker-offline|docker_offline|docker-pack)
    exec bash "${ROOT}/start.sh" docker pack "$@"
    ;;

  *)
    echo "error: 未知打包目标 '$target'" >&2
    echo "运行 './scripts/packaging/package.sh list' 查看可用目标" >&2
    exit 1
    ;;
esac
