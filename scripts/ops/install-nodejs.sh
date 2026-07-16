#!/bin/bash
#
# Install Node.js via nvm (non-interactive).
# - Installs nvm to ~/.nvm if missing
# - Installs Node.js v24 if missing
# - Ensures npm is available
# - Optionally installs pnpm globally
#
# Notes:
# - This script is designed to be safe to run multiple times.
# - It does NOT modify your shell rc files beyond what nvm's official installer does.

set -euo pipefail

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
BLUE=$'\033[0;34m'
NC=$'\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

NODE_VERSION_DEFAULT="24"
NVM_VERSION_DEFAULT="v0.40.3"

usage() {
  cat <<'EOF'
用法:
  scripts/ops/install-nodejs.sh [--node-version <major>] [--with-pnpm|--no-pnpm]

示例:
  scripts/ops/install-nodejs.sh
  scripts/ops/install-nodejs.sh --node-version 24 --with-pnpm
EOF
}

NODE_VERSION="$NODE_VERSION_DEFAULT"
WITH_PNPM="true"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --node-version)
      NODE_VERSION="${2:-}"
      shift 2
      ;;
    --with-pnpm)
      WITH_PNPM="true"
      shift
      ;;
    --no-pnpm)
      WITH_PNPM="false"
      shift
      ;;
    *)
      print_error "未知参数: $1"
      usage
      exit 2
      ;;
  esac
done

if ! command -v curl >/dev/null 2>&1; then
  print_error "curl 未安装，无法自动安装 nvm"
  print_warn "请先安装 curl（macOS: brew install curl / 系统自带；Debian/Ubuntu: apt-get install -y curl）"
  exit 1
fi

export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"

load_nvm() {
  if [ -s "$NVM_DIR/nvm.sh" ]; then
    # shellcheck disable=SC1090
    . "$NVM_DIR/nvm.sh"
    return 0
  fi
  return 1
}

install_nvm_if_needed() {
  if load_nvm; then
    return 0
  fi

  print_info "未检测到 nvm，开始安装 nvm (${NVM_VERSION_DEFAULT}) ..."
  curl -o- "https://raw.githubusercontent.com/nvm-sh/nvm/${NVM_VERSION_DEFAULT}/install.sh" | bash

  if ! load_nvm; then
    print_error "nvm 安装完成但加载失败：$NVM_DIR/nvm.sh 不存在或不可读"
    print_warn "请重新打开一个终端，或手动执行: . \"$HOME/.nvm/nvm.sh\""
    exit 1
  fi

  print_success "nvm 安装并加载成功"
}

install_node_if_needed() {
  if command -v node >/dev/null 2>&1; then
    print_success "已存在 Node.js: $(node -v)"
    return 0
  fi

  print_info "开始安装 Node.js v${NODE_VERSION} ..."
  nvm install "${NODE_VERSION}"
  nvm use "${NODE_VERSION}"

  if ! command -v node >/dev/null 2>&1; then
    print_error "Node.js 安装后仍未出现在 PATH 中"
    exit 1
  fi
  print_success "Node.js 安装成功: $(node -v)"
}

ensure_npm() {
  if command -v npm >/dev/null 2>&1; then
    print_success "npm 可用: $(npm -v)"
    return 0
  fi
  print_error "npm 不可用（正常情况下随 Node.js 一起安装）"
  exit 1
}

install_pnpm_if_enabled() {
  if [ "$WITH_PNPM" != "true" ]; then
    return 0
  fi

  if command -v pnpm >/dev/null 2>&1; then
    print_success "pnpm 已安装: $(pnpm -v)"
    return 0
  fi

  print_info "安装 pnpm（全局）..."
  if npm install -g pnpm; then
    print_success "pnpm 安装成功: $(pnpm -v)"
    return 0
  fi

  print_warn "pnpm 安装失败（不影响使用 npm 构建 webui）。如确实需要 pnpm，请先修正 npm 代理/registry 配置后重试。"
  return 0
}

main() {
  print_info "========================================"
  print_info "Node.js Auto Installer (nvm-based)"
  print_info "========================================"

  install_nvm_if_needed
  install_node_if_needed
  ensure_npm
  install_pnpm_if_enabled

  print_success "========================================"
  print_success "Node.js 环境已就绪"
  print_success "========================================"
  print_info "node: $(node -v)"
  print_info "npm:  $(npm -v)"
  if command -v pnpm >/dev/null 2>&1; then
    print_info "pnpm: $(pnpm -v)"
  fi
}

main

