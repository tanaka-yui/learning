#!/bin/bash
set -euo pipefail

# DLQ を先に作り、ARN を本体キューの RedrivePolicy に埋め込む
awslocal sqs create-queue --queue-name orders-dlq
DLQ_ARN=$(awslocal sqs get-queue-attributes \
  --queue-url "http://localhost:4566/000000000000/orders-dlq" \
  --attribute-names QueueArn --query 'Attributes.QueueArn' --output text)

awslocal sqs create-queue --queue-name orders --attributes "{
  \"VisibilityTimeout\": \"5\",
  \"RedrivePolicy\": \"{\\\"deadLetterTargetArn\\\":\\\"${DLQ_ARN}\\\",\\\"maxReceiveCount\\\":\\\"2\\\"}\"
}"

awslocal kinesis create-stream --stream-name orders --shard-count 1

echo "init done: sqs orders(+orders-dlq), kinesis orders"
