#!/bin/bash
# =============================================================
# Centag fnOS (.fpk) 统一打包脚本
#
# 发行版（默认 minimal）:
#   ./deploy/fnos/build-fpk.sh --mode native --edition minimal
#   ./deploy/fnos/build-fpk.sh --mode native --edition personal --arch amd64
#   ./deploy/fnos/build-fpk.sh --mode native --edition team --arch amd64
#
# 管理员密码（优先顺序）:
#   1) --admin-password
#   2) 环境变量 PACKAGE_ADMIN_PASSWORD
#   3) config/secrets/.env 的 LLM_PROXY_ADMIN_PASSWORD
#   写入包内 config/runtime.env，由 native/cmd/main 启动时加载
#
# Docker 模式：
#   ./deploy/fnos/build-fpk.sh --mode docker --arch amd64
#   ./deploy/fnos/build-fpk.sh --mode docker --arch arm64 --image-prefix ghcr.io/marmotcai/
#
# Native 模式：
#   ./deploy/fnos/build-fpk.sh --mode native --arch amd64
#   ./deploy/fnos/build-fpk.sh --mode native --skip-build
#   ./deploy/fnos/build-fpk.sh --mode native --install
#
# 通用参数:
#     --edition E        minimal | personal | team（默认 packaging.env / minimal）
#     --admin-password P 管理员密码（写入 runtime.env）
#     --admin-username U 管理员用户名（默认 admin）
#     --output DIR       指定输出目录（默认 bin/packages/）
#     --image-prefix P   镜像名前缀
#     --install          打包后自动安装到 fnOS
#     --help             显示帮助
# =============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# 加载根目录第三方打包默认参数（可由环境变量 / CLI 覆盖）
if [ -f "${REPO_ROOT}/packaging.env" ]; then
  # shellcheck disable=SC1091
  set -a
  # shellcheck source=/dev/null
  . "${REPO_ROOT}/packaging.env"
  set +a
fi

