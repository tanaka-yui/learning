# cdn-nginx アーキテクチャ

nginx の `proxy_cache` モジュールによるディスクベースキャッシュ。設定ファイルのみでキャッシュ動作を制御する、本番環境で広く使われるパターン。

- ポート: 8083
- キャッシュストレージ: ディスク (`/var/cache/nginx`) + 共有メモリ (`keys_zone`)
- TTL管理: `proxy_cache_valid` ディレクティブ

## リクエストフロー

```mermaid
sequenceDiagram
    participant C as Client
    participant N as cdn-nginx :8083
    participant D as ディスクキャッシュ<br/>/var/cache/nginx
    participant B as Backend (1 or 2)

    C->>N: GET /heavy?n=30

    N->>N: キャッシュキー生成<br/>scheme + method + host + uri

    N->>D: キャッシュ検索
    alt Cache HIT (ディスクにデータあり)
        D-->>N: キャッシュデータ返却
        N-->>C: 200 OK<br/>X-Cache-Status: HIT
        Note right of N: upstream addr = "-"
    else Cache MISS
        D-->>N: キャッシュなし
        N->>B: GET /heavy?n=30 (proxy_pass)
        Note right of N: upstream: ラウンドロビン
        B-->>N: 200 OK<br/>Cache-Control: public, max-age=10<br/>X-Backend-Instance: backend-1
        alt HTTP 200 レスポンス
            N->>D: キャッシュ保存 (10秒有効)
        end
        N-->>C: 200 OK<br/>X-Cache-Status: MISS<br/>X-Backend-Instance: backend-1
    end
```

## コンポーネント構成

```mermaid
flowchart TD
    REQ[HTTP Request] --> KEY[キャッシュキー生成<br/>proxy_cache_key<br/>$scheme$request_method$host$request_uri]
    KEY --> LOOKUP{ディスクキャッシュ検索}

    LOOKUP -->|HIT| RESP_HIT[レスポンス返却<br/>X-Cache-Status: HIT]
    LOOKUP -->|MISS| UPSTREAM[upstream backend へプロキシ<br/>proxy_pass]

    UPSTREAM --> LB{ラウンドロビン<br/>ロードバランシング}
    LB --> B1[backend-1 :8080]
    LB --> B2[backend-2 :8080]

    B1 --> VALID{proxy_cache_valid<br/>200 レスポンス?}
    B2 --> VALID

    VALID -->|200 OK| STORE[ディスクに保存<br/>有効期限: 10秒]
    VALID -->|その他| SKIP[保存スキップ]

    STORE --> RESP_MISS[レスポンス返却<br/>X-Cache-Status: MISS]
    SKIP --> RESP_MISS

    subgraph nginx キャッシュストレージ
        ZONE["keys_zone: cdn_cache:10m<br/>（キャッシュメタデータ用共有メモリ）"]
        DISK["proxy_cache_path: /var/cache/nginx<br/>levels=1:2<br/>max_size=100m"]
        INACTIVE["inactive=60m<br/>（アクセスなし60分で削除）"]
        ZONE --- DISK
        DISK --- INACTIVE
    end

    STORE --> DISK
    LOOKUP -.->|読み取り| ZONE
```
