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
- `make build-all`: 7 image (same-image:{dev,test,prod}, runtime-go:{distroless,scratch}, runtime-node:distroless, tool-runner:latest) 構築
- `make smoke`: `curl /healthz` で `ok` を 2 回取得
- `make lint`: hadolint error 0
- `make size-report`: scratch < distroless < same-image:prod < same-image:dev
- tool-runner: 6 tool のバージョン出力

## 既知の環境固有スキップ

### make smoke — ポート競合
`make smoke` は dev サービスをポート 8080、prod サービスをポート 8081 で起動する。
同ポートを別プロセスが占有している場合（例: 他プロジェクトの開発サーバー）、
curl は競合プロセスに到達し `/healthz` が 404 を返す。

**事前確認:**
```sh
lsof -i :8080 -i :8081   # 他プロセスが表示されれば停止してから実行
```

**手動確認済み（クリーンな環境での単体確認）:**
```sh
docker run -d --rm -p 8090:8080 same-image:prod
curl http://127.0.0.1:8090/healthz  # => ok

docker run -d --rm -p 8092:8080 same-image:dev
sleep 10 && curl http://127.0.0.1:8092/healthz  # => ok
```

両イメージとも `127.0.0.1` 経由で `/healthz → ok` を返すことを確認済み。
Makefile の `smoke` ターゲットは `127.0.0.1` を使用し、
`trap` による確実なコンテナ停止を実装している。
