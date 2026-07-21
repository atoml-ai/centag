#!/usr/bin/env bash
# Build release artifacts and draft/publish a GitHub Release for install.sh.
#
# Usage:
#   ./scripts/release/publish-binaries.sh --version 0.2.7
#   ./scripts/release/publish-binaries.sh --version 0.2.7 --release
#   DRY_RUN=1 ./scripts/release/publish-binaries.sh --release
#
# Env:
#   GH_TOKEN / gh auth   required for --release
#   DRY_RUN=1            build only, skip gh release
#   CENTAG_RELEASE_REPO  default atoml-ai/centag
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPO="${CENTAG_RELEASE_REPO:-atoml-ai/centag}"

log() { echo "==> $*" >&2; }
fail() { echo "error: $*" >&2; exit 1; }

VERSION=""
DO_RELEASE=0
COMPONENTS="personal,proxyctl"
EXTRA_BUILD_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --release) DO_RELEASE=1; shift ;;
    --components) COMPONENTS="${2:-}"; shift 2 ;;
    --platforms) EXTRA_BUILD_ARGS+=(--platforms "$2"); shift 2 ;;
    --skip-frontend) EXTRA_BUILD_ARGS+=(--skip-frontend); shift ;;
    -h|--help)
      sed -n '2,18p' "$0"
      exit 0
      ;;
    *) fail "unknown arg: $1" ;;
  esac
done

BUILD_ARGS=()
if [[ -n "$VERSION" ]]; then
  BUILD_ARGS+=(--version "$VERSION")
fi
BUILD_ARGS+=(--components "$COMPONENTS")
# macOS bash 3.2 + set -u: empty array expansion is "unbound"
if [[ ${#EXTRA_BUILD_ARGS[@]} -gt 0 ]]; then
  BUILD_ARGS+=("${EXTRA_BUILD_ARGS[@]}")
fi

OUT_DIR="$(bash "${ROOT}/scripts/release/build-artifacts.sh" "${BUILD_ARGS[@]}")"
[[ -d "$OUT_DIR" ]] || fail "build output missing: $OUT_DIR"

if [[ -z "$VERSION" ]]; then
  VERSION="$(basename "$OUT_DIR")"
fi
VERSION="${VERSION#v}"
TAG="v${VERSION}"

if [[ "${DRY_RUN:-}" == "1" ]]; then
  log "DRY_RUN=1: skipping GitHub release (${TAG})"
  ls -lh "$OUT_DIR"
  exit 0
fi

if [[ "$DO_RELEASE" != "1" ]]; then
  log "built ${TAG} artifacts in ${OUT_DIR} (pass --release to upload)"
  exit 0
fi

command -v gh >/dev/null 2>&1 || fail "gh is required for --release"

ASSETS=()
while IFS= read -r f; do
  ASSETS+=("$f")
done < <(find "$OUT_DIR" -maxdepth 1 \( -name 'centag-*.tar.gz' -o -name 'checksums.txt' \) | sort)
[[ ${#ASSETS[@]} -gt 0 ]] || fail "no assets to upload in ${OUT_DIR}"

NOTES="$(cat <<EOF
## Centag ${TAG}

Install:

\`\`\`bash
curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | bash
curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | bash -s -- --only proxyctl
\`\`\`

### Artifacts

- \`centag-personal-<goos>-<goarch>.tar.gz\` — personal CLI + WebUI static
- \`centag-proxyctl-<goos>-<goarch>.tar.gz\` — system/process proxy helper
- \`checksums.txt\` — SHA-256 sums
EOF
)"

if gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1; then
  log "uploading assets to existing release ${TAG}"
  gh release upload "$TAG" --repo "$REPO" --clobber "${ASSETS[@]}"
else
  log "creating draft release ${TAG}"
  gh release create "$TAG" \
    --repo "$REPO" \
    --draft \
    --title "Centag ${VERSION}" \
    --notes "$NOTES" \
    "${ASSETS[@]}"
fi

log "OK release ${TAG} → https://github.com/${REPO}/releases/tag/${TAG}"
