# Pi Sandbox

Pi Agent HTTP 网关服务包，可**独立部署**，不依赖 stack 内其他服务。

## 快速启动

```bash
# 在 centag-stack 根目录
cp .env.example .env    # 配置 API Key
./start.sh start pi-sandbox
```

访问 http://localhost:20062/api/health

## 目录

```
services/pi-sandbox/
├── docker-compose.yml    # 生产：单容器 pi-server
├── Dockerfile
├── go-client/            # Go HTTP 网关源码
├── pi-config/            # models.json / settings.json
├── docker/               # 开发：双容器 pi-agent + go-client
└── scripts/
    ├── pi-docker.sh      # 轻量 dev 容器
    └── manage.sh         # 双容器编排
```

完整操作手册：[docs/pi-sandbox-guide.md](../../docs/pi-sandbox-guide.md)