#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  ./cli/scripts/start-cli.sh <relay-url> <access-code> [response-timeout]
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

RELAY_URL="${1:-${RELAY_URL:-}}"
ACCESS_CODE="${2:-${ACCESS_CODE:-}}"
RESPONSE_TIMEOUT="${3:-${RESPONSE_TIMEOUT:-30s}}"

if [[ -z "${RELAY_URL}" || -z "${ACCESS_CODE}" ]]; then
  usage
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN_PATH="${CLI_DIR}/bin/openclaw-cli"

exec "${BIN_PATH}" -relay-url "${RELAY_URL}" -access-code "${ACCESS_CODE}" -response-timeout "${RESPONSE_TIMEOUT}"
