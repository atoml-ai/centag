# Centag Stack — 编码规范

## 1. Docker Compose 规范

### 1.1 服务命名
- 容器名使用 `centag-<服务名>` 格式
- 服务名使用短横线命名

### 1.2 端口映射
- 宿主机端口与环境变量绑定（`${VAR:-默认值}`）

### 1.3 环境变量
- 使用 `env_file` 加载 `.env`
- 禁止在 compose 文件中硬编码密码

### 1.4 健康检查
- 所有服务必须配置 `healthcheck`

### 1.5 Profiles
- 使用 profiles 控制服务启动组合

## 2. Shell 脚本规范

### 2.1 头部
```bash
#!/bin/bash
set -euo pipefail
```

### 2.2 颜色输出
```bash
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly NC='\033[0m'
```

### 2.3 日志函数
```bash
print_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
```

## 3. 环境变量规范

- 使用大写字母
- 使用下划线分隔
- 用途前缀：`POSTGRES_`、`MEM0_`、`NEO4J_` 等