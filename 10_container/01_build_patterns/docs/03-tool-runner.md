# 03 tool-runner

## 用途: 開発ツールを image 1 つに集約

プロジェクトには migrate・lint・codegen など複数の開発ツールが必要になる。
これらを各自の手元や CI に個別インストールすると、バージョン差異・OS 差異・インストール手順のメンテコストが積み重なる。

`c-tool-runner/Dockerfile` はすべてのツールを 1 つのイメージに収める。

```dockerfile
FROM golang:1.23 AS tools
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1 && \
    go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0             && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2 && \
    go install github.com/bufbuild/buf/cmd/buf@v1.47.2               && \
    go install go.uber.org/mock/mockgen@v0.5.0

FROM hadolint/hadolint:v2.12.1-beta-debian AS hadolint

FROM debian:12-slim
COPY --from=tools    /go/bin/migrate       /usr/local/bin/
COPY --from=tools    /go/bin/sqlc          /usr/local/bin/
COPY --from=tools    /go/bin/golangci-lint /usr/local/bin/
...
COPY --from=hadolint /bin/hadolint         /usr/local/bin/
```

multi-stage build でビルドツール (Go SDK) と最終イメージ (debian:12-slim) を分離しているため、
Go コンパイラはランタイムイメージに含まれない。

## volume mount + WORKDIR でホストコードに対してツール実行

`WORKDIR /work` を設定し、ホストの作業ディレクトリを `/work` にマウントして使う。

```bash
# ホスト $PWD のコードに対して golangci-lint を実行
docker run --rm \
  -v "$PWD":/work \
  tool-runner:latest \
  golangci-lint run ./...

# migrate でマイグレーション適用
docker run --rm \
  -v "$PWD":/work \
  -e DATABASE_URL="postgres://..." \
  tool-runner:latest \
  migrate -path ./migrations -database "$DATABASE_URL" up
```

`--rm` で使い捨て実行、`-v` でホストのソースツリーをコンテナ内 `/work` に見せる。
ツールはコンテナ内で動くが、操作対象はホストのファイルシステムにある。

## CI とローカルのツールバージョン固定

Dockerfile にバージョンを `@vX.Y.Z` 形式でピン留めすることで、
ローカルと CI が同一バイナリを使うことを保証できる。

```makefile
# c-tool-runner/Makefile (抜粋)
lint:
    docker run --rm -v "$$PWD":/work tool-runner:latest golangci-lint run ./...
```

ルート `Makefile` の `make lint` はこのイメージを経由して Dockerfile のリントを実行する。

```makefile
lint:
    docker run --rm -v "$$PWD":/work tool-runner:latest hadolint \
      a-same-image/Dockerfile \
      b-runtime-only/Dockerfile \
      c-tool-runner/Dockerfile
```

バージョンアップは Dockerfile の `@vX.Y.Z` を変更して `docker build` し直すだけ。
`asdf install` や `brew upgrade` を全員に周知する必要がない。

## 既存運用との対比

| 方法 | バージョン統一 | セットアップ手順 | CI 再現性 |
|------|--------------|----------------|----------|
| ホスト直接インストール | △ 個人差が出る | 複雑 (OS 依存) | △ 環境差が出やすい |
| asdf / mise | ○ `.tool-versions` で管理 | asdf 自体のインストールが必要 | ○ CI も asdf が必要 |
| **tool-runner image** | ◎ Dockerfile でピン | `docker pull` だけ | ◎ 完全同一バイナリ |

ただし tool-runner image は Go・Node 以外の言語ツールを混在させると肥大化しやすい。
言語ごとに image を分割するか、`bake` で複数 image をまとめてビルドする構成も有効。

## ENTRYPOINT でサブコマンド振り分け

`entrypoint.sh` が受け取ったコマンドをホワイトリストで検証してから `exec` に渡す。

```sh
#!/bin/sh
set -e
case "$1" in
  migrate|sqlc|golangci-lint|buf|mockgen|hadolint) exec "$@";;
  help|"") cat <<'EOF'
tool-runner: migrate, sqlc, golangci-lint, buf, mockgen, hadolint
Usage: docker run --rm -v "$PWD":/work tool-runner <tool> <args...>
EOF
  ;;
  *) exec "$@";;
esac
```

`exec "$@"` で PID 1 をツールプロセスに引き渡すため、シグナルが正しくツールに届く。
`help` や引数なし実行でヘルプを表示するので、チームへの onboarding コストも低い。

docker-compose.yml でサービスとして定義し、`docker compose run tools migrate up` のように使うこともできる。
詳細は `../docker-compose.yml` の `tools` サービス定義を参照。
