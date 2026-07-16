# Scripts

运维脚本目录，按功能分类管理。

## 目录结构

```
scripts/
├── README.md           # 本文件
├── db/                 # 数据库脚本
│   ├── db-migrate.sh
│   └── init-postgres.sh
├── deploy/docker/             # Docker 脚本
│   └── docker-run.sh
├── cert/               # 证书脚本
│   ├── llm-proxy-ca.crt
│   ├── linux-cert-download.sh
│   └── setup-cert.sh
├── test/               # 测试脚本
│   ├── quick-test.bat
│   ├── test-cache-scenarios.sh
│   ├── test-chatbox-direct.sh
│   ├── test-chatbox-proxy.sh
│   ├── verify-daemon-logs.sh
│   └── verify-utf8-fix.sh
└── ops/                # 运维脚本
    ├── clean-macos-files.sh
    ├── generate-secrets.sh
    ├── load-secrets.sh
    ├── llmproxy.sh
    ├── windows-cert-install.bat
    ├── windows-cert-setup.ps1
    └── windows-proxy-diagnose.bat
```

## 快速参考

### 构建

```bash
make build
make frontend
```

### 数据库

```bash
./scripts/db/init-postgres.sh
./scripts/db/db-migrate.sh
```

### Docker

```bash
./scripts/docker/docker-run.sh
```

### 证书

```bash
# 安装证书（Linux）
./scripts/cert/linux-cert-download.sh

# 设置证书
./scripts/cert/setup-cert.sh
```

### 测试

```bash
# 测试缓存策略
./scripts/test/e2e/test-cache-scenarios.sh

# 测试代理
./scripts/test/e2e/test-chatbox-proxy.sh
```

### 运维

```bash
# 生成密钥
./scripts/ops/generate-secrets.sh

# 自动安装 Node.js（nvm）
./scripts/ops/install-nodejs.sh

# 加载密钥
./scripts/ops/load-secrets.sh
```

## 相关文档

- 部署文档：`docs/docker/`
- 故障排查：`docs/troubleshooting.md`
