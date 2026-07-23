#!/usr/bin/env bash
# scripts/publish-centag-npm.sh
#
# Build & publish the centag npm distribution.
#
# What it does:
#   1. Cross-compile the Go binary (dist/personal) for darwin/linux/windows × amd64/arm64
#      into apps/centag-npm/bin/vendor/<goos>-<goarch>/centag-personal[.exe]
#   2. Copy config/initdata seed data into each platform directory.
#   3. Generate apps/centag-npm/bin/vendor/checksums.txt (sha256).
#   4. Build the Vue frontend → apps/centag-npm/static/.
#   5. Pack the main npm package (centag) — lazy-download, small.
#   6. Pack the offline variant (centag-offline) — bundles all binaries.
#   7. Optionally draft a GitHub Release (v<version>) uploading the binaries + checksums.
#
# Usage:
#   ./scripts/publish-centag-npm.sh                 # build + pack only
#   ./scripts/publish-centag-npm.sh --release       # also draft GitHub release
#   CENTAG_NPM_TOKEN=xxx ./scripts/publish-centag-npm.sh --release
#
# Env:
#   CENTAG_NPM_TOKEN   npm token for `npm publish` (skipped if empty)
#   GH_TOKEN            GitHub token for release upload (with --release)
#   DRY_RUN=1           build + pack, but do not publish / release
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/centag-layout.sh
source "${ROOT}/scripts/lib/centag-layout.sh"
centag_layout_init
NPM_DIR="${ROOT}/apps/centag-npm"
VENDOR_DIR="${NPM_DIR}/bin/vendor"
DIST_DIR="${ROOT}/dist/personal"
OUT_ROOT="${CENTAG_CROSS_DIR}/centag-npm"

PLATFORMS=(darwin-amd64 darwin-arm64 linux-amd64 linux-arm64 windows-amd64 windows-arm64)

# Version is sourced from the npm package.json (kept in lockstep with Centag).
VERSION="$(node -p "require('${NPM_DIR}/package.json').version")"
RELEASE_TAG="v${VERSION}"

RELEASE=0
[[ "${1:-}" == "--release" ]] && RELEASE=1

if ! command -v go >/dev/null 2>&1; then echo "error: go is required" >&2; exit 1; fi
if ! command -v npm >/dev/null 2>&1; then echo "error: npm is required" >&2; exit 1; fi

echo "==> version: ${VERSION} (release tag ${RELEASE_TAG})"

# --- 1. Cross-compile -------------------------------------------------------
mkdir -p "${VENDOR_DIR}"
for p in "${PLATFORMS[@]}"; do
  goos="${p%-*}"; goarch="${p##*-}"
  ext=""; [[ "$goos" == "windows" ]] && ext=".exe"
  out="${VENDOR_DIR}/${p}/centag-personal${ext}"
  mkdir -p "$(dirname "$out")"
  echo "==> build centag-personal ${p}"
  (
    cd "${DIST_DIR}"
    # Build tags match personal edition (same as dist/personal/build.sh).
    TAGS="${BUILD_TAGS:-protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure}"
    GOWORK=off CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
      go build -trimpath -tags "$TAGS" -ldflags="-s -w" -o "${out}" .
  )
done

