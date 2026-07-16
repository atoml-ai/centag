# 构建上下文：services/pi-sandbox/（见 docker-compose.yml）
FROM docker.m.daocloud.io/library/golang:1.23-alpine AS gobuild

WORKDIR /src
COPY go-client/go.mod go-client/go.sum /src/
COPY go-client/ /src/
RUN go mod download && CGO_ENABLED=0 GOOS=linux go build -o /pi-server ./cmd/server

FROM docker.m.daocloud.io/library/node:22-bookworm-slim

ARG PI_PACKAGE=@earendil-works/pi-coding-agent
ARG PI_VERSION=0.78.1
ARG NPM_REGISTRY=https://registry.npmmirror.com

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates curl git bash ripgrep fd-find \
    && ln -sf /usr/bin/fdfind /usr/bin/fd \
    && rm -rf /var/lib/apt/lists/*

RUN npm config set registry "${NPM_REGISTRY}" \
    && npm install -g "${PI_PACKAGE}@${PI_VERSION}"

WORKDIR /workspace
RUN mkdir -p /data/sessions /workspace /root/.pi/agent

COPY --from=gobuild /pi-server /usr/local/bin/pi-server

ENV PORT=:8080
ENV PI_SKIP_VERSION_CHECK=1
ENV PATH="/usr/local/lib/node_modules/.bin:${PATH}"

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=90s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/api/health >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/pi-server"]