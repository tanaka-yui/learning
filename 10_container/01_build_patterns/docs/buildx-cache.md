# BuildKit / buildx / cache

## BuildKit 概要

BuildKit は Docker 18.09 以降に組み込まれた次世代ビルドエンジン。
Dockerfile 先頭の構文ディレクティブでフロントエンドバージョンを固定できる。

```dockerfile
# syntax=docker/dockerfile:1.7
```

この 1 行を追加することで:

- `--mount=type=cache` / `--mount=type=secret` / `--mount=type=ssh` などの拡張 RUN 構文が使える。
- BuildKit が自動的に stage の並列ビルドと依存グラフ解析を行う。
- Dockerfile の構文バージョンが Dockerfile 自体に記録されるため、CI/CD 環境の buildkit バージョンに依存しなくなる。

本章のすべての Dockerfile は `# syntax=docker/dockerfile:1.7` を先頭に持つ。

## `--mount=type=cache` のターゲット指定

キャッシュマウントはビルドレイヤーをまたいでディレクトリを永続化する。
`target` に指定したパスへの書き込みがキャッシュとして保持され、次回ビルドで再利用される。

### Go modules / go-build cache

```dockerfile
# go mod download — モジュールキャッシュ
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# go build — コンパイルキャッシュ
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /out/server .
```

`/go/pkg/mod` はダウンロード済みモジュール、`/root/.cache/go-build` はコンパイル中間成果物。
両方をキャッシュすることで 2 回目以降のビルドが大幅に高速化する。

### pnpm store

```dockerfile
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile
```

pnpm はコンテンツアドレス可能なストアにパッケージを保持するため、
`target` に store パスを指定するだけでレイヤー間共有が実現できる。

## その他の `--mount` タイプ

### `--mount=type=bind`

ホストのファイルをビルドコンテキスト外から読み取り専用でマウントする。
`COPY` と異なりビルドレイヤーに含まれない。

```dockerfile
RUN --mount=type=bind,source=scripts/gen.sh,target=/gen.sh \
    /gen.sh
```

### `--mount=type=secret`

API キーや SSH 鍵などの秘密情報をビルド時にレイヤーに残さず渡す。

```bash
docker build --secret id=npmrc,src=$HOME/.npmrc .
```

```dockerfile
RUN --mount=type=secret,id=npmrc,target=/root/.npmrc \
    npm install
```

ビルド完了後にシークレットはレイヤーから消え、`docker history` にも残らない。

### `--mount=type=ssh`

SSH エージェントのソケットをビルドコンテナに転送し、
プライベートリポジトリへの git アクセスに利用する。

```bash
docker build --ssh default .
```

```dockerfile
RUN --mount=type=ssh \
    git clone git@github.com:org/private-repo.git /src
```

## buildx + bake (HCL) で複数ターゲットを 1 コマンド

`docker buildx bake` は HCL ファイルに複数のビルドターゲットを宣言し、並列実行できる。

```hcl
# docker-bake.hcl
group "default" {
  targets = ["same-image-dev", "same-image-prod", "runtime-go", "tool-runner"]
}

target "same-image-dev" {
  context = "./a-same-image"
  target  = "dev"
  tags    = ["same-image:dev"]
}

target "same-image-prod" {
  context = "./a-same-image"
  target  = "prod"
  tags    = ["same-image:prod"]
}

target "runtime-go" {
  context = "./b-runtime-only"
  target  = "distroless"
  tags    = ["runtime-go:distroless"]
}

target "tool-runner" {
  context = "./c-tool-runner"
  tags    = ["tool-runner:latest"]
}
```

```bash
# すべてのターゲットを並列ビルド
docker buildx bake --file docker-bake.hcl
```

`group "default"` 内のターゲットは BuildKit が依存グラフを解析して並列実行する。

## BuildKit registry cache (`type=registry` / `type=gha`)

ローカルキャッシュはビルドホストが変わると無効になる。
リモートキャッシュを使うことで CI の異なるランナー間でもキャッシュを共有できる。

### type=registry

ビルドキャッシュを OCI イメージとして registry に保存する。

```bash
docker buildx build \
  --cache-from type=registry,ref=ghcr.io/org/app:cache \
  --cache-to   type=registry,ref=ghcr.io/org/app:cache,mode=max \
  .
```

`mode=max` はすべての中間レイヤーを保存する (デフォルトの `min` は最終レイヤーのみ)。

### type=gha

GitHub Actions の cache API を直接バックエンドとして使う。
`docker/build-push-action` と組み合わせると設定が最小化できる。

```yaml
- uses: docker/build-push-action@v6
  with:
    cache-from: type=gha
    cache-to:   type=gha,mode=max
```

GitHub が提供するキャッシュストレージ (10 GB / リポジトリ) を自動的に利用し、
ブランチ間のキャッシュスコープも Actions の仕様に従って管理される。
