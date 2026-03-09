#!/bin/bash
set -euo pipefail

# 各キャッシュ層にリクエストを2回送り、HIT/MISS動作を確認する

BASE_PATH="/heavy?n=30"

ENDPOINTS=(
  "app-cache:8081"
  "shared-cache:8082"
  "cdn-nginx:8083"
  "cdn-go:8084"
)

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo "============================================"
echo " キャッシュ動作テスト (HIT/MISS確認)"
echo "============================================"
echo ""

for endpoint in "${ENDPOINTS[@]}"; do
  name="${endpoint%%:*}"
  port="${endpoint##*:}"
  url="http://localhost:${port}${BASE_PATH}"

  echo -e "${CYAN}--- ${name} (port ${port}) ---${NC}"

  # 1回目のリクエスト (MISS期待)
  echo -n "  1回目 (MISS期待): "
  response=$(curl -s -o /dev/null -w "%{http_code} %{time_total}" -D /tmp/headers1.txt "$url" 2>/dev/null)
  http_code=$(echo "$response" | awk '{print $1}')
  time_total=$(echo "$response" | awk '{print $2}')
  x_cache=$(grep -i "^x-cache:" /tmp/headers1.txt 2>/dev/null | tr -d '\r' | awk '{print $2}' || echo "N/A")
  x_backend=$(grep -i "^x-backend-instance:" /tmp/headers1.txt 2>/dev/null | tr -d '\r' | awk '{print $2}' || echo "N/A")

  if [[ "$x_cache" == *"MISS"* ]]; then
    echo -e "${GREEN}OK${NC} (X-Cache: ${x_cache}, Backend: ${x_backend}, Time: ${time_total}s, Status: ${http_code})"
  else
    echo -e "${YELLOW}WARN${NC} (X-Cache: ${x_cache}, Backend: ${x_backend}, Time: ${time_total}s, Status: ${http_code})"
  fi

  # 少し待つ
  sleep 0.5

  # 2回目のリクエスト (HIT期待)
  echo -n "  2回目 (HIT期待):  "
  response=$(curl -s -o /dev/null -w "%{http_code} %{time_total}" -D /tmp/headers2.txt "$url" 2>/dev/null)
  http_code=$(echo "$response" | awk '{print $1}')
  time_total=$(echo "$response" | awk '{print $2}')
  x_cache=$(grep -i "^x-cache:" /tmp/headers2.txt 2>/dev/null | tr -d '\r' | awk '{print $2}' || echo "N/A")
  x_backend=$(grep -i "^x-backend-instance:" /tmp/headers2.txt 2>/dev/null | tr -d '\r' | awk '{print $2}' || echo "N/A")

  if [[ "$x_cache" == *"HIT"* ]]; then
    echo -e "${GREEN}OK${NC} (X-Cache: ${x_cache}, Backend: ${x_backend:-cached}, Time: ${time_total}s, Status: ${http_code})"
  else
    echo -e "${RED}FAIL${NC} (X-Cache: ${x_cache}, Backend: ${x_backend}, Time: ${time_total}s, Status: ${http_code})"
  fi

  echo ""
done

rm -f /tmp/headers1.txt /tmp/headers2.txt
echo "テスト完了"
