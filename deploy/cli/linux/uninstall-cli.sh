#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root: sudo $0" >&2
  exit 1
fi

rm -f /usr/local/bin/openclaw-cli

echo "CLI binary removed from /usr/local/bin/openclaw-cli."
