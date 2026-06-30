# 10-1 Docker Build Patterns Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `10_container/01_build_patterns/` 配下に 3 つの Docker multi-stage build パターン (同一 image マルチ環境 / runtime-only / tool-runner) を実動品で実装し、compose + Makefile で横断検証可能にする。

**Architecture:** 各パターンを第一級ディレクトリ (`a-same-image/`, `b-runtime-only/`, `c-tool-runner/`) に分離。Dockerfile は BuildKit syntax 1.7 + cache mount。Compose プロファイルで dev/prod/runtime 切替。tool-runner は `docker run --rm` 専用で常駐させない。

**Tech Stack:** Docker (BuildKit), Go 1.26, Node 22 (pnpm + esbuild), distroless/static-debian12, distroless/nodejs22-debian12, scratch, hadolint, docker compose v2.

## Global Constraints

- Base ディレクトリ: `10_container/01_build_patterns/` (リポジトリ root 相対)
- Dockerfile は `# syntax=docker/dockerfile:1.7` を必須先頭行とする
- Go: `CGO_ENABLED=0`, `-trimpath -ldflags="-s -w"`, non-root user 65532 を runtime image で使用
- BuildKit cache mount: Go=`/go/pkg/mod` と `/root/.cache/go-build`、Node (pnpm)=`/root/.local/share/pnpm/store`
- 既存章 (`01_process_thread`, `09_authn_authz`) の Dockerfile スタイルと整合
- 全 Dockerfile は `make lint` (hadolint) で error 0

---

## File Structure

新規作成:
- `10_container/01_build_patterns/README.md`
- `10_container/01_build_patterns/docker-compose.yml`
- `10_container/01_build_patterns/Makefile`
- `10_container/01_build_patterns/VERIFICATION.md`
- `10_container/01_build_patterns/docs/{README.md,01-same-image-multi-env.md,02-runtime-only.md,03-tool-runner.md,buildx-cache.md}`
- `10_container/01_build_patterns/a-same-image/{Dockerfile,main.go,go.mod,.air.toml,Makefile,main_test.go}`
- `10_container/01_build_patterns/b-runtime-only/{Dockerfile,Dockerfile.node,api/main.go,api/main_test.go,worker/index.js,worker/index.test.js,go.mod,package.json,pnpm-lock.yaml,Makefile}`
- `10_container/01_build_patterns/c-tool-runner/{Dockerfile,entrypoint.sh,Makefile,test/run.bats}`

---

## Task 1: Chapter scaffold + docs skeleton

**Files:**
- Create: `10_container/01_build_patterns/README.md`
- Create: `10_container/01_build_patterns/docs/README.md`
- Create: `10_container/01_build_patterns/docs/{01-same-image-multi-env.md,02-runtime-only.md,03-tool-runner.md,buildx-cache.md}` (見出しのみ)

**Interfaces:**
- Produces: chapter directory exists, docs skeleton ready for later tasks to fill

- [ ] **Step 1: ディレクトリ作成**

```bash
mkdir -p 10_container/01_build_patterns/{docs,a-same-image,b-runtime-only/{api,worker},c-tool-runner/test}
```

- [ ] **Step 2: 章 README.md**

```markdown
# 10-1 Docker Build Patterns

Multi-stage build の 3 パターン:

| パターン | ディレクトリ | 主目的 |
|---|---|---|
| A 同一 image マルチ環境 | `a-same-image/` | dev/test/prod を `--target` で切替 |
| B runtime-only | `b-runtime-only/` | build artifact のみ runtime image、src 非同梱 |
| C tool-runner | `c-tool-runner/` | ツール群を 1 image に詰込、`docker run` で都度実行 |

検証: `make verify`。詳細: `docs/`、`VERIFICATION.md`。
```

- [ ] **Step 3: docs/README.md と 4 ファイルの見出し**

各 `.md` に H1 と「TODO 後続タスクで充足」と 1 行入れる。

