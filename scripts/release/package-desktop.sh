#!/usr/bin/env bash
# Build host-native desktop (or Linux CLI) package for personal edition.
#
# Product form (desktop; does not wipe CLI artifacts in the same OUT_DIR):
#   darwin  → Centag.app + centag-desktop-<edition>-macos-<arch>.{dmg,zip}
#   windows → centag-desktop-<edition>-windows-<arch>.zip
#   linux   → redirects conceptually to CLI; emits centag-cli-<edition>-linux-<arch>.tar.gz
#
# Usage:
#   ./scripts/release/package-desktop.sh [--version 0.2.7] [--skip-frontend]
#
# Output: ~/.centag/var/release/<version>/
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/lib/centag-layout.sh
source "${ROOT}/scripts/lib/centag-layout.sh"
centag_layout_init
NPM_PKG="${ROOT}/apps/wrap-npm/package.json"

log() { echo "==> $*" >&2; }
fail() { echo "error: $*" >&2; exit 1; }

VERSION=""
EDITION="personal"
SKIP_FRONTEND="${CENTAG_RELEASE_SKIP_FRONTEND:-0}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --edition) EDITION="${2:-}"; shift 2 ;;
    --skip-frontend) SKIP_FRONTEND=1; shift ;;
    -h|--help)
      sed -n '2,16p' "$0"
      exit 0
      ;;
    *) fail "unknown arg: $1" ;;
  esac
done

case "$EDITION" in
  personal|minimal) ;;
  *) fail "edition must be personal|minimal (got: $EDITION)" ;;
esac

if [[ -z "$VERSION" ]]; then
  if [[ -f "$NPM_PKG" ]] && command -v node >/dev/null 2>&1; then
    VERSION="$(node -p "require('${NPM_PKG}').version")"
  else
    VERSION="$(git -C "$ROOT" describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || true)"
  fi
fi
[[ -n "$VERSION" ]] || fail "version required (--version or apps/wrap-npm/package.json)"
VERSION="${VERSION#v}"

command -v go >/dev/null 2>&1 || fail "go is required"

HOST_GOOS="$(go env GOOS)"
HOST_GOARCH="$(go env GOARCH)"
OUT_DIR="${CENTAG_RELEASE_DIR}/${VERSION}"
STAGE="${OUT_DIR}/.desktop-stage"
mkdir -p "$OUT_DIR"
rm -rf "$STAGE"
mkdir -p "$STAGE"

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
      echo "minimal,protocol_openai,protocol_anthropic,backend_openai,backend_ollama,backend_anthropic"
      ;;
    personal)
      echo "protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure"
      ;;
    *) fail "unknown edition: $1" ;;
  esac
}

# --- frontend -------------------------------------------------------------
STATIC_SRC="${CENTAG_STATIC_DIR}"
if [[ "$SKIP_FRONTEND" != "1" ]]; then
  log "building frontend → ${STATIC_SRC}"
  (
    cd "${ROOT}/web"
    export CENTAG_INSTALL_ROOT CENTAG_EDITION="$EDITION" CENTAG_STATIC_DIR="${STATIC_SRC}"
    if [[ -f package-lock.json ]]; then npm ci; else npm install; fi
    npm run build
  )
fi
[[ -d "$STATIC_SRC" ]] || fail "frontend static missing at ${STATIC_SRC} (build web or pass --skip-frontend)"

