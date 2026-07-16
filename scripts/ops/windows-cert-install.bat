@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

REM Proxy Claw CA证书一键安装脚本
REM 此脚本会自动下载并安装CA证书到Windows系统

echo.
echo ========================================
echo   Proxy Claw CA证书安装工具
echo ========================================
echo.

REM 检查管理员权限
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo [错误] 此脚本需要管理员权限运行
    echo.
    echo 请右键点击此文件,选择"以管理员身份运行"
    echo.
    pause
    exit /b 1
)

REM 配置
set PROXY_URL=http://127.0.0.1:20060
set CERT_URL=%PROXY_URL%/api/v1/proxy/ca.crt
set TEMP_CERT=%TEMP%\proxyclaw-ca.crt

echo [信息] 正在从 %CERT_URL% 下载证书...
echo.

REM 下载证书
powershell -Command "try { Invoke-WebRequest -Uri '%CERT_URL%' -OutFile '%TEMP_CERT%' -UseBasicParsing; exit 0 } catch { Write-Host '[错误] 下载失败:' $_.Exception.Message; exit 1 }"

if %errorlevel% neq 0 (
    echo.
    echo [错误] 下载证书失败
    echo.
    echo 可能的原因:
    echo   1. Proxy Claw 服务未运行
    echo   2. 服务地址不正确
    echo   3. 系统代理功能未启用
    echo.
    echo 请检查后重试
    pause
    exit /b 1
)

echo [成功] 证书下载完成
echo.

REM 检查证书文件
if not exist "%TEMP_CERT%" (
    echo [错误] 证书文件不存在: %TEMP_CERT%
    pause
    exit /b 1
)

echo [信息] 正在安装证书到受信任的根证书颁发机构...
echo.

REM 使用certutil安装证书
certutil -addstore -f "Root" "%TEMP_CERT%" >nul 2>&1

if %errorlevel% neq 0 (
    echo [错误] 证书安装失败
    echo.
    echo 请尝试手动安装:
    echo   1. 打开 certmgr.msc
    echo   2. 导航到: 受信任的根证书颁发机构 ^> 证书
    echo   3. 右键 ^> 所有任务 ^> 导入
    echo   4. 选择文件: %TEMP_CERT%
    echo.
    pause
    exit /b 1
)

echo [成功] 证书安装完成!
echo.

REM 验证证书
echo [信息] 正在验证证书...
certutil -store Root "Proxy Claw CA" >nul 2>&1

if %errorlevel% eq 0 (
    echo [成功] 证书验证通过
    echo.
    certutil -store Root "Proxy Claw CA" | findstr /C:"Subject" /C:"Issuer" /C:"NotBefore" /C:"NotAfter"
) else (
    echo [警告] 无法验证证书,但可能已成功安装
)

echo.
echo ========================================
echo   安装完成!
echo ========================================
echo.
echo 重要提示:
echo   1. 请重启浏览器或应用程序(如 Cherry Studio)
echo   2. 在 Cherry Studio 中配置代理:
echo      - 代理地址: 127.0.0.1
echo      - 代理端口: 8081
echo   3. 确保在 WebUI 中添加了目标域名
echo.

REM 清理临时文件
if exist "%TEMP_CERT%" (
    del /f /q "%TEMP_CERT%" >nul 2>&1
)

echo 按任意键退出...
pause >nul
