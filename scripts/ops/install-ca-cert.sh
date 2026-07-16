#!/bin/bash

# MITM CA 证书安装脚本 (macOS)

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

BACKEND_PORT=20060
CA_CERT_FILE="centag-ca.crt"

echo "================================"
echo "MITM CA 证书安装脚本"
echo "================================"
echo ""

# 1. 下载 CA 证书
echo "1. 下载 CA 证书..."
if curl -s "http://localhost:${BACKEND_PORT}/api/v1/proxy/ca.crt" -o "$CA_CERT_FILE"; then
    echo -e "${GREEN}✓ CA 证书下载成功: $CA_CERT_FILE${NC}"
else
    echo -e "${RED}✗ CA 证书下载失败${NC}"
    echo -e "${YELLOW}请确保服务正在运行: ./start.sh run${NC}"
    exit 1
fi

# 2. 在 macOS 上安装证书
echo ""
echo "2. 安装 CA 证书到系统钥匙串..."
echo -e "${YELLOW}提示: 需要输入管理员密码${NC}"

if sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$CA_CERT_FILE"; then
    echo -e "${GREEN}✓ CA 证书已安装到系统信任库${NC}"
else
    echo -e "${RED}✗ CA 证书安装失败${NC}"
    exit 1
fi

echo ""
echo "================================"
echo "安装完成！"
echo "================================"
echo ""
echo "现在可以测试系统代理了:"
echo "  curl -v -x http://127.0.0.1:8081 https://www.baidu.com"
echo "  curl -v -x http://127.0.0.1:8081 https://api.github.com"
echo ""
echo "卸载证书（如需要）:"
echo "  sudo security delete-certificate -c \"Proxy Claw CA\" /Library/Keychains/System.keychain"
echo ""
