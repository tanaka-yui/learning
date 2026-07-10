#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."

printf "localstack"
until docker compose exec -T localstack awslocal sqs get-queue-url --queue-name orders >/dev/null 2>&1; do
  printf .; sleep 2
done
echo " ok"

printf "kafka"
until docker compose exec -T kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:29092 --list 2>/dev/null | grep -q '^orders$'; do
  printf .; sleep 2
done
echo " ok"

printf "activemq"
until nc -z localhost 61613 >/dev/null 2>&1; do
  printf .; sleep 2
done
echo " ok"
