#!/usr/bin/env bash
# Centag one-line installer (OpenCode-style).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash
#   curl -fsSL .../install.sh | bash -s -- --only proxyctl
#   curl -fsSL .../install.sh | bash -s -- --from-source
#
# Default (no args): personal CLI + proxyctl
# Asset convention (GitHub Releases, tag v<version>):
#   centag-personal-<goos>-<goarch>.tar.gz
#   centag-proxyctl-<goos>-<goarch>.tar.gz
#   checksums.txt
set -euo pipefail

APP=centag
REPO="${CENTAG_REPO:-atoml-ai/centag}"
RELEASE_BASE="${CENTAG_RELEASE_BASE:-}" # override e.g. https://cdn.example.com/centag/v0.2.7
INSTALL_ROOT="${CENTAG_INSTALL_ROOT:-$HOME/.centag}"
BIN_DIR="${CENTAG_BIN_DIR:-$INSTALL_ROOT/bin}"
LIB_DIR="${INSTALL_ROOT}/lib"

MUTED='\033[0;2m'
RED='\033[0;31m'
ORANGE='\033[38;5;214m'
GREEN='\033[0;32m'
NC='\033[0m'

usage() {
  cat <<'EOF'
Centag installer

Usage:
  curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash
  curl -fsSL .../install.sh | bash -s -- [options] [component]

Components: personal | proxyctl
Default (no args): personal + proxyctl

Options:
  --only <personal|proxyctl>    Install only one component
  --with <a,b>                  Explicit list (comma-separated)
  -v, --version <ver>           Install a specific version (e.g. 0.2.7 or v0.2.7)
  --from-source                 Clone + build instead of downloading releases
  --prefix <dir>                Install root (default: ~/.centag)
  --bin-dir <dir>               PATH directory (default: <prefix>/bin)
  --no-modify-path              Do not edit shell rc files
  -h, --help                    Show this help

Examples:
  bash install.sh
  bash install.sh --only proxyctl
  bash install.sh personal
  bash install.sh proxyctl
EOF
}

log() { printf '%b\n' "$*"; }
info() { log "${MUTED}$*${NC}"; }
warn() { log "${ORANGE}warn:${NC} $*" >&2; }
fail() { log "${RED}error:${NC} $*" >&2; exit 1; }

# --- defaults / args -------------------------------------------------------
requested_version="${VERSION:-${CENTAG_VERSION:-}}"
from_source=false
no_modify_path=false
only_component=""
with_components=""
positional_component=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    -v|--version)
      [[ -n "${2:-}" ]] || fail "--version requires an argument"
      requested_version="$2"; shift 2 ;;
    --only)
      [[ -n "${2:-}" ]] || fail "--only requires personal|proxyctl"
      only_component="$2"; shift 2 ;;
    --with)
      [[ -n "${2:-}" ]] || fail "--with requires a comma-separated list"
      with_components="$2"; shift 2 ;;
    --from-source) from_source=true; shift ;;
    --prefix)
      [[ -n "${2:-}" ]] || fail "--prefix requires a directory"
      INSTALL_ROOT="$2"
      BIN_DIR="${CENTAG_BIN_DIR:-$INSTALL_ROOT/bin}"
      LIB_DIR="${INSTALL_ROOT}/lib"
      shift 2 ;;
    --bin-dir)
      [[ -n "${2:-}" ]] || fail "--bin-dir requires a directory"
      BIN_DIR="$2"; shift 2 ;;
    --no-modify-path) no_modify_path=true; shift ;;
    # deferred: minimal / launcher / launcher-tray
    --edition|--launcher|--launcher-tray)
      fail "option '$1' is not supported yet (installer currently ships personal + proxyctl only)"
      ;;
    personal|proxyctl)
      positional_component="$1"; shift ;;
    minimal|launcher|launcher-tray)
      fail "component '$1' is not supported yet (installer currently ships personal + proxyctl only)"
      ;;
    *)
      warn "Unknown option '$1'"
      shift ;;
  esac
done

