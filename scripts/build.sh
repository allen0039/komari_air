#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_DIR="${ROOT_DIR}/build"
THEME_DIR="${ROOT_DIR}/server/web/public/defaultTheme"
THEME_ARCHIVE="${THEME_DIR}/dist.tar.zst"

for command_name in npm go tar zstd; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "Missing required command: ${command_name}" >&2
    exit 1
  fi
done

echo "Building web frontend..."
(
  cd "${ROOT_DIR}/web"
  npm ci
  npm run build
)

echo "Packing frontend for server embedding..."
mkdir -p "${THEME_DIR}"
TEMP_TAR="$(mktemp "${TMPDIR:-/tmp}/komari-air-web.XXXXXX")"
trap 'rm -f "${TEMP_TAR}"' EXIT
tar -cf "${TEMP_TAR}" -C "${ROOT_DIR}/web/dist" .
zstd -19 -T0 -f "${TEMP_TAR}" -o "${THEME_ARCHIVE}"
cp "${ROOT_DIR}/web/komari-theme.json" "${THEME_DIR}/komari-theme.json"

mkdir -p "${BUILD_DIR}"

echo "Building server..."
(
  cd "${ROOT_DIR}/server"
  go build -o "${BUILD_DIR}/komari" .
)

echo "Building agent..."
(
  cd "${ROOT_DIR}/agent"
  go build -o "${BUILD_DIR}/komari-agent" .
)

echo "Build completed: ${BUILD_DIR}"
