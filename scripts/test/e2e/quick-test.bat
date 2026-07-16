@echo off
chcp 65001 >nul

REM 快速测试脚本 - 验证代理是否工作

echo.
echo ========================================
echo   Centag 快速测试
echo ========================================
echo.

set PROXY=http://127.0.0.1:8081
set TEST_URL=https://api.openai.com/v1/models

echo 测试配置:
echo   代理地址: %PROXY%
echo   测试URL: %TEST_URL%
echo.
echo 正在测试...
echo.

REM 设置代理环境变量
set "http_proxy=%PROXY%"
set "https_proxy=%PROXY%"

REM 执行测试
curl -v -x %PROXY% %TEST_URL% -m 10 2>&1 | findstr /C:"Connected" /C:"HTTP" /C:"SSL" /C:"error"

if %errorlevel% eq 0 (
    echo.
    echo [✓] 代理连接成功!
    echo.
    echo Cherry Studio 配置建议:
    echo   代理类型: HTTP
    echo   主机: 127.0.0.1
    echo   端口: 8081
) else (
    echo.
    echo [✗] 代理连接失败
    echo.
    echo 请检查:
    echo   1. Centag 服务是否运行
    echo   2. 系统代理功能是否启用
    echo   3. CA 证书是否已安装
    echo   4. 域名是否添加到 PAC 规则
    echo.
    echo 运行诊断: windows-proxy-diagnose.bat
)

echo.
pause