# Resolve component set.
# Priority: --only > --with > positional > default (personal + proxyctl).
declare -a COMPONENTS=()
resolve_components() {
  if [[ -n "$only_component" ]]; then
    COMPONENTS=("$only_component")
  elif [[ -n "$with_components" ]]; then
    IFS=',' read -r -a COMPONENTS <<< "$with_components"
  elif [[ -n "$positional_component" ]]; then
    COMPONENTS=("$positional_component")
  else
    COMPONENTS=("personal" "proxyctl")
  fi

  local out=() c
  for c in "${COMPONENTS[@]}"; do
    c="$(printf '%s' "$c" | tr -d '[:space:]')"
    [[ -z "$c" ]] && continue
    case "$c" in
      personal|proxyctl) out+=("$c") ;;
      *) fail "unknown component '$c' (want personal|proxyctl)" ;;
    esac
  done
  COMPONENTS=("${out[@]}")
  [[ ${#COMPONENTS[@]} -gt 0 ]] || fail "no components selected"
}

resolve_components

# --- platform --------------------------------------------------------------
detect_platform() {
  local raw_os os arch
  raw_os="$(uname -s)"
  os="$(echo "$raw_os" | tr '[:upper:]' '[:lower:]')"
  case "$raw_os" in
    Darwin*) os="darwin" ;;
    Linux*) os="linux" ;;
    MINGW*|MSYS*|CYGWIN*) os="windows" ;;
  esac

  arch="$(uname -m)"
  case "$arch" in
    aarch64|arm64) arch="arm64" ;;
    x86_64|amd64) arch="amd64" ;;
    *) fail "unsupported arch: $arch" ;;
  esac

  # Rosetta: prefer native arm64
  if [[ "$os" == "darwin" && "$arch" == "amd64" ]]; then
    local rosetta
    rosetta="$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)"
    if [[ "$rosetta" == "1" ]]; then
      arch="arm64"
    fi
  fi

  case "${os}-${arch}" in
    darwin-amd64|darwin-arm64|linux-amd64|linux-arm64|windows-amd64|windows-arm64) ;;
    *) fail "unsupported OS/Arch: ${os}/${arch}" ;;
  esac

  GOOS="$os"
  GOARCH="$arch"
  PLATFORM_KEY="${GOOS}-${GOARCH}"
  EXT=""
  if [[ "$GOOS" == "windows" ]]; then
    EXT=".exe"
  fi
}

detect_platform

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "'$1' is required but not installed"
}

need_cmd curl
need_cmd tar
need_cmd mktemp

# --- version / URLs --------------------------------------------------------
strip_v() { echo "${1#v}"; }

latest_version() {
  local json tag
  json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || true)"
  tag="$(printf '%s' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
  [[ -n "$tag" ]] || return 1
  strip_v "$tag"
}

resolve_version() {
  if [[ -n "$requested_version" ]]; then
    VERSION="$(strip_v "$requested_version")"
  else
    VERSION="$(latest_version)" || return 1
  fi
  TAG="v${VERSION}"
  if [[ -n "$RELEASE_BASE" ]]; then
    BASE_URL="${RELEASE_BASE%/}"
  else
    BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
  fi
}

asset_name() {
  # $1 = component
  echo "centag-${1}-${PLATFORM_KEY}.tar.gz"
}

download() {
  local url="$1" dest="$2"
  curl -fsSL --connect-timeout 30 --retry 3 -o "$dest" "$url"
}

verify_checksum() {
  local file="$1" asset="$2" sums_file="$3"
  [[ -f "$sums_file" ]] || return 0
  local want got
  want="$(awk -v a="$asset" '$2 == a { print $1; exit }' "$sums_file" || true)"
  [[ -n "$want" ]] || return 0
  if command -v shasum >/dev/null 2>&1; then
    got="$(shasum -a 256 "$file" | awk '{print $1}')"
  elif command -v sha256sum >/dev/null 2>&1; then
    got="$(sha256sum "$file" | awk '{print $1}')"
  else
    warn "neither shasum nor sha256sum found; skipping checksum"
    return 0
  fi
  [[ "$got" == "$want" ]] || fail "checksum mismatch for ${asset}: want ${want}, got ${got}"
}

write_wrapper_centag() {
  local target_edition="$1"
  if [[ "$GOOS" == "windows" ]]; then
    local cmd="${BIN_DIR}/centag.cmd"
    cat > "$cmd" <<EOF
@echo off
set ROOT=%~dp0..
set EDITION=${target_edition}
set LIB=%ROOT%\\lib\\%EDITION%
set BIN=%LIB%\\centag-%EDITION%.exe
set CENTAG_EDITION=%EDITION%
if "%STATIC_PATH%"=="" set STATIC_PATH=%LIB%\\static
"%BIN%" %*
EOF
    return 0
  fi

  local wrapper="${BIN_DIR}/centag"
  cat > "$wrapper" <<EOF
#!/usr/bin/env bash
set -euo pipefail
ROOT="\$(cd "\$(dirname "\$0")/.." && pwd)"
EDITION="${target_edition}"
LIB="\$ROOT/lib/\$EDITION"
BIN="\$LIB/centag-\${EDITION}"
export CENTAG_EDITION="\$EDITION"
export STATIC_PATH="\${STATIC_PATH:-\$LIB/static}"
[[ -x "\$BIN" ]] || { echo "missing \$BIN" >&2; exit 1; }
exec "\$BIN" "\$@"
EOF
  chmod 755 "$wrapper"
}

