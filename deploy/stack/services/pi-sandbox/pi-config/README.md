# Pi Agent 自定义 provider / model 配置
#
# 文档：https://github.com/badlogic/pi-mono (docs/models.md)
# 加载路径：~/.pi/agent/models.json（docker-compose 中挂载到 /root/.pi/agent/models.json:ro）
#
# 注意：
# 1. 本文件提交到 git。**禁止**直接写明文 apiKey；使用环境变量引用（如 "$OPENAI_API_KEY"），
#    实际密钥在 centag-stack/.env（或部署侧环境变量）中注入。
# 2. pi 的 env var `OPENAI_BASE_URL` 不会自动让内置 openai provider 改路由，
#    因此自定义 OpenAI-兼容 provider 必须通过本文件 + models.json 注入。
# 3. 修改后需重启容器：`./start.sh restart pi-sandbox`
