#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONNECTOR_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN="${BIN:-${CONNECTOR_DIR}/bin/openclaw-connector}"
CONFIG="${CONFIG:-${CONNECTOR_DIR}/config/connector.json}"

if [[ ! -f "${CONFIG}" ]]; then
  echo "config not found: ${CONFIG}" >&2
  echo "copy ${CONNECTOR_DIR}/config/connector.json.example to ${CONNECTOR_DIR}/config/connector.json and edit it" >&2
  exit 1
fi

exec "${BIN}" -config "${CONFIG}"