install_component_from_archive() {
  local component="$1"
  local asset tmp dir
  asset="$(asset_name "$component")"
  tmp="$(mktemp -d)"
  info "Downloading ${asset} ..."
  if ! download "${BASE_URL}/${asset}" "${tmp}/${asset}"; then
    rm -rf "$tmp"
    return 1
  fi
  if download "${BASE_URL}/checksums.txt" "${tmp}/checksums.txt" 2>/dev/null; then
    verify_checksum "${tmp}/${asset}" "$asset" "${tmp}/checksums.txt"
  else
    warn "checksums.txt not found; continuing without verification"
  fi

  mkdir -p "$BIN_DIR" "$LIB_DIR"
  tar -xzf "${tmp}/${asset}" -C "$tmp"

  case "$component" in
    personal)
      mkdir -p "${LIB_DIR}/${component}"
      # archive root: centag-<component>-<platform>/...
      local stage
      stage="$(find "$tmp" -maxdepth 1 -type d -name "centag-${component}-*" | head -1)"
      [[ -n "$stage" ]] || stage="$tmp"
      if [[ -f "${stage}/centag-${component}${EXT}" ]]; then
        cp -f "${stage}/centag-${component}${EXT}" "${LIB_DIR}/${component}/"
      elif [[ -f "${tmp}/centag-${component}${EXT}" ]]; then
        cp -f "${tmp}/centag-${component}${EXT}" "${LIB_DIR}/${component}/"
      else
        local found
        found="$(find "$tmp" -type f -name "centag-${component}${EXT}" | head -1 || true)"
        [[ -n "$found" ]] || fail "archive missing centag-${component}${EXT}"
        cp -f "$found" "${LIB_DIR}/${component}/"
      fi
      chmod 755 "${LIB_DIR}/${component}/centag-${component}${EXT}"
      if [[ -d "${stage}/static" ]]; then
        rm -rf "${LIB_DIR}/${component}/static"
        cp -R "${stage}/static" "${LIB_DIR}/${component}/static"
      elif [[ -d "${tmp}/static" ]]; then
        rm -rf "${LIB_DIR}/${component}/static"
        cp -R "${tmp}/static" "${LIB_DIR}/${component}/static"
      fi
      ln -sfn "${LIB_DIR}/${component}/centag-${component}${EXT}" "${BIN_DIR}/centag-${component}${EXT}"
      write_wrapper_centag "$component"
      ;;
    proxyctl)
      local binname="centag-proxyctl${EXT}"
      local found
      found="$(find "$tmp" -type f -name "$binname" | head -1 || true)"
      [[ -n "$found" ]] || fail "archive missing ${binname}"
      cp -f "$found" "${BIN_DIR}/${binname}"
      chmod 755 "${BIN_DIR}/${binname}"
      ;;
    *)
      fail "unsupported component in archive install: $component"
      ;;
  esac

  rm -rf "$tmp"
  log "${GREEN}OK${NC} ${component} → ${BIN_DIR}"
  return 0
}

