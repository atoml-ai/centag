#!/bin/bash
# =============================================================
# Centag fnOS (.fpk) 统一打包脚本
#
# 支持 Docker 和 Native 两种模式:
#
#   Docker 模式：
#     ./deploy/fnos/build-fpk.sh --mode docker             # Docker 打包
#     ./deploy/fnos/build-fpk.sh --mode docker --arch amd64              # Docker 打包 amd64
#     ./deploy/fnos/build-fpk.sh --mode docker --arch arm64              # Docker 打包 arm64
#     ./deploy/fnos/build-fpk.sh --mode docker --arch arm64 --image-prefix ghcr.io/marmotcai/  # 指定镜像前缀
#     ./deploy/fnos/build-fpk.sh --mode docker --install   # 打包并安装
#
#   Native 模式：
#     ./deploy/fnos/build-fpk.sh --mode native                          # native 打包
#     ./deploy/fnos/build-fpk.sh --mode native --arch amd64              # 交叉编译 amd64
#     ./deploy/fnos/build-fpk.sh --mode native --arch arm64              # 交叉编译 arm64
#     ./deploy/fnos/build-fpk.sh --mode native --skip-build              # 跳过构建，使用已有二进制
#     ./deploy/fnos/build-fpk.sh --mode native --skip-build --install    # 跳过构建 + 安装
#
# 通用参数:
#     --output DIR       指定输出目录（默认 dist/）
#     --image-prefix P   镜像名前缀（如 ghcr.io/marmotcai/，为空则不加前缀）
#     --install          打包后自动安装到 fnOS
#     --help             显示帮助
# =============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BUILD_DIR=$(mktemp -d /tmp/centag-fpk-XXXXXX)
OUTPUT_DIR="${REPO_ROOT}/dist"
IMAGE_PREFIX="ghcr.io/marmotcai/"

# 模式参数
MODE="docker"           # docker | native
ARCH="$(uname -m)"      # 当前架构（默认）
SKIP_BUILD=false
INSTALL_AFTER=false

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
  sed -n '2,26p' "$0" | sed 's/^# \{0,1\}//'
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

mkdir -p "$OUTPUT_DIR"

echo "============================================"
echo " Centag fnOS 打包工具"
echo "============================================"
echo "模式:      ${MODE}"
echo "架构:      ${GOARCH}"
echo "源码目录:  ${REPO_ROOT}"
echo "构建目录:  ${BUILD_DIR}"
echo "输出目录:  ${OUTPUT_DIR}"
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

  # docker-compose.yaml（按架构注入镜像标签）
  sed "s|^\([[:space:]]*image:[[:space:]]*\).*|\1${DOCKER_IMAGE_TAG}|" \
    "${SCRIPT_DIR}/docker-compose.yaml" > "${APP_DIR}/docker/docker-compose.yaml"

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

  if [ "$SKIP_BUILD" = false ]; then
    echo "[1/5] 构建 Go 后端 (${GOARCH})..."

    cd "$REPO_ROOT"
    GOOS=linux GOARCH="${GOARCH}" GOTOOLCHAIN=auto go build \
      -ldflags="-s -w -X 'main.Version=v$(date +%Y%m%d-%H%M%S)' -X 'main.BuildTime=$(date +%Y-%m-%d\ %H:%M:%S)'" \
      -o bin/server/centag cmd/centag/main.go

    echo "[OK] 后端构建完成 (${GOARCH})"
    echo "      大小: $(du -h bin/server/centag | cut -f1)"

    echo ""
    echo "[2/5] 构建前端..."
    cd "$REPO_ROOT"
    cd web/webui && npm install --silent && npm run build 2>&1 | tail -3
    cd "$REPO_ROOT"
    mkdir -p bin/server/static
    [ -d "web/dist" ] && cp -r web/dist/* bin/server/static/ 2>/dev/null || true
    echo "[OK] 前端构建完成"
  else
    echo "[SKIP] 跳过构建，使用已有的 bin/server/ 目录"
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
    echo "[ERROR] bin/server/centag 不存在，请先构建或使用 --skip-build"
    echo "  构建: go build -o bin/server/centag cmd/centag/main.go"
    exit 1
  fi

  # 前端静态文件
  if [ -d "${REPO_ROOT}/bin/server/static" ]; then
    cp -r "${REPO_ROOT}/bin/server/static/"* "${APP_DIR}/webui/"
    echo "  静态文件: $(find "${APP_DIR}/webui" -type f | wc -l) 个文件"
  else
    echo "[WARN] 未找到静态文件目录 bin/server/static"
  fi

  # 初始数据（pipeline-templates, initial-backends.yaml, rule, secrets 等）
  if [ -d "${REPO_ROOT}/config/initdata" ]; then
    for item in "${REPO_ROOT}/config/initdata/"*; do
      base=$(basename "$item")
      if [ "$base" != "data" ] && [ "$base" != "scripts" ] && [ "$base" != "update" ]; then
        cp -r "$item" "${APP_DIR}/config/initdata/"
      fi
    done
    echo "  初始数据: $(find "${APP_DIR}/config/initdata" -type f | wc -l) 个文件"
  fi

  # 初始数据库
  if [ -f "${REPO_ROOT}/config/initdata/data/centag.db" ]; then
    mkdir -p "${APP_DIR}/storage"
    cp "${REPO_ROOT}/config/initdata/data/centag.db" "${APP_DIR}/storage/centag.db"
    echo "  初始数据库: centag.db"
  fi

  # 脚本
  if [ -d "${REPO_ROOT}/config/initdata/scripts" ]; then
    cp -r "${REPO_ROOT}/config/initdata/scripts/"* "${APP_DIR}/scripts/" 2>/dev/null || true
  fi

  # update_config.yml（Native 模式需要）
  if [ -f "${REPO_ROOT}/config/initdata/update/update_config.yml" ]; then
    cp "${REPO_ROOT}/config/initdata/update/update_config.yml" "${APP_DIR}/update_config.yml"
  fi

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
CHECKSUM=$(md5sum manifest | cut -d" " -f1)
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
FPK_FILE="${OUTPUT_DIR}/centag-${MODE}-${GOARCH}.fpk"
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
