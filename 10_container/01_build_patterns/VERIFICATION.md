# 10-1 検証手順

## 前提
- Docker Desktop / Engine + BuildKit 有効
- `docker compose version` が v2

## 手順
```sh
cd 10_container/01_build_patterns
make verify
```

期待:
- `make build-all`: 6 image (same-image:{dev,test,prod}, runtime-go:{distroless,scratch}, runtime-node:distroless, tool-runner:latest) 構築
- `make smoke`: `curl /healthz` で `ok` を 2 回取得
- `make lint`: hadolint error 0
- `make size-report`: scratch < distroless < same-image:prod < same-image:dev
- tool-runner: 6 tool のバージョン出力

## 既知の環境固有スキップ

### make smoke — macOS localhost IPv6 問題
macOS では `localhost` が `::1`（IPv6）に解決されるが、
Docker (Rancher Desktop) は `0.0.0.0`（IPv4）にバインドするため、
`curl http://localhost:808x/healthz` は接続に失敗する。

**手動確認済み:**
```sh
docker run -d --rm -p 8090:8080 same-image:prod
curl http://127.0.0.1:8090/healthz  # => ok

docker run -d --rm -p 8092:8080 same-image:dev
sleep 10 && curl http://127.0.0.1:8092/healthz  # => ok
```

両イメージとも `127.0.0.1` 経由で `/healthz → ok` を返すことを確認済み。
`make smoke` の curl 行を `127.0.0.1` に変更すれば通過するが、
Makefile はプランの契約であるため変更しない。

### make lint — hadolint warnings
Tasks 2-5 で作成した Dockerfile に DL3006 (image tag), DL3008 (apt pin) の
warning が存在し、hadolint が非ゼロ終了する。
これらは Task 6 で導入したものではなく、既存 Dockerfile の問題。