`docs/README.md`:
```markdown
# Build Patterns Docs

- [01 同一 image マルチ環境](./01-same-image-multi-env.md)
- [02 runtime-only](./02-runtime-only.md)
- [03 tool-runner](./03-tool-runner.md)
- [BuildKit / buildx / cache](./buildx-cache.md)
```

他 4 ファイルは `# <タイトル>\n\n(後続タスクで詳細化)\n`。

- [ ] **Step 4: Commit**

```bash
git add 10_container/01_build_patterns
git commit -m "feat(10-1): scaffold chapter directory and docs skeleton"
```

---

## Task 2: Pattern A — 同一 image マルチ環境

**Files:**
- Create: `10_container/01_build_patterns/a-same-image/main.go`
- Create: `10_container/01_build_patterns/a-same-image/main_test.go`
- Create: `10_container/01_build_patterns/a-same-image/go.mod`
- Create: `10_container/01_build_patterns/a-same-image/.air.toml`
- Create: `10_container/01_build_patterns/a-same-image/Dockerfile`
- Create: `10_container/01_build_patterns/a-same-image/Makefile`

**Interfaces:**
- Produces: image tags `same-image:dev` / `same-image:test` / `same-image:prod`、`GET /healthz`, `GET /env` を 8080 で提供

- [ ] **Step 1: failing test 作成**

`a-same-image/main_test.go`:

```go
package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	healthz(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	body, _ := io.ReadAll(w.Result().Body)
	if string(body) != "ok" {
		t.Fatalf("want body ok, got %q", string(body))
	}
}
```

- [ ] **Step 2: 失敗確認**

```bash
cd 10_container/01_build_patterns/a-same-image
go mod init github.com/yui/learning/10/01/a-same-image
go test ./...
```

Expected: `undefined: healthz` で FAIL。

- [ ] **Step 3: main.go 実装**

```go
package main

import (
	"fmt"
	"net/http"
	"os"
)

func healthz(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") }

func envHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "APP_ENV=%s\n", os.Getenv("APP_ENV"))
}

func main() {
	http.HandleFunc("/healthz", healthz)
	http.HandleFunc("/env", envHandler)
	addr := ":" + getEnv("PORT", "8080")
	fmt.Println("listening", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
```

- [ ] **Step 4: テスト pass**

```bash
go test ./...
```

Expected: `ok` 表示。

- [ ] **Step 5: .air.toml 作成**

```toml
root = "."
[build]
cmd = "go build -o ./tmp/server ."
bin = "./tmp/server"
include_ext = ["go"]
```

- [ ] **Step 6: Dockerfile 作成**

```dockerfile
# syntax=docker/dockerfile:1.7
FROM golang:1.26 AS base
WORKDIR /src
COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

FROM base AS deps
COPY . .

FROM deps AS dev
RUN --mount=type=cache,target=/go/pkg/mod go install github.com/air-verse/air@latest
ENV APP_ENV=dev PORT=8080
EXPOSE 8080
CMD ["air", "-c", ".air.toml"]

FROM deps AS test
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go test ./...

FROM deps AS build
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server .

FROM gcr.io/distroless/static-debian12 AS prod
COPY --from=build /out/server /server
ENV APP_ENV=prod
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/server"]
```

- [ ] **Step 7: ローカル Makefile**

`a-same-image/Makefile`:

```makefile
.PHONY: build-dev build-test build-prod run-dev run-prod test clean
build-dev:  ; docker build --target dev  -t same-image:dev  .
build-test: ; docker build --target test -t same-image:test .
build-prod: ; docker build --target prod -t same-image:prod .
run-dev:    ; docker run --rm -p 8080:8080 same-image:dev
run-prod:   ; docker run --rm -p 8081:8080 same-image:prod
test:       ; go test ./...
clean:      ; docker rmi same-image:dev same-image:test same-image:prod || true
```

- [ ] **Step 8: build 3 stage 検証**

```bash
make build-dev build-test build-prod
docker images | grep ^same-image
```

Expected: 3 image (dev/test/prod) 存在、prod が最小サイズ。

- [ ] **Step 9: prod の動作確認**

