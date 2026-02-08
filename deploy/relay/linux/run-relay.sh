#!/usr/bin/env bash
set -euo pipefail

ADDR="${ADDR:-:8080}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RELAY_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN="${BIN:-${RELAY_DIR}/bin/openclaw-relay}"

exec "${BIN}" -addr "${ADDR}"
