#!/usr/bin/env bash
set -euo pipefail

TARGET="${TARGET:-/usr/local/bin/openclaw-connector}"
CONFIG="${CONFIG:-/etc/openclaw-bridge/connector.json}"
PURGE_CONFIG="${PURGE_CONFIG:-false}"

if [[ -f "${TARGET}" ]]; then
  rm -f "${TARGET}"
  echo "Removed ${TARGET}"
else
  echo "Not found: ${TARGET}"
fi

if [[ "${PURGE_CONFIG}" == "true" && -f "${CONFIG}" ]]; then
  rm -f "${CONFIG}"
  echo "Removed ${CONFIG}"
fi
