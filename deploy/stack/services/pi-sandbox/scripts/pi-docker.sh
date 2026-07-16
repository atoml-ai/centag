#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
STACK_ROOT="$(cd "${SERVICE_ROOT}/../.." && pwd)"
DOCKER_DIR="${SERVICE_ROOT}/docker"

IMAGE="${PI_DOCKER_IMAGE:-pi-sandbox}"
CONTAINER="${PI_DOCKER_CONTAINER:-pi-dev}"
DOCKERFILE="${PI_DOCKER_DOCKERFILE:-Dockerfile.pi}"
AGENT_VOLUME="${PI_DOCKER_AGENT_VOLUME:-pi-agent-home}"
WORKSPACE="${PI_DOCKER_WORKSPACE:-$PWD}"
PI_SOURCE_DIR="${PI_SOURCE_DIR:-}"
ONE_SHOT=false
NO_ENV=false
USE_SOURCE=false
PI_ARGS=()

# 与 pi 官方 pi-docker.sh / packages/ai/src/env-api-keys.ts 对齐
API_KEY_VARS=(
	ANTHROPIC_API_KEY
	ANTHROPIC_OAUTH_TOKEN
	OPENAI_API_KEY
	GEMINI_API_KEY
	GROQ_API_KEY
	CEREBRAS_API_KEY
	XAI_API_KEY
	OPENROUTER_API_KEY
	ZAI_API_KEY
	MISTRAL_API_KEY
	MINIMAX_API_KEY
	MINIMAX_CN_API_KEY
	AI_GATEWAY_API_KEY
	OPENCODE_API_KEY
	COPILOT_GITHUB_TOKEN
	GH_TOKEN
	GITHUB_TOKEN
	HF_TOKEN
	GOOGLE_APPLICATION_CREDENTIALS
	GOOGLE_CLOUD_PROJECT
	GCLOUD_PROJECT
	GOOGLE_CLOUD_LOCATION
	GOOGLE_API_KEY
	DEEPSEEK_API_KEY
	KIMI_API_KEY
	AWS_PROFILE
	AWS_ACCESS_KEY_ID
	AWS_SECRET_ACCESS_KEY
	AWS_SESSION_TOKEN
	AWS_REGION
	AWS_DEFAULT_REGION
	AWS_BEARER_TOKEN_BEDROCK
	AWS_CONTAINER_CREDENTIALS_RELATIVE_URI
	AWS_CONTAINER_CREDENTIALS_FULL_URI
	AWS_WEB_IDENTITY_TOKEN_FILE
	AZURE_OPENAI_API_KEY
	AZURE_OPENAI_BASE_URL
	AZURE_OPENAI_RESOURCE_NAME
)

usage() {
	cat <<EOF
Usage: $(basename "$0") <command> [options] [--] [pi args...]

Centag Stack 版 Pi Docker 封装（默认 ${DOCKER_DIR}/${DOCKERFILE}）。

Commands:
  build              构建镜像（--source 从 PI_SOURCE_DIR 源码构建）
  start              启动持久化开发容器
  stop               停止并删除开发容器
  restart            重启开发容器
  status             查看容器/镜像状态
  run                交互式 TUI（默认）
  print, p           非交互 print 模式 (-p)
  json               JSON 输出 (--mode json -p)
  rpc                JSON-RPC 模式 (--mode rpc)
  shell, sh          进入容器 shell
  exec               透传任意 pi 参数
  help               显示帮助

Options:
  --workspace DIR    挂载到 /workspace 的宿主机目录（默认: \$PWD）
  --image NAME       镜像标签（默认: ${IMAGE}）
  --container NAME   容器名（默认: ${CONTAINER}）
  --source           使用 PI_SOURCE_DIR 下的 Dockerfile.pi-source 构建
  --one-shot         使用 docker run 而非持久化容器
  --no-env           不转发宿主机 API Key

Examples:
  $(basename "$0") build
  PI_SOURCE_DIR=~/workspaces/pi $(basename "$0") build --source
  $(basename "$0") start && $(basename "$0") run
  $(basename "$0") print "Explain centag-stack"

Environment:
  PI_SOURCE_DIR          本地 pi 源码目录（配合 --source）
  PI_DOCKER_IMAGE, PI_DOCKER_CONTAINER, PI_DOCKER_DOCKERFILE
  PI_DOCKER_AGENT_VOLUME, PI_DOCKER_WORKSPACE, PI_DOCKER_AGENT_DIR
  PI_DOCKER_USE_HOST_AGENT=1  挂载 ~/.pi/agent
  PI_DOCKER_SKIP_VERSION_CHECK=1（默认）在容器内设置 PI_SKIP_VERSION_CHECK
  PI_PACKAGE, PI_VERSION  npm 全局安装时的包名与版本（默认 @earendil-works/pi-coding-agent@0.78.1）
EOF
}

