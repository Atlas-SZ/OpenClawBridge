#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root: sudo $0" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONNECTOR_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

install -m 755 "${CONNECTOR_DIR}/bin/openclaw-connector" /usr/local/bin/openclaw-connector
install -m 644 "${CONNECTOR_DIR}/systemd/openclaw-bridge-connector.service" /etc/systemd/system/openclaw-bridge-connector.service

mkdir -p /etc/openclaw-bridge
if [[ ! -f /etc/openclaw-bridge/connector.json ]]; then
  install -m 640 "${CONNECTOR_DIR}/config/connector.json.example" /etc/openclaw-bridge/connector.json
  echo "created /etc/openclaw-bridge/connector.json from template; please edit gateway token and relay_url"
fi

systemctl daemon-reload
systemctl enable --now openclaw-bridge-connector

echo "Connector service installed and started."
