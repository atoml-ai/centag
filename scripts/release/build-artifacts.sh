#!/usr/bin/env bash
# Build GitHub Release artifacts consumed by scripts/install.sh.
#
# Usage:
#   ./scripts/release/build-artifacts.sh [--version 0.2.7]
#   ./scripts/release/build-artifacts.sh --components personal
#   CENTAG_RELEASE_PLATFORMS=linux-amd64,darwin-arm64 ./scripts/release/build-artifacts.sh
#
# Outputs under ~/.centag/var/release/<version>/ (default components):
#   centag-cli-<edition>-<goos>-<goarch>.tar.gz   # CLI form; includes `centag wrap`
#   checksums.txt                                 # merged over all artifacts in OUT_DIR
#
# Does NOT wipe the whole OUT_DIR (desktop packages may coexist).
# Optional: wrap / minimal / desktop via --components.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/lib/centag-layout.sh
source "${ROOT}/scripts/lib/centag-layout.sh"
centag_layout_init
NPM_PKG="${ROOT}/apps/wrap-npm/package.json"

log() { echo "==> $*" >&2; }
fail() { echo "error: $*" >&2; exit 1; }

VERSION=""
COMPONENTS="personal"
PLATFORMS="${CENTAG_RELEASE_PLATFORMS:-darwin-amd64,darwin-arm64,linux-amd64,linux-arm64,windows-amd64,windows-arm64}"
SKIP_FRONTEND="${CENTAG_RELEASE_SKIP_FRONTEND:-0}"
BUILD_DESKTOP="${CENTAG_RELEASE_DESKTOP:-0}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --components) COMPONENTS="${2:-}"; shift 2 ;;
    --platforms) PLATFORMS="${2:-}"; shift 2 ;;
    --skip-frontend) SKIP_FRONTEND=1; shift ;;
    --desktop) BUILD_DESKTOP=1; shift ;;
    -h|--help)
      sed -n '2,24p' "$0"
      exit 0
      ;;
    *) fail "unknown arg: $1" ;;
  esac
done

if [[ -z "$VERSION" ]]; then
  if [[ -f "$NPM_PKG" ]] && command -v node >/dev/null 2>&1; then
    VERSION="$(node -p "require('${NPM_PKG}').version")"
  else
    VERSION="$(git -C "$ROOT" describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || true)"
  fi
fi
[[ -n "$VERSION" ]] || fail "version required (--version or apps/wrap-npm/package.json)"
VERSION="${VERSION#v}"

OUT_DIR="${CENTAG_RELEASE_DIR}/${VERSION}"
mkdir -p "$OUT_DIR"
# Only refresh our staging trees; never delete sibling desktop/other artifacts.
rm -rf "${OUT_DIR}/.build" "${OUT_DIR}/.stage"
mkdir -p "${OUT_DIR}/.build" "${OUT_DIR}/.stage"

IFS=',' read -r -a COMP_ARR <<< "$COMPONENTS"
IFS=',' read -r -a PLAT_ARR <<< "$PLATFORMS"

need_component() {
  local want="$1" c
  for c in "${COMP_ARR[@]}"; do
    c="$(printf '%s' "$c" | tr -d '[:space:]')"
    [[ "$c" == "$want" ]] && return 0
  done
  return 1
}

sha256_of() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    sha256sum "$1" | awk '{print $1}'
  fi
}

dist_tags() {
  case "$1" in
    minimal)
      echo "minimal,protocol_openai,protocol_anthropic,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic"
      ;;
    personal)
      echo "protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure"
      ;;
    *) fail "unknown edition: $1" ;;
  esac
}

HOST_GOOS="$(go env GOOS)"
HOST_GOARCH="$(go env GOARCH)"

# --- frontend (shared static) ---------------------------------------------
STATIC_SRC="${CENTAG_STATIC_DIR}"
if need_component personal || need_component minimal; then
  if [[ "$SKIP_FRONTEND" != "1" ]]; then
    log "building frontend → ${STATIC_SRC}"
    (
      cd "${ROOT}/web"
      export CENTAG_INSTALL_ROOT CENTAG_EDITION="$EDITION" CENTAG_STATIC_DIR="${STATIC_SRC}"
      if [[ -f package-lock.json ]]; then npm ci; else npm install; fi
      npm run build
    )
  fi
  [[ -d "$STATIC_SRC" ]] || fail "frontend static missing at ${STATIC_SRC} (build web or omit --skip-frontend)"