die() {
	echo "Error: $*" >&2
	exit 1
}

require_docker() {
	command -v docker >/dev/null 2>&1 || die "docker is not installed or not on PATH"
}

collect_env_args() {
	ENV_ARGS=()
	if [[ "$NO_ENV" != "true" ]]; then
		local var
		for var in "${API_KEY_VARS[@]}"; do
			if [[ -n "${!var:-}" ]]; then
				ENV_ARGS+=(-e "$var")
			fi
		done
		if [[ -n "${PI_OFFLINE:-}" ]]; then
			ENV_ARGS+=(-e "PI_OFFLINE=${PI_OFFLINE}")
		fi
	fi
	if [[ "${PI_DOCKER_SKIP_VERSION_CHECK:-1}" == "1" ]]; then
		ENV_ARGS+=(-e PI_SKIP_VERSION_CHECK=1)
	fi
	ENV_ARGS+=(-e "TERM=${TERM:-xterm-256color}")
}

common_run_args() {
	collect_env_args
	COMMON_ARGS=(-v "${WORKSPACE}:/workspace")
	local models_json="${STACK_ROOT}/services/pi-sandbox/pi-config/models.json"
	local settings_json="${STACK_ROOT}/services/pi-sandbox/pi-config/settings.json"
	if [[ -n "${PI_DOCKER_AGENT_DIR:-}" ]]; then
		COMMON_ARGS+=(-v "${PI_DOCKER_AGENT_DIR}:/root/.pi/agent")
	elif [[ -d "${HOME:-}/.pi/agent" && "${PI_DOCKER_USE_HOST_AGENT:-}" == "1" ]]; then
		COMMON_ARGS+=(-v "${HOME}/.pi/agent:/root/.pi/agent")
	else
		COMMON_ARGS+=(-v "${AGENT_VOLUME}:/root/.pi/agent")
		if [[ -f "${WORKSPACE}/.pi/agent/models.json" ]]; then
			COMMON_ARGS+=(-v "${WORKSPACE}/.pi/agent/models.json:/root/.pi/agent/models.json:ro")
		elif [[ -f "${WORKSPACE}/models.json" ]]; then
			COMMON_ARGS+=(-v "${WORKSPACE}/models.json:/root/.pi/agent/models.json:ro")
		elif [[ -f "${models_json}" ]]; then
			COMMON_ARGS+=(-v "${models_json}:/root/.pi/agent/models.json:ro")
		fi
		if [[ -f "${settings_json}" ]]; then
			COMMON_ARGS+=(-v "${settings_json}:/root/.pi/agent/settings.json:ro")
		fi
	fi
	COMMON_ARGS+=("${ENV_ARGS[@]}")
}

image_exists() {
	docker image inspect "$IMAGE" >/dev/null 2>&1
}

container_running() {
	docker container inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null | grep -qx true
}

container_exists() {
	docker container inspect "$CONTAINER" >/dev/null 2>&1
}

cmd_build() {
	require_docker
	local no_cache=()
	if [[ "${1:-}" == "--no-cache" ]]; then
		no_cache=(--no-cache)
	fi
	local build_args=(
		--build-arg "PI_PACKAGE=${PI_PACKAGE:-@earendil-works/pi-coding-agent}"
		--build-arg "PI_VERSION=${PI_VERSION:-0.78.1}"
	)
	if [[ "$USE_SOURCE" == "true" ]]; then
		[[ -n "$PI_SOURCE_DIR" && -d "$PI_SOURCE_DIR" ]] || die "PI_SOURCE_DIR must point to a pi checkout when using --source"
		echo "Building ${IMAGE} from source: ${PI_SOURCE_DIR} ..."
		docker build "${no_cache[@]}" -t "$IMAGE" \
			-f "${DOCKER_DIR}/Dockerfile.pi-source" \
			"${PI_SOURCE_DIR}"
		return
	fi
	echo "Building ${IMAGE} from ${DOCKER_DIR}/${DOCKERFILE} ..."
	docker build "${no_cache[@]}" "${build_args[@]}" -t "$IMAGE" \
		-f "${DOCKER_DIR}/${DOCKERFILE}" \
		"${DOCKER_DIR}"
}

