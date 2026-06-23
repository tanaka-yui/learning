#!/usr/bin/env bash
# gen-certs.sh: M2Mデモ用のCA・サーバ・クライアント証明書を生成する
# 生成先: スクリプトと同じディレクトリの certs/ ディレクトリ
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERTS_DIR="${SCRIPT_DIR}/certs"
mkdir -p "${CERTS_DIR}"

echo "==> CA証明書を生成します..."
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
  -keyout "${CERTS_DIR}/ca.key" \
  -out "${CERTS_DIR}/ca.crt" \
  -days 365 -nodes \
  -subj "/CN=M2M Demo CA"

echo "==> サーバ証明書を生成します (CN=localhost, SAN=localhost)..."
openssl req -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
  -keyout "${CERTS_DIR}/server.key" \
  -out "${CERTS_DIR}/server.csr" \
  -nodes \
  -subj "/CN=localhost"

openssl x509 -req \
  -in "${CERTS_DIR}/server.csr" \
  -CA "${CERTS_DIR}/ca.crt" \
  -CAkey "${CERTS_DIR}/ca.key" \
  -CAcreateserial \
  -out "${CERTS_DIR}/server.crt" \
  -days 365 \
  -extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1\nextendedKeyUsage=serverAuth")

echo "==> クライアント証明書を生成します (CN=m2m-client)..."
openssl req -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
  -keyout "${CERTS_DIR}/client.key" \
  -out "${CERTS_DIR}/client.csr" \
  -nodes \
  -subj "/CN=m2m-client"

openssl x509 -req \
  -in "${CERTS_DIR}/client.csr" \
  -CA "${CERTS_DIR}/ca.crt" \
  -CAkey "${CERTS_DIR}/ca.key" \
  -CAcreateserial \
  -out "${CERTS_DIR}/client.crt" \
  -days 365 \
  -extfile <(printf "extendedKeyUsage=clientAuth")

# CSRファイルはクリーンアップ
rm -f "${CERTS_DIR}/server.csr" "${CERTS_DIR}/client.csr"

echo ""
echo "==> 証明書の生成が完了しました: ${CERTS_DIR}"
ls -la "${CERTS_DIR}"