# --- 2. Copy config/initdata seed data --------------------------------------
echo "==> copying config seed data"
INITDATA_SRC="${ROOT}/config/initdata"
PROFILES_SRC="${ROOT}/config/profiles"
for p in "${PLATFORMS[@]}"; do
  dest="${VENDOR_DIR}/${p}/config"
  mkdir -p "${dest}"
  if [[ -d "${INITDATA_SRC}" ]]; then
    rm -rf "${dest}/initdata"
    cp -R "${INITDATA_SRC}" "${dest}/initdata"
    # Strip non-personal seed data (same as build-artifacts.sh)
    rm -rf "${dest}/initdata/postgresql" \
      "${dest}/initdata/scripts" \
      "${dest}/initdata/update" \
      "${dest}/initdata/secrets" 2>/dev/null || true
    find "${dest}/initdata" \( -name 'README.md' -o -name 'AGENTS.md' \) -delete 2>/dev/null || true
    rm -rf "${dest}/initdata/pipeline-templates/personal"
    rm -rf "${dest}/initdata/pipeline-templates/team"
  fi
  if [[ -d "${PROFILES_SRC}/personal/initdata" ]]; then
    mkdir -p "${dest}/profiles/personal"
    rm -rf "${dest}/profiles/personal/initdata"
    cp -R "${PROFILES_SRC}/personal/initdata" "${dest}/profiles/personal/initdata"
    rm -rf "${dest}/profiles/personal/initdata/pipeline-templates/personal" \
      "${dest}/profiles/personal/initdata/pipeline-templates/team"
  fi
done

# --- 3. Checksums -----------------------------------------------------------
echo "==> checksums.txt"
: > "${VENDOR_DIR}/checksums.txt"
for p in "${PLATFORMS[@]}"; do
  goos="${p%-*}"; goarch="${p##*-}"
  ext=""; [[ "$goos" == "windows" ]] && ext=".exe"
  f="${VENDOR_DIR}/${p}/centag-personal${ext}"
  sha="$(shasum -a 256 "$f" | awk '{print $1}')"
  printf '%s  %s/centag-personal%s\n' "$sha" "$p" "$ext" >> "${VENDOR_DIR}/checksums.txt"
done
cat "${VENDOR_DIR}/checksums.txt"

# --- 4. Build frontend ------------------------------------------------------
echo "==> building frontend"
STATIC_DIR="${NPM_DIR}/static"
mkdir -p "${STATIC_DIR}"
(
  cd "${ROOT}/web"
  if [[ -f package-lock.json ]]; then npm ci; else npm install; fi
  CENTAG_INSTALL_ROOT="${ROOT}" CENTAG_EDITION=personal CENTAG_STATIC_DIR="${STATIC_DIR}" npm run build
)
[[ -f "${STATIC_DIR}/index.html" ]] || { echo "error: frontend build failed (missing index.html in ${STATIC_DIR})" >&2; exit 1; }

# --- 5. Pack main npm package (lazy-download: NO bundled binaries) ----------
echo "==> pack centag (lazy-download, no bundled binaries)"
MAIN_STAGE="$(mktemp -d)"
cp -R "${NPM_DIR}/bin" "${MAIN_STAGE}/bin"
cp -R "${NPM_DIR}/lib" "${MAIN_STAGE}/lib"
cp -R "${NPM_DIR}/static" "${MAIN_STAGE}/static"
cp "${NPM_DIR}/README.md" "${MAIN_STAGE}/README.md"
cp "${NPM_DIR}/package.json" "${MAIN_STAGE}/package.json"
cp "${NPM_DIR}/.npmignore" "${MAIN_STAGE}/.npmignore"
# Strip the cross-compiled binaries so the main package stays small.
rm -rf "${MAIN_STAGE}/bin/vendor"
mkdir -p "${OUT_ROOT}"
( cd "${MAIN_STAGE}" && npm pack --dry-run )
MAIN_TGZ="$(cd "${MAIN_STAGE}" && npm pack | tail -1)"
mv "${MAIN_STAGE}/${MAIN_TGZ}" "${OUT_ROOT}/${MAIN_TGZ}"
rm -rf "${MAIN_STAGE}"

