#!/usr/bin/env bash

set -euo pipefail

TUNNEL_NAME="anchor-local"
HOSTNAME="local-anchor.anchor.dev"
LOCAL_URL="http://localhost:8080"

if ! command -v cloudflared >/dev/null 2>&1; then
  echo "cloudflared is required. Install it first."
  exit 1
fi

echo "Checking tunnel '${TUNNEL_NAME}'..."
if ! cloudflared tunnel info "${TUNNEL_NAME}" >/dev/null 2>&1; then
  echo "Creating tunnel '${TUNNEL_NAME}'..."
  cloudflared tunnel create "${TUNNEL_NAME}"
fi

echo "Ensuring DNS route '${HOSTNAME}'..."
set +e
route_output="$(cloudflared tunnel route dns --overwrite-dns "${TUNNEL_NAME}" "${HOSTNAME}" 2>&1)"
route_status=$?
set -e
if [[ ${route_status} -ne 0 ]] && [[ "${route_output}" != *"already exists"* ]]; then
  echo "${route_output}"
  exit ${route_status}
fi

TUNNEL_ID="$({ cloudflared tunnel list --output json 2>/dev/null || cloudflared tunnel list -o json 2>/dev/null; } | python3 -c '
import json,sys
target = sys.argv[1]
for item in json.load(sys.stdin):
    if item.get("name") == target:
        print(item.get("id", ""))
        break
' "${TUNNEL_NAME}")"

if [[ -z "${TUNNEL_ID}" ]]; then
  echo "Could not resolve tunnel ID for '${TUNNEL_NAME}'."
  exit 1
fi

CREDENTIALS_FILE="${HOME}/.cloudflared/${TUNNEL_ID}.json"
if [[ ! -f "${CREDENTIALS_FILE}" ]]; then
  echo "Missing credentials file: ${CREDENTIALS_FILE}"
  exit 1
fi

TMP_CONFIG="$(mktemp -t anchor-cloudflared.XXXXXX.yml)"
cleanup() {
  rm -f "${TMP_CONFIG}"
}
trap cleanup EXIT

cat > "${TMP_CONFIG}" <<EOF
tunnel: ${TUNNEL_ID}
credentials-file: ${CREDENTIALS_FILE}

ingress:
  - hostname: ${HOSTNAME}
    service: ${LOCAL_URL}
  - service: http_status:404
EOF

echo ""
echo "Tunnel ready"
echo "  Hostname: ${HOSTNAME}"
echo "  Local:    ${LOCAL_URL}"
echo "  Stop:     Ctrl+C"
echo ""

cloudflared tunnel --config "${TMP_CONFIG}" run "${TUNNEL_NAME}"