fi

package_tar() {
  local stage_parent="$1" stage_name="$2" out_tarball="$3"
  # macOS: strip Apple xattrs / provenance so Linux tar does not spam
  # "Ignoring unknown extended header keyword 'LIBARCHIVE.xattr…'".
  if [[ "$(uname -s 2>/dev/null || true)" == "Darwin" ]] && command -v xattr >/dev/null 2>&1; then
    xattr -cr "${stage_parent}/${stage_name}" 2>/dev/null || true
  fi
  (
    cd "$stage_parent"
    export COPYFILE_DISABLE=1
    # Prefer portable flags when available (bsdtar / GNU tar).
    if tar --help 2>&1 | grep -q -- '--no-xattrs'; then
      if tar --help 2>&1 | grep -q -- '--no-mac-metadata'; then
        tar --no-xattrs --no-mac-metadata -czf "$out_tarball" "$stage_name"
      else
        tar --no-xattrs -czf "$out_tarball" "$stage_name"
      fi
    else
      tar -czf "$out_tarball" "$stage_name"
    fi
  )
}

# --- personal / minimal ---------------------------------------------------
build_edition() {
  local edition="$1" goos="$2" goarch="$3"
  local ext="" tags out_bin stage_parent stage_name tarball
  if [[ "$goos" == "windows" ]]; then ext=".exe"; fi
  tags="$(dist_tags "$edition")"
  out_bin="${OUT_DIR}/.build/${edition}-${goos}-${goarch}/centag-${edition}${ext}"
  mkdir -p "$(dirname "$out_bin")"

  log "build centag-${edition} ${goos}/${goarch}"
  local build_time ldflags
  build_time="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  # Inject version into main.Version / main.BuildTime for `centag version`
  ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${build_time}'"
  (
    cd "${ROOT}/dist/${edition}"
    GOWORK=off CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -trimpath -tags "$tags" -ldflags="$ldflags" -o "$out_bin" .
  )

  stage_parent="${OUT_DIR}/.stage"
  stage_name="centag-cli-${edition}-${goos}-${goarch}"
  rm -rf "${stage_parent}/${stage_name}"
  mkdir -p "${stage_parent}/${stage_name}"
  cp -f "$out_bin" "${stage_parent}/${stage_name}/centag-${edition}${ext}"
  chmod 755 "${stage_parent}/${stage_name}/centag-${edition}${ext}"
  cp -R "$STATIC_SRC" "${stage_parent}/${stage_name}/static"

  # Ship pipeline/backends seed beside the binary (one-line install → ~/.centag/lib/<edition>/).
  # personal/minimal: only pipeline-templates/common/
  # team (when packaged here): common/ + team/
  if [[ -d "${ROOT}/config/initdata" ]]; then
    mkdir -p "${stage_parent}/${stage_name}/config"
    rm -rf "${stage_parent}/${stage_name}/config/initdata"
    cp -R "${ROOT}/config/initdata" "${stage_parent}/${stage_name}/config/initdata"
    rm -rf "${stage_parent}/${stage_name}/config/initdata/postgresql" \
      "${stage_parent}/${stage_name}/config/initdata/scripts" \
      "${stage_parent}/${stage_name}/config/initdata/update" \
      "${stage_parent}/${stage_name}/config/initdata/secrets" 2>/dev/null || true
    find "${stage_parent}/${stage_name}/config/initdata" \( -name 'README.md' -o -name 'AGENTS.md' \) -delete 2>/dev/null || true
    # Drop obsolete personal/ dir name if still present; strip team/ for non-team editions.
    rm -rf "${stage_parent}/${stage_name}/config/initdata/pipeline-templates/personal"
    case "$edition" in
      personal|minimal)
        rm -rf "${stage_parent}/${stage_name}/config/initdata/pipeline-templates/team"
        ;;
    esac
  fi
  # Profile overlay (initial-backends + common pipeline overrides only)
  if [[ -d "${ROOT}/config/profiles/${edition}/initdata" ]]; then
    mkdir -p "${stage_parent}/${stage_name}/config/profiles/${edition}"
    rm -rf "${stage_parent}/${stage_name}/config/profiles/${edition}/initdata"
    cp -R "${ROOT}/config/profiles/${edition}/initdata" \
      "${stage_parent}/${stage_name}/config/profiles/${edition}/initdata"
    rm -rf "${stage_parent}/${stage_name}/config/profiles/${edition}/initdata/pipeline-templates/personal" \
      "${stage_parent}/${stage_name}/config/profiles/${edition}/initdata/pipeline-templates/team"
  fi

  # Ship billing/pricing seed data (default pricing rules)
  if [[ -d "${ROOT}/config/pricing" ]]; then
    rm -rf "${stage_parent}/${stage_name}/config/pricing"
    mkdir -p "${stage_parent}/${stage_name}/config/pricing"
    cp -R "${ROOT}/config/pricing/." "${stage_parent}/${stage_name}/config/pricing/"
  fi

  tarball="${OUT_DIR}/centag-cli-${edition}-${goos}-${goarch}.tar.gz"
  # Replace only this CLI artifact (do not touch desktop / other OS packages).
  rm -f "$tarball" \
    "${OUT_DIR}/centag-${edition}-${goos}-${goarch}.tar.gz"
  package_tar "$stage_parent" "$stage_name" "$tarball"
  log "OK ${tarball}"
}

