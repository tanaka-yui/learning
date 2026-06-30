#!/bin/sh
set -e
ENDPOINT="${ENDPOINT:-http://localhost:4566}"
AWS="aws --endpoint-url=$ENDPOINT --region us-east-1"
export AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1

# NOTE: LocalStack community edition does not emulate ECR (Pro-only).
# We use a local Docker registry (registry:2) at localhost:5000 as a substitute.
# The ECR_REGISTRY variable mirrors the ECR URI format for documentation purposes.
ECR_REGISTRY="localhost:5000"
REPO="$ECR_REGISTRY/demo-api"

echo "ECR repo (local registry substitute): $REPO"
docker tag demo-api:v1 "$REPO:v1"
docker push "$REPO:v1"
echo "$REPO"
