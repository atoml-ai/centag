#!/usr/bin/env bash
# 列出参与 CI 的 Go 包路径（排除 web/node_modules 下被误识别为 Go 模块的树；
# 排除 cmd/tools：多 main 的本地调试脚本，非可发布包；
# 排除 …/deploy/stack/…：子项目树（go list 下为 centag/deploy/stack/...），pi-sandbox/examples 等依赖独立 go-client 模块）。
set -euo pipefail
cd "$(dirname "$0")/.."
# -e: 跳过 deploy/stack/pi-sandbox 等缺依赖的包，避免整表失败
go list -e ./... | grep -v '/node_modules/' | grep -v '^centag/cmd/tools$' | grep -v '/deploy/stack/'
