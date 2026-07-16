#!/usr/bin/env bash
# Unified release packaging for service and desktop artifacts.
#
# Usage:
#   ./scripts/release/package.sh service --version <v>
#   ./scripts/release/package.sh desktop --version <v> --target-id <target> --release-dir <dir>
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

log_info() {
  echo "==> $*" >&2
}

log_warn() {
  echo "warn: $*" >&2
}

fail() {
  echo "error: $*" >&2
  exit 1
}

sha256_of_file() {
  local file="$1"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi
  fail "neither shasum nor sha256sum found"
}

file_size_bytes() {
  local file="$1"
  if stat -f%z "$file" >/dev/null 2>&1; then
    stat -f%z "$file"
    return
  fi
  stat -c%s "$file"
}

write_artifact_metadata() {
  local artifact="$1"
  local version="$2"
  local target="$3"
  local format="$4"
  local build_time="$5"
  local metadata="${artifact}.manifest.json"
  local checksum
  local size

  checksum="$(sha256_of_file "$artifact")"
  size="$(file_size_bytes "$artifact")"

  {
    echo "{"
    echo "  \"name\": \"Centag\","
    echo "  \"version\": \"${version}\","
    echo "  \"target\": \"${target}\","
    echo "  \"format\": \"${format}\","
    echo "  \"build_time\": \"${build_time}\","
    echo "  \"artifact\": {"
    echo "    \"file\": \"$(basename "$artifact")\","
    echo "    \"size\": ${size},"
    echo "    \"sha256\": \"${checksum}\""
    echo "  }"
    echo "}"
  } > "$metadata"

  echo "${checksum}  $(basename "$artifact")" > "${artifact}.sha256"
}

package_service() {
  local version=""
  local build_time
  build_time="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  local source_bin="${ROOT}/bin/server/centag"
  local source_static="${ROOT}/bin/server/static"
  local out_dir="${ROOT}/bin/packages"

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --version)
        version="${2:-}"
        shift 2
        ;;
      --build-time)
        build_time="${2:-}"
        shift 2
        ;;
      --source-bin)
        source_bin="${2:-}"
        shift 2
        ;;
      --source-static)
        source_static="${2:-}"
        shift 2
        ;;
      --out-dir)
        out_dir="${2:-}"
        shift 2
        ;;
      *)
        fail "unknown service packaging arg: $1"
        ;;
    esac
  done

  [[ -n "$version" ]] || fail "service packaging requires --version"
  [[ -f "$source_bin" ]] || fail "service binary not found: $source_bin"

  local temp_dir
  temp_dir="$(mktemp -d)"

  local package_name="update-package-centag-${version}"
  local package_dir="${temp_dir}/${package_name}"
  mkdir -p "$package_dir"
  cp "$source_bin" "${package_dir}/centag"
  chmod +x "${package_dir}/centag"

  local has_static=false
  if [[ -d "$source_static" ]]; then
    mkdir -p "${package_dir}/static"
    cp -R "${source_static}/." "${package_dir}/static/"
    has_static=true
  else
    log_warn "static assets not found at ${source_static}; package will contain binary only"
  fi

  {
    echo "version: \"v${version}\""
    echo "description: \"Centag System Update - v${version}\""
    echo ""
    echo "files:"
    echo "  - source: \"centag\""
    echo "    target: \"centag\""
    echo "    permission: \"0755\""
    echo "    backup: true"
    echo "    recursive: false"
    echo "    description: \"Centag 主程序\""
    if [[ "$has_static" == "true" ]]; then
      echo "  - source: \"static/\""
      echo "    target: \"static/\""
      echo "    permission: \"0644\""
      echo "    backup: false"
      echo "    recursive: true"
      echo "    description: \"Web前端静态文件\""
    fi
    echo ""
    echo "init_scripts: []"
    echo "pre_checks:"
    echo "  - check: \"disk_space\""
    echo "    min_space: \"500M\""
    echo "rollback:"
    echo "  enabled: true"
    echo "  keep_backups: 3"
    echo "  backup_dir: \"storage/backups\""
    echo "behavior:"
    echo "  auto_restart: true"
    echo "  restart_delay: 2"
    echo "  health_check_timeout: 30"
  } > "${package_dir}/update_config.yml"

  mkdir -p "$out_dir"
  local artifact="${out_dir}/${package_name}.tar.gz"
  log_info "Packaging service update: ${artifact}"
  (
    cd "$temp_dir"
    tar -czf "$artifact" "$package_name"
  )
  write_artifact_metadata "$artifact" "$version" "service" "tar.gz" "$build_time"
  rm -rf "$temp_dir"
  echo "$artifact"
}

package_desktop() {
  local version=""
  local build_time
  build_time="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  local target_id=""
  local release_dir=""
  local out_dir=""
  local format="auto"

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --version)
        version="${2:-}"
        shift 2
        ;;
      --build-time)
        build_time="${2:-}"
        shift 2
        ;;
      --target-id)
        target_id="${2:-}"
        shift 2
        ;;
      --release-dir)
        release_dir="${2:-}"
        shift 2
        ;;
      --out-dir)
        out_dir="${2:-}"
        shift 2
        ;;
      --format)
        format="${2:-}"
        shift 2
        ;;
      *)
        fail "unknown desktop packaging arg: $1"
        ;;
    esac
  done

  [[ -n "$version" ]] || fail "desktop packaging requires --version"
  [[ -n "$target_id" ]] || fail "desktop packaging requires --target-id"
  [[ -n "$release_dir" ]] || fail "desktop packaging requires --release-dir"
  [[ -d "$release_dir" ]] || fail "desktop release dir not found: $release_dir"

  if [[ -z "$out_dir" ]]; then
    out_dir="$release_dir"
  fi
  mkdir -p "$out_dir"

  if [[ "$format" == "auto" ]]; then
    case "$target_id" in
      darwin-*|windows-*)
        format="zip"
        ;;
      *)
        format="tar.gz"
        ;;
    esac
  fi

  local base_name="centag-desktop-${version}-${target_id}"
  local temp_dir
  temp_dir="$(mktemp -d)"
  local stage_dir="${temp_dir}/${base_name}"
  mkdir -p "$stage_dir"
  cp -R "${release_dir}/." "$stage_dir/"

  local artifact=""
  case "$format" in
    zip)
      if ! command -v zip >/dev/null 2>&1; then
        fail "zip command not found; install zip or pass --format tar.gz"
      fi
      artifact="${out_dir}/${base_name}.zip"
      log_info "Packaging desktop release: ${artifact}"
      (
        cd "$temp_dir"
        rm -f "$artifact"
        zip -qr "$artifact" "$base_name"
      )
      ;;
    tar.gz)
      artifact="${out_dir}/${base_name}.tar.gz"
      log_info "Packaging desktop release: ${artifact}"
      (
        cd "$temp_dir"
        tar -czf "$artifact" "$base_name"
      )
      ;;
    *)
      fail "unsupported desktop package format: $format"
      ;;
  esac

  write_artifact_metadata "$artifact" "$version" "desktop:${target_id}" "$format" "$build_time"
  rm -rf "$temp_dir"
  echo "$artifact"
}

main() {
  local mode="${1:-}"
  if [[ -z "$mode" ]]; then
    fail "usage: package.sh <service|desktop> [options]"
  fi
  shift || true

  case "$mode" in
    service)
      package_service "$@"
      ;;
    desktop)
      package_desktop "$@"
      ;;
    *)
      fail "unsupported mode: ${mode}"
      ;;
  esac
}

main "$@"