# --- wrap -------------------------------------------------------------
build_wrap() {
  local goos="$1" goarch="$2"
  local ext="" out_bin stage_parent stage_name tarball
  if [[ "$goos" == "windows" ]]; then ext=".exe"; fi
  out_bin="${OUT_DIR}/.build/wrap-${goos}-${goarch}/centag-wrap${ext}"
  mkdir -p "$(dirname "$out_bin")"

  log "build centag-wrap ${goos}/${goarch}"
  (
    cd "${ROOT}/apps/wrap"
    GOWORK=off CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -trimpath -ldflags="-s -w" -o "$out_bin" .
  )

  stage_parent="${OUT_DIR}/.stage"
  stage_name="centag-wrap-${goos}-${goarch}"
  rm -rf "${stage_parent}/${stage_name}"
  mkdir -p "${stage_parent}/${stage_name}"
  cp -f "$out_bin" "${stage_parent}/${stage_name}/centag-wrap${ext}"
  chmod 755 "${stage_parent}/${stage_name}/centag-wrap${ext}"

  tarball="${OUT_DIR}/centag-wrap-${goos}-${goarch}.tar.gz"
  package_tar "$stage_parent" "$stage_name" "$tarball"
  log "OK ${tarball}"
}

# --- launcher lite (cross-compile, no CGO) --------------------------------
build_launcher_lite() {
  local goos="$1" goarch="$2"
  local ext="" out_bin stage_parent stage_name tarball
  if [[ "$goos" == "windows" ]]; then ext=".exe"; fi

  log "build centag-launcher (lite) ${goos}/${goarch}"
  (
    cd "$ROOT"
    CENTAG_LAUNCHER_GOOS="$goos" CENTAG_LAUNCHER_GOARCH="$goarch" \
      bash scripts/build-launcher.sh
  )
  out_bin="${CENTAG_CROSS_DIR}/launcher/${goos}-${goarch}/centag-launcher${ext}"
  [[ -f "$out_bin" ]] || fail "launcher binary missing: $out_bin"

  stage_parent="${OUT_DIR}/.stage"
  stage_name="centag-launcher-${goos}-${goarch}"
  rm -rf "${stage_parent}/${stage_name}"
  mkdir -p "${stage_parent}/${stage_name}"
  cp -f "$out_bin" "${stage_parent}/${stage_name}/centag-launcher${ext}"
  chmod 755 "${stage_parent}/${stage_name}/centag-launcher${ext}"

  tarball="${OUT_DIR}/centag-launcher-${goos}-${goarch}.tar.gz"
  package_tar "$stage_parent" "$stage_name" "$tarball"
  log "OK ${tarball}"
}