# --- sidecar tree ---------------------------------------------------------
stage_sidecar_tree() {
  local dest="$1"
  local ext="" tags out_bin build_time ldflags
  if [[ "$HOST_GOOS" == "windows" ]]; then ext=".exe"; fi
  tags="$(dist_tags "$EDITION")"
  out_bin="${STAGE}/centag-${EDITION}${ext}"
  build_time="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${build_time}'"

  log "build centag-${EDITION} ${HOST_GOOS}/${HOST_GOARCH}"
  (
    cd "${ROOT}/dist/${EDITION}"
    # Allow go.mod toolchain download (e.g. 1.25) when host go is older.
    GOWORK=off CGO_ENABLED=0 GOOS="$HOST_GOOS" GOARCH="$HOST_GOARCH" \
      go build -trimpath -tags "$tags" -ldflags="$ldflags" -o "$out_bin" .
  ) || fail "sidecar build failed for ${EDITION}"
  [[ -f "$out_bin" ]] || fail "sidecar binary missing: $out_bin"

  mkdir -p "$dest"
  cp -f "$out_bin" "${dest}/centag-${EDITION}${ext}"
  chmod 755 "${dest}/centag-${EDITION}${ext}"
  cp -R "$STATIC_SRC" "${dest}/static"

  if [[ -d "${ROOT}/config/initdata" ]]; then
    mkdir -p "${dest}/config"
    rm -rf "${dest}/config/initdata"
    cp -R "${ROOT}/config/initdata" "${dest}/config/initdata"
    rm -rf "${dest}/config/initdata/postgresql" \
      "${dest}/config/initdata/scripts" \
      "${dest}/config/initdata/update" \
      "${dest}/config/initdata/secrets" 2>/dev/null || true
    find "${dest}/config/initdata" \( -name 'README.md' -o -name 'AGENTS.md' \) -delete 2>/dev/null || true
    rm -rf "${dest}/config/initdata/pipeline-templates/personal"
    case "$EDITION" in
      personal|minimal)
        rm -rf "${dest}/config/initdata/pipeline-templates/team"
        ;;
    esac
  fi
  if [[ -d "${ROOT}/config/profiles/${EDITION}/initdata" ]]; then
    mkdir -p "${dest}/config/profiles/${EDITION}"
    rm -rf "${dest}/config/profiles/${EDITION}/initdata"
    cp -R "${ROOT}/config/profiles/${EDITION}/initdata" \
      "${dest}/config/profiles/${EDITION}/initdata"
    rm -rf "${dest}/config/profiles/${EDITION}/initdata/pipeline-templates/personal" \
      "${dest}/config/profiles/${EDITION}/initdata/pipeline-templates/team"
  fi
}

build_tray() {
  log "build centag-launcher-tray ${HOST_GOOS}/${HOST_GOARCH} (CGO)"
  local out_bin
  out_bin="$(
    cd "$ROOT"
    CENTAG_LAUNCHER_GOOS="$HOST_GOOS" CENTAG_LAUNCHER_GOARCH="$HOST_GOARCH" \
      bash scripts/build-launcher.sh --tray
  )"
  out_bin="$(printf '%s' "$out_bin" | tail -n 1)"
  [[ -f "$out_bin" ]] || fail "tray binary missing: $out_bin"
  echo "$out_bin"
}

write_info_plist() {
  local plist="$1"
  cat > "$plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>en</string>
  <key>CFBundleExecutable</key>
  <string>Centag</string>
  <key>CFBundleIdentifier</key>
  <string>ai.atoml.centag</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>Centag</string>
  <key>CFBundleDisplayName</key>
  <string>Centag</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>${VERSION}</string>
  <key>CFBundleVersion</key>
  <string>${VERSION}</string>
  <key>LSMinimumSystemVersion</key>
  <string>12.0</string>
  <key>NSHighResolutionCapable</key>
  <true/>
  <key>LSUIElement</key>
  <true/>
</dict>
</plist>
EOF
}

package_macos() {
  local tray_bin="$1"
  local app="${STAGE}/Centag.app"
  local macos_dir="${app}/Contents/MacOS"
  local res_dir="${app}/Contents/Resources"
  rm -rf "$app"
  mkdir -p "$macos_dir" "$res_dir"

  stage_sidecar_tree "$res_dir"
  cp -f "$tray_bin" "${macos_dir}/Centag"
  chmod 755 "${macos_dir}/Centag"
  write_info_plist "${app}/Contents/Info.plist"

  # Local convenience copy (not a release asset name).
  local app_out="${OUT_DIR}/Centag.app"
  rm -rf "$app_out"
  cp -R "$app" "$app_out"
  log "OK ${app_out}"

  local vol_name="Centag ${VERSION}"
  local dmg_stage="${STAGE}/dmg"
  local dmg_out="${OUT_DIR}/centag-desktop-${EDITION}-macos-${HOST_GOARCH}.dmg"
  local zip_out="${OUT_DIR}/centag-desktop-${EDITION}-macos-${HOST_GOARCH}.zip"
  rm -rf "$dmg_stage"
  mkdir -p "$dmg_stage"
  cp -R "$app" "${dmg_stage}/Centag.app"
  ln -s /Applications "${dmg_stage}/Applications"

  # Drop legacy desktop names if present.
  rm -f \
    "${OUT_DIR}/Centag-${VERSION}-macos-${HOST_GOARCH}.dmg" \
    "${OUT_DIR}/Centag-${VERSION}-macos-${HOST_GOARCH}.zip" \
    "$dmg_out" "$zip_out"

  if command -v zip >/dev/null 2>&1; then
    (
      cd "$STAGE"
      zip -qr "$zip_out" "Centag.app"
    )
    log "OK ${zip_out}"
  else
    log "warn: zip not found; skipping macOS zip (dmg only)"
  fi

  log "creating dmg → ${dmg_out}"
  hdiutil create \
    -volname "$vol_name" \
    -srcfolder "$dmg_stage" \
    -ov -format UDZO \
    "$dmg_out" >/dev/null
  log "OK ${dmg_out}"
  echo "$dmg_out"
}