install_from_releases() {
  resolve_version || {
    warn "could not resolve latest GitHub release for ${REPO}"
    return 1
  }
  info "Installing Centag ${TAG} (${PLATFORM_KEY})"
  info "Components: ${COMPONENTS[*]}"
  info "Prefix: ${INSTALL_ROOT}"

  local c failed=()
  for c in "${COMPONENTS[@]}"; do
    if ! install_component_from_archive "$c"; then
      failed+=("$c")
    fi
  done

  if [[ ${#failed[@]} -gt 0 ]]; then
    warn "release download failed for: ${failed[*]}"
    return 1
  fi
  return 0
}

install_from_source() {
  need_cmd git
  need_cmd go
  need_cmd npm

  local src_dir branch_arg=()
  src_dir="$(mktemp -d)/centag-src"
  info "Building from source → ${src_dir}"

  if [[ -n "$requested_version" ]]; then
    branch_arg=(--branch "v$(strip_v "$requested_version")")
  fi

  git clone --depth 1 "${branch_arg[@]}" "https://github.com/${REPO}.git" "$src_dir"
  (
    cd "$src_dir"
    local need_fe=false c
    for c in "${COMPONENTS[@]}"; do
      [[ "$c" == "personal" ]] && need_fe=true
    done
    if [[ "$need_fe" == true ]]; then
      (cd web && npm ci && npm run build)
    fi

    for c in "${COMPONENTS[@]}"; do
      case "$c" in
        personal)
          ./start.sh build personal
          mkdir -p "${LIB_DIR}/personal"
          cp -f "bin/server/centag-personal" "${LIB_DIR}/personal/centag-personal${EXT}"
          chmod 755 "${LIB_DIR}/personal/centag-personal${EXT}"
          if [[ -d bin/server/static ]]; then
            rm -rf "${LIB_DIR}/personal/static"
            cp -R bin/server/static "${LIB_DIR}/personal/static"
          fi
          mkdir -p "$BIN_DIR"
          ln -sfn "${LIB_DIR}/personal/centag-personal${EXT}" "${BIN_DIR}/centag-personal${EXT}"
          write_wrapper_centag personal
          log "${GREEN}OK${NC} personal (from source)"
          ;;
        proxyctl)
          ./start.sh build proxyctl
          mkdir -p "$BIN_DIR"
          cp -f "bin/proxyctl/centag-proxyctl${EXT}" "${BIN_DIR}/centag-proxyctl${EXT}"
          chmod 755 "${BIN_DIR}/centag-proxyctl${EXT}"
          log "${GREEN}OK${NC} proxyctl (from source)"
          ;;
      esac
    done
  )
}

add_to_path() {
  local config_file=$1
  local command=$2
  if grep -Fxq "$command" "$config_file" 2>/dev/null; then
    info "PATH entry already in ${config_file}"
  elif [[ -w "$config_file" ]]; then
    {
      echo ""
      echo "# centag"
      echo "$command"
    } >> "$config_file"
    info "Added ${BIN_DIR} to PATH in ${config_file}"
  else
    warn "Manually add to ${config_file}: ${command}"
  fi
}

ensure_path() {
  [[ "$no_modify_path" == true ]] && return 0
  mkdir -p "$BIN_DIR"

  if [[ -n "${GITHUB_ACTIONS:-}" && "${GITHUB_ACTIONS}" == "true" ]]; then
    echo "$BIN_DIR" >> "$GITHUB_PATH"
    info "Added ${BIN_DIR} to GITHUB_PATH"
    return 0
  fi

  case ":${PATH}:" in
    *":${BIN_DIR}:"*) return 0 ;;
  esac

  local XDG_CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
  local current_shell config_files config_file=""
  current_shell="$(basename "${SHELL:-bash}")"
  case "$current_shell" in
    fish) config_files="$HOME/.config/fish/config.fish" ;;
    zsh) config_files="${ZDOTDIR:-$HOME}/.zshrc ${ZDOTDIR:-$HOME}/.zshenv $XDG_CONFIG_HOME/zsh/.zshrc" ;;
    bash) config_files="$HOME/.bashrc $HOME/.bash_profile $HOME/.profile" ;;
    *) config_files="$HOME/.bashrc $HOME/.profile" ;;
  esac

  local file
  for file in $config_files; do
    if [[ -f "$file" ]]; then
      config_file="$file"
      break
    fi
  done

  if [[ -z "$config_file" ]]; then
    warn "No shell config found. Add manually:"
    info "  export PATH=${BIN_DIR}:\$PATH"
    return 0
  fi

  if [[ "$current_shell" == "fish" ]]; then
    add_to_path "$config_file" "fish_add_path ${BIN_DIR}"
  else
    add_to_path "$config_file" "export PATH=${BIN_DIR}:\$PATH"
  fi
}

print_next_steps() {
  echo ""
  log "${MUTED}Centag installed.${NC}"
  echo ""
  local c
  for c in "${COMPONENTS[@]}"; do
    case "$c" in
      personal)
        info "  centag                 # start personal CLI (port 20060)"
        info "  centag-personal        # direct binary"
        ;;
      proxyctl)
        info "  centag-proxyctl run -- opencode"
        info "  centag-proxyctl doctor"
        ;;
    esac
  done
  echo ""
  info "Install root: ${INSTALL_ROOT}"
  info "Bin dir:      ${BIN_DIR}"
  info "Docs:         https://github.com/${REPO}"
  echo ""
}

# --- main ------------------------------------------------------------------
mkdir -p "$INSTALL_ROOT" "$BIN_DIR" "$LIB_DIR"

if [[ "$from_source" == true ]]; then
  install_from_source
else
  if ! install_from_releases; then
    warn "falling back to --from-source (release assets missing or incomplete)"
    if ! install_from_source; then
      fail "install failed via release download and from-source build"
    fi
  fi
fi

ensure_path
print_next_steps
