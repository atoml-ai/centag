FROM docker.m.daocloud.io/library/golang:1.23.7-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct
ENV GO111MODULE=on

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /pi-cli ./cmd/cli
RUN CGO_ENABLED=0 GOOS=linux go build -o /pi-server ./cmd/server

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

WORKDIR /app

COPY --from=builder /pi-cli /usr/local/bin/pi-cli
COPY --from=builder /pi-server /usr/local/bin/pi-server

ENV PI_SKIP_VERSION_CHECK=1
ENV PATH="/usr/local/lib/node_modules/.bin:${PATH}"

EXPOSE 8080

ENTRYPOINT ["pi-server"]