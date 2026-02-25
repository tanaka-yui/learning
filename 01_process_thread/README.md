# Process & Thread Comparison

TypeScript (Node.js) / Go / Java / PHP / Ruby / Rust / Python の HTTP サーバーで、並行リクエスト処理の違いを比較するデモプロジェクト。

## 目的

- **Node.js (シングルスレッド)**: CPU バウンドな処理でイベントループがブロックされ、リクエストが直列処理されることを確認
- **Go (goroutine)**: リクエストごとに goroutine が生成され、並行処理されることを確認
- **Go (GOMAXPROCS=1)**: Go ランタイムの OS スレッドを 1 に制限し、goroutine が時分割で処理されることを確認
- **Java (スレッドプール)**: リクエストごとに異なるスレッドで並行処理されることを確認
- **Java (Tomcat シングルスレッド)**: Tomcat のスレッドプールを 1 に制限し、リクエストが直列処理されることを確認
- **PHP (FPM マルチプロセス)**: PHP-FPM のワーカープロセスが別々にリクエストを処理し、並行処理されることを確認
- **PHP (FPM シングルプロセス)**: PHP-FPM のワーカープロセスを 1 つに制限し、リクエストが直列処理されることを確認
- **Ruby (Puma マルチプロセス)**: Puma のワーカープロセスが別々にリクエストを処理し、並行処理されることを確認
- **Ruby (Puma シングルプロセス)**: Puma のワーカープロセスを 1 つに制限し、リクエストが直列処理されることを確認
- **Rust / Actix-web (マルチスレッド)**: Actix-web の worker threads が別々にリクエストを処理し、並行処理されることを確認
- **Rust / Actix-web (シングルスレッド)**: Actix-web の worker を 1 つに制限し、リクエストが直列処理されることを確認
- **Rust / Axum (Tokio マルチスレッド)**: Tokio ランタイムの worker threads が別々にリクエストを処理し、並行処理されることを確認
- **Rust / Axum (Tokio シングルスレッド)**: Tokio ランタイムの worker を 1 つに制限し、リクエストが直列処理されることを確認
- **Python / FastAPI (Uvicorn マルチプロセス)**: Uvicorn のワーカープロセスが別々にリクエストを処理し、並行処理されることを確認
- **Python / FastAPI (Uvicorn シングルプロセス)**: Uvicorn のワーカープロセスを 1 つに制限し、CPU バウンド処理がイベントループをブロックしてリクエストが直列処理されることを確認

## 構成

| 言語 | フレームワーク | ポート | 並行処理モデル |
|------|---------------|--------|--------------|
| TypeScript | Fastify | 3000 | シングルスレッド |
| Go | net/http | 8081 | goroutine |
| Go (single) | net/http | 8082 | GOMAXPROCS=1 |
| Java | Spring Boot | 8083 | スレッドプール |
| Java (single) | Spring Boot | 8084 | シングルスレッド (Tomcat threads.max=1) |
| PHP | Laravel (php-fpm) | 8085 | マルチプロセス (FPM workers=4) |
| PHP (single) | Laravel (php-fpm) | 8086 | シングルプロセス (FPM workers=1) |
| Ruby | Ruby on Rails (Puma) | 8087 | マルチプロセス (Puma workers=4) |
| Ruby (single) | Ruby on Rails (Puma) | 8088 | シングルプロセス (Puma workers=1) |
| Rust | Actix-web | 8089 | マルチスレッド (Actix workers=4) |
| Rust (single) | Actix-web | 8090 | シングルスレッド (Actix workers=1) |
| Rust | Axum | 8091 | マルチスレッド (Tokio workers=4) |
| Rust (single) | Axum | 8092 | シングルスレッド (Tokio workers=1) |
| Python | FastAPI (Uvicorn) | 8093 | マルチプロセス (Uvicorn workers=4) |
| Python (single) | FastAPI (Uvicorn) | 8094 | シングルプロセス (Uvicorn workers=1) |

## 起動

```bash
docker compose up --build -d
```

## 負荷テスト

```bash
# TypeScript (直列処理されることを確認)
./scripts/load-test.sh 3000

# Go (並行処理されることを確認)
./scripts/load-test.sh 8081

# Go GOMAXPROCS=1 (時分割処理されることを確認)
./scripts/load-test.sh 8082

# Java (並行処理されることを確認)
./scripts/load-test.sh 8083

# Java シングルスレッド (直列処理されることを確認)
./scripts/load-test.sh 8084

# PHP マルチプロセス (並行処理されることを確認)
./scripts/load-test.sh 8085

# PHP シングルプロセス (直列処理されることを確認)
./scripts/load-test.sh 8086

# Ruby マルチプロセス (並行処理されることを確認)
./scripts/load-test.sh 8087

# Ruby シングルプロセス (直列処理されることを確認)
./scripts/load-test.sh 8088

# Rust / Actix-web マルチスレッド (並行処理されることを確認)
./scripts/load-test.sh 8089

# Rust / Actix-web シングルスレッド (直列処理されることを確認)
./scripts/load-test.sh 8090

# Rust / Axum マルチスレッド (並行処理されることを確認)
./scripts/load-test.sh 8091

# Rust / Axum シングルスレッド (直列処理されることを確認)
./scripts/load-test.sh 8092

# Python / FastAPI マルチプロセス (並行処理されることを確認)
./scripts/load-test.sh 8093

# Python / FastAPI シングルプロセス (直列処理されることを確認)
./scripts/load-test.sh 8094
```

同時リクエスト数を変更する場合は第2引数で指定:

```bash
./scripts/load-test.sh 8081 5
```

## エンドポイント

| パス | 説明 |
|------|------|
| `GET /health` | ヘルスチェック |
| `GET /heavy` | CPU バウンド処理 (フィボナッチ再帰計算) を実行し、スレッド/プロセス ID と処理時間を返す |

### `/heavy` レスポンス例

```json
{
  "language": "go",
  "threadId": "goroutine-22",
  "startedAt": "2026-02-25T05:11:03.063Z",
  "finishedAt": "2026-02-25T05:11:03.586Z",
  "durationMs": 523
}
```

## 環境変数

| 変数 | 説明 | デフォルト |
|------|------|-----------|
| `HEAVY_CALC_N` | フィボナッチ計算の N 値 (大きいほど重い) | 43 |

```bash
HEAVY_CALC_N=35 docker compose up --build -d
```

## 停止

```bash
docker compose down
```