# --- 6. Pack offline variant ------------------------------------------------
echo "==> build centag-offline (bundled binaries)"
OFFLINE_STAGE="$(mktemp -d)"
cp -R "${NPM_DIR}/bin" "${OFFLINE_STAGE}/bin"
cp -R "${NPM_DIR}/lib" "${OFFLINE_STAGE}/lib"
cp -R "${NPM_DIR}/static" "${OFFLINE_STAGE}/static"
cp "${NPM_DIR}/README.md" "${OFFLINE_STAGE}/README.md"
cp "${NPM_DIR}/package.offline.json" "${OFFLINE_STAGE}/package.json"
# Do NOT copy .npmignore — offline variant must include bin/vendor/ and static/
( cd "${OFFLINE_STAGE}" && npm pack --dry-run )
OFFLINE_TGZ="$(cd "${OFFLINE_STAGE}" && npm pack | tail -1)"
mv "${OFFLINE_STAGE}/${OFFLINE_TGZ}" "${OUT_ROOT}/${OFFLINE_TGZ}"
rm -rf "${OFFLINE_STAGE}"

echo "==> artifacts:"
ls -lh "${OUT_ROOT}"/*.tgz 2>/dev/null || echo "(no .tgz in ${OUT_ROOT})"

# --- 7. Publish -------------------------------------------------------------
if [[ "${DRY_RUN:-}" == "1" ]]; then
  echo "==> DRY_RUN: skipping npm publish and GitHub release"
  exit 0
fi

# Re-pack for publish (use clean staging to ensure correct contents)
publish_centag() {
  local stage="$(mktemp -d)"
  cp -R "${NPM_DIR}/bin" "${stage}/bin"
  cp -R "${NPM_DIR}/lib" "${stage}/lib"
  cp -R "${NPM_DIR}/static" "${stage}/static"
  cp "${NPM_DIR}/README.md" "${stage}/README.md"
  cp "${NPM_DIR}/package.json" "${stage}/package.json"
  cp "${NPM_DIR}/install.js" "${stage}/install.js"
  cp "${NPM_DIR}/.npmignore" "${stage}/.npmignore"
  rm -rf "${stage}/bin/vendor"
  if [[ -n "${CENTAG_NPM_TOKEN:-}" ]]; then
    ( cd "${stage}" && npm publish --access public \
      --//registry.npmjs.org/:_authToken="${CENTAG_NPM_TOKEN}" )
  else
    ( cd "${stage}" && npm publish --access public )
  fi
  rm -rf "${stage}"
}

publish_centag_offline() {
  local stage="$(mktemp -d)"
  cp -R "${NPM_DIR}/bin" "${stage}/bin"
  cp -R "${NPM_DIR}/lib" "${stage}/lib"
  cp -R "${NPM_DIR}/static" "${stage}/static"
  cp "${NPM_DIR}/README.md" "${stage}/README.md"
  cp "${NPM_DIR}/package.offline.json" "${stage}/package.json"
  # Do NOT copy .npmignore — offline variant must include bin/vendor/ and static/
  if [[ -n "${CENTAG_NPM_TOKEN:-}" ]]; then
    ( cd "${stage}" && npm publish --access public \
      --//registry.npmjs.org/:_authToken="${CENTAG_NPM_TOKEN}" )
  else
    ( cd "${stage}" && npm publish --access public )
  fi
  rm -rf "${stage}"
}

if [[ -n "${CENTAG_NPM_TOKEN:-}" ]]; then
  echo "==> npm publish centag (using CENTAG_NPM_TOKEN)"
  publish_centag
  echo "==> npm publish centag-offline (using CENTAG_NPM_TOKEN)"
  publish_centag_offline
elif npm whoami >/dev/null 2>&1; then
  echo "==> npm publish centag (using npm login session)"
  publish_centag
  echo "==> npm publish centag-offline (using npm login session)"
  publish_centag_offline
else
  echo "==> skipping npm publish (no CENTAG_NPM_TOKEN and not logged in)"
fi

if [[ "$RELEASE" == "1" ]]; then
  echo "==> building assets for npm (GitHub Release)"
  bash "${ROOT}/scripts/release/publish-binaries.sh" \
    --version "${VERSION}" \
    --components personal \
    --release
fi

echo "OK"
