#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root: sudo $0" >&2
  exit 1
fi

PURGE_CONFIG="${PURGE_CONFIG:-false}"

systemctl disable --now openclaw-bridge-connector 2>/dev/null || true
rm -f /etc/systemd/system/openclaw-bridge-connector.service
systemctl daemon-reload
rm -f /usr/local/bin/openclaw-connector

if [[ "${PURGE_CONFIG}" == "true" ]]; then
  rm -f /etc/openclaw-bridge/connector.json
  rmdir --ignore-fail-on-non-empty /etc/openclaw-bridge 2>/dev/null || true
  echo "Connector config removed (/etc/openclaw-bridge/connector.json)."
else
  echo "Connector config kept at /etc/openclaw-bridge/connector.json (set PURGE_CONFIG=true to remove)."
fi

echo "Connector service uninstalled."
