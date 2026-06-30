# 10-1 Docker Build Patterns — 設計仕様

- 作成日: 2026-07-01
- 章: `10_container/01_build_patterns/`
- 関連: `09_authn_authz` (multi-stage Dockerfile を既に採用済み)、`10-2`/`10-3` (image を共有可能)

## 目的

Docker multi-stage build の代表的な 3 パターンを「動く成果物 + docs」で学習する。

1. **同一 image マルチ環境** — dev/stg/prod で同じ Dockerfile を使い、stage `--target` で切替える
2. **runtime-only** — build artifact だけを runtime image に同梱、src/ツールチェイン非同梱
3. **tool-runner** — 各種 CLI ツールを 1 image に詰込、`docker run` で都度実行

## スコープ

- Go を主軸とし、`b-runtime-only` のみ Node も並べて比較
- BuildKit / `buildx` / cache mount を共通解説
- `hadolint` で Dockerfile lint
- compose 統合 (3 パターン同時起動)

スコープ外:
- Java/Ruby/PHP/Python の全パターン展開 (`01_process_thread` に既存)
- 本番レジストリ push (CI/CD は別章)
- SBOM/cosign 等のサプライチェーン (将来章の余地として docs 末尾に予告のみ)

## アーキテクチャ

```
10_container/01_build_patterns/
├── docs/
│   ├── README.md
│   ├── 01-same-image-multi-env.md
│   ├── 02-runtime-only.md
│   ├── 03-tool-runner.md
│   └── buildx-cache.md
├── a-same-image/
│   ├── Dockerfile             # stages: base, deps, dev, test, prod
│   ├── main.go
│   ├── go.mod
│   └── Makefile
├── b-runtime-only/
│   ├── Dockerfile             # Go: builder → distroless / scratch
│   ├── Dockerfile.node        # Node: builder (esbuild) → distroless-nodejs
│   ├── api/main.go
│   ├── worker/index.js
│   ├── go.mod
│   ├── package.json
│   └── Makefile
├── c-tool-runner/
│   ├── Dockerfile             # migrate, golangci-lint, sqlc, buf, mockgen, hadolint
│   ├── entrypoint.sh
│   └── Makefile
├── docker-compose.yml
├── Makefile                   # 横串
├── README.md
└── VERIFICATION.md
```

## パターン詳細

### A: 同一 image マルチ環境 (`a-same-image/`)

Dockerfile (要点):

```dockerfile
# syntax=docker/dockerfile:1.7
FROM golang:1.26 AS base
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

FROM base AS deps
COPY . .

FROM deps AS dev
RUN go install github.com/air-verse/air@latest
ENV PORT=8080
EXPOSE 8080
CMD ["air", "-c", ".air.toml"]

FROM deps AS test
RUN go test ./...

FROM deps AS build
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /out/server .

FROM gcr.io/distroless/static-debian12 AS prod
COPY --from=build /out/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
```

ビルド:

```bash
docker build --target dev  -t same-image:dev  .
docker build --target prod -t same-image:prod .
```

compose (抜粋、ルートの `docker-compose.yml`):

```yaml
services:
  # パターンA: target 切替
  app-dev:
    build: { context: ./a-same-image, target: dev }
    ports: ["8080:8080"]
    profiles: ["dev"]
  app-prod:
    build: { context: ./a-same-image, target: prod }
    ports: ["8081:8080"]
    profiles: ["prod"]
  # パターンB: runtime-only (Go + Node 並列)
  runtime-go:
    build: { context: ./b-runtime-only, target: distroless }
    ports: ["8082:8080"]
    profiles: ["runtime"]
  runtime-node:
    build: { context: ./b-runtime-only, dockerfile: Dockerfile.node }
    profiles: ["runtime"]
```

注: パターンC (tool-runner) は常駐型ではなく `docker run --rm` で都度実行するため compose には載せない。

学習ポイント:
- `--target` で同一 Dockerfile から複数成果物
- layer 共有による再ビルド高速化
- `docker buildx bake` (`docker-bake.hcl`) で複数 target を 1 コマンドで

### B: runtime-only (`b-runtime-only/`)

意図: 「言語別の差」ではなく「runtime-only パターンの普遍性」を示すため、Go と Node を同一フォルダに同居させて並列比較する。Go は static binary → `scratch`/`distroless`、Node は esbuild で単一 bundle → `distroless-nodejs`。docs/02 でこの設計意図を明記する。

Go (Dockerfile):

```dockerfile
# syntax=docker/dockerfile:1.7
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./api

FROM gcr.io/distroless/static-debian12 AS distroless
COPY --from=build /out/api /api
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/api"]

FROM scratch AS scratch-final
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
```

Node (Dockerfile.node):

```dockerfile
# syntax=docker/dockerfile:1.7
FROM node:22-alpine AS build
WORKDIR /src
COPY package.json pnpm-lock.yaml* ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    corepack enable && pnpm install --frozen-lockfile
COPY worker ./worker
RUN pnpm exec esbuild worker/index.js --bundle --platform=node --target=node22 --outfile=/out/worker.js

FROM gcr.io/distroless/nodejs22-debian12
COPY --from=build /out/worker.js /app/worker.js
USER 65532:65532
ENTRYPOINT ["/nodejs/bin/node", "/app/worker.js"]
```

