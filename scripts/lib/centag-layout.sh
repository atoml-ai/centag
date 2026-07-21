#!/usr/bin/env bash
# Centag install-compatible layout for local build / run.
# Mirrors scripts/install.sh defaults so dev artifacts land under the same tree.
#
# Default root: ${CENTAG_INSTALL_ROOT:-$HOME/.centag}
#
#   $ROOT/
#   ├── bin/                 # PATH: centag wrapper, centag-<edition> symlink, centag-wrap
#   ├── lib/<edition>/       # binary + static + config + runtime (storage/logs/certs)
#   ├── var/                 # non-shipped intermediates (packages/release/cross)
#   ├── wrap/ · proxyctl/    # runtime state (owned by those tools)
#   └── env                  # optional PATH helper (written by install.sh)
#
# Usage:
#   # shellcheck source=scripts/lib/centag-layout.sh
#   source "$ROOT/scripts/lib/centag-layout.sh"
#   centag_layout_init            # or: centag_layout_use_edition personal
#
# Override: CENTAG_INSTALL_ROOT, CENTAG_BIN_DIR, CENTAG_EDITION

# Prevent double-init noise when sourced multiple times
_CENTAG_LAYOUT_LOADED=1

centag_layout_resolve_home() {
  if [[ -n "${HOME:-}" ]]; then
    printf '%s' "$HOME"
    return 0
  fi
  if command -v getent >/dev/null 2>&1; then
    getent passwd "$(id -un)" 2>/dev/null | cut -d: -f6
    return 0
  fi
  printf '%s' "$(cd ~ && pwd)"
}

# Populate path variables for the active edition (default: personal).
centag_layout_use_edition() {
  local edition="${1:-${CENTAG_EDITION:-personal}}"
  CENTAG_EDITION="$edition"
  export CENTAG_EDITION

  local home
  home="$(centag_layout_resolve_home)"
  CENTAG_INSTALL_ROOT="${CENTAG_INSTALL_ROOT:-${home}/.centag}"
  # Always re-bind paths to the active INSTALL_ROOT so a stale CENTAG_BIN_DIR
  # from a previous shell/session cannot point outside the new root.
  # Optional override: CENTAG_BIN_DIR_OVERRIDE=<dir>
  if [[ -n "${CENTAG_BIN_DIR_OVERRIDE:-}" ]]; then
    CENTAG_BIN_DIR="${CENTAG_BIN_DIR_OVERRIDE}"
  else
    CENTAG_BIN_DIR="${CENTAG_INSTALL_ROOT}/bin"
  fi
  CENTAG_LIB_DIR="${CENTAG_INSTALL_ROOT}/lib"
  CENTAG_VAR_DIR="${CENTAG_INSTALL_ROOT}/var"
  CENTAG_EDITION_LIB="${CENTAG_LIB_DIR}/${CENTAG_EDITION}"
  CENTAG_STATIC_DIR="${CENTAG_EDITION_LIB}/static"
  CENTAG_PACKAGES_DIR="${CENTAG_VAR_DIR}/packages"
  CENTAG_RELEASE_DIR="${CENTAG_VAR_DIR}/release"
  CENTAG_CROSS_DIR="${CENTAG_VAR_DIR}/cross"
  CENTAG_SERVER_BIN="centag-${CENTAG_EDITION}"

  export CENTAG_INSTALL_ROOT CENTAG_BIN_DIR CENTAG_LIB_DIR CENTAG_VAR_DIR
  export CENTAG_EDITION_LIB CENTAG_STATIC_DIR
  export CENTAG_PACKAGES_DIR CENTAG_RELEASE_DIR CENTAG_CROSS_DIR
  export CENTAG_SERVER_BIN
}

centag_layout_init() {
  centag_layout_use_edition "${CENTAG_EDITION:-personal}"
}

centag_bin_name() {
  local edition="${1:-${CENTAG_EDITION:-personal}}"
  printf 'centag-%s' "$edition"
}

centag_edition_lib() {
  local edition="${1:-${CENTAG_EDITION:-personal}}"
  printf '%s/lib/%s' "${CENTAG_INSTALL_ROOT}" "$edition"
}

centag_server_bin_path() {
  local edition="${1:-${CENTAG_EDITION:-personal}}"
  printf '%s/%s' "$(centag_edition_lib "$edition")" "$(centag_bin_name "$edition")"
}

# Ensure directories exist for the active (or given) edition.
centag_layout_ensure_dirs() {
  local edition="${1:-${CENTAG_EDITION:-personal}}"
  local lib
  lib="$(centag_edition_lib "$edition")"
  mkdir -p \
    "${CENTAG_BIN_DIR}" \
    "${lib}" \
    "${lib}/static" \
    "${lib}/storage" \
    "${CENTAG_PACKAGES_DIR}" \
    "${CENTAG_RELEASE_DIR}" \
    "${CENTAG_CROSS_DIR}"
}

# Write PATH wrapper (same contract as scripts/install.sh write_wrapper_centag).
# Args: edition [goos]
centag_write_wrapper() {
  local target_edition="$1"
  local goos="${2:-${GOOS:-}}"
  # Avoid `go env` here — GOTOOLCHAIN=auto may block on toolchain download.
  if [[ -z "$goos" ]]; then
    case "$(uname -s 2>/dev/null || true)" in
      MINGW*|MSYS*|CYGWIN*|Windows_NT) goos=windows ;;
      *) goos=unix ;;
    esac
  fi
  goos="$(printf '%s' "$goos" | tr '[:upper:]' '[:lower:]')"

  mkdir -p "${CENTAG_BIN_DIR}"

  if [[ "$goos" == "windows" ]]; then
    local cmd="${CENTAG_BIN_DIR}/centag.cmd"
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

  local wrapper="${CENTAG_BIN_DIR}/centag"
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

# Symlink bin/centag-<edition> → lib/<edition>/centag-<edition> and refresh wrapper.
centag_install_edition_links() {
  local edition="${1:-${CENTAG_EDITION:-personal}}"
  local ext="${2:-}"
  local lib bin_path
  lib="$(centag_edition_lib "$edition")"
  bin_path="${lib}/centag-${edition}${ext}"
  mkdir -p "${CENTAG_BIN_DIR}" "$lib"
  if [[ -e "$bin_path" || -L "$bin_path" ]]; then
    ln -sfn "$bin_path" "${CENTAG_BIN_DIR}/centag-${edition}${ext}"
  fi
  centag_write_wrapper "$edition"
}

# Copy host wrap binary into install bin/ (matches install.sh wrap layout).
centag_install_wrap_bin() {
  local src="$1"
  local ext="${2:-}"
  mkdir -p "${CENTAG_BIN_DIR}"
  cp -f "$src" "${CENTAG_BIN_DIR}/centag-wrap${ext}"
  chmod 755 "${CENTAG_BIN_DIR}/centag-wrap${ext}"
}
