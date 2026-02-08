#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root: sudo $0" >&2
  exit 1
fi

systemctl disable --now openclaw-bridge-relay 2>/dev/null || true
rm -f /etc/systemd/system/openclaw-bridge-relay.service
systemctl daemon-reload
rm -f /usr/local/bin/openclaw-relay

echo "Relay service uninstalled."