package_windows() {
  local tray_bin="$1"
  local dir_name="Centag"
  local stage_dir="${STAGE}/${dir_name}"
  rm -rf "$stage_dir"
  mkdir -p "$stage_dir"

  stage_sidecar_tree "$stage_dir"
  cp -f "$tray_bin" "${stage_dir}/Centag.exe"
  chmod 755 "${stage_dir}/Centag.exe" 2>/dev/null || true

  local zip_out="${OUT_DIR}/centag-desktop-${EDITION}-windows-${HOST_GOARCH}.zip"
  rm -f \
    "${OUT_DIR}/Centag-${VERSION}-windows-${HOST_GOARCH}.zip" \
    "$zip_out"
  if command -v zip >/dev/null 2>&1; then
    (
      cd "$STAGE"
      zip -qr "$zip_out" "$dir_name"
    )
  elif command -v powershell.exe >/dev/null 2>&1 || command -v powershell >/dev/null 2>&1; then
    local ps=powershell
    command -v powershell.exe >/dev/null 2>&1 && ps=powershell.exe
    (
      cd "$STAGE"
      "$ps" -NoProfile -Command "Compress-Archive -Path '${dir_name}' -DestinationPath '${zip_out}' -Force"
    )
  else
    fail "zip or powershell Compress-Archive required for windows packaging"
  fi
  log "OK ${zip_out}"
  echo "$zip_out"
}

package_linux_cli() {
  # desktop entry on linux is unsupported; emit CLI artifact with cli naming.
  local stage_name="centag-cli-${EDITION}-linux-${HOST_GOARCH}"
  local stage_dir="${STAGE}/${stage_name}"
  rm -rf "$stage_dir"
  stage_sidecar_tree "$stage_dir"

  local tarball="${OUT_DIR}/${stage_name}.tar.gz"
  rm -f "$tarball" "${OUT_DIR}/centag-${EDITION}-linux-${HOST_GOARCH}.tar.gz"
  (
    cd "$STAGE"
    export COPYFILE_DISABLE=1
    tar -czf "$tarball" "$stage_name"
  )
  log "OK ${tarball}"
  echo "$tarball"
}

ARTIFACT=""
case "$HOST_GOOS" in
  darwin)
    TRAY="$(build_tray)"
    ARTIFACT="$(package_macos "$TRAY")"
    ;;
  windows)
    TRAY="$(build_tray)"
    ARTIFACT="$(package_windows "$TRAY")"
    ;;
  linux)
    ARTIFACT="$(package_linux_cli)"
    ;;
  *)
    fail "unsupported host GOOS=$HOST_GOOS"
    ;;
esac

# Merge checksums for all coexisting artifacts (cli + desktop).
log "checksums.txt (all artifacts in ${OUT_DIR})"
: > "${OUT_DIR}/checksums.txt"
while IFS= read -r path; do
  [[ -f "$path" ]] || continue
  base="$(basename "$path")"
  printf '%s  %s\n' "$(sha256_of "$path")" "$base" >> "${OUT_DIR}/checksums.txt"
done < <(find "$OUT_DIR" -maxdepth 1 -type f \( \
  -name 'centag-cli-*.tar.gz' -o \
  -name 'centag-desktop-*.dmg' -o \
  -name 'centag-desktop-*.zip' -o \
  -name 'centag-personal-*.tar.gz' -o \
  -name 'Centag-*.dmg' -o \
  -name 'Centag-*.zip' \
\) | sort)
cat "${OUT_DIR}/checksums.txt" >&2

rm -rf "$STAGE"
log "desktop package ready in ${OUT_DIR}"
ls -lh "$OUT_DIR" >&2
echo "$OUT_DIR"