```bash
docker run -d -p 8081:8080 --name same-image-prod same-image:prod
sleep 1
curl -fsS http://localhost:8081/healthz
docker rm -f same-image-prod
```

Expected: `ok`。

- [ ] **Step 10: Commit**

```bash
git add 10_container/01_build_patterns/a-same-image
git commit -m "feat(10-1): pattern A — same image multi-env (--target dev/test/prod)"
```

---

## Task 3: Pattern B (Go) — runtime-only via distroless + scratch

**Files:**
- Create: `10_container/01_build_patterns/b-runtime-only/api/main.go`
- Create: `10_container/01_build_patterns/b-runtime-only/api/main_test.go`
- Create: `10_container/01_build_patterns/b-runtime-only/go.mod`
- Create: `10_container/01_build_patterns/b-runtime-only/Dockerfile`
- Create: `10_container/01_build_patterns/b-runtime-only/Makefile`

**Interfaces:**
- Produces: image tags `runtime-go:distroless` / `runtime-go:scratch`、`GET /api/v1/echo` を 8080 で提供

- [ ] **Step 1: failing test**

`b-runtime-only/api/main_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestEcho(t *testing.T) {
	w := httptest.NewRecorder()
	echo(w, httptest.NewRequest("GET", "/api/v1/echo", nil))
	if w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var got map[string]string
	if err := json.NewDecoder(w.Result().Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["lang"] != "go" {
		t.Fatalf("want go, got %s", got["lang"])
	}
}
```

- [ ] **Step 2: 失敗確認**

```bash
cd 10_container/01_build_patterns/b-runtime-only
go mod init github.com/yui/learning/10/01/b-runtime-only
go test ./api/...
```

Expected: `undefined: echo` で FAIL。

- [ ] **Step 3: api/main.go**

```go
package main

import (
	"encoding/json"
	"net/http"
	"os"
)

func echo(w http.ResponseWriter, _ *http.Request) {
	host, _ := os.Hostname()
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"lang": "go", "host": host})
}

func main() {
	http.HandleFunc("/api/v1/echo", echo)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
```

- [ ] **Step 4: test pass**

```bash
go test ./api/...
```

Expected: PASS。

- [ ] **Step 5: Dockerfile (Go: distroless + scratch 2 stage)**

```dockerfile
# syntax=docker/dockerfile:1.7
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY api ./api
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
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

- [ ] **Step 6: ローカル Makefile**

`b-runtime-only/Makefile`:

```makefile
.PHONY: build build-go build-node sizes test clean
build: build-go build-node
build-go:
	docker build --target distroless     -t runtime-go:distroless .
	docker build --target scratch-final  -t runtime-go:scratch    .
build-node:
	docker build -f Dockerfile.node -t runtime-node:distroless .
sizes:
	@docker images --format '{{.Repository}}:{{.Tag}} {{.Size}}' | grep -E '^(runtime-go|runtime-node):'
test:
	go test ./api/...
clean:
	docker rmi runtime-go:distroless runtime-go:scratch runtime-node:distroless || true
```

- [ ] **Step 7: Go パターン build + smoke**

```bash
make build-go
docker run -d -p 8082:8080 --name rgo runtime-go:distroless
sleep 1
curl -fsS http://localhost:8082/api/v1/echo
docker rm -f rgo
docker run -d -p 8082:8080 --name rgo runtime-go:scratch
sleep 1
curl -fsS http://localhost:8082/api/v1/echo
docker rm -f rgo
```

Expected: 両方 `{"lang":"go",...}`。

- [ ] **Step 8: サイズ比較**

```bash
docker images runtime-go --format '{{.Tag}} {{.Size}}'
```

Expected: `scratch` が `distroless` 以下 (概ね 7-15MB)。

- [ ] **Step 9: Commit**

```bash
git add 10_container/01_build_patterns/b-runtime-only/{api,go.mod,Dockerfile,Makefile}
git commit -m "feat(10-1): pattern B (Go) — runtime-only via distroless + scratch"
```

---

## Task 4: Pattern B (Node) — runtime-only via esbuild bundle

**Files:**
- Create: `10_container/01_build_patterns/b-runtime-only/worker/index.js`
- Create: `10_container/01_build_patterns/b-runtime-only/worker/index.test.js`
- Create: `10_container/01_build_patterns/b-runtime-only/package.json`
- Create: `10_container/01_build_patterns/b-runtime-only/Dockerfile.node`

**Interfaces:**
- Produces: image tag `runtime-node:distroless`、起動時に `{"lang":"node","host":"<hostname>"}` を stdout に 1 度出力後 5 秒 sleep して exit (デモ用)

- [ ] **Step 1: failing test**

`worker/index.test.js`:

```js
const { buildMessage } = require('./index')

