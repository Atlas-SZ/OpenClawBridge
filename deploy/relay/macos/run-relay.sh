#!/usr/bin/env bash
set -euo pipefail

ADDR="${ADDR:-:8080}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -n "${BIN:-}" ]]; then
  BIN_PATH="${BIN}"
elif [[ -x "${SCRIPT_DIR}/openclaw-relay" ]]; then
  BIN_PATH="${SCRIPT_DIR}/openclaw-relay"
else
  BIN_PATH="${SCRIPT_DIR}/../bin/openclaw-relay"
fi

exec "${BIN_PATH}" -addr "${ADDR}"
