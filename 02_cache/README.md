# 02_cache: キャッシュ動作理解プロジェクト

キャッシュの仕組み理解とパフォーマンス比較を目的とした学習環境。
各キャッシュ層（アプリ/共有/CDN）を独立したコンポーネントとして実装し、キャッシュの有無によるパフォーマンス差を比較する構成。

## アーキテクチャ

```
クライアント (curl)
  ├─ app-cache    (:8081) → [インメモリキャッシュ + 自身で計算]
  ├─ shared-cache (:8082) → [Valkey キャッシュ + 自身で計算]
  ├─ cdn-nginx    (:8083) → [nginx proxy_cache]      → backend-1/2
  └─ cdn-go       (:8084) → [Go自作CDNキャッシュ]      → backend-1/2

バックエンド: Go HTTPサーバー × 2インスタンス (cdn-nginx/cdn-go 用)
```

## 前提条件

- Docker / Docker Compose
- curl
- bc (パフォーマンス比較スクリプト用)

## セットアップ

```bash
# 全サービス起動
make up

# 動作確認
curl -D - http://localhost:8081/heavy?n=30   # app-cache
curl -D - http://localhost:8082/heavy?n=30   # shared-cache (Valkey)
curl -D - http://localhost:8083/heavy?n=30   # cdn-nginx
curl -D - http://localhost:8084/heavy?n=30   # cdn-go
```

## コマンド

| コマンド | 説明 |
|---------|------|
| `make up` | 全サービス起動（ビルド込み） |
| `make down` | 全サービス停止 |
| `make test` | キャッシュHIT/MISS動作テスト |
| `make compare` | パフォーマンス比較 |
| `make clean` | キャッシュクリア |
| `make logs` | 全サービスのログ表示 |

## キャッシュ方式の比較

各パターンの詳細なアーキテクチャ図は [docs/overview.md](docs/overview.md) を参照。

### アプリキャッシュ (app-cache)

アプリケーションプロセス内の変数（`map + sync.RWMutex`）にキャッシュ。
自身でエンドポイント（`/heavy`）を持ち、キャッシュMISS時はfibonacci計算を直接実行する。
最も高速だが、プロセス再起動でキャッシュが消失する。複数プロセス間で共有不可。

-> [アーキテクチャ詳細](docs/app-cache.md)

### 共有キャッシュ (shared-cache + Valkey)

Valkey（Redis互換）にキャッシュを保存。
自身でエンドポイント（`/heavy`）を持ち、キャッシュMISS時はfibonacci計算を直接実行し、結果をValkeyに保存する。
複数プロセスからキャッシュを共有可能。ネットワーク通信のオーバーヘッドがある。

-> [アーキテクチャ詳細](docs/shared-cache.md)

### CDNキャッシュ - nginx版 (cdn-nginx)

nginxの`proxy_cache`モジュールでキャッシュ。
本番環境で広く使われるパターン。設定ファイルベースで制御。

-> [アーキテクチャ詳細](docs/cdn-nginx.md)

### CDNキャッシュ - Go自作版 (cdn-go)

Goでリバースプロキシ + キャッシュを自作。
キャッシュキーにリクエストヘッダー（ホワイトリスト方式）を含める。
`Cache-Control`ヘッダーを解析してキャッシュ可否・TTLを判定。

-> [アーキテクチャ詳細](docs/cdn-go.md)

## レスポンスヘッダー

全キャッシュ層で以下のヘッダーを統一的に返す:

- `X-Cache: HIT` / `MISS` — キャッシュのヒット/ミス
- `X-Backend-Instance` — 応答したバックエンドインスタンス（キャッシュHIT時は含まれない場合あり）
