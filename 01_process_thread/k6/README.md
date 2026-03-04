# k6 負荷テスト

[k6](https://k6.io/) を使った HTTP サーバーの負荷テスト。
VU (仮想ユーザー) を段階的に増やし、タイムアウトが発生し始める限界値を探る。

## 基本的な使い方 (Docker Compose)

```bash
# サーバー起動
docker compose up --build -d go-server

# デフォルト設定 (ramping, 50 VU, Go multi-goroutine)
docker compose run --rm k6

# 設定を変更して実行
K6_TARGET_VUS=30 K6_TEST_PROFILE=constant docker compose run --rm k6

# テスト対象サーバーを変更 (Go シングルスレッド)
K6_BASE_URL=http://go-server-single:8080 docker compose run --rm k6
```

Docker Compose 経由では `K6_` プレフィックス付きの環境変数で設定を渡す。

### ローカル実行 (k6 直接)

```bash
# インストール (macOS)
brew install k6

# 実行
k6 run k6/heavy-load.js

# 環境変数でカスタマイズ
k6 run -e BASE_URL=http://localhost:8082 -e TARGET_VUS=30 k6/heavy-load.js
```

## 環境変数一覧

Docker Compose 経由の場合は `K6_` プレフィックスを付ける (例: `K6_TARGET_VUS=100`)。
ローカル実行の場合は `-e` フラグで直接渡す (例: `-e TARGET_VUS=100`)。

| 変数名 | Docker Compose 変数名 | 説明 | デフォルト |
|---|---|---|---|
| `BASE_URL` | `K6_BASE_URL` | テスト対象の URL | Docker: `http://go-server:8080` / ローカル: `http://localhost:8081` |
| `ENDPOINT` | `K6_ENDPOINT` | テスト対象のパス | `/heavy` |
| `REQUEST_TIMEOUT` | `K6_REQUEST_TIMEOUT` | リクエストタイムアウト | `30s` |
| `TEST_PROFILE` | `K6_TEST_PROFILE` | テストプロファイル | `ramping` |
| `TARGET_VUS` | `K6_TARGET_VUS` | 目標 VU 数 | `50` |
| `DURATION` | `K6_DURATION` | ピーク維持時間 | `60s` |
| `RAMP_UP` | `K6_RAMP_UP` | 各ステージのランプアップ時間 | `30s` |
| `RAMP_DOWN` | `K6_RAMP_DOWN` | ランプダウン時間 | `10s` |
| `MAX_VUS` | `K6_MAX_VUS` | stress/spike テストの最大 VU | `200` |
| `SOAK_DURATION` | `K6_SOAK_DURATION` | soak テストの持続時間 | `5m` |
| `THRESHOLD_P95` | `K6_THRESHOLD_P95` | p95 レスポンスタイム閾値 (ms) | `10000` |
| `THRESHOLD_P99` | `K6_THRESHOLD_P99` | p99 レスポンスタイム閾値 (ms) | `15000` |
| `THRESHOLD_TIMEOUT_RATE` | `K6_THRESHOLD_TIMEOUT_RATE` | タイムアウト率の上限 | `0.1` |
| `THRESHOLD_ERROR_RATE` | `K6_THRESHOLD_ERROR_RATE` | エラー率の上限 | `0.05` |

## テストプロファイル

### `constant` — ベースライン計測

固定 VU 数で一定時間実行。安定動作の確認に使用。

```bash
K6_TEST_PROFILE=constant K6_TARGET_VUS=10 K6_DURATION=30s docker compose run --rm k6
```

### `ramping` — 段階的増加 (デフォルト・推奨)

VU を 25% → 50% → 75% → 100% と段階的に増加。タイムアウトが発生し始める閾値の特定に最適。

```bash
K6_TARGET_VUS=50 K6_RAMP_UP=20s docker compose run --rm k6
```

### `stress` — ブレイクポイント特定

VU を `MAX_VUS` まで段階的に引き上げ、限界を超えるまで追い込む。

```bash
K6_TEST_PROFILE=stress K6_MAX_VUS=200 K6_RAMP_UP=15s docker compose run --rm k6
```

### `spike` — 突発負荷

急激に VU を最大値まで増加。突発的なトラフィック増加への耐性を確認。

```bash
K6_TEST_PROFILE=spike K6_MAX_VUS=100 docker compose run --rm k6
```

### `soak` — 長時間安定性

中程度の負荷を長時間維持。メモリリークやリソース枯渇を検出。

```bash
K6_TEST_PROFILE=soak K6_TARGET_VUS=20 K6_SOAK_DURATION=10m docker compose run --rm k6
```

## 推奨ワークフロー: タイムアウト限界値の特定

```bash
# Step 1: サーバー起動
docker compose up --build -d go-server go-server-single

# Step 2: ベースライン (少 VU で正常動作を確認)
K6_TEST_PROFILE=constant K6_TARGET_VUS=5 K6_DURATION=30s docker compose run --rm k6

# Step 3: ランピング (段階的に負荷を増加して閾値を探索)
K6_TARGET_VUS=50 K6_RAMP_UP=20s K6_DURATION=30s docker compose run --rm k6

# Step 4: ストレス (限界まで追い込む)
K6_TEST_PROFILE=stress K6_MAX_VUS=200 K6_RAMP_UP=15s K6_DURATION=20s docker compose run --rm k6
```

## カスタムメトリクスの見方

k6 の標準メトリクスに加え、以下のカスタムメトリクスが出力される:

| メトリクス | 説明 |
|---|---|
| `timeout_count` | p95 閾値を超えたリクエストの総数 |
| `timeout_rate` | p95 閾値を超えたリクエストの割合 |
| `error_rate` | HTTP エラー (非 200) の割合 |
| `server_duration_ms` | サーバー側の処理時間 (レスポンスボディの `durationMs`) |

`server_duration_ms` と `http_req_duration` の差分を見ることで、ネットワーク遅延やキュー待ち時間を推定できる。

## テスト結果

テスト結果は `k6/results/` に JSON 形式で自動保存される (`.gitignore` 済み)。

## サーバーポート一覧

| サーバー | ホストポート | Docker 内 `K6_BASE_URL` |
|---|---|---|
| TypeScript (Fastify) | 3000 | `http://ts-server:3000` |
| Go (multi-goroutine) | 8081 | `http://go-server:8080` (デフォルト) |
| Go (GOMAXPROCS=1) | 8082 | `http://go-server-single:8080` |
| Java (multi-thread) | 8083 | `http://java-server:8081` |
| Java (single-thread) | 8084 | `http://java-server-single:8081` |
| PHP (multi-process) | 8085 | `http://php-server:8080` |
| PHP (single-process) | 8086 | `http://php-server-single:8080` |
| Ruby (multi-process) | 8087 | `http://ruby-server:8080` |
| Ruby (single-process) | 8088 | `http://ruby-server-single:8080` |
| Rust Actix (multi-thread) | 8089 | `http://rust-server:8080` |
| Rust Actix (single-thread) | 8090 | `http://rust-server-single:8080` |
| Rust Axum (multi-thread) | 8091 | `http://rust-axum-server:8080` |
| Rust Axum (single-thread) | 8092 | `http://rust-axum-server-single:8080` |
| Python (multi-process) | 8093 | `http://python-server:8080` |
| Python (single-process) | 8094 | `http://python-server-single:8080` |
