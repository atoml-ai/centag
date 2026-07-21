#!/usr/bin/env bash
# Centag one-line installer (OpenCode-style).
#
# Usage (activate PATH in the same command — curl|bash cannot mutate the parent shell):
#   curl -fsSL .../install.sh | bash && . "$HOME/.centag/env"
#   curl -fsSL .../install.sh | bash -s personal && . "$HOME/.centag/env"
#   curl -fsSL .../install.sh | bash -s wrap && . "$HOME/.centag/env"
#   curl -fsSL .../install.sh | bash -s wrap 0.2.7 && . "$HOME/.centag/env"
#
# Why "bash -s" / occasional "--"?
#   -s = read the script from stdin (the curl pipe). Args after -s go to the script.
#   "--" is only needed when an arg starts with "-" (e.g. --only); prefer "wrap" / "personal".
#
# Why not source ~/.zshrc inside install.sh?
#   Piped bash is a child process; it cannot change your interactive shell PATH.
#   Chain: … | bash && . ~/.centag/env
#
# Default (no args): personal CLI + wrap (centag-wrap)
# Asset convention (GitHub Releases, tag v<version>):
#   centag-personal-<goos>-<goarch>.tar.gz
#   centag-wrap-<goos>-<goarch>.tar.gz
#   checksums.txt
#
# Ordinary installs download Release assets only. Source builds require --from-source.
set -euo pipefail

APP=centag
REPO="${CENTAG_REPO:-atoml-ai/centag}"
RELEASE_BASE="${CENTAG_RELEASE_BASE:-}" # override e.g. https://cdn.example.com/centag/v0.2.7
# Layout matches local build/run (scripts/lib/centag-layout.sh). This file stays
# self-contained for curl|bash; layout.sh is the shared helper inside the repo.
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
  curl -fsSL .../install.sh | bash && . "$HOME/.centag/env"
  curl -fsSL .../install.sh | bash -s personal && . "$HOME/.centag/env"
  curl -fsSL .../install.sh | bash -s wrap [version] && . "$HOME/.centag/env"

Components (positional): personal | wrap
Default (no args): personal + wrap → latest GitHub release

Options:
  --only <personal|wrap>        Same as positional component
  --with <a,b>                  Explicit list (comma-separated)
  -v, --version <ver>           Pin version (or pass as 2nd positional: wrap 0.2.7)
  --from-source                 Explicitly clone + build (NOT used automatically)
  --prefix <dir>                Install root (default: ~/.centag)
  --bin-dir <dir>               PATH directory (default: <prefix>/bin)
  --no-modify-path              Do not edit shell rc files
  -h, --help                    Show this help

Examples:
  bash install.sh
  bash install.sh wrap
  bash install.sh wrap 0.2.7
  bash install.sh personal
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
      [[ -n "${2:-}" ]] || fail "--only requires personal|wrap"
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
      fail "option '$1' is not supported yet (installer currently ships personal + wrap only)"
      ;;
    personal|wrap)
      positional_component="$1"; shift ;;
    minimal|launcher|launcher-tray)
      fail "component '$1' is not supported yet (installer currently ships personal + wrap only)"
      ;;
    *)
      # Short pin: wrap 0.2.7 / personal v0.2.7 / 0.2.7 (default components)
      if [[ "$1" =~ ^v?[0-9]+([.][0-9]+)+([.-][0-9A-Za-z]+)*$ ]]; then
        requested_version="$1"; shift
      else
        warn "Unknown option '$1'"
        shift
      fi
      ;;
  esac
done

# Resolve component set.
# Priority: --only > --with > positional > default (personal + wrap).
declare -a COMPONENTS=()
resolve_components() {
  if [[ -n "$only_component" ]]; then
    COMPONENTS=("$only_component")
  elif [[ -n "$with_components" ]]; then
    IFS=',' read -r -a COMPONENTS <<< "$with_components"
  elif [[ -n "$positional_component" ]]; then
    COMPONENTS=("$positional_component")
  else
    COMPONENTS=("personal" "wrap")
  fi

  local out=() c
  for c in "${COMPONENTS[@]}"; do
    c="$(printf '%s' "$c" | tr -d '[:space:]')"
    [[ -z "$c" ]] && continue
    case "$c" in
      personal|wrap) out+=("$c") ;;
      *) fail "unknown component '$c' (want personal|wrap)" ;;
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

