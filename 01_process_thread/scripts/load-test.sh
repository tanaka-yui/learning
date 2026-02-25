#!/usr/bin/env bash
set -uo pipefail

PORT=${1:-3000}
CONCURRENT=${2:-3}
URL="http://localhost:${PORT}/heavy"

echo "=== Load Test ==="
echo "Target: ${URL}"
echo "Concurrent requests: ${CONCURRENT}"
echo ""

for i in $(seq 1 "${CONCURRENT}"); do
  (
    RESPONSE=$(curl -s "${URL}")
    echo "[Request ${i}] ${RESPONSE}"
  ) &
done

wait
