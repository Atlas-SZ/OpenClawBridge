#!/usr/bin/env bash
set -euo pipefail

TARGET="${TARGET:-/usr/local/bin/openclaw-cli}"

if [[ -f "${TARGET}" ]]; then
  rm -f "${TARGET}"
  echo "Removed ${TARGET}"
else
  echo "Not found: ${TARGET}"
fi
