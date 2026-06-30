# 01 同一 image マルチ環境

## `--target` による stage 切替の仕組み

Docker の multi-stage build では、`FROM … AS <name>` で名前付き stage を定義し、
`docker build --target <name>` で任意の stage を終端として指定できる。
後続の stage はビルドされず、キャッシュも無駄にならない。

`a-same-image/Dockerfile` では 4 つの stage を定義している。

```dockerfile
FROM golang:1.26 AS base   # 共通依存
FROM base AS deps           # ソースコピー
FROM deps AS dev            # air (hot-reload) を追加
FROM deps AS test           # go test ./... を実行
FROM deps AS build          # static binary を生成
FROM gcr.io/distroless/static-debian12 AS prod  # 最小 runtime
```

`dev`・`test`・`build` はすべて `deps` を継承するため、
`go mod download` のレイヤーが各 target で共有される。

## dev / test / prod の使い分けと CI 連携

| target | 用途 | コマンド |
|--------|------|---------|
| `dev`  | ローカル開発・hot-reload | `make run-dev` |
| `test` | CI でのユニットテスト   | `docker build --target test .` |
| `prod` | 本番デプロイ            | `make build-prod` |

`a-same-image/Makefile` はそれぞれのターゲットを明示している。

```makefile
build-test: ; docker build --target test -t same-image:test .
```

CI パイプラインでは `--target test` を叩くだけでテストが完結する。
テスト結果をアーティファクトとして取り出す必要がある場合は
`--output type=local,dest=./reports` と組み合わせる。

## 同一 Dockerfile に複数環境を畳む利点と欠点

**利点**
- `go mod download` などの共通レイヤーをすべての stage が再利用できる。
- 環境差異がコードとして 1 ファイルに集約されるので diff が取りやすい。
- `--target` を変えるだけで同一 CI ジョブからすべてのイメージを生成できる。

**欠点**
- Dockerfile が長くなると可読性が下がる。
- 1 つの stage のベース image を変えると下流の stage 全体に影響が波及する。
- 環境ごとに全く異なる言語・ランタイムを使う場合は分割した方が管理しやすい。

## `docker buildx bake` (HCL) で複数 target を一括ビルド

`buildx bake` はビルドマトリクスを HCL ファイルで宣言的に管理できる。

```hcl
# docker-bake.hcl
variable "TAG" { default = "latest" }

group "default" { targets = ["dev", "prod"] }

target "dev" {
  context    = "./a-same-image"
  target     = "dev"
  tags       = ["same-image:dev-${TAG}"]
  cache-from = ["type=registry,ref=same-image:cache-dev"]
  cache-to   = ["type=registry,ref=same-image:cache-dev,mode=max"]
}

target "prod" {
  context    = "./a-same-image"
  target     = "prod"
  tags       = ["same-image:prod-${TAG}"]
  cache-from = ["type=registry,ref=same-image:cache-prod"]
  cache-to   = ["type=registry,ref=same-image:cache-prod,mode=max"]
}
```

```bash
# dev と prod を並列ビルド
docker buildx bake --file docker-bake.hcl
```

`buildx bake` はデフォルトで `group "default"` のターゲットを並列実行するため、
CI の総ビルド時間を大幅に短縮できる。

## 比較: 環境別 Dockerfile との trade-off

| 観点 | 同一 Dockerfile + `--target` | 環境別 Dockerfile |
|------|------------------------------|------------------|
| 共通レイヤー共有 | ◎ stage の継承で自動共有 | △ 手動で `COPY` を重複 |
| 独立性 | △ stage が密結合しやすい | ◎ ファイル単位で変更できる |
| CI コマンド統一 | ◎ `--target` を変えるだけ | △ `-f Dockerfile.xxx` の指定が必要 |
| 可読性 (小規模) | ◎ | ○ |
| 可読性 (大規模) | △ 行数が膨張する | ◎ ファイルで分離できる |

同一言語・同一依存で "dev/test/prod" を切り替えるケースには **同一 Dockerfile + `--target`** が有効。
言語ランタイムやベース image が環境間で大きく異なる場合は `Dockerfile.dev` / `Dockerfile.prod` に分割する方が保守しやすい。