学習ポイント:
- src 非同梱で attack surface 縮小
- `scratch` (Go static binary), `distroless` (system libs 最小), `alpine` のサイズ実測比較
- BuildKit cache mount (`go mod` / pnpm store) で 2 回目以降の build 高速化
- Node は `esbuild` で単一 bundle にして `node_modules` 非同梱可能

### C: tool-runner (`c-tool-runner/`)

Dockerfile (要点):

```dockerfile
# syntax=docker/dockerfile:1.7
FROM golang:1.26 AS tools
RUN go install -v \
    github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1 \
    github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 \
    github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2 \
    github.com/bufbuild/buf/cmd/buf@v1.47.2 \
    go.uber.org/mock/mockgen@v0.5.0

FROM hadolint/hadolint:latest-debian AS hadolint

FROM debian:12-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git make curl && rm -rf /var/lib/apt/lists/*
COPY --from=tools /go/bin/* /usr/local/bin/
COPY --from=hadolint /bin/hadolint /usr/local/bin/hadolint
COPY entrypoint.sh /entrypoint.sh
WORKDIR /work
ENTRYPOINT ["/entrypoint.sh"]
```

entrypoint.sh:

```sh
#!/bin/sh
set -e
case "$1" in
  migrate|sqlc|golangci-lint|buf|mockgen|hadolint) exec "$@";;
  help|"") cat <<EOF
tool-runner: migrate sqlc golangci-lint buf mockgen hadolint
Usage: docker run --rm -v \$PWD:/work tool-runner <tool> <args...>
EOF
  ;;
  *) exec "$@";;
esac
```

学習ポイント:
- 開発環境を image 1 つで配布、CI と local の tool バージョン一致
- volume mount で host のソースに対しツール実行
- 各 tool の version を Dockerfile で固定 (再現性)

## Makefile (10-1 root)

```makefile
.PHONY: build-all run-dev run-prod size-report verify smoke lint clean

build-all:
	docker build --target dev  -t same-image:dev  ./a-same-image
	docker build --target prod -t same-image:prod ./a-same-image
	docker build --target distroless     -t runtime-go:distroless ./b-runtime-only
	docker build --target scratch-final  -t runtime-go:scratch    ./b-runtime-only
	docker build -f b-runtime-only/Dockerfile.node -t runtime-node:distroless ./b-runtime-only
	docker build -t tool-runner:latest ./c-tool-runner

run-dev:    ; docker compose --profile dev up -d
run-prod:   ; docker compose --profile prod up -d

size-report:
	@printf "%-40s %s\n" IMAGE SIZE
	@docker images --format '{{.Repository}}:{{.Tag}} {{.Size}}' | \
	  grep -E '^(same-image|runtime-go|runtime-node|tool-runner):' | sort

lint:
	docker run --rm -v $$PWD:/work tool-runner:latest hadolint \
	  a-same-image/Dockerfile b-runtime-only/Dockerfile b-runtime-only/Dockerfile.node c-tool-runner/Dockerfile

smoke:
	docker compose --profile dev up -d
	sleep 3
	curl -fsS http://localhost:8080/healthz
	docker compose --profile dev down

verify: smoke lint size-report
	docker run --rm tool-runner:latest migrate -version
	docker run --rm tool-runner:latest sqlc version
	docker run --rm tool-runner:latest golangci-lint --version
	docker run --rm tool-runner:latest buf --version

clean:
	docker compose --profile dev down -v || true
	docker compose --profile prod down -v || true
	docker rmi same-image:dev same-image:prod runtime-go:distroless runtime-go:scratch runtime-node:distroless tool-runner:latest || true
```

## docs/ 構成

- `README.md`: 章の俯瞰、3 パターンの使い分け早見表
- `01-same-image-multi-env.md`: target 機能、CI での test stage 活用、欠点 (image が肥大化しない理由)
- `02-runtime-only.md`: distroless / scratch / alpine 比較、Go static binary 条件、Node の bundle 戦略
- `03-tool-runner.md`: tool version 固定、volume mount、CI と local の一致
- `buildx-cache.md`: BuildKit, `--mount=type=cache`, `--mount=type=bind`, `docker buildx bake`

## 検証

- `make verify` が exit 0
- `make size-report` 出力:
  - `runtime-go:scratch` < `runtime-go:distroless` < `runtime-node:distroless` < `same-image:prod` < `same-image:dev`
- `VERIFICATION.md` に手順 + 期待出力

## テスト

- Dockerfile lint: `make lint` (hadolint, error 0)
- compose smoke: `make smoke`
- tool-runner: `make verify` で 4 tool 全て `--version` 応答

## 既存資産との関係

- `09_authn_authz/*/Dockerfile` の builder→distroless パターンは B の縮約版。docs/02 で参照リンク
- `01_process_thread/*/Dockerfile` の多言語 Dockerfile は別目的 (process/thread モデル)。混乱回避のため docs 冒頭で「本章はビルドパターンに集中」と明記