cmd_start() {
	require_docker
	common_run_args
	if container_running; then
		echo "Dev container '${CONTAINER}' is already running."
		return 0
	fi
	if container_exists; then
		echo "Starting existing dev container '${CONTAINER}' ..."
		docker start "$CONTAINER" >/dev/null
		return 0
	fi
	if ! image_exists; then
		echo "Image '${IMAGE}' not found; building ..."
		cmd_build
	fi
	echo "Creating dev container '${CONTAINER}' ..."
	docker run -d \
		--name "$CONTAINER" \
		--entrypoint sleep \
		-w /workspace \
		"${COMMON_ARGS[@]}" \
		"$IMAGE" infinity >/dev/null
	echo "Dev container '${CONTAINER}' is ready. Run: $(basename "$0") run"
}

cmd_stop() {
	require_docker
	if container_exists; then
		docker rm -f "$CONTAINER" >/dev/null
		echo "Removed dev container '${CONTAINER}'."
	else
		echo "Dev container '${CONTAINER}' is not running."
	fi
}

cmd_restart() {
	cmd_stop
	cmd_start
}

cmd_status() {
	require_docker
	if container_running; then
		echo "Dev container '${CONTAINER}': running"
		docker ps --filter "name=^/${CONTAINER}$" --format '  image: {{.Image}}  uptime: {{.Status}}'
	elif container_exists; then
		echo "Dev container '${CONTAINER}': stopped"
	else
		echo "Dev container '${CONTAINER}': not created"
	fi
	if image_exists; then
		echo "Image '${IMAGE}': present"
	else
		echo "Image '${IMAGE}': missing (run: $(basename "$0") build)"
	fi
}

ensure_dev_container() {
	if [[ "$ONE_SHOT" == "true" ]]; then
		return 1
	fi
	if container_running; then
		return 0
	fi
	cmd_start
}

has_host_api_keys() {
	if [[ "$NO_ENV" == "true" ]]; then
		return 1
	fi
	local var
	for var in "${API_KEY_VARS[@]}"; do
		if [[ -n "${!var:-}" ]]; then
			return 0
		fi
	done
	return 1
}

warn_missing_api_keys() {
	if has_host_api_keys; then
		return
	fi
	echo "Warning: no provider API keys found in the host environment." >&2
	echo "  export KIMI_API_KEY=... or OPENAI_API_KEY=..." >&2
	echo "  Or run '$(basename "$0") run' and use /login inside pi." >&2
}

