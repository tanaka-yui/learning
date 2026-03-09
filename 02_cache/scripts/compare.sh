#!/bin/bash
set -euo pipefail

# 各キャッシュ層に連続リクエストを送り、パフォーマンスを比較する

BASE_PATH="/heavy?n=30"
REQUESTS=10

ENDPOINTS=(
  "direct:8080"
  "app-cache:8081"
  "shared-cache:8082"
  "cdn-nginx:8083"
  "cdn-go:8084"
)

GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

echo "============================================"
echo " パフォーマンス比較 (${REQUESTS}回リクエスト)"
echo "============================================"
echo ""

printf "%-20s %10s %10s %10s %10s\n" "キャッシュ層" "初回(s)" "平均(s)" "HIT率" "合計(s)"
printf "%-20s %10s %10s %10s %10s\n" "--------------------" "----------" "----------" "----------" "----------"

for endpoint in "${ENDPOINTS[@]}"; do
  name="${endpoint%%:*}"
  port="${endpoint##*:}"
  url="http://localhost:${port}${BASE_PATH}"

  total_time=0
  first_time=""
  hit_count=0

  for i in $(seq 1 $REQUESTS); do
    response=$(curl -s -o /dev/null -w "%{time_total}" -D /tmp/perf_headers.txt "$url" 2>/dev/null)
    time_val=$response
    x_cache=$(grep -i "^x-cache:" /tmp/perf_headers.txt 2>/dev/null | tr -d '\r' | awk '{print $2}' || echo "")

    if [[ -z "$first_time" ]]; then
      first_time=$time_val
    fi

    if [[ "$x_cache" == *"HIT"* ]]; then
      hit_count=$((hit_count + 1))
    fi

    total_time=$(echo "$total_time + $time_val" | bc 2>/dev/null || echo "0")
  done

  avg_time=$(echo "scale=4; $total_time / $REQUESTS" | bc 2>/dev/null || echo "N/A")

  if [[ "$name" == "direct" ]]; then
    printf "%-20s %10s %10s %10s %10s\n" "$name" "$first_time" "$avg_time" "N/A" "$total_time"
  else
    hit_rate=$(echo "scale=1; $hit_count * 100 / $REQUESTS" | bc 2>/dev/null || echo "N/A")
    printf "%-20s %10s %10s %8s%% %10s\n" "$name" "$first_time" "$avg_time" "$hit_rate" "$total_time"
  fi
done

rm -f /tmp/perf_headers.txt
echo ""
echo "比較完了"