# --- desktop shell binary only (host/CGO; full .app/.zip via package-desktop.sh)
build_desktop_host() {
  local goos="$HOST_GOOS" goarch="$HOST_GOARCH"
  local ext="" out_bin stage_parent stage_name tarball
  if [[ "$goos" == "windows" ]]; then ext=".exe"; fi

  log "build centag-desktop ${goos}/${goarch} (host/CGO)"
  (
    cd "$ROOT"
    CENTAG_LAUNCHER_GOOS="$goos" CENTAG_LAUNCHER_GOARCH="$goarch" \
      bash scripts/build-launcher.sh --desktop
  )
  out_bin="${CENTAG_CROSS_DIR}/launcher/${goos}-${goarch}/centag-desktop${ext}"
  [[ -f "$out_bin" ]] || fail "desktop binary missing: $out_bin"

  # Optional raw-shell tarball (not the product centag-desktop-<edition>-*.zip/dmg).
  stage_parent="${OUT_DIR}/.stage"
  stage_name="centag-desktop-shell-${goos}-${goarch}"
  rm -rf "${stage_parent}/${stage_name}"
  mkdir -p "${stage_parent}/${stage_name}"
  cp -f "$out_bin" "${stage_parent}/${stage_name}/centag-desktop${ext}"
  chmod 755 "${stage_parent}/${stage_name}/centag-desktop${ext}"

  tarball="${OUT_DIR}/centag-desktop-shell-${goos}-${goarch}.tar.gz"
  package_tar "$stage_parent" "$stage_name" "$tarball"
  log "OK ${tarball}"
}

# --- drive builds ---------------------------------------------------------
command -v go >/dev/null 2>&1 || fail "go is required"

if need_component desktop; then
  BUILD_DESKTOP=1
fi

for plat in "${PLAT_ARR[@]}"; do
  plat="$(printf '%s' "$plat" | tr -d '[:space:]')"
  [[ -z "$plat" ]] && continue
  goos="${plat%-*}"
  goarch="${plat##*-}"

  if need_component personal; then
    build_edition personal "$goos" "$goarch"
  fi
  if need_component minimal; then
    build_edition minimal "$goos" "$goarch"
  fi
  if need_component wrap; then
    build_wrap "$goos" "$goarch"
  fi
  if need_component launcher; then
    build_launcher_lite "$goos" "$goarch"
  fi
done

if [[ "$BUILD_DESKTOP" == "1" ]]; then
  build_desktop_host
fi

# --- checksums (merge all release artifacts in OUT_DIR) -------------------
refresh_release_checksums() {
  local dir="$1"
  log "checksums.txt (all artifacts in ${dir})"
  : > "${dir}/checksums.txt"
  while IFS= read -r path; do
    [[ -f "$path" ]] || continue
    base="$(basename "$path")"
    printf '%s  %s\n' "$(sha256_of "$path")" "$base" >> "${dir}/checksums.txt"
  done < <(find "$dir" -maxdepth 1 -type f \( \
    -name 'centag-cli-*.tar.gz' -o \
    -name 'centag-desktop-*.dmg' -o \
    -name 'centag-desktop-*.zip' -o \
    -name 'centag-desktop-shell-*.tar.gz' -o \
    -name 'centag-wrap-*.tar.gz' -o \
    -name 'centag-launcher*.tar.gz' -o \
    -name 'centag-personal-*.tar.gz' -o \
    -name 'centag-minimal-*.tar.gz' -o \
    -name 'Centag-*.dmg' -o \
    -name 'Centag-*.zip' \
  \) | sort)
  cat "${dir}/checksums.txt" >&2
}

refresh_release_checksums "$OUT_DIR"

# cleanup staging
rm -rf "${OUT_DIR}/.build" "${OUT_DIR}/.stage"

log "artifacts in ${OUT_DIR}"
ls -lh "$OUT_DIR" >&2
echo "$OUT_DIR"
