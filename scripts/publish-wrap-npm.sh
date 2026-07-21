#!/usr/bin/env bash
# scripts/publish-wrap-npm.sh
#
# Build & publish the centag-wrap npm distribution.
#
# What it does:
#   1. Cross-compile the Go binary (apps/wrap) for darwin/linux/windows × amd64/arm64
#      into apps/wrap-npm/bin/vendor/<goos>-<goarch>/centag-wrap[.exe]
#      (reusing the same naming as scripts/build-wrap.sh).
#   2. Generate apps/wrap-npm/bin/vendor/checksums.txt (sha256).
#   3. Pack the main npm package (centag-wrap) — lazy-download, small.
#   4. Pack the offline variant (centag-wrap-offline) — bundles all binaries.
#   5. Optionally draft a GitHub Release (v<version>) uploading the binaries + checksums.
#
# Usage:
#   ./scripts/publish-wrap-npm.sh                 # build + pack only
#   ./scripts/publish-wrap-npm.sh --release       # also draft GitHub release
#   CENTAG_WRAP_NPM_TOKEN=xxx ./scripts/publish-wrap-npm.sh --release
#
# Env:
#   CENTAG_WRAP_NPM_TOKEN   npm token for `npm publish` (skipped if empty)
#   GH_TOKEN                    GitHub token for release upload (with --release)
#   DRY_RUN=1                   build + pack, but do not publish / release
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROXYCTL_DIR="${ROOT}/apps/wrap"
NPM_DIR="${ROOT}/apps/wrap-npm"
VENDOR_DIR="${NPM_DIR}/bin/vendor"
OUT_ROOT="${ROOT}/bin/wrap"

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
  out="${VENDOR_DIR}/${p}/centag-wrap${ext}"
  mkdir -p "$(dirname "$out")"
  echo "==> build ${p}"
  (
    cd "${PROXYCTL_DIR}"
    GOWORK=off CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
      go build -trimpath -ldflags="-s -w" -o "${out}" .
  )
done

# --- 2. Checksums -----------------------------------------------------------
echo "==> checksums.txt"
: > "${VENDOR_DIR}/checksums.txt"
for p in "${PLATFORMS[@]}"; do
  goos="${p%-*}"; goarch="${p##*-}"
  ext=""; [[ "$goos" == "windows" ]] && ext=".exe"
  f="${VENDOR_DIR}/${p}/centag-wrap${ext}"
  sha="$(shasum -a 256 "$f" | awk '{print $1}')"
  # Asset name matches what lib/download.js expects: <goos>-<goarch>/<binary>
  printf '%s  %s/centag-wrap%s\n' "$sha" "$p" "$ext" >> "${VENDOR_DIR}/checksums.txt"
done
cat "${VENDOR_DIR}/checksums.txt"

# --- 3. Pack main npm package (lazy-download: NO bundled binaries) ----------
echo "==> pack centag-wrap (lazy-download, no bundled binaries)"
MAIN_STAGE="$(mktemp -d)"
cp -R "${NPM_DIR}/bin" "${MAIN_STAGE}/bin"
cp -R "${NPM_DIR}/lib" "${MAIN_STAGE}/lib"
cp "${NPM_DIR}/README.md" "${MAIN_STAGE}/README.md"
cp "${NPM_DIR}/package.json" "${MAIN_STAGE}/package.json"
# Strip the cross-compiled binaries so the main package stays small (downloads at runtime).
rm -rf "${MAIN_STAGE}/bin/vendor"
( cd "${MAIN_STAGE}" && npm pack --dry-run )
MAIN_TGZ="$(cd "${MAIN_STAGE}" && npm pack | tail -1)"
mv "${MAIN_STAGE}/${MAIN_TGZ}" "${OUT_ROOT}/${MAIN_TGZ}"
rm -rf "${MAIN_STAGE}"

# --- 4. Pack offline variant ------------------------------------------------
echo "==> build centag-wrap-offline (bundled binaries)"
OFFLINE_STAGE="$(mktemp -d)"
cp -R "${NPM_DIR}/bin" "${OFFLINE_STAGE}/bin"
cp -R "${NPM_DIR}/lib" "${OFFLINE_STAGE}/lib"
cp "${NPM_DIR}/README.md" "${OFFLINE_STAGE}/README.md"
cp "${NPM_DIR}/package.offline.json" "${OFFLINE_STAGE}/package.json"
( cd "${OFFLINE_STAGE}" && npm pack --dry-run )
OFFLINE_TGZ="$(cd "${OFFLINE_STAGE}" && npm pack | tail -1)"
mv "${OFFLINE_STAGE}/${OFFLINE_TGZ}" "${OUT_ROOT}/${OFFLINE_TGZ}"
rm -rf "${OFFLINE_STAGE}"

echo "==> artifacts:"
ls -lh "${OUT_ROOT}"/centag-wrap*.tgz

# --- 5. Publish -------------------------------------------------------------
if [[ "${DRY_RUN:-}" == "1" ]]; then
  echo "==> DRY_RUN: skipping npm publish and GitHub release"
  exit 0
fi

if [[ -n "${CENTAG_WRAP_NPM_TOKEN:-}" ]]; then
  echo "==> npm publish centag-wrap"
  ( cd "${NPM_DIR}" && npm publish --access public )
  echo "==> npm publish centag-wrap-offline"
  OFFLINE_PUB="$(mktemp -d)"
  cp -R "${NPM_DIR}/bin" "${OFFLINE_PUB}/bin"
  cp -R "${NPM_DIR}/lib" "${OFFLINE_PUB}/lib"
  cp "${NPM_DIR}/README.md" "${OFFLINE_PUB}/README.md"
  cp "${NPM_DIR}/package.offline.json" "${OFFLINE_PUB}/package.json"
  ( cd "${OFFLINE_PUB}" && npm publish --access public )
  rm -rf "${OFFLINE_PUB}"
fi

if [[ "$RELEASE" == "1" ]]; then
  # Prefer the unified release pipeline (personal/minimal/launcher/wrap + install.sh assets).
  echo "==> delegating GitHub release assets to scripts/release/publish-binaries.sh"
  bash "${ROOT}/scripts/release/publish-binaries.sh" \
    --version "${VERSION}" \
    --components wrap \
    --release
fi

echo "OK"
