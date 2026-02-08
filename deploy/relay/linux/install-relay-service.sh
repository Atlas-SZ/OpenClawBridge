#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root: sudo $0" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RELAY_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

install -m 755 "${RELAY_DIR}/bin/openclaw-relay" /usr/local/bin/openclaw-relay
install -m 644 "${RELAY_DIR}/systemd/openclaw-bridge-relay.service" /etc/systemd/system/openclaw-bridge-relay.service

systemctl daemon-reload
systemctl enable --now openclaw-bridge-relay

echo "Relay service installed and started."
echo "Nginx config template: ${RELAY_DIR}/nginx/openclaw-bridge.conf"