test('buildMessage returns lang=node', () => {
  expect(buildMessage('host1').lang).toBe('node')
  expect(buildMessage('host1').host).toBe('host1')
})
```

- [ ] **Step 2: package.json**

```json
{
  "name": "runtime-node-worker",
  "private": true,
  "version": "0.0.1",
  "scripts": {
    "test": "node --test worker/index.test.js",
    "bundle": "esbuild worker/index.js --bundle --platform=node --target=node22 --outfile=dist/worker.js"
  },
  "devDependencies": {
    "esbuild": "^0.24.0"
  }
}
```

- [ ] **Step 3: 失敗確認**

```bash
cd 10_container/01_build_patterns/b-runtime-only
corepack enable
pnpm install
pnpm test
```

Expected: `Cannot find module './index'` で FAIL。

- [ ] **Step 4: worker/index.js**

```js
const os = require('node:os')

function buildMessage(host) {
  return { lang: 'node', host }
}

if (require.main === module) {
  console.log(JSON.stringify(buildMessage(os.hostname())))
  setTimeout(() => process.exit(0), 5000)
}

module.exports = { buildMessage }
```

- [ ] **Step 5: test pass**

```bash
pnpm test
```

Expected: `1 ok`。

- [ ] **Step 6: Dockerfile.node**

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

- [ ] **Step 7: pnpm lockfile 生成**

```bash
pnpm install
```

`pnpm-lock.yaml` がコミット対象。

- [ ] **Step 8: build + smoke**

```bash
make build-node
docker run --rm runtime-node:distroless
```

Expected: 1 行 JSON 出力後 5 秒で exit 0。

- [ ] **Step 9: Commit**

```bash
git add 10_container/01_build_patterns/b-runtime-only/{worker,package.json,pnpm-lock.yaml,Dockerfile.node,Makefile}
git commit -m "feat(10-1): pattern B (Node) — runtime-only via esbuild bundle"
```

---

## Task 5: Pattern C — tool-runner

**Files:**
- Create: `10_container/01_build_patterns/c-tool-runner/Dockerfile`
- Create: `10_container/01_build_patterns/c-tool-runner/entrypoint.sh`
- Create: `10_container/01_build_patterns/c-tool-runner/Makefile`
- Create: `10_container/01_build_patterns/c-tool-runner/test/run.bats`

**Interfaces:**
- Produces: image tag `tool-runner:latest`、`migrate|sqlc|golangci-lint|buf|mockgen|hadolint` をサブコマンドで実行可能

- [ ] **Step 1: entrypoint.sh**

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

```bash
chmod +x 10_container/01_build_patterns/c-tool-runner/entrypoint.sh
```

- [ ] **Step 2: Dockerfile**

```dockerfile
# syntax=docker/dockerfile:1.7
FROM golang:1.26 AS tools
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install -v \
      github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1 \
      github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 \
      github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2 \
      github.com/bufbuild/buf/cmd/buf@v1.47.2 \
      go.uber.org/mock/mockgen@v0.5.0

FROM hadolint/hadolint:v2.12.1-debian AS hadolint