BUILD_DIR=$(mktemp -d /tmp/centag-fpk-XXXXXX)
OUTPUT_DIR="${PACKAGE_OUTPUT:-${REPO_ROOT}/bin/packages}"
case "${OUTPUT_DIR}" in
  /*) ;;
  *) OUTPUT_DIR="${REPO_ROOT}/${OUTPUT_DIR}" ;;
esac
IMAGE_PREFIX="${IMAGE_PREFIX:-ghcr.io/marmotcai/}"

# 模式参数（packaging.env 的 PACKAGE_* 作为默认值）
MODE="${PACKAGE_MODE:-native}"           # docker | native
ARCH="${PACKAGE_ARCH:-$(uname -m)}"      # 当前架构（默认）
EDITION="${PACKAGE_EDITION:-minimal}"    # minimal | personal | team
SKIP_BUILD=false
INSTALL_AFTER=false
ADMIN_PASSWORD_CLI=""
ADMIN_USERNAME_CLI=""

# macOS 无 md5sum，需兼容 md5 / openssl
file_md5() {
  local f="$1"
  if command -v md5sum >/dev/null 2>&1; then
    md5sum "$f" | awk '{print $1}'
  elif command -v md5 >/dev/null 2>&1; then
    md5 -q "$f"
  elif command -v openssl >/dev/null 2>&1; then
    openssl md5 -r "$f" | awk '{print $1}'
  else
    echo "[ERROR] 需要 md5sum、md5 或 openssl 以计算 manifest checksum" >&2
    exit 1
  fi
}

# 从 KEY=VALUE 文件读取值（不 source，避免特殊字符副作用）
read_env_key() {
  local file="$1"
  local key="$2"
  [ -f "$file" ] || return 1
  local line
  line="$(grep -E "^[[:space:]]*(export[[:space:]]+)?${key}=" "$file" 2>/dev/null | tail -1 || true)"
  [ -n "$line" ] || return 1
  line="${line#*=}"
  line="${line%\"}"
  line="${line#\"}"
  line="${line%\'}"
  line="${line#\'}"
  printf '%s' "$line"
}

# personal → personal dist；runtime edition 仍为 personal
edition_to_dist() {
  case "$1" in
    personal) echo "personal" ;;
    minimal) echo "minimal" ;;
    team) echo "team" ;;
    *) echo "$1" ;;
  esac
}

edition_to_runtime() {
  case "$1" in
    personal) echo "personal" ;;
    *) echo "$1" ;;
  esac
}

edition_build_tags() {
  case "$1" in
    minimal)
      echo "minimal,protocol_openai,protocol_anthropic,backend_openai,backend_ollama,backend_anthropic"
      ;;
    personal|team)
      echo "protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure"
      ;;
    *)
      echo "[ERROR] 未知发行版: $1（支持 minimal|personal|team）" >&2
      exit 1
      ;;
  esac
}

resolve_admin_credentials() {
  local secrets="${REPO_ROOT}/config/secrets/.env"
  if [ -n "${ADMIN_PASSWORD_CLI}" ]; then
    ADMIN_PASSWORD="${ADMIN_PASSWORD_CLI}"
  elif [ -n "${PACKAGE_ADMIN_PASSWORD:-}" ]; then
    ADMIN_PASSWORD="${PACKAGE_ADMIN_PASSWORD}"
  elif ADMIN_PASSWORD="$(read_env_key "$secrets" "LLM_PROXY_ADMIN_PASSWORD" 2>/dev/null)"; then
    :
  else
    ADMIN_PASSWORD=""
  fi

  if [ -n "${ADMIN_USERNAME_CLI}" ]; then
    ADMIN_USERNAME="${ADMIN_USERNAME_CLI}"
  elif [ -n "${PACKAGE_ADMIN_USERNAME:-}" ]; then
    ADMIN_USERNAME="${PACKAGE_ADMIN_USERNAME}"
  elif ADMIN_USERNAME="$(read_env_key "$secrets" "LLM_PROXY_ADMIN_USERNAME" 2>/dev/null)"; then
    :
  else
    ADMIN_USERNAME="admin"
  fi

  ADMIN_API_KEY=""
  if [ -n "${PACKAGE_ADMIN_API_KEY:-}" ]; then
    ADMIN_API_KEY="${PACKAGE_ADMIN_API_KEY}"
  else
    ADMIN_API_KEY="$(read_env_key "$secrets" "LLM_PROXY_ADMIN_API_KEY" 2>/dev/null || true)"
    if [ -z "${ADMIN_API_KEY}" ]; then
      ADMIN_API_KEY="$(read_env_key "$secrets" "LLM_PROXY_DEFAULT_ADMIN_API_KEY" 2>/dev/null || true)"
    fi
  fi

  # 界面「复制完整 API Key」依赖加密存储；打包必须写入 STORAGE_SECRET
  API_KEY_STORAGE_SECRET=""
  if [ -n "${PACKAGE_API_KEY_STORAGE_SECRET:-}" ]; then
    API_KEY_STORAGE_SECRET="${PACKAGE_API_KEY_STORAGE_SECRET}"
  else
    API_KEY_STORAGE_SECRET="$(read_env_key "$secrets" "LLM_PROXY_API_KEY_STORAGE_SECRET" 2>/dev/null || true)"
  fi
  if [ -z "${API_KEY_STORAGE_SECRET}" ]; then
    if command -v openssl >/dev/null 2>&1; then
      API_KEY_STORAGE_SECRET="$(openssl rand -hex 32)"
      echo "[OK] 未在 .env 找到 LLM_PROXY_API_KEY_STORAGE_SECRET，已现场生成并写入 runtime.env"
    else
      API_KEY_STORAGE_SECRET="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
      echo "[OK] 未在 .env 找到 LLM_PROXY_API_KEY_STORAGE_SECRET，已用 /dev/urandom 生成并写入 runtime.env"
    fi
  else
    echo "[OK] API Key 存储密钥已解析（界面可反复复制完整 Key）"
  fi

  if [ -z "${ADMIN_PASSWORD}" ]; then
    echo "[WARN] 未解析到管理员密码（--admin-password / PACKAGE_ADMIN_PASSWORD / config/secrets/.env）"
    echo "       minimal 首次需 Web 设置密码；personal/team 首轮 seed 将使用内置默认口令"
  else
    echo "[OK] 管理员密码已解析（来源: CLI/packaging.env/secrets，不会回显）用户=${ADMIN_USERNAME}"
  fi
  if [ -n "${ADMIN_API_KEY}" ]; then
    echo "[OK] 默认管理员 API Key 已解析（将写入 runtime.env 供首轮 seed）"
  else
    echo "[WARN] 未解析到 LLM_PROXY_ADMIN_API_KEY / DEFAULT；首轮不会预置 API Key"
  fi
}

shell_single_quote() {
  # 安全写入可被 `set -a; . file` 加载的单引号字符串
  local s="$1"
  s="${s//\'/\'\\\'\'}"
  printf "'%s'" "$s"
}

write_runtime_env() {
  local dest_dir="$1"
  local runtime_edition
  runtime_edition="$(edition_to_runtime "$EDITION")"
  mkdir -p "${dest_dir}"
  {
    echo "# Generated by deploy/fnos/build-fpk.sh — do not commit secrets from private builds"
    echo "CENTAG_EDITION=$(shell_single_quote "${runtime_edition}")"
    echo "LLM_PROXY_ADMIN_USERNAME=$(shell_single_quote "${ADMIN_USERNAME}")"
    echo "LLM_PROXY_ADMIN_PASSWORD=$(shell_single_quote "${ADMIN_PASSWORD}")"
    echo "LLM_PROXY_DB_DRIVER='sqlite'"
    echo "SERVER_HOST='0.0.0.0'"
    echo "SERVER_PORT='20060'"
    echo "LLM_PROXY_SERVER_HOST='0.0.0.0'"
    echo "LLM_PROXY_SERVER_PORT='20060'"
    echo "LOG_LEVEL='info'"
    echo "LLM_PROXY_LOG_OUTPUT='both'"
    echo "TZ='Asia/Shanghai'"
    # ResolveDataDir 读取 CENTAG_DATA_DIR；由 cmd/main 再落到数据共享卷
    echo "# CENTAG_DATA_DIR is set at runtime by cmd/main to TRIM data share"
    if [ -n "${ADMIN_API_KEY}" ]; then
      echo "LLM_PROXY_ADMIN_API_KEY=$(shell_single_quote "${ADMIN_API_KEY}")"
      echo "LLM_PROXY_DEFAULT_ADMIN_API_KEY=$(shell_single_quote "${ADMIN_API_KEY}")"
    fi
    # 必须写入：否则 seed 的 Key 只有哈希，Web 无法复制完整密钥
    if [ -n "${API_KEY_STORAGE_SECRET:-}" ]; then
      echo "LLM_PROXY_API_KEY_STORAGE_SECRET=$(shell_single_quote "${API_KEY_STORAGE_SECRET}")"
    fi
  } > "${dest_dir}/runtime.env"
  chmod 600 "${dest_dir}/runtime.env"
  echo "  运行时配置: config/runtime.env (edition=${runtime_edition})"
}

# 将 uname 架构映射为 Go 架构
arch_to_go() {
  case "$1" in
    x86_64|amd64)  echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    armv7l|arm)    echo "arm" ;;
    *)             echo "$1" ;;
  esac
}

GOARCH="$(arch_to_go "$ARCH")"

# 将 Go 架构映射为 Docker 平台
goarch_to_docker_platform() {
  case "$1" in
    amd64) echo "linux/amd64" ;;
    arm64) echo "linux/arm64" ;;
    arm)   echo "linux/arm/v7" ;;
    *)
      echo "[ERROR] Docker 模式不支持架构: $1（支持 amd64/arm64/arm）" >&2
      exit 1
      ;;
  esac
}

# 为镜像仓库名附加架构后缀（centag:latest -> centag-arm64:latest）
with_arch_suffix() {
  local image="$1"
  local arch="$2"
  local repo
  local tag
  if [[ "$image" == *:* ]]; then
    repo="${image%:*}"
    tag="${image##*:}"
  else
    repo="$image"
    tag="latest"
  fi
  echo "${repo}-${arch}:${tag}"
}

# 按需给镜像仓库名添加前缀（为空时不添加）
with_image_prefix() {
  local image="$1"
  local prefix="$2"
  local repo
  local tag

  if [[ "$image" == *:* ]]; then
    repo="${image%:*}"
    tag="${image##*:}"
  else
    repo="$image"
    tag="latest"
  fi

  if [[ -z "${prefix}" ]]; then
    echo "${repo}:${tag}"
    return
  fi

  prefix="${prefix%/}"
  echo "${prefix}/${repo}:${tag}"
}

# 显示脚本头部帮助
print_help() {
  sed -n '2,34p' "$0" | sed 's/^# \{0,1\}//'
}

# 无参数时仅显示帮助，避免误触发默认打包
if [[ $# -eq 0 ]]; then
  print_help
  exit 0
fi

# ----- 参数解析 -----
while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode)
      MODE="$2"
      shift 2
      ;;
    --arch)
      GOARCH="$(arch_to_go "$2")"
      shift 2
      ;;
    --edition)
      EDITION="$2"
      shift 2
      ;;
    --admin-password)
      ADMIN_PASSWORD_CLI="$2"
      shift 2
      ;;
    --admin-username)
      ADMIN_USERNAME_CLI="$2"
      shift 2
      ;;
    --skip-build)
      SKIP_BUILD=true
      shift
      ;;
    --install)
      INSTALL_AFTER=true
      shift
      ;;
    --output)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --image-prefix)
      IMAGE_PREFIX="$2"
      shift 2
      ;;
    --help)
      print_help
      exit 0
      ;;
    *)
      echo "[ERROR] 未知参数: $1"
      echo "  使用 --help 查看帮助"
      exit 1
      ;;
  esac
done

case "$EDITION" in
  minimal|personal|team) ;;
  *)
    echo "[ERROR] 未知 --edition: ${EDITION}（支持 minimal|personal|team）"
    exit 1
    ;;
esac

DIST_NAME="$(edition_to_dist "$EDITION")"
RUNTIME_EDITION="$(edition_to_runtime "$EDITION")"
BUILD_TAGS="$(edition_build_tags "$EDITION")"

# 相对路径按仓库根解析
case "${OUTPUT_DIR}" in
  /*) ;;
  *) OUTPUT_DIR="${REPO_ROOT}/${OUTPUT_DIR}" ;;
esac

mkdir -p "$OUTPUT_DIR"

resolve_admin_credentials

echo "============================================"
echo " Centag fnOS 打包工具"
echo "============================================"
echo "模式:      ${MODE}"
echo "发行版:    ${EDITION} (dist=${DIST_NAME}, runtime=${RUNTIME_EDITION})"
echo "架构:      ${GOARCH}"
echo "源码目录:  ${REPO_ROOT}"
echo "构建目录:  ${BUILD_DIR}"
echo "输出目录:  ${OUTPUT_DIR}"
if [ "$MODE" = "native" ]; then
  echo "构建 tags: ${BUILD_TAGS}"
fi
echo ""

# ============================================================
# 1. 构建（Docker 或 Native）
# ============================================================
if [ "$MODE" = "docker" ]; then
  # ----- Docker 模式 -----
  DOCKER_PLATFORM="$(goarch_to_docker_platform "$GOARCH")"
  DOCKER_IMAGE_BASE="centag:latest"
  DOCKER_IMAGE_BASE="$(with_image_prefix "$DOCKER_IMAGE_BASE" "$IMAGE_PREFIX")"
  DOCKER_IMAGE_TAG="$(with_arch_suffix "$DOCKER_IMAGE_BASE" "$GOARCH")"

  # 检查 Docker 镜像
  IMAGE_ID=$(docker images "${DOCKER_IMAGE_TAG}" --format "{{.ID}}" 2>/dev/null || true)
  if [ -z "$IMAGE_ID" ]; then
    echo "[1/5] Docker 镜像 ${DOCKER_IMAGE_TAG} 不存在，正在构建..."
    echo "      平台: ${DOCKER_PLATFORM}"
    docker build --platform "${DOCKER_PLATFORM}" \
      -t "${DOCKER_IMAGE_TAG}" \
      -f "$REPO_ROOT/deploy/docker/Dockerfile" "$REPO_ROOT"
    echo "[OK] 镜像构建完成"
  else
    echo "[1/5] Docker 镜像: ${DOCKER_IMAGE_TAG} (${IMAGE_ID})"
    echo "      平台: ${DOCKER_PLATFORM}"
  fi

  # 构建 app 目录结构
  APP_DIR="${BUILD_DIR}/app"
  mkdir -p "${APP_DIR}/docker"
  mkdir -p "${APP_DIR}/ui/images"

  # docker-compose.yaml（按架构注入镜像标签 + 管理员密码）
  sed "s|^\([[:space:]]*image:[[:space:]]*\).*|\1${DOCKER_IMAGE_TAG}|" \
    "${SCRIPT_DIR}/docker-compose.yaml" > "${APP_DIR}/docker/docker-compose.yaml"
  if [ -n "${ADMIN_PASSWORD}" ]; then
    # 兼容 macOS / GNU sed
    if [ "$(uname)" = "Darwin" ]; then
      sed -i '' "s|LLM_PROXY_ADMIN_PASSWORD=.*|LLM_PROXY_ADMIN_PASSWORD=${ADMIN_PASSWORD}|" \
        "${APP_DIR}/docker/docker-compose.yaml"
    else
      sed -i "s|LLM_PROXY_ADMIN_PASSWORD=.*|LLM_PROXY_ADMIN_PASSWORD=${ADMIN_PASSWORD}|" \
        "${APP_DIR}/docker/docker-compose.yaml"
    fi
  fi
  if ! grep -q "CENTAG_EDITION" "${APP_DIR}/docker/docker-compose.yaml"; then
    # 在 environment 段追加 edition（简单追加一行）
    if [ "$(uname)" = "Darwin" ]; then
      sed -i '' "/LLM_PROXY_ADMIN_PASSWORD=/a\\
      - CENTAG_EDITION=${RUNTIME_EDITION}
" "${APP_DIR}/docker/docker-compose.yaml"
    else
      sed -i "/LLM_PROXY_ADMIN_PASSWORD=/a\\      - CENTAG_EDITION=${RUNTIME_EDITION}" \
        "${APP_DIR}/docker/docker-compose.yaml"
    fi
  fi
  if [ "$EDITION" = "minimal" ]; then
    echo "[WARN] Docker 模式当前 Dockerfile 基于 dist/personal；minimal 请优先使用 --mode native"
  fi

  mkdir -p "${APP_DIR}/config"
  write_runtime_env "${APP_DIR}/config"

  # UI 图标
  if [ -f "${SCRIPT_DIR}/res/icon_64.png" ]; then
    cp "${SCRIPT_DIR}/res/icon_64.png" "${APP_DIR}/ui/images/icon_64.png"
  else
    echo "[WARN] 未找到 icon_64.png"
  fi
  if [ -f "${SCRIPT_DIR}/res/icon_256.png" ]; then
    cp "${SCRIPT_DIR}/res/icon_256.png" "${APP_DIR}/ui/images/icon_256.png"
  fi

  # UI 配置 - Docker 模式使用 .url 格式，配置 WebUI 访问入口
  cat > "${APP_DIR}/ui/config" << CONFIG_EOF
{
  ".url": {
    "centag.Application": {
      "title": "Centag",
      "icon": "images/icon_{0}.png",
      "type": "url",
      "protocol": "http",
      "port": "20060"
    }
  }
}
CONFIG_EOF

  # Docker 镜像导出为 tar 包（fnOS 安装时直接从 fpk 加载，无需 registry）
  echo "  导出 Docker 镜像..."
  docker save "${DOCKER_IMAGE_TAG}" | gzip > "${APP_DIR}/docker/image.tar.gz"
  echo "  镜像包: image.tar.gz ($(du -h "${APP_DIR}/docker/image.tar.gz" | cut -f1))"

elif [ "$MODE" = "native" ]; then
  # ----- Native 模式 -----

  APP_DIR="${BUILD_DIR}/app"
  mkdir -p "${APP_DIR}/bin"
  mkdir -p "${APP_DIR}/webui"
  mkdir -p "${APP_DIR}/scripts"
  mkdir -p "${APP_DIR}/ui/images"
  mkdir -p "${APP_DIR}/config/initdata"

  DIST_DIR="${REPO_ROOT}/dist/${DIST_NAME}"
  if [ ! -d "${DIST_DIR}" ]; then
    echo "[ERROR] 发行版目录不存在: ${DIST_DIR}"
    exit 1
  fi

  if [ "$SKIP_BUILD" = false ]; then
    echo "[1/5] 构建 Go 后端 edition=${EDITION} dist=${DIST_NAME} (${GOARCH})..."

    cd "${DIST_DIR}"
    GOOS=linux GOARCH="${GOARCH}" GOTOOLCHAIN=auto go build \
      -tags "${BUILD_TAGS}" \
      -ldflags="-s -w -X 'main.Version=v$(date +%Y%m%d-%H%M%S)' -X 'main.BuildTime=$(date +%Y-%m-%d\ %H:%M:%S)'" \
      -o "${REPO_ROOT}/bin/server/centag" .

    echo "[OK] 后端构建完成 (${GOARCH})"
    echo "      大小: $(du -h "${REPO_ROOT}/bin/server/centag" | cut -f1)"

    echo ""
    echo "[2/5] 构建前端..."
    cd "$REPO_ROOT"
    cd web/webui && npm install --silent && npm run build 2>&1 | tail -3
    cd "$REPO_ROOT"
    mkdir -p bin/server/static
    [ -d "web/dist" ] && cp -r web/dist/* bin/server/static/ 2>/dev/null || true
    echo "[OK] 前端构建完成"
  else
    echo "[SKIP] 跳过构建，使用已有的 bin/server/centag"
  fi

  echo ""
  echo "[3/5] 复制文件到 app 目录..."

  # 主二进制
  if [ -f "${REPO_ROOT}/bin/server/centag" ]; then
    cp "${REPO_ROOT}/bin/server/centag" "${APP_DIR}/bin/centag"
    chmod +x "${APP_DIR}/bin/centag"
    echo "  二进制: $(file "${APP_DIR}/bin/centag" | cut -d: -f2)"
    echo "  大小: $(du -h "${APP_DIR}/bin/centag" | cut -f1)"
  else
    echo "[ERROR] bin/server/centag 不存在，请先构建或去掉 --skip-build"
    echo "  例如: ./deploy/fnos/build-fpk.sh --mode native --edition ${EDITION} --arch ${GOARCH}"
    exit 1
  fi

  # 前端静态文件
  if [ -d "${REPO_ROOT}/bin/server/static" ]; then
    cp -r "${REPO_ROOT}/bin/server/static/"* "${APP_DIR}/webui/"
    echo "  静态文件: $(find "${APP_DIR}/webui" -type f | wc -l) 个文件"
  else
    echo "[WARN] 未找到静态文件目录 bin/server/static"
  fi

  # 初始数据：minimal 用 profile，并合并全局 common 中 minimal 可用的流水线
  # （开发时 start.sh 会合并全局+profile；fpk 只有包内 INITDATA_PATH，须自包含）
  if [ "$EDITION" = "minimal" ] && [ -d "${REPO_ROOT}/config/profiles/minimal/initdata" ]; then
    cp -r "${REPO_ROOT}/config/profiles/minimal/initdata/"* "${APP_DIR}/config/initdata/"
    mkdir -p "${APP_DIR}/config/initdata/pipeline-templates/common"
    for f in router-mode.yaml smart-scheduling.yaml; do
      src="${REPO_ROOT}/config/initdata/pipeline-templates/common/${f}"
      if [ -f "$src" ]; then
        cp "$src" "${APP_DIR}/config/initdata/pipeline-templates/common/${f}"
      fi
    done
    echo "  初始数据: profiles/minimal/initdata + common/{router-mode,smart-scheduling}"
  elif [ -d "${REPO_ROOT}/config/initdata" ]; then
    for item in "${REPO_ROOT}/config/initdata/"*; do
      base=$(basename "$item")
      if [ "$base" != "data" ] && [ "$base" != "scripts" ] && [ "$base" != "update" ]; then
        cp -r "$item" "${APP_DIR}/config/initdata/"
      fi
    done
    echo "  初始数据: $(find "${APP_DIR}/config/initdata" -type f | wc -l) 个文件"
  fi

  # 不打包预置 centag.db，确保 personal/team 首轮 seed 使用 runtime.env 密码

  # 脚本
  if [ -d "${REPO_ROOT}/config/initdata/scripts" ]; then
    cp -r "${REPO_ROOT}/config/initdata/scripts/"* "${APP_DIR}/scripts/" 2>/dev/null || true
  fi

  # update_config.yml（Native 模式需要）
  if [ -f "${REPO_ROOT}/config/initdata/update/update_config.yml" ]; then
    cp "${REPO_ROOT}/config/initdata/update/update_config.yml" "${APP_DIR}/update_config.yml"
  fi

  write_runtime_env "${APP_DIR}/config"

  # UI 配置 - Native 模式使用 native/ui-config
  if [ -f "${SCRIPT_DIR}/native/ui-config" ]; then
    cp "${SCRIPT_DIR}/native/ui-config" "${APP_DIR}/ui/config"
  elif [ -f "${SCRIPT_DIR}/ui-config" ]; then
    cp "${SCRIPT_DIR}/ui-config" "${APP_DIR}/ui/config"
  fi

else
  echo "[ERROR] 未知模式: ${MODE}，仅支持 docker / native"
  exit 1
fi

# ============================================================
# 2. 组装 fpk 顶层结构（两种模式共用）
# ============================================================
echo ""
echo "[4/5] 组装 fpk 包..."

mkdir -p "${BUILD_DIR}/cmd"
mkdir -p "${BUILD_DIR}/config"

# manifest
cp "${SCRIPT_DIR}/manifest" "${BUILD_DIR}/manifest"

# cmd 脚本（根据模式选择）
if [ "$MODE" = "docker" ]; then
  # Docker 模式使用顶级 cmd/main（简单状态回显，生命周期由 Docker 管理）
  cp "${SCRIPT_DIR}/cmd/main" "${BUILD_DIR}/cmd/main"
  chmod +x "${BUILD_DIR}/cmd/main"
  # 生命周期脚本：直接使用 native/cmd/ 下有实际内容的脚本（非空占位符）
  # fnOS 验证会检查脚本有效性，空文件会导致"不符合系统要求"
  for hook in install_init install_callback uninstall_init uninstall_callback upgrade_init upgrade_callback config_init config_callback; do
    if [ -f "${SCRIPT_DIR}/native/cmd/${hook}" ]; then
      cp "${SCRIPT_DIR}/native/cmd/${hook}" "${BUILD_DIR}/cmd/${hook}"
    else
      : > "${BUILD_DIR}/cmd/${hook}"
    fi
    chmod +x "${BUILD_DIR}/cmd/${hook}"
  done
elif [ "$MODE" = "native" ]; then
  # Native 模式使用 native 目录下的全套 cmd 脚本（已包含生命周期脚本）
  cp -r "${SCRIPT_DIR}/native/cmd/"* "${BUILD_DIR}/cmd/" 2>/dev/null || true
  chmod +x "${BUILD_DIR}/cmd/"* 2>/dev/null || true
fi

# config（privilege, resource）- Docker/默认用 base config，Native 用 native/ 下的
if [ "$MODE" = "native" ]; then
  cp "${SCRIPT_DIR}/native/config/privilege" "${BUILD_DIR}/config/privilege"
  cp "${SCRIPT_DIR}/native/config/resource" "${BUILD_DIR}/config/resource"
else
  cp "${SCRIPT_DIR}/config/privilege" "${BUILD_DIR}/config/"
  cp "${SCRIPT_DIR}/config/resource" "${BUILD_DIR}/config/"
fi

# 图标（优先公共路径，fallback 到 native 路径）
ICON_SRC="${SCRIPT_DIR}/res"
if [ ! -f "${ICON_SRC}/icon_256.png" ]; then
  ICON_SRC="${REPO_ROOT}/deploy/fnos/native/res"
fi
if [ -f "${ICON_SRC}/icon_256.png" ]; then
  cp "${ICON_SRC}/icon_256.png" "${APP_DIR}/ui/images/icon_256.png"
  cp "${ICON_SRC}/icon_256.png" "${BUILD_DIR}/ICON_256.PNG"
  echo "  图标: icon_256.png"
fi
if [ -f "${ICON_SRC}/icon_64.png" ]; then
  cp "${ICON_SRC}/icon_64.png" "${APP_DIR}/ui/images/icon_64.png"
  cp "${ICON_SRC}/icon_64.png" "${BUILD_DIR}/ICON.PNG"
  echo "  图标: icon_64.png"
fi

# ============================================================
# 3. 打包 app.tgz
# ============================================================
cd "${BUILD_DIR}"
tar czf app.tgz -C app .
rm -rf app
echo "[OK] app.tgz 创建完成 ($(du -h app.tgz | cut -f1))"

# ============================================================
# 4. 生成 checksum
# ============================================================
CHECKSUM="$(file_md5 manifest)"
if grep -q "^checksum=" manifest 2>/dev/null; then
  # 兼容 GNU sed (Linux) 和 BSD sed (macOS)
  if [ "$(uname)" = "Darwin" ]; then
    sed -i .bak "s/^checksum=.*/checksum=${CHECKSUM}/" manifest && rm -f manifest.bak
  else
    sed -i "s/^checksum=.*/checksum=${CHECKSUM}/" manifest
  fi
