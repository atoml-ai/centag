# Centag - 全栈 Docker 镜像（前端 + 后端）
# 日常发行版请优先使用：./start.sh dist docker-build <gateway|team|minimal>

FROM docker.m.daocloud.io/library/node:20-alpine AS frontend-builder
WORKDIR /apps
RUN npm config set registry https://registry.npmmirror.com
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM docker.m.daocloud.io/library/golang:1.25-alpine AS backend-builder
WORKDIR /app
ENV CGO_ENABLED=0 \
    GOPROXY=https://goproxy.cn,direct \
    GOTOOLCHAIN=local \
    LANG=C.UTF-8
COPY core/ ./core/
COPY plugins/ ./plugins/
COPY cmd/ ./cmd/
COPY dist/gateway/ ./dist/gateway/
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG BUILD_TAGS=protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd dist/gateway && go build -tags "${BUILD_TAGS}" \
      -ldflags "-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}'" \
      -o /app/centag .

FROM docker.m.daocloud.io/library/debian:bookworm-slim
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates tzdata curl bash && \
    rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=backend-builder /app/centag ./bin/centag
COPY --from=frontend-builder /bin/server/static ./static
COPY deploy/docker/entrypoint.sh ./entrypoint.sh
COPY config/initdata/ ./config/initdata/
RUN chmod +x ./entrypoint.sh ./bin/centag && mkdir -p ./storage ./logs
EXPOSE 20060
CMD ["./entrypoint.sh"]
