#!/usr/bin/env bash
# Build GitHub Release artifacts (install.sh channel) and draft/publish.
#
# GitHub product form:
#   linux   → personal CLI tarball
#   darwin  → tray desktop (.zip + .dmg)
#   windows → tray desktop (.zip)
#
# npm channel is separate and still publishes CLI for all platforms.
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
#   CENTAG_RELEASE_ALLOW_ANY_BRANCH=1  emergency bypass of version-branch gate (on --release)
#   CENTAG_RELEASE_GITHUB_DESKTOP=0    skip host tray package (linux CLI only)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPO="${CENTAG_RELEASE_REPO:-atoml-ai/centag}"

log() { echo "==> $*" >&2; }
fail() { echo "error: $*" >&2; exit 1; }

VERSION=""
DO_RELEASE=0
EXTRA_BUILD_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --release) DO_RELEASE=1; shift ;;
    --skip-frontend) EXTRA_BUILD_ARGS+=(--skip-frontend); shift ;;
    --no-desktop) EXTRA_BUILD_ARGS+=(--no-desktop); shift ;;
    # legacy flags kept for callers; GitHub channel ignores CLI component matrix
    --components|--platforms)
      log "warn: ignoring $1 for GitHub channel (use build-artifacts.sh / npm for CLI matrices)"
      shift 2
      ;;
    -h|--help)
      sed -n '2,22p' "$0"
      exit 0
      ;;
    *) fail "unknown arg: $1" ;;
  esac
done

BUILD_ARGS=()
if [[ -n "$VERSION" ]]; then
  BUILD_ARGS+=(--version "$VERSION")
fi
if [[ ${#EXTRA_BUILD_ARGS[@]} -gt 0 ]]; then
  BUILD_ARGS+=("${EXTRA_BUILD_ARGS[@]}")
fi

# build-github-artifacts prints OUT_DIR on the last stdout line
OUT_DIR="$(bash "${ROOT}/scripts/release/build-github-artifacts.sh" "${BUILD_ARGS[@]}" | tail -n 1)"
OUT_DIR="${OUT_DIR//$'\r'/}"
[[ -d "$OUT_DIR" ]] || fail "build output missing: ${OUT_DIR:-<empty>}"

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

bash "${ROOT}/scripts/release/require-release-branch.sh" --version "${VERSION}"

command -v gh >/dev/null 2>&1 || fail "gh is required for --release"

ASSETS=()
while IFS= read -r f; do
  ASSETS+=("$f")
done < <(find "$OUT_DIR" -maxdepth 1 -type f \( \
  -name 'centag-cli-*-linux-*.tar.gz' -o \
  -name 'centag-desktop-*.dmg' -o \
  -name 'centag-desktop-*.zip' -o \
  -name 'centag-personal-linux-*.tar.gz' -o \
  -name 'Centag-*.dmg' -o \
  -name 'Centag-*.zip' -o \
  -name 'checksums.txt' \
\) | sort)
[[ ${#ASSETS[@]} -gt 0 ]] || fail "no assets to upload in ${OUT_DIR}"

NOTES="$(cat <<EOF
## Centag ${TAG}

### Install

\`\`\`bash
# Linux / curl installer (desktop OS → desktop; Linux → CLI)
curl -fsSL https://raw.githubusercontent.com/${REPO}/${TAG}/scripts/install.sh | bash -s ${VERSION} && . "\$HOME/.centag/env"
\`\`\`

\`\`\`bash
# npm — CLI for all platforms
npm install -g centag
\`\`\`

Default install root: \`~/.centag\`.  
**GitHub / install.sh**: macOS & Windows → desktop; Linux → CLI.  
**npm**: CLI on all platforms.

### Artifacts

- \`centag-desktop-personal-macos-<arch>.dmg\` / \`.zip\` — macOS desktop
- \`centag-desktop-personal-windows-<arch>.zip\` — Windows desktop
- \`centag-cli-personal-linux-<arch>.tar.gz\` — Linux CLI
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