else
  echo "checksum=${CHECKSUM}" >> manifest
fi
echo "[OK] 清单校验和: ${CHECKSUM}"

# ============================================================
# 5. 打包为 .fpk
# ============================================================
FPK_FILE="${OUTPUT_DIR}/centag-${EDITION}-${MODE}-${GOARCH}.fpk"
rm -f "${FPK_FILE}"

cd "${BUILD_DIR}"
tar czf "${FPK_FILE}" \
  manifest app.tgz cmd/ config/ \
  $(ls ICON*.PNG 2>/dev/null | tr "\n" " ")

echo "[OK] fpk 包已生成: ${FPK_FILE}"
echo "    文件大小: $(du -h "${FPK_FILE}" | cut -f1)"

# ============================================================
# 6. 可选安装到 fnOS
# ============================================================
if [ "$INSTALL_AFTER" = true ]; then
  echo ""
  echo "正在安装到 fnOS..."
  if command -v fpkg &>/dev/null; then
    fpkg install "${FPK_FILE}" 2>&1
    echo "[OK] 安装完成"
  else
    echo "[WARN] fpkg 命令不可用，请手动安装"
    echo "       ${FPK_FILE}"
  fi
fi

# 清理
rm -rf "${BUILD_DIR}"

echo ""
echo "============================================"
echo " 打包完成！"
echo " 输出: ${FPK_FILE}"
echo "============================================"
echo ""
echo "安装方式:"
echo "  方式1: 手动安装 - 在 fnOS 应用中心上传 .fpk 文件"
echo "  方式2: 命令行安装 - fpkg install ${FPK_FILE}"
