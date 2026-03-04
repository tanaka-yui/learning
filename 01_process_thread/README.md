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

## 前提条件

- Docker Desktop または Rancher Desktop (dockerd モード)
- [kind](https://kind.sigs.k8s.io/) — Kubernetes オートスケーリング実験に必要

```bash
brew install kind
```

## 起動

```bash
docker compose up --build -d
```

## 負荷テスト (シンプル版)

```bash
# Go (並行処理されることを確認)
./scripts/load-test.sh 8081

# 同時リクエスト数を変更する場合は第2引数で指定
./scripts/load-test.sh 8081 5
```

各ポートの対応は上記の「構成」テーブルを参照。

## 負荷テスト (k6)

[k6](https://k6.io/) (Docker 経由) を使った本格的な負荷テスト。3 種類のテストスクリプトを用意している。

#### 用語

- **VU (Virtual User)**: k6 が同時にリクエストを送信する仮想ユーザー数。実際のユーザーの同時アクセスをシミュレートする。VU=50 なら 50 人が同時にアクセスしている状況に相当
- **P95 / P99 (パーセンタイル)**: レスポンスタイムの統計指標。P95 は「全リクエストの 95% がこの時間以内に完了した」ことを意味する。平均値よりも外れ値の影響を受けにくく、ユーザー体感に近い指標として使われる
- **HPA (Horizontal Pod Autoscaler)**: Kubernetes のリソースで、Pod の CPU 使用率などのメトリクスに基づいて Pod 数を自動的にスケールアウト/インする
- **GIL (Global Interpreter Lock)**: Python (CPython) のインタプリタが持つグローバルロック。一度に 1 つのスレッドしか Python バイトコードを実行できないため、マルチスレッドでも CPU バウンド処理は並列化されない
- **JIT (Just-In-Time) コンパイル**: 実行時にバイトコードをネイティブマシンコードに変換する技術。V8 (Node.js)、JVM (Java) などが採用しており、インタプリタ言語より高速に動作する
- **FPM (FastCGI Process Manager)**: PHP の実行方式の一つ。複数のワーカープロセスを管理し、Web サーバーからのリクエストを各プロセスに振り分ける
- **GOMAXPROCS**: Go ランタイムが同時に使用する OS スレッドの最大数。デフォルトは CPU コア数。1 に設定すると goroutine が時分割で 1 スレッド上で実行される

### docker-compose で起動したサーバーに対して実行

```bash
# サーバーを起動
docker compose up --build -d

# Go サーバーに負荷テスト (ramping プロファイル)
K6_BASE_URL=http://go-server:8080 docker compose run --rm k6

# テストプロファイルや VU 数を変更
K6_BASE_URL=http://java-server:8081 K6_TEST_PROFILE=stress K6_TARGET_VUS=100 docker compose run --rm k6
```

### テストプロファイル

| プロファイル | 説明 |
|------------|------|
| `ramping` | VU を 25% → 50% → 75% → 100% と段階的に増加 (デフォルト) |
| `constant` | 固定 VU 数で一定時間実行 |
| `stress` | MAX_VUS まで段階的に引き上げてブレイクポイントを探索 |
| `spike` | 急激に最大 VU まで増加させて挙動を確認 |
| `soak` | 中程度の負荷を長時間維持して安定性を確認 |

### タイムアウト閾値探索 (`threshold-finder.js`)

VU 数を段階的に増やし、レスポンスタイムがタイムアウト閾値を超え始めるポイントを特定する。

```bash
# docker-compose 環境
K6_BASE_URL=http://go-server:8080 \
  docker compose run --rm k6 run /scripts/threshold-finder.js
```

| 変数 | 説明 | デフォルト |
|------|------|-----------|
| `START_VUS` | 開始 VU 数 | 1 |
| `MAX_VUS` | 最大 VU 数 | 100 |
| `STEP_SIZE` | VU 増加ステップ | 5 |
| `STEP_DURATION` | 各ステップの持続時間 | 30s |
| `TIMEOUT_THRESHOLD` | タイムアウトとみなすレスポンスタイム (ms) | 5000 |

## Kubernetes オートスケーリング実験

kind で Kubernetes クラスタを構築し、HPA (Horizontal Pod Autoscaler) によるオートスケーリングを体験する。負荷が増加すると Pod 数が自動的に増え、負荷が下がると Pod 数が減る様子を観察できる。

### アーキテクチャ

```
[k6 (Docker)] → [Ingress Nginx (port 8080)] → [Service] → [Pod(s)]
                                                           ↑
                                                    HPA (CPU 50%)
                                                    min=1, max=10
```

- Ingress がパスベースで各言語にルーティング (`/go/heavy` → Go, `/ts/heavy` → TS, etc.)
- HPA が CPU 使用率 50% を超えると自動的に Pod を追加

### クイックスタート

```bash
# 全環境を一発で構築 (ビルド → kind クラスタ作成 → デプロイ)
make up
```

`make up` は以下を順番に実行する:

1. 8 言語の Docker イメージをビルド
2. kind クラスタを作成 (1 control-plane + 3 worker ノード)
3. イメージを kind クラスタにロード
4. metrics-server + ingress-nginx をデプロイ
5. 全言語の Deployment / Service / HPA をデプロイ

### エンドポイント確認

```bash
curl http://localhost:8080/go/health
curl http://localhost:8080/go/heavy
curl http://localhost:8080/java/heavy
```

| パス | 対象サーバー |
|------|-------------|
| `/go/*` | Go |
| `/ts/*` | TypeScript |
| `/java/*` | Java |
| `/php/*` | PHP |
| `/ruby/*` | Ruby |
| `/rust-actix/*` | Rust (Actix-web) |
| `/rust-axum/*` | Rust (Axum) |
| `/python/*` | Python |

### 負荷テスト実行

k6 は Docker (`grafana/k6`) 経由で実行される。テスト中はリアルタイムダッシュボード、テスト後は HTML レポートで結果を確認できる。

```bash
make test K6_TARGET=go
make test-find-threshold K6_TARGET=go
make test-autoscale K6_TARGET=go
```

#### k6 Web Dashboard

テスト実行中に http://localhost:5665 でリアルタイムダッシュボードを表示できる。テスト終了後は `k6/results/report.html` に HTML レポートが自動保存される。

- **Overview タブ**: リクエスト数、エラー率、レスポンスタイムの推移
- **Timings タブ**: p95/p99 などレスポンスタイムの詳細
- **Summary タブ**: テスト全体の集計結果

#### 通常の負荷テスト (`make test`)

VU (仮想ユーザー) 数を段階的に増加させて、サーバーのレスポンスタイムやエラー率を測定する。テストプロファイルで負荷パターンを変更できる。

```bash
# デフォルト (ramping プロファイル、50 VU)
make test K6_TARGET=go

# プロファイルや VU 数を変更
make test K6_TARGET=ts K6_TEST_PROFILE=stress K6_TARGET_VUS=100
```

テストプロファイル:

| プロファイル | 説明 |
|------------|------|
| `ramping` | VU を 25% → 50% → 75% → 100% と段階的に増加 (デフォルト) |
| `constant` | 固定 VU 数で一定時間実行 |
| `stress` | MAX_VUS まで段階的に引き上げてブレイクポイントを探索 |
| `spike` | 急激に最大 VU まで増加させて挙動を確認 |
| `soak` | 中程度の負荷を長時間維持して安定性を確認 |

#### タイムアウト閾値探索 (`make test-find-threshold`)

VU 数を `STEP_SIZE` ずつ段階的に増やし、レスポンスタイムがタイムアウト閾値を超え始めるポイントを特定する。「この言語のサーバーは何 VU まで耐えられるか？」を調べるのに使う。

```bash
make test-find-threshold K6_TARGET=go

# ステップ幅やステップ時間を変更
make test-find-threshold K6_TARGET=ts K6_MAX_VUS=100 K6_STEP_SIZE=10
```

#### HPA オートスケーリングテスト (`make test-autoscale`)

4 フェーズで HPA の動作を観察する:

| フェーズ | 内容 | VU 数 | 時間 |
|---------|------|-------|------|
| 1. Warmup | ベースライン計測 | 5 | 30s |
| 2. Ramp-up | 負荷を急増させスケールアウトを誘発 | → 80 | 1m |
| 3. Sustain | ピーク負荷を維持して Pod 増加を観察 | 80 | 5m |
| 4. Cooldown | 負荷を下げてスケールダウンを観察 | → 5 | 2m |

別ターミナルで HPA や Pod の状態を監視しながら実行するのがおすすめ:

```bash
# ターミナル 1: HPA 監視 (Pod 数の変化をリアルタイムで確認)
make watch-hpa

# ターミナル 2: 負荷テスト実行
make test-autoscale K6_TARGET=go
```

### モニタリング

```bash
make status       # 全リソースの状態を一覧表示 (Nodes / HPAs / Pods / Services / Ingress)
make watch-hpa    # HPA のスケーリング状態をリアルタイム監視
make watch-pods   # Pod の状態をリアルタイム監視
make logs K6_TARGET=go   # 指定サービスのログを表示
make events       # Kubernetes イベント (スケールアウト/イン) を表示
```

### make コマンド一覧

| コマンド | 説明 |
|---------|------|
| `make up` | 全環境構築 (ビルド → クラスタ作成 → デプロイ) |
| `make down` | クラスタ削除 |
| `make build` | 全言語の Docker イメージをビルド |
| `make build-<lang>` | 個別ビルド (例: `make build-go`) |
| `make cluster-create` | kind クラスタ作成 |
| `make cluster-delete` | kind クラスタ削除 |
| `make load-images` | イメージを kind にロード |
| `make deploy` | インフラ + アプリをデプロイ |
| `make redeploy` | イメージ再ビルド + 再ロード + ローリング再デプロイ |
| `make test` | k6 負荷テスト |
| `make test-find-threshold` | タイムアウト閾値探索 |
| `make test-find-threshold-all` | 全言語の閾値探索を一括実行 |
| `make test-autoscale` | HPA 動作確認テスト |
| `make status` | 全リソース状態表示 |
| `make watch-hpa` | HPA リアルタイム監視 |
| `make watch-pods` | Pod リアルタイム監視 |
| `make logs` | サービスログ表示 |
| `make events` | Kubernetes イベント表示 |

### 設定変数

| 変数 | 説明 | デフォルト |
|------|------|-----------|
| `K6_TARGET` | テスト対象の言語 | `go` |
| `K6_TARGET_VUS` | 目標 VU 数 | `50` |
| `K6_TEST_PROFILE` | テストプロファイル | `ramping` |
| `K6_MAX_VUS` | 最大 VU 数 | `200` |
| `K6_PEAK_VUS` | autoscale テストのピーク VU 数 | `80` |
| `K6_STEP_SIZE` | 閾値探索のステップ幅 | `5` |
| `K6_STEP_DURATION` | 閾値探索の各ステップ時間 | `30s` |
| `K6_HEAVY_CALC_N` | k6 テスト時のフィボナッチ N 値 (言語別デフォルトあり) | 言語依存 |

`K6_TARGET` に指定可能な値: `go`, `ts`, `java`, `php`, `ruby`, `rust-actix`, `rust-axum`, `python`

#### 言語別 HEAVY_CALC_N デフォルト値

k6 テスト時に `?n=` クエリパラメータで各サーバーに送信される。1 リクエストあたり約 200ms を目標に調整済み。

| 言語 | N | 速度特性 |
|------|---|---------|
| Go / Rust / Java | 39 | ネイティブ / JIT コンパイル |
| TypeScript | 37 | V8 JIT |
| Ruby / PHP | 30 | インタプリタ |
| Python | 29 | インタプリタ |

```bash
# 言語別デフォルト値が自動選択される
make test K6_TARGET=ts          # N=37

# 手動オーバーライド
make test K6_TARGET=ts K6_HEAVY_CALC_N=35
```

### 負荷閾値探索の結果

`make test-find-threshold-exclusive` で全言語の閾値を探索した結果。K8s 環境 (kind, CPU limit: 1) で、1 Pod (1 プロセス / 1 ワーカー) あたり何 VU でタイムアウト (>5000ms) が発生し始めるかを測定。

**テスト条件**: MAX_VUS=100, STEP_SIZE=5, STEP_DURATION=15s, タイムアウト閾値=5000ms

> **注**: 各言語とも 1 プロセス / 1 ワーカーで統一。Go は GOMAXPROCS=1、Java は Tomcat threads.max=1、TypeScript はシングルスレッド (Node.js)。各言語のみを 1 Pod デプロイし、1 CPU を占有した状態でテスト。

| 言語 | N | 初回タイムアウト | タイムアウト率 | 平均応答時間 | P95 | スループット |
|------|:-:|:---------------:|:------------:|:-----------:|:---:|:-----------:|
| Go | 39 | ~31 VUs | 54.21% | 6.19s | 13.35s | 7.67/s |
| TypeScript | 37 | ~11 VUs | 2.70% | 501ms | 1.81s | 50.11/s |
| Java | 39 | ~36 VUs | 57.82% | 5.59s | 10.03s | 8.43/s |
| PHP | 30 | なし | 0.00% | 795ms | 2.36s | 39.30/s |
| Ruby | 30 | なし | 0.00% | 91ms | 325ms | 86.11/s |
| Rust Actix | 39 | ~16 VUs | 31.28% | 5.16s | 13.97s | 6.24/s |
| Rust Axum | 39 | ~26 VUs | 8.74% | 1.13s | 9.29s | 30.63/s |
| Python | 29 | ~46 VUs | 6.10% | 1.33s | 6.95s | 27.93/s |

#### 考察

- **Ruby / PHP はタイムアウトなし** — Ruby (Puma) は 86.11/s、PHP (FPM) は 39.30/s と高スループット。1 ワーカーでも CPU バウンド処理を効率的にさばいている。ただし N 値が 30 と他言語より軽い計算である点に注意
- **Go / Java は高タイムアウト率** — Go (54.21%) と Java (57.82%) は CPU バウンド処理で全 goroutine/スレッドが飽和し、キュー全体が遅延
- **TypeScript は独特のパターン** — 初回タイムアウト ~11 VUs と早いが、タイムアウト率は 2.70% と非常に低い。シングルスレッドのため CPU バウンド処理中は他のリクエストがブロックされるが、V8 JIT の高速性で 1 リクエストの処理が速く、大部分のリクエストは間に合う
- **Rust Axum > Rust Actix** — 同じ N=39 でも Axum (8.74%, 30.63/s) が Actix (31.28%, 6.24/s) を大きく上回る。Tokio ランタイムのスケジューリングが CPU バウンド処理に優位に働いている可能性
- **Python は N=29 で健闘** — GIL の制約がありつつも、初回タイムアウト ~46 VUs と最も遅く、タイムアウト率 6.10% と低い。uvicorn の非同期 I/O と軽い計算負荷 (N=29) が寄与
- HEAVY_CALC_N が言語によって異なるため、直接的な言語間比較には注意が必要
- この結果は **1 Pod (1 ワーカー) での限界**を示しており、HPA オートスケーリングで Pod 数を増やすことで改善される — `make test-autoscale` で確認可能

### クリーンアップ

```bash
make down
```

## エンドポイント

| パス | 説明 |
|------|------|
| `GET /health` | ヘルスチェック |
| `GET /heavy` | CPU バウンド処理 (フィボナッチ再帰計算) を実行し、スレッド/プロセス ID と処理時間を返す |
| `GET /heavy?n=35` | クエリパラメータで N 値を指定 (省略時は環境変数 `HEAVY_CALC_N` のデフォルト値を使用) |

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
