# 10-1 Docker Build Patterns

Multi-stage build の 3 パターン:

| パターン | ディレクトリ | 主目的 |
|---|---|---|
| A 同一 image マルチ環境 | `a-same-image/` | dev/test/prod を `--target` で切替 |
| B runtime-only | `b-runtime-only/` | build artifact のみ runtime image、src 非同梱 |
| C tool-runner | `c-tool-runner/` | ツール群を 1 image に詰込、`docker run` で都度実行 |

検証: `make verify`。詳細: `docs/`、`VERIFICATION.md`。
