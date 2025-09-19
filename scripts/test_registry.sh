#!/usr/bin/env bash
set -euo pipefail
REG=${1:-http://localhost:8090}
echo "[*] healthz:"
curl -fsS ${REG}/healthz && echo
echo "[*] assignments (host-unknown):"
curl -fsS ${REG}/artifacts/for-host/host-unknown | jq .
echo "[*] assignments (web-01):"
curl -fsS ${REG}/artifacts/for-host/web-01 | jq .
echo "[*] bundle seg-v1 (headers):"
curl -I ${REG}/bundles/seg-v1
echo "[*] bundle security-monitor-v1 (headers):"
curl -I ${REG}/bundles/security-monitor-v1


