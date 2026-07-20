#!/usr/bin/env bash
# 列出参与 CI / make test 的 Go 包路径。
#
# - 覆盖 go.work 中全部模块（不仅是根 go.mod，否则只会测到 cmd/centag + sdk）
# - 跳过：build constraints 排除全部文件的包、web/node_modules、deploy/stack、
#   cmd/tools、edition dist stub、无测试的 cmd/centag 主入口
set -euo pipefail
cd "$(dirname "$0")/.."

mods=()
while IFS= read -r dir; do
	[[ -n "$dir" ]] || continue
	mods+=("${dir}/...")
done < <(go list -m -f '{{.Dir}}')

# -e + 过滤 .Error：跳过「build constraints exclude all」的 plugin 包
go list -e -f '{{if not .Error}}{{.ImportPath}}{{end}}' "${mods[@]}" 2>/dev/null \
	| grep -v '/node_modules/' \
	| grep -v '^centag/cmd/tools$' \
	| grep -v '/deploy/stack/' \
	| grep -v '^centag/dist/' \
	| grep -v '^centag/cmd/centag$' \
	| grep -v '^$'
