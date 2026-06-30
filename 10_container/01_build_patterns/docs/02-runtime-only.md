# 02 runtime-only

## builder stage と runtime stage の責務分離

multi-stage build の本質は「ビルドに必要なもの」と「実行に必要なもの」を完全に切り離すことにある。
`b-runtime-only/Dockerfile` では builder stage でコンパイル・リンクを行い、
最終 stage には生成されたバイナリだけをコピーする。

```dockerfile
FROM golang:1.26 AS build      # コンパイラ・SDK を含む ~1 GB
...
RUN CGO_ENABLED=0 go build -o /out/api ./api

FROM gcr.io/distroless/static-debian12 AS distroless  # ~2 MB
COPY --from=build /out/api /api
```

`golang:1.26` イメージは本番には一切含まれない。
`COPY --from=build` 1 行がビルド成果物だけを runtime stage に持ち込む。

## distroless / scratch / alpine 比較

`b-runtime-only/Dockerfile` は `distroless` と `scratch` の両方を stage として定義しており、
用途に応じて `--target` で切り替えられる。

| base image | サイズ目安 | シェル | libc | デバッグ容易性 |
|-----------|-----------|--------|------|--------------|
| `gcr.io/distroless/static-debian12` | ~2 MB | なし | なし (static のみ) | 困難 (debug 版あり) |
| `scratch`  | 0 B | なし | なし | 不可 |
| `alpine:3` | ~8 MB | ash | musl | 可 (`apk add`) |

- **distroless/static**: CA 証明書・タイムゾーンデータを含む。HTTPS 通信・時刻処理を行うアプリに適する。
- **scratch**: 完全ゼロスタート。バイナリが自己完結している場合の最軽量選択肢。
- **alpine**: デバッグや追加パッケージが必要な場合の現実的な妥協点。ただし musl libc との非互換に注意。

## Go static binary の条件

Go で `scratch` / `distroless/static` を使うには、動的リンクを排除した static binary が必須。

```dockerfile
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" -o /out/api ./api
```

| フラグ | 効果 |
|--------|------|
| `CGO_ENABLED=0` | cgo を無効化し pure Go の static binary を生成 |
| `GOOS=linux` | クロスコンパイル対象を明示 |
| `-trimpath` | バイナリからホストのパスを除去 (再現性向上) |
| `-ldflags="-s -w"` | デバッグシンボル・DWARF 情報を除去してサイズ削減 |

## Node.js: esbuild バンドル → `node_modules` 非同梱

Node.js アプリでも同様のアプローチが有効。`package.json` の `bundle` スクリプトで esbuild を使い、
すべての依存を 1 ファイルに結合してから runtime stage にコピーする。

```json
"scripts": {
  "bundle": "esbuild worker/index.js --bundle --platform=node --target=node22 --outfile=dist/worker.js"
}
```

```dockerfile
FROM node:22-slim AS build
RUN corepack enable pnpm
COPY pnpm-lock.yaml package.json ./
RUN pnpm install --frozen-lockfile
COPY worker ./worker
RUN pnpm run bundle           # dist/worker.js に全依存を結合

FROM gcr.io/distroless/nodejs22-debian12 AS runtime
COPY --from=build /app/dist/worker.js /app/worker.js
ENTRYPOINT ["/nodejs/bin/node", "/app/worker.js"]
```

`node_modules` (数百 MB になることもある) は runtime stage に一切含まれない。

## non-root user (`USER 65532`) の意義

distroless イメージには `nonroot` ユーザー (UID=65532) が定義されている。

```dockerfile
USER 65532:65532
```

コンテナ内プロセスを root で動かさないことで:

1. ホスト kernel との UID マッピングによる権限昇格リスクを低減する。
2. `CAP_NET_BIND_SERVICE` など不要なケーパビリティをデフォルトで持たない。
3. OPA/Kyverno などのポリシーエンジンによる「non-root 強制」ルールをパスできる。

## 攻撃面縮小と SBOM への波及

distroless/scratch ベースのイメージはシェル・パッケージマネージャー・デバッグツールを持たないため、
コンテナへの侵入後に攻撃者が利用できるツールを最小化できる。
CVE スキャナー (Trivy / Grype) が報告するパッケージ数も激減し、脆弱性対応コストが下がる。

SBOM (Software Bill of Materials) の観点では、含まれるパッケージが少ないほど
生成された SBOM の信頼性が上がり、サプライチェーン攻撃への対応が容易になる。
SBOM 生成・署名については後続章 (セキュリティ/サプライチェーン) で扱う。
