#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./deploy/cli/start-cli.sh <relay-url> <access-code> [response-timeout]

Example:
  ./deploy/cli/start-cli.sh wss://bridge.example.com/client A-123456 30s
EOF
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
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BIN_PATH="${ROOT_DIR}/openclaw-cli"

if [[ ! -x "${BIN_PATH}" ]]; then
  echo "openclaw-cli not found: ${BIN_PATH}" >&2
  exit 1
fi

exec "${BIN_PATH}" \
  -relay-url "${RELAY_URL}" \
  -access-code "${ACCESS_CODE}" \
  -response-timeout "${RESPONSE_TIMEOUT}"