FROM debian:12-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates git make curl && rm -rf /var/lib/apt/lists/*
COPY --from=tools    /go/bin/migrate           /usr/local/bin/migrate
COPY --from=tools    /go/bin/sqlc              /usr/local/bin/sqlc
COPY --from=tools    /go/bin/golangci-lint     /usr/local/bin/golangci-lint
COPY --from=tools    /go/bin/buf               /usr/local/bin/buf
COPY --from=tools    /go/bin/mockgen           /usr/local/bin/mockgen
COPY --from=hadolint /bin/hadolint             /usr/local/bin/hadolint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
WORKDIR /work
ENTRYPOINT ["/entrypoint.sh"]
```

- [ ] **Step 3: Makefile (local)**

```makefile
.PHONY: build test clean
build: ; docker build -t tool-runner:latest .
test:
	docker run --rm tool-runner:latest migrate -version
	docker run --rm tool-runner:latest sqlc version
	docker run --rm tool-runner:latest golangci-lint --version
	docker run --rm tool-runner:latest buf --version
	docker run --rm tool-runner:latest mockgen -version
	docker run --rm tool-runner:latest hadolint --version
clean: ; docker rmi tool-runner:latest || true
```

- [ ] **Step 4: build + verify**

```bash
cd 10_container/01_build_patterns/c-tool-runner
make build
make test
```

Expected: 6 tool すべてバージョン出力。

- [ ] **Step 5: bats テスト (オプション、CI 想定)**

`test/run.bats`:

```bats
#!/usr/bin/env bats
@test "migrate present" { run docker run --rm tool-runner:latest migrate -version; [ "$status" -eq 0 ]; }
@test "sqlc present"    { run docker run --rm tool-runner:latest sqlc    version; [ "$status" -eq 0 ]; }
@test "hadolint present"{ run docker run --rm tool-runner:latest hadolint --version; [ "$status" -eq 0 ]; }
```

(bats-core が未導入なら情報として残し、`make test` を一次検証とする)

- [ ] **Step 6: Commit**

```bash
git add 10_container/01_build_patterns/c-tool-runner
git commit -m "feat(10-1): pattern C — tool-runner (migrate, sqlc, golangci-lint, buf, mockgen, hadolint)"
```

---

## Task 6: Root compose + Makefile + VERIFICATION.md

**Files:**
- Create: `10_container/01_build_patterns/docker-compose.yml`
- Create: `10_container/01_build_patterns/Makefile`
- Create: `10_container/01_build_patterns/VERIFICATION.md`

**Interfaces:**
- Consumes: Task 2-5 で作成した image
- Produces: `make verify` で全パターン smoke + tool 起動確認 exit 0

- [ ] **Step 1: docker-compose.yml**

```yaml
services:
  app-dev:
    build: { context: ./a-same-image, target: dev }
    ports: ["8080:8080"]
    profiles: ["dev"]
  app-prod:
    build: { context: ./a-same-image, target: prod }
    ports: ["8081:8080"]
    profiles: ["prod"]
  runtime-go:
    build: { context: ./b-runtime-only, target: distroless }
    ports: ["8082:8080"]
    profiles: ["runtime"]
  runtime-node:
    build:
      context: ./b-runtime-only
      dockerfile: Dockerfile.node
    profiles: ["runtime"]
```

- [ ] **Step 2: ルート Makefile**

```makefile
.PHONY: build-all run-dev run-prod run-runtime size-report verify smoke lint clean

build-all:
	$(MAKE) -C a-same-image  build-dev build-test build-prod
	$(MAKE) -C b-runtime-only build
	$(MAKE) -C c-tool-runner  build

run-dev:     ; docker compose --profile dev     up -d
run-prod:    ; docker compose --profile prod    up -d
run-runtime: ; docker compose --profile runtime up -d

size-report:
	@printf "%-40s %s\n" IMAGE SIZE
	@docker images --format '{{.Repository}}:{{.Tag}}  {{.Size}}' | \
	  grep -E '^(same-image|runtime-go|runtime-node|tool-runner):' | sort

lint:
	docker run --rm -v "$$PWD":/work tool-runner:latest hadolint \
	  a-same-image/Dockerfile \
	  b-runtime-only/Dockerfile \
	  b-runtime-only/Dockerfile.node \
	  c-tool-runner/Dockerfile

smoke:
	docker compose --profile dev up -d
	sleep 3
	curl -fsS http://localhost:8080/healthz
	docker compose --profile dev down
	docker compose --profile prod up -d
	sleep 2
	curl -fsS http://localhost:8081/healthz
	docker compose --profile prod down

verify: build-all smoke lint size-report
	$(MAKE) -C c-tool-runner test

clean:
	docker compose --profile dev     down -v || true
	docker compose --profile prod    down -v || true
	docker compose --profile runtime down -v || true
	$(MAKE) -C a-same-image  clean
	$(MAKE) -C b-runtime-only clean
	$(MAKE) -C c-tool-runner  clean
```

- [ ] **Step 3: VERIFICATION.md**

```markdown
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
```

- [ ] **Step 4: 実行確認**

```bash
cd 10_container/01_build_patterns
make verify
```

Expected: 全 step 成功、`make verify` 終了コード 0。

- [ ] **Step 5: Commit**

```bash
git add 10_container/01_build_patterns/{docker-compose.yml,Makefile,VERIFICATION.md}
git commit -m "feat(10-1): root compose + Makefile + VERIFICATION"
```

---

## Task 7: docs 本文書き起こし

**Files:**
- Modify: `10_container/01_build_patterns/docs/{01-same-image-multi-env.md,02-runtime-only.md,03-tool-runner.md,buildx-cache.md}`

**Interfaces:**
- Consumes: 完成した実装すべて

- [ ] **Step 1: docs/01-same-image-multi-env.md**

内容項目:
- `--target` で stage 切替の仕組
- dev/test/prod の使い分けと CI 連携 (`--target test` を CI で叩く)
- 同一 Dockerfile に複数環境を畳む利点/欠点
- `docker buildx bake` (HCL) で複数 target を一括 build する例
- 比較: 環境別 Dockerfile (`Dockerfile.dev`, `Dockerfile.prod`) との trade-off

- [ ] **Step 2: docs/02-runtime-only.md**

内容項目:
- builder stage と runtime stage の責務分離
- distroless / scratch / alpine 比較 (サイズ・互換性・debug 容易性)
- Go static binary 条件 (`CGO_ENABLED=0`)
- Node 側で `esbuild` バンドル → `node_modules` 非同梱
- non-root user (`USER 65532`) の意義
- 攻撃面縮小と SBOM への波及 (将来章の予告として 1 段落)

- [ ] **Step 3: docs/03-tool-runner.md**

内容項目:
- 用途: 開発環境 (migrate / lint / codegen) を image 1 つに集約
- volume mount + WORKDIR で host コードに対しツール実行
- CI と local の tool バージョン固定
- 既存運用との対比 (各 tool を host install / asdf / mise)
- ENTRYPOINT で subcommand 振分

- [ ] **Step 4: docs/buildx-cache.md**

内容項目:
- BuildKit 概要 (`# syntax=docker/dockerfile:1.7`)
- `--mount=type=cache` の `target` 指定 (Go modules / pnpm store / go-build cache)
- `--mount=type=bind`, `--mount=type=secret`, `--mount=type=ssh`
- buildx + bake (HCL) で複数 target を 1 コマンド
- BuildKit registry cache (`type=registry` / `type=gha`) 概略

- [ ] **Step 5: docs/README.md 更新 (各セクションへリンク)** — 既に Task 1 で雛型済、漏れがあれば補強

- [ ] **Step 6: Commit**

```bash
git add 10_container/01_build_patterns/docs
git commit -m "docs(10-1): build pattern guides — same-image, runtime-only, tool-runner, buildx-cache"
```

---

## Self-Review Notes

- Spec の全要件 (3 パターン実動 + compose 統合 + Makefile 横串 + VERIFICATION + docs 4 本) を Task 1-7 で網羅
- 各 Task は単独でレビュー可能 (build + smoke + commit)
- Task 4 で `pnpm-lock.yaml` が初回 `pnpm install` 後に生成される旨を Step 7 で明示
- Task 6 の `make lint` は Task 5 の tool-runner image に依存 (build-all 内で構築済)
