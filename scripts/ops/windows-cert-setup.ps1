# Proxy Claw CA证书安装脚本 for Windows
# 此脚本自动下载并安装LLM Proxy的CA证书到Windows系统受信任的根证书颁发机构

param(
    [string]$ProxyUrl = "http://127.0.0.1:20060",
    [switch]$Force,
    [switch]$Verify,
    [switch]$Remove,
    [switch]$Help
)

# 显示帮助信息
if ($Help) {
    Write-Host @"
Proxy Claw CA证书安装脚本

用法:
    .\windows-cert-setup.ps1 [选项]

选项:
    -ProxyUrl <url>    指定LLM Proxy服务地址 (默认: http://127.0.0.1:20060)
    -Force             强制重新安装证书(即使已存在)
    -Verify            验证证书是否已正确安装
    -Remove            移除已安装的证书
    -Help              显示此帮助信息

示例:
    # 安装证书
    .\windows-cert-setup.ps1

    # 安装证书(自定义服务地址)
    .\windows-cert-setup.ps1 -ProxyUrl http://192.168.1.100:20060

    # 验证证书
    .\windows-cert-setup.ps1 -Verify

    # 移除证书
    .\windows-cert-setup.ps1 -Remove

注意:
    - 需要管理员权限运行
    - 安装后需要重启应用程序(如浏览器、Cherry Studio)才能生效
"@
    exit 0
}

# 检查管理员权限
function Test-Administrator {
    $user = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal $user
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# 颜色输出函数
function Write-ColorOutput {
    param([string]$Message, [string]$Color = "White")
    Write-Host $Message -ForegroundColor $Color
}

# 错误输出
function Write-Error-Message {
    param([string]$Message)
    Write-ColorOutput "错误: $Message" "Red"
}

# 成功输出
function Write-Success-Message {
    param([string]$Message)
    Write-ColorOutput "成功: $Message" "Green"
}

# 警告输出
function Write-Warning-Message {
    param([string]$Message)
    Write-ColorOutput "警告: $Message" "Yellow"
}

# 信息输出
function Write-Info-Message {
    param([string]$Message)
    Write-ColorOutput "信息: $Message" "Cyan"
}

# 查找证书
function Find-LLMProxyCert {
    try {
        $certs = Get-ChildItem -Path Cert:\LocalMachine\Root | Where-Object {
            $_.Subject -like "*Proxy Claw CA*" -or $_.Issuer -like "*Proxy Claw CA*"
        }
        return $certs
    } catch {
        Write-Error-Message "查找证书时出错: $_"
        return $null
    }
}

# 验证证书
function Verify-Certificate {
    Write-Info-Message "正在验证证书安装状态..."
    
    $certs = Find-LLMProxyCert
    
    if ($null -eq $certs -or $certs.Count -eq 0) {
        Write-Warning-Message "未找到 Proxy Claw CA 证书"
        Write-Host "证书状态: " -NoNewline
        Write-ColorOutput "未安装" "Red"
        return $false
    }
    
    Write-Success-Message "找到 Proxy Claw CA 证书:"
    foreach ($cert in $certs) {
        Write-Host ""
        Write-Host "  证书主题: $($cert.Subject)"
        Write-Host "  颁发者:   $($cert.Issuer)"
        Write-Host "  序列号:   $($cert.SerialNumber)"
        Write-Host "  有效期:   $($cert.NotBefore) 至 $($cert.NotAfter)"
        Write-Host "  指纹:     $($cert.Thumbprint)"
        
        # 检查证书是否过期
        $now = Get-Date
        if ($cert.NotAfter -lt $now) {
            Write-Warning-Message "  警告: 证书已过期!"
        } elseif ($cert.NotBefore -gt $now) {
            Write-Warning-Message "  警告: 证书尚未生效!"
        } else {
            $daysLeft = ($cert.NotAfter - $now).Days
            Write-ColorOutput "  状态: 有效 (剩余 $daysLeft 天)" "Green"
        }
    }
    
    Write-Host ""
    Write-Success-Message "证书验证完成"
    return $true
}

# 移除证书
function Remove-Certificate {
    Write-Info-Message "正在移除 Proxy Claw CA 证书..."
    
    $certs = Find-LLMProxyCert
    
    if ($null -eq $certs -or $certs.Count -eq 0) {
        Write-Warning-Message "未找到需要移除的证书"
        return
    }
    
    foreach ($cert in $certs) {
        try {
            Write-Host "移除证书: $($cert.Subject)"
            Remove-Item -Path "Cert:\LocalMachine\Root\$($cert.Thumbprint)" -Force
            Write-Success-Message "证书已移除"
        } catch {
            Write-Error-Message "移除证书失败: $_"
        }
    }
}

# 下载并安装证书
function Install-Certificate {
    param([bool]$ForceReinstall = $false)
    
    # 检查证书是否已存在
    if (-not $ForceReinstall) {
        $existingCerts = Find-LLMProxyCert
        if ($null -ne $existingCerts -and $existingCerts.Count -gt 0) {
            Write-Warning-Message "证书已安装"
            Write-Host "如需重新安装,请使用 -Force 参数"
            return
        }
    }
    
    # 构建证书下载URL
    $certUrl = "$ProxyUrl/api/v1/proxy/ca.crt"
    $tempCertPath = Join-Path $env:TEMP "proxyclaw-ca.crt"
    
    Write-Info-Message "正在从 $certUrl 下载证书..."
    
    try {
        # 下载证书
        Invoke-WebRequest -Uri $certUrl -OutFile $tempCertPath -UseBasicParsing
        Write-Success-Message "证书下载成功"
    } catch {
        Write-Error-Message "下载证书失败: $_"
        Write-Host ""
        Write-Host "请确保:"
        Write-Host "  1. Proxy Claw 服务正在运行"
        Write-Host "  2. 服务地址正确: $ProxyUrl"
        Write-Host "  3. 系统代理功能已启用"
        exit 1
    }
    
    # 检查证书文件
    if (-not (Test-Path $tempCertPath)) {
        Write-Error-Message "证书文件不存在: $tempCertPath"
        exit 1
    }
    
    Write-Info-Message "正在安装证书到受信任的根证书颁发机构..."
    
    try {
        # 导入证书到本地计算机的受信任根证书颁发机构
        $cert = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2
        $cert.Import($tempCertPath)
        
        $store = New-Object System.Security.Cryptography.X509Certificates.X509Store(
            [System.Security.Cryptography.X509Certificates.StoreName]::Root,
            [System.Security.Cryptography.X509Certificates.StoreLocation]::LocalMachine
        )
        
        $store.Open([System.Security.Cryptography.X509Certificates.OpenFlags]::ReadWrite)
        $store.Add($cert)
        $store.Close()
        
        Write-Success-Message "证书安装成功!"
        Write-Host ""
        Write-Host "证书信息:"
        Write-Host "  主题:   $($cert.Subject)"
        Write-Host "  颁发者: $($cert.Issuer)"
        Write-Host "  有效期: $($cert.NotBefore) 至 $($cert.NotAfter)"
        Write-Host ""
        Write-ColorOutput "重要提示:" "Yellow"
        Write-Host "  1. 请重启浏览器或应用程序(如 Cherry Studio)使证书生效"
        Write-Host "  2. 如果仍然无法连接,请检查应用程序的代理设置"
        Write-Host "  3. 确保在 Proxy Claw WebUI 中添加了目标域名到 PAC 规则"
        
    } catch {
        Write-Error-Message "安装证书失败: $_"
        exit 1
    } finally {
        # 清理临时文件
        if (Test-Path $tempCertPath) {
            Remove-Item $tempCertPath -Force
        }
    }
}

# 主程序
Write-Host ""
Write-ColorOutput "========================================" "Cyan"
Write-ColorOutput "  Proxy Claw CA证书管理工具" "Cyan"
Write-ColorOutput "========================================" "Cyan"
Write-Host ""

# 检查管理员权限
if (-not (Test-Administrator)) {
    Write-Error-Message "此脚本需要管理员权限运行"
    Write-Host ""
    Write-Host "请以管理员身份运行 PowerShell,然后重新执行此脚本:"
    Write-Host "  1. 右键点击 PowerShell"
    Write-Host "  2. 选择 '以管理员身份运行'"
    Write-Host "  3. 执行: .\windows-cert-setup.ps1"
    Write-Host ""
    exit 1
}

# 根据参数执行相应操作
if ($Verify) {
    Verify-Certificate
} elseif ($Remove) {
    Remove-Certificate
} else {
    Install-Certificate -ForceReinstall $Force
}

Write-Host ""
Write-ColorOutput "========================================" "Cyan"
Write-Host ""
