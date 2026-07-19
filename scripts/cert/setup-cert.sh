#!/bin/bash

# Centag 证书安装脚本
# 用于安装MITM代理的CA证书到系统
#
# DEPRECATED: 请优先使用 apps/proxyctl（centag-proxyctl enable/disable），
# 见 docs/guide/system-proxy-egress.md

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CERT_DIR="$PROJECT_ROOT/bin/server/certs"

# CA证书路径
CA_CERT="$CERT_DIR/ca.crt"
CA_KEY="$CERT_DIR/ca.key"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Centag 证书安装向导${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 检查操作系统
OS="$(uname -s)"
case "$OS" in
    Linux*)     OS_TYPE="Linux";;
    Darwin*)    OS_TYPE="macOS";;
    MINGW*)     OS_TYPE="Windows";;
    CYGWIN*)    OS_TYPE="Windows";;
    *)          OS_TYPE="Unknown";;
esac

echo -e "${GREEN}检测到操作系统: $OS_TYPE${NC}"
echo ""

# 创建证书目录
mkdir -p "$CERT_DIR"
mkdir -p "$CERT_DIR/domains"

# 检查证书是否已存在
if [ -f "$CA_CERT" ] && [ -f "$CA_KEY" ]; then
    echo -e "${YELLOW}CA证书已存在${NC}"
    echo -e "证书路径: $CA_CERT"
    echo ""
    read -p "是否重新生成证书? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${GREEN}使用现有证书${NC}"
    else
        echo -e "${BLUE}重新生成证书...${NC}"
        rm -f "$CA_CERT" "$CA_KEY"
    fi
fi

# 如果证书不存在,需要先启动服务生成
if [ ! -f "$CA_CERT" ]; then
    echo -e "${YELLOW}CA证书尚未生成${NC}"
    echo ""
    echo "请先执行以下步骤:"
    echo "1. 通过 Web 管理界面或 API 启用系统代理（system_proxy.enabled = true）"
    echo ""
    echo "2. 启动LLM Proxy服务:"
    echo "   ./start.sh run"
    echo ""
    echo "3. 服务会自动生成CA证书"
    echo ""
    exit 1
fi

echo ""
echo -e "${BLUE}开始安装CA证书到系统...${NC}"
echo ""

case "$OS_TYPE" in
    "macOS")
        echo -e "${GREEN}macOS 系统证书安装${NC}"
        echo ""

        # 方法1: 命令行安装
        echo "方法1: 使用命令行安装"
        echo "================================"

        if sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$CA_CERT"; then
            echo -e "${GREEN}✓ 证书已成功添加到系统${NC}"
        else
            echo -e "${RED}✗ 命令行安装失败${NC}"
            echo ""
            echo "方法2: 图形界面手动安装"
            echo "================================"
            echo "请按以下步骤操作:"
            echo "1. 双击文件: $CA_CERT"
            echo "2. 在'钥匙串访问'中找到'Centag CA'"
            echo "3. 双击证书,打开详情"
            echo "4. 展开'信任'选项"
            echo "5. 将'使用此证书时'设置为'始终信任'"
            echo "6. 关闭窗口,输入管理员密码"
        fi
        ;;

    "Linux")
        echo -e "${GREEN}Linux 系统证书安装${NC}"
        echo ""

        # 检测Linux发行版
        if [ -f /etc/debian_version ]; then
            DISTRO="Debian/Ubuntu"
        elif [ -f /etc/redhat-release ]; then
            DISTRO="RedHat/CentOS/Fedora"
        elif [ -f /etc/arch-release ]; then
            DISTRO="Arch"
        else
            DISTRO="Unknown"
        fi

        echo "检测到发行版: $DISTRO"
        echo ""

        case "$DISTRO" in
            "Debian/Ubuntu")
                echo "安装到Debian/Ubuntu系统..."
                sudo cp "$CA_CERT" /usr/local/share/ca-certificates/centag-ca.crt
                sudo update-ca-certificates
                echo -e "${GREEN}✓ 证书已成功安装${NC}"
                echo "已运行: sudo update-ca-certificates"
                ;;
            "RedHat/CentOS/Fedora")
                echo "安装到RedHat/CentOS/Fedora系统..."
                sudo cp "$CA_CERT" /etc/pki/ca-trust/source/anchors/centag-ca.crt
                sudo update-ca-trust
                echo -e "${GREEN}✓ 证书已成功安装${NC}"
                echo "已运行: sudo update-ca-trust"
                ;;
            "Arch")
                echo "安装到Arch系统..."
                sudo cp "$CA_CERT" /etc/ca-certificates/trust-source/anchors/centag-ca.crt
                sudo trust extract-compat
                echo -e "${GREEN}✓ 证书已成功安装${NC}"
                ;;
            *)
                echo -e "${YELLOW}未知的Linux发行版${NC}"
                echo "请手动将证书添加到系统的CA证书存储中"
                echo "证书文件: $CA_CERT"
                ;;
        esac
        ;;

    "Windows")
        echo -e "${GREEN}Windows 系统证书安装${NC}"
        echo ""
        echo "请按以下步骤操作:"
        echo "================================"
        echo "1. 双击文件: $CA_CERT"
        echo "2. 点击'安装证书'"
        echo "3. 选择'本地计算机'(需要管理员权限)"
        echo "4. 选择'将所有证书放入下列存储'"
        echo "5. 浏览到'受信任的根证书颁发机构'"
        echo "6. 点击'完成'"
        echo ""
        echo -e "${YELLOW}注意: 安装后可能需要重启浏览器${NC}"
        ;;

    *)
        echo -e "${RED}未知的操作系统类型${NC}"
        echo "请手动安装证书: $CA_CERT"
        ;;
esac

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}证书安装完成!${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "下一步操作:"
echo "1. 启动LLM Proxy服务: ./start.sh run"
echo "2. 配置系统代理使用PAC文件:"
echo "   http://127.0.0.1:20060/api/v1/proxy/pac"
echo ""
echo "或者直接下载CA证书:"
echo "   http://127.0.0.1:20060/api/v1/proxy/ca.crt"
echo ""
