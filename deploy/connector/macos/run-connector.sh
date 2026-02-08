#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -n "${BIN:-}" ]]; then
  BIN_PATH="${BIN}"
elif [[ -x "${SCRIPT_DIR}/openclaw-connector" ]]; then
  BIN_PATH="${SCRIPT_DIR}/openclaw-connector"
else
  BIN_PATH="${SCRIPT_DIR}/../bin/openclaw-connector"
fi

if [[ -n "${CONFIG:-}" ]]; then
  CONFIG_PATH="${CONFIG}"
elif [[ -f "${SCRIPT_DIR}/config/connector.json" ]]; then
  CONFIG_PATH="${SCRIPT_DIR}/config/connector.json"
else
  CONFIG_PATH="${SCRIPT_DIR}/../config/connector.json"
fi

if [[ ! -f "${CONFIG_PATH}" ]]; then
  echo "config not found: ${CONFIG_PATH}" >&2
  if [[ -f "${SCRIPT_DIR}/config/connector.json.example" ]]; then
    echo "copy ${SCRIPT_DIR}/config/connector.json.example to ${SCRIPT_DIR}/config/connector.json and edit it" >&2
  else
    echo "copy ${SCRIPT_DIR}/../config/connector.json.example to ${SCRIPT_DIR}/../config/connector.json and edit it" >&2
  fi
  exit 1
fi

exec "${BIN_PATH}" -config "${CONFIG_PATH}"
