#!/usr/bin/env bash
set -euo pipefail

TARGET="${TARGET:-/usr/local/bin/openclaw-relay}"

if [[ -f "${TARGET}" ]]; then
  rm -f "${TARGET}"
  echo "Removed ${TARGET}"
else
  echo "Not found: ${TARGET}"
fi
