#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
OpenClawBridge one-click installer

Usage:
  ./deploy/install.sh --role relay
  ./deploy/install.sh --role connector
  ./deploy/install.sh --role both

Options:
  --role <relay|connector|both>   Install target role (required)
  --config <path>                 Connector config path (default: /etc/openclaw-bridge/connector.json)
  --bin-dir <path>                Binary install directory (default: /usr/local/bin)
  --systemd-dir <path>            Systemd unit directory (default: /etc/systemd/system)
  -h, --help                      Show this help
EOF
}

log() {
  printf '[install] %s\n' "$*"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

if [[ ${EUID:-0} -ne 0 ]]; then
  printf 'please run as root (or with sudo)\n' >&2
  exit 1
fi

ROLE=""
CONFIG_PATH="/etc/openclaw-bridge/connector.json"
BIN_DIR="/usr/local/bin"
SYSTEMD_DIR="/etc/systemd/system"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --role)
      ROLE="${2:-}"
      shift 2
      ;;
    --config)
      CONFIG_PATH="${2:-}"
      shift 2
      ;;
    --bin-dir)
      BIN_DIR="${2:-}"
      shift 2
      ;;
    --systemd-dir)
      SYSTEMD_DIR="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'unknown argument: %s\n' "$1" >&2
      usage
      exit 1
      ;;
  esac
done

case "$ROLE" in
  relay|connector|both)
    ;;
  *)
    printf '--role is required and must be relay|connector|both\n' >&2
    usage
    exit 1
    ;;
esac

require_cmd go
require_cmd systemctl

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

install_relay() {
  log "building relay"
  go build -o "${BIN_DIR}/openclaw-relay" "${REPO_ROOT}/relay"
  chmod +x "${BIN_DIR}/openclaw-relay"

  log "installing relay systemd unit"
  install -m 0644 "${REPO_ROOT}/deploy/systemd/openclaw-bridge-relay.service" "${SYSTEMD_DIR}/openclaw-bridge-relay.service"

  systemctl daemon-reload
  systemctl enable --now openclaw-bridge-relay
  systemctl status openclaw-bridge-relay --no-pager || true
}

install_connector() {
  log "building connector"
  go build -o "${BIN_DIR}/openclaw-connector" "${REPO_ROOT}/connector"
  chmod +x "${BIN_DIR}/openclaw-connector"

  log "installing connector systemd unit"
  install -m 0644 "${REPO_ROOT}/deploy/systemd/openclaw-bridge-connector.service" "${SYSTEMD_DIR}/openclaw-bridge-connector.service"

  mkdir -p "$(dirname "${CONFIG_PATH}")"
  if [[ ! -f "${CONFIG_PATH}" ]]; then
    log "creating connector config from example: ${CONFIG_PATH}"
    install -m 0600 "${REPO_ROOT}/connector/config.example.json" "${CONFIG_PATH}"
    log "please edit ${CONFIG_PATH} before production use"
  fi

  systemctl daemon-reload
  systemctl enable --now openclaw-bridge-connector
  systemctl status openclaw-bridge-connector --no-pager || true
}

log "repo root: ${REPO_ROOT}"
log "role: ${ROLE}"

case "$ROLE" in
  relay)
    install_relay
    ;;
  connector)
    install_connector
    ;;
  both)
    install_relay
    install_connector
    ;;
esac

log "done"