# Prefer github.com redirect (no API quota). Fall back to api.github.com.
latest_version() {
  local tag hdr json
  # e.g. Location: https://github.com/atoml-ai/centag/releases/tag/v0.2.7
  hdr="$(curl -fsSIL "https://github.com/${REPO}/releases/latest" 2>/dev/null || true)"
  tag="$(printf '%s' "$hdr" | tr -d '\r' | sed -n 's/^[Ll]ocation: .*\/releases\/tag\/\([^[:space:]]*\).*/\1/p' | head -1)"
  if [[ -z "$tag" ]]; then
    json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || true)"
    tag="$(printf '%s' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
  fi
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
  # $1 = component (personal|wrap)
  echo "centag-${1}-${PLATFORM_KEY}.tar.gz"
}

download() {
  local url="$1" dest="$2"
  # Prefer HTTP/1.1: some networks break on HTTP/2 framing to GitHub CDN.
  curl -fsSL --http1.1 --connect-timeout 30 --retry 5 --retry-delay 2 --retry-all-errors \
    -o "$dest" "$url"
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
if "%PROJECT_ROOT%"=="" set PROJECT_ROOT=%LIB%
if exist "%LIB%\\config\\profiles\\%EDITION%\\initdata" if "%INITDATA_PATH%"=="" set INITDATA_PATH=%LIB%\\config\\profiles\\%EDITION%\\initdata
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
# Seed data ships beside the binary (config/initdata, profile initdata).
export PROJECT_ROOT="\${PROJECT_ROOT:-\$LIB}"
if [[ -d "\$LIB/config/profiles/\$EDITION/initdata" ]]; then
  export INITDATA_PATH="\${INITDATA_PATH:-\$LIB/config/profiles/\$EDITION/initdata}"
fi
[[ -x "\$BIN" ]] || { echo "missing \$BIN" >&2; exit 1; }
exec "\$BIN" "\$@"
EOF
  chmod 755 "$wrapper"
}

install_component_from_archive() {
  local component="$1"
  local asset tmp
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
  # macOS-built archives may carry Apple xattrs; GNU tar warns loudly but still extracts.
  local tar_err="${tmp}/tar.err"
  if tar --help 2>&1 | grep -q 'warning=no-unknown-keyword'; then
    tar --warning=no-unknown-keyword -xzf "${tmp}/${asset}" -C "$tmp"
  else
    if ! tar -xzf "${tmp}/${asset}" -C "$tmp" 2>"$tar_err"; then
      cat "$tar_err" >&2 || true
      rm -rf "$tmp"
      fail "failed to extract ${asset}"
    fi
    if [[ -s "$tar_err" ]]; then
      # Drop harmless Apple xattr noise; keep real tar diagnostics.
      grep -Ev 'LIBARCHIVE\.xattr|unknown extended header keyword' "$tar_err" >&2 || true
    fi
  fi

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
      # Pipeline / backend seed (config/initdata + profile overlay)
      if [[ -d "${stage}/config" ]]; then
        rm -rf "${LIB_DIR}/${component}/config"
        cp -R "${stage}/config" "${LIB_DIR}/${component}/config"
      fi
      ln -sfn "${LIB_DIR}/${component}/centag-${component}${EXT}" "${BIN_DIR}/centag-${component}${EXT}"
      write_wrapper_centag "$component"
      ;;
    wrap)
      local binname="centag-wrap${EXT}"
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
    warn "could not resolve latest GitHub release for ${REPO} (api.github.com / releases/latest unreachable?)"
    warn "Pin a version to skip discovery, e.g.:  curl …/install.sh | bash -s 0.2.7"
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

  local parent src_dir branch_arg=()
  parent="$(mktemp -d)"
  src_dir="${parent}/centag-src"
  info "Building from source → ${src_dir}"

  if [[ -n "$requested_version" ]]; then
    branch_arg=(--branch "v$(strip_v "$requested_version")")
  fi

  if ! git clone --depth 1 "${branch_arg[@]}" "https://github.com/${REPO}.git" "$src_dir"; then
    rm -rf "$parent"
    fail "git clone failed; cannot build from source (check network / repo access)"
  fi
  [[ -d "$src_dir" ]] || fail "clone directory missing: ${src_dir}"

  (
    cd "$src_dir"
    # Local build/run layout matches install root (see scripts/lib/centag-layout.sh).
    export CENTAG_INSTALL_ROOT="${INSTALL_ROOT}"
    export CENTAG_BIN_DIR="${BIN_DIR}"
    local need_fe=false c
    for c in "${COMPONENTS[@]}"; do
      [[ "$c" == "personal" ]] && need_fe=true
    done
    if [[ "$need_fe" == true ]]; then
      (cd web && npm ci && CENTAG_INSTALL_ROOT="${INSTALL_ROOT}" CENTAG_EDITION=personal npm run build)
    fi

    for c in "${COMPONENTS[@]}"; do
      case "$c" in
        personal)
          ./start.sh build personal
          [[ -x "${LIB_DIR}/personal/centag-personal${EXT}" ]] || fail "from-source build missing ${LIB_DIR}/personal/centag-personal${EXT}"
          mkdir -p "$BIN_DIR"
          ln -sfn "${LIB_DIR}/personal/centag-personal${EXT}" "${BIN_DIR}/centag-personal${EXT}"
          write_wrapper_centag personal
          log "${GREEN}OK${NC} personal (from source)"
          ;;
        wrap)
          ./start.sh build wrap
          mkdir -p "$BIN_DIR"
          [[ -f "${BIN_DIR}/centag-wrap${EXT}" ]] || fail "from-source build missing ${BIN_DIR}/centag-wrap${EXT}"
          chmod 755 "${BIN_DIR}/centag-wrap${EXT}"
          log "${GREEN}OK${NC} wrap (from source)"
          ;;
      esac
    done
  )
  rm -rf "$parent"
}

