#!/bin/bash

# Centag CA证书安装脚本 for Linux/WSL
# 此脚本会下载CA证书并提供在Windows中安装的说明

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
PROXY_URL="${PROXY_URL:-http://127.0.0.1:20060}"
CERT_URL="$PROXY_URL/api/v1/proxy/ca.crt"
CERT_FILE="centag-ca.crt"

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Centag CA证书管理工具 (Linux/WSL)${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 检查服务是否运行
check_service() {
    echo -e "${BLUE}[1/4] 检查 Centag 服务...${NC}"
    if curl -s -m 5 "$PROXY_URL/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ 服务运行正常${NC}"
        return 0
    else
        echo -e "${RED}✗ 无法连接到服务: $PROXY_URL${NC}"
        echo ""
        echo "请确保:"
        echo "  1. Centag 服务正在运行"
        echo "  2. 服务地址正确"
        echo ""
        return 1
    fi
}

# 下载证书
download_cert() {
    echo ""
    echo -e "${BLUE}[2/4] 下载 CA 证书...${NC}"
    
    if curl -s -o "$CERT_FILE" "$CERT_URL"; then
        if [ -f "$CERT_FILE" ] && [ -s "$CERT_FILE" ]; then
            echo -e "${GREEN}✓ 证书下载成功: $CERT_FILE${NC}"
            
            # 显示证书信息
            echo ""
            echo "证书信息:"
            openssl x509 -in "$CERT_FILE" -noout -subject -issuer -dates 2>/dev/null || true
            return 0
        else
            echo -e "${RED}✗ 下载的证书文件无效${NC}"
            return 1
        fi
    else
        echo -e "${RED}✗ 下载证书失败${NC}"
        return 1
    fi
}

# 复制到 Windows 下载目录
copy_to_windows() {
    echo ""
    echo -e "${BLUE}[3/4] 复制证书到 Windows...${NC}"
    
    # 尝试找到 Windows 用户下载目录
    WIN_USER=$(cmd.exe /c "echo %USERNAME%" 2>/dev/null | tr -d '\r' || echo "")
    
    if [ -n "$WIN_USER" ]; then
        WIN_DOWNLOADS="/mnt/c/Users/$WIN_USER/Downloads"
        if [ -d "$WIN_DOWNLOADS" ]; then
            cp "$CERT_FILE" "$WIN_DOWNLOADS/"
            echo -e "${GREEN}✓ 证书已复制到: $WIN_DOWNLOADS/$CERT_FILE${NC}"
            echo -e "${YELLOW}提示: 您可以在 Windows 下载文件夹中找到此证书${NC}"
            return 0
        fi
    fi
    
    echo -e "${YELLOW}! 无法自动复制到 Windows 下载目录${NC}"
    echo "证书文件位置: $(pwd)/$CERT_FILE"
    return 0
}

# 显示安装说明
show_instructions() {
    echo ""
    echo -e "${BLUE}[4/4] Windows 安装说明${NC}"
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  证书下载完成！${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo -e "${YELLOW}接下来在 Windows 中安装证书:${NC}"
    echo ""
    echo "方法 1: 使用文件资源管理器"
    echo "  1. 打开 Windows 文件资源管理器"
    echo "  2. 在下载文件夹中找到 $CERT_FILE"
    echo "  3. 双击该文件"
    echo "  4. 点击 '安装证书...'"
    echo "  5. 选择 '本地计算机' (需要管理员权限)"
    echo "  6. 选择 '将所有的证书都放入下列存储'"
    echo "  7. 点击 '浏览...'，选择 '受信任的根证书颁发机构'"
    echo "  8. 点击 '确定' → '下一步' → '完成'"
    echo ""
    echo "方法 2: 使用 certutil 命令"
    echo "  1. 以管理员身份打开 PowerShell"
    echo "  2. 运行命令:"
    WIN_USER=$(cmd.exe /c "echo %USERNAME%" 2>/dev/null | tr -d '\r' || echo "用户名")
    echo "     certutil -addstore -f Root C:\\Users\\$WIN_USER\\Downloads\\$CERT_FILE"
    echo ""
    echo "方法 3: 在 Windows 中运行批处理脚本"
    echo "  1. 在 Windows 文件资源管理器中打开:"
    echo "     \\\\wsl.localhost\\Ubuntu$(pwd | sed 's|/mnt/c|C:|' | sed 's|/|\\|g')"
    echo "  2. 右键点击 windows-cert-install.bat"
    echo "  3. 选择 '以管理员身份运行'"
    echo ""
    echo -e "${YELLOW}重要提示:${NC}"
    echo "  • 安装证书需要管理员权限"
    echo "  • 安装完成后，重启 Chatbox 或 Cherry Studio"
    echo "  • 确保在 WebUI 中添加了目标域名到 PAC 规则"
    echo ""
    echo -e "${BLUE}配置 Chatbox/Cherry Studio:${NC}"
    echo "  代理类型: HTTP"
    echo "  主机: 127.0.0.1"
    echo "  端口: 8081"
    echo ""
}

# 清理函数
cleanup() {
    if [ -f "$CERT_FILE" ] && [ "$1" != "keep" ]; then
        rm -f "$CERT_FILE"
    fi
}

# 主流程
main() {
    # 检查服务
    if ! check_service; then
        exit 1
    fi
    
    # 下载证书
    if ! download_cert; then
        cleanup
        exit 1
    fi
    
    # 复制到 Windows
    copy_to_windows
    
    # 显示安装说明
    show_instructions
    
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo ""
    echo "证书文件保存在当前目录: $(pwd)/$CERT_FILE"
    echo ""
}

# 运行主流程
main

# 不删除证书文件，保留给用户使用
trap 'cleanup keep' EXIT