run_pi() {
	warn_missing_api_keys
	local tty_args=()
	if [[ -t 0 && -t 1 ]]; then
		tty_args=(-it)
	fi
	common_run_args
	if ensure_dev_container; then
		if [[ ${#tty_args[@]} -gt 0 ]]; then
			docker exec "${tty_args[@]}" "${ENV_ARGS[@]}" -w /workspace "$CONTAINER" pi "$@"
		else
			docker exec "${ENV_ARGS[@]}" -w /workspace "$CONTAINER" pi "$@"
		fi
	else
		if ! image_exists; then
			echo "Image '${IMAGE}' not found; building ..."
			cmd_build
		fi
		if [[ ${#tty_args[@]} -gt 0 ]]; then
			docker run --rm "${tty_args[@]}" -w /workspace "${COMMON_ARGS[@]}" "$IMAGE" "$@"
		else
			docker run --rm -w /workspace "${COMMON_ARGS[@]}" "$IMAGE" "$@"
		fi
	fi
}

run_shell() {
	local tty_args=()
	if [[ -t 0 && -t 1 ]]; then
		tty_args=(-it)
	fi
	common_run_args
	if ensure_dev_container; then
		if [[ ${#tty_args[@]} -gt 0 ]]; then
			docker exec "${tty_args[@]}" "${ENV_ARGS[@]}" -w /workspace "$CONTAINER" bash "$@"
		else
			docker exec "${ENV_ARGS[@]}" -w /workspace "$CONTAINER" bash "$@"
		fi
	else
		if ! image_exists; then
			echo "Image '${IMAGE}' not found; building ..."
			cmd_build
		fi
		if [[ ${#tty_args[@]} -gt 0 ]]; then
			docker run --rm "${tty_args[@]}" -w /workspace --entrypoint bash "${COMMON_ARGS[@]}" "$IMAGE" "$@"
		else
			docker run --rm -w /workspace --entrypoint bash "${COMMON_ARGS[@]}" "$IMAGE" "$@"
		fi
	fi
}

cmd_run() { run_pi "$@"; }
cmd_print() {
	[[ $# -gt 0 ]] || die "print mode requires a prompt argument"
	run_pi -p "$@"
}
cmd_json() {
	[[ $# -gt 0 ]] || die "json mode requires a prompt argument"
	run_pi --mode json -p "$@"
}
cmd_rpc() { run_pi --mode rpc "$@"; }
cmd_exec() { run_pi "$@"; }
cmd_shell() { run_shell "$@"; }

parse_global_options() {
	while [[ $# -gt 0 ]]; do
		case "$1" in
		--workspace)
			[[ $# -ge 2 ]] || die "--workspace requires a directory"
			WORKSPACE="$(cd "$2" && pwd)"
			shift 2
			;;
		--image)
			[[ $# -ge 2 ]] || die "--image requires a value"
			IMAGE="$2"
			shift 2
			;;
		--container)
			[[ $# -ge 2 ]] || die "--container requires a value"
			CONTAINER="$2"
			shift 2
			;;
		--source)
			USE_SOURCE=true
			shift
			;;
		--one-shot)
			ONE_SHOT=true
			shift
			;;
		--no-env)
			NO_ENV=true
			shift
			;;
		-h | --help)
			usage
			exit 0
			;;
		--)
			PI_ARGS+=("$@")
			return 0
			;;
		-*)
			PI_ARGS+=("$1")
			shift
			;;
		*)
			PI_ARGS+=("$1")
			shift
			;;
		esac
	done
}

main() {
	local command="${1:-run}"
	case "$command" in
	build)
		shift || true
		parse_global_options "$@"
		cmd_build ${PI_ARGS[@]+"${PI_ARGS[@]}"}
		;;
	start | up)
		shift || true
		parse_global_options "$@"
		cmd_start
		;;
	stop | down)
		shift || true
		parse_global_options "$@"
		cmd_stop
		;;
	restart)
		shift || true
		parse_global_options "$@"
		cmd_restart
		;;
	status)
		shift || true
		parse_global_options "$@"
		cmd_status
		;;
	run | it | interactive)
		shift || true
		parse_global_options "$@"
		cmd_run ${PI_ARGS[@]+"${PI_ARGS[@]}"}
		;;
	print | p)
		shift || true
		parse_global_options "$@"
		cmd_print ${PI_ARGS[@]+"${PI_ARGS[@]}"}
		;;
	json)
		shift || true
		parse_global_options "$@"
		cmd_json ${PI_ARGS[@]+"${PI_ARGS[@]}"}
		;;
	rpc)
		shift || true
		parse_global_options "$@"
		cmd_rpc ${PI_ARGS[@]+"${PI_ARGS[@]}"}
		;;
	shell | sh)
		shift || true
		parse_global_options "$@"
		cmd_shell ${PI_ARGS[@]+"${PI_ARGS[@]}"}
		;;
	exec)
		shift || true
		parse_global_options "$@"
		cmd_exec ${PI_ARGS[@]+"${PI_ARGS[@]}"}
		;;
	help | -h | --help)
		usage
		;;
	-*)
		parse_global_options "$command" "$@"
		cmd_run ${PI_ARGS[@]+"${PI_ARGS[@]}"}
		;;
	*)
		parse_global_options "$command" "$@"
		if [[ ${#PI_ARGS[@]} -eq 0 ]]; then
			cmd_run
		else
			cmd_run ${PI_ARGS[@]+"${PI_ARGS[@]}"}
		fi
		;;
	esac
}

main "$@"