# Make bins executable and clear macOS quarantine so double-click/CLI is not blocked.
finalize_permissions() {
  mkdir -p "$BIN_DIR" "$LIB_DIR"
  local f
  # bin wrappers / binaries / symlinks targets
  if [[ -d "$BIN_DIR" ]]; then
    for f in "$BIN_DIR"/*; do
      [[ -e "$f" ]] || continue
      chmod u+rwx,go+rx "$f" 2>/dev/null || chmod 755 "$f" 2>/dev/null || true
    done
  fi
  if [[ -d "$LIB_DIR" ]]; then
    find "$LIB_DIR" -type f \( -name 'centag-*' -o -name '*.so' -o -name '*.dylib' \) \
      -exec chmod u+rwx,go+rx {} \; 2>/dev/null || true
  fi

  # Downloaded archives often carry com.apple.quarantine → "cannot be opened" / odd exec failures.
  if [[ "$(uname -s 2>/dev/null || true)" == "Darwin" ]] && command -v xattr >/dev/null 2>&1; then
    xattr -dr com.apple.quarantine "$INSTALL_ROOT" 2>/dev/null || true
    info "Cleared macOS quarantine flags under ${INSTALL_ROOT}"
  fi
}

# Written every install. Source this in the *current* shell after curl|bash
# (a child bash cannot mutate the caller's PATH).
write_env_files() {
  mkdir -p "$INSTALL_ROOT" "$BIN_DIR"
  cat > "${INSTALL_ROOT}/env" <<EOF
# Centag PATH helper — source after install, or from your shell rc:
#   source ${INSTALL_ROOT}/env
case ":\$PATH:" in
  *":${BIN_DIR}:"*) ;;
  *) export PATH="${BIN_DIR}:\$PATH" ;;
esac
hash -r 2>/dev/null || true
EOF
  cat > "${INSTALL_ROOT}/env.fish" <<EOF
# Centag PATH helper for fish — source after install:
#   source ${INSTALL_ROOT}/env.fish
fish_add_path -g ${BIN_DIR}
EOF
}

add_to_path() {
  local config_file=$1
  local command=$2
  # Match either the env source line or a legacy inline PATH export for BIN_DIR.
  if grep -Fqh "${INSTALL_ROOT}/env" "$config_file" 2>/dev/null \
    || grep -Fqh "${BIN_DIR}" "$config_file" 2>/dev/null; then
    info "PATH entry already in ${config_file}"
    PATH_RC_FILE="$config_file"
    return 0
  fi
  if [[ ! -e "$config_file" ]]; then
    # Create a starter rc so PATH persists for new terminals.
    if touch "$config_file" 2>/dev/null; then
      :
    else
      warn "Cannot create ${config_file}; add manually: ${command}"
      return 1
    fi
  fi
  if [[ -w "$config_file" ]]; then
    {
      echo ""
      echo "# centag (added by install.sh)"
      echo "$command"
    } >> "$config_file"
    info "Added ${BIN_DIR} to PATH in ${config_file}"
    PATH_RC_FILE="$config_file"
  else
    warn "Manually add to ${config_file}: ${command}"
    return 1
  fi
}

# Persist PATH for future shells. (curl|bash cannot change the caller's shell PATH.)
PATH_RC_FILE=""
ensure_path() {
  write_env_files
  [[ "$no_modify_path" == true ]] && return 0
  mkdir -p "$BIN_DIR"

  if [[ -n "${GITHUB_ACTIONS:-}" && "${GITHUB_ACTIONS}" == "true" ]]; then
    echo "$BIN_DIR" >> "$GITHUB_PATH"
    info "Added ${BIN_DIR} to GITHUB_PATH"
    return 0
  fi

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
    if [[ -f "$file" || "$file" == "${ZDOTDIR:-$HOME}/.zshrc" || "$file" == "$HOME/.bashrc" ]]; then
      config_file="$file"
      break
    fi
  done
  # Prefer creating ~/.zshrc / ~/.bashrc when none exist.
  if [[ -z "$config_file" ]]; then
    case "$current_shell" in
      zsh) config_file="${ZDOTDIR:-$HOME}/.zshrc" ;;
      fish) config_file="$HOME/.config/fish/config.fish"; mkdir -p "$(dirname "$config_file")" ;;
      *) config_file="$HOME/.bashrc" ;;
    esac
  fi

  if [[ "$current_shell" == "fish" ]]; then
    add_to_path "$config_file" "source ${INSTALL_ROOT}/env.fish"
  else
    # Prefer sourcing env file so upgrades keep a single PATH snippet.
    add_to_path "$config_file" "[ -f \"${INSTALL_ROOT}/env\" ] && . \"${INSTALL_ROOT}/env\""
  fi
}

print_next_steps() {
  local has_personal=false has_wrap=false c
  for c in "${COMPONENTS[@]}"; do
    case "$c" in
      personal) has_personal=true ;;
      wrap) has_wrap=true ;;
    esac
  done

  echo ""
  log "${GREEN}Centag installed.${NC}"
  echo ""
  info "Install root: ${INSTALL_ROOT}"
  info "Bin dir:      ${BIN_DIR}"
  echo ""
  log "${ORANGE}# Important: curl|bash cannot change THIS shell's PATH.${NC}"
  log "${MUTED}# Activate now (pick one):${NC}"
  echo "  source \"${INSTALL_ROOT}/env\""
  if [[ -n "${PATH_RC_FILE:-}" ]]; then
    echo "  # or: source \"${PATH_RC_FILE}\""
  fi
  echo "  # or open a new terminal"
  echo ""
  if [[ "$has_personal" == true ]]; then
    log "${MUTED}# Start the gateway (http://127.0.0.1:20060):${NC}"
    echo "  centag"
    echo "  # absolute path (works before sourcing PATH):"
    echo "  ${BIN_DIR}/centag"
    echo ""
  fi
  if [[ "$has_wrap" == true ]]; then
    log "${MUTED}# Optional: wrap third-party CLIs with Centag egress:${NC}"
    echo "  centag-wrap doctor"
    echo "  # absolute path:"
    echo "  ${BIN_DIR}/centag-wrap doctor"
    echo ""
  fi
  if [[ "$(uname -s 2>/dev/null || true)" == "Darwin" ]]; then
    log "${MUTED}# macOS: if Gatekeeper blocks the binary, allow it in${NC}"
    log "${MUTED}# System Settings → Privacy & Security, then re-run the command.${NC}"
    echo ""
  fi
  info "Docs: https://github.com/${REPO}"
  echo ""
}

# --- main ------------------------------------------------------------------
mkdir -p "$INSTALL_ROOT" "$BIN_DIR" "$LIB_DIR"

if [[ "$from_source" == true ]]; then
  install_from_source
else
  if ! install_from_releases; then
    fail "release download failed for one or more components.

Ordinary installs never build from source. Please:
  1) Pin version (skips latest lookup):  curl …/install.sh | bash -s 0.2.7
  2) Retry (network/CDN blips are common)
  3) Install one component:             curl …/install.sh | bash -s personal
  4) Developers only:                  bash install.sh --from-source

Unset CENTAG_RELEASE_BASE if it points at a local/unreachable mirror."
  fi
fi

finalize_permissions
ensure_path
print_next_steps
