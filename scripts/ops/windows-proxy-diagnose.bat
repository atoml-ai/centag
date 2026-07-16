@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

REM Proxy Claw 代理诊断工具
REM 此脚本会检查代理配置和证书状态

echo.
echo ========================================
echo   Proxy Claw 代理诊断工具
echo ========================================
echo.

REM 配置
set PROXY_HOST=127.0.0.1
set PROXY_PORT=8081
set API_PORT=8060
set TEST_DOMAIN=api.openai.com

echo [步骤 1/6] 检查 Proxy Claw 主服务...
echo.

REM 检查主服务
curl -s -m 5 http://%PROXY_HOST%:%API_PORT%/health >nul 2>&1
if %errorlevel% eq 0 (
    echo [✓] 主服务运行正常 (端口 %API_PORT%)
) else (
    echo [✗] 主服务未运行或无法访问
    echo     请启动 Proxy Claw 服务
    goto :end
)

echo.
echo [步骤 2/6] 检查 MITM 代理服务...
echo.

REM 检查MITM代理端口
netstat -an | findstr ":%PROXY_PORT% " >nul 2>&1
if %errorlevel% eq 0 (
    echo [✓] MITM 代理服务运行正常 (端口 %PROXY_PORT%)
) else (
    echo [✗] MITM 代理服务未运行
    echo     请在 WebUI 中启用系统代理功能
    goto :end
)

echo.
echo [步骤 3/6] 检查 CA 证书...
echo.

REM 检查证书是否安装
certutil -store Root "Proxy Claw CA" >nul 2>&1
if %errorlevel% eq 0 (
    echo [✓] CA 证书已安装
    echo.
    echo 证书详情:
    certutil -store Root "Proxy Claw CA" | findstr /C:"Subject" /C:"NotBefore" /C:"NotAfter"
) else (
    echo [✗] CA 证书未安装
    echo     请运行 windows-cert-install.bat 安装证书
    goto :end
)

echo.
echo [步骤 4/6] 检查 PAC 配置...
echo.

REM 获取PAC配置
curl -s -m 5 http://%PROXY_HOST%:%API_PORT%/api/v1/proxy/status > "%TEMP%\pac_status.json" 2>nul
if %errorlevel% eq 0 (
    echo [✓] PAC 配置可访问
    echo.
    echo PAC 域名列表:
    type "%TEMP%\pac_status.json" | findstr "pac_domains"
    del "%TEMP%\pac_status.json" >nul 2>&1
) else (
    echo [✗] 无法获取 PAC 配置
)

echo.
echo [步骤 5/6] 测试代理连接...
echo.

echo 测试通过代理访问: %TEST_DOMAIN%
echo 代理地址: http://%PROXY_HOST%:%PROXY_PORT%
echo.

REM 使用代理测试连接
set "http_proxy=http://%PROXY_HOST%:%PROXY_PORT%"
set "https_proxy=http://%PROXY_HOST%:%PROXY_PORT%"

curl -v -x http://%PROXY_HOST%:%PROXY_PORT% https://%TEST_DOMAIN%/v1/models -m 10 2>&1 | findstr /C:"Connected" /C:"SSL" /C:"TLS" /C:"error" /C:"failed"

if %errorlevel% eq 0 (
    echo.
    echo [✓] 代理连接测试完成
) else (
    echo.
    echo [!] 无法完成测试或出现错误
)

echo.
echo [步骤 6/6] 系统代理设置检查...
echo.

REM 检查系统代理设置
reg query "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyEnable 2>nul | findstr "0x1" >nul
if %errorlevel% eq 0 (
    echo [!] 系统代理已启用
    reg query "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyServer 2>nul
) else (
    echo [✓] 系统代理未启用 (应用程序应手动配置代理)
)

echo.
echo ========================================
echo   诊断完成
echo ========================================
echo.
echo 如果上述检查都通过,但 Cherry Studio 仍然无法使用:
echo.
echo 1. 确保 Cherry Studio 代理设置正确:
echo    - 代理类型: HTTP/HTTPS (不要用 SOCKS5)
echo    - 主机: %PROXY_HOST%
echo    - 端口: %PROXY_PORT%
echo.
echo 2. 在 WebUI 的"系统代理管理"中添加目标域名:
echo    - 例如: api.openai.com, api.anthropic.com
echo.
echo 3. 重启 Cherry Studio
echo.
echo 4. 检查 Proxy Claw 日志:
echo    - 位置: ./bin/logs/proxyclaw.log
echo    - 查找: TLS handshake failed
echo.
echo 5. 如果看到 "unknown certificate" 错误:
echo    - 可能需要在 Cherry Studio 设置中禁用SSL验证
echo    - 或者完全重启 Windows 系统
echo.

:end
echo 按任意键退出...
pause >nul
