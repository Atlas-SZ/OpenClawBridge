#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root: sudo $0 [web-root]" >&2
  exit 1
fi

WEB_ROOT="${1:-/var/www/openclaw-bridge-web}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RELAY_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${RELAY_DIR}/../.." && pwd)"
SRC_HTML="${REPO_ROOT}/web/client/index.html"

if [[ ! -f "${SRC_HTML}" ]]; then
  echo "Missing web client file: ${SRC_HTML}" >&2
  exit 1
fi

install -d -m 755 "${WEB_ROOT}"
install -m 644 "${SRC_HTML}" "${WEB_ROOT}/index.html"

echo "Published web client:"
echo "  ${SRC_HTML} -> ${WEB_ROOT}/index.html"
