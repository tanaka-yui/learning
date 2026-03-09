# cdn-go アーキテクチャ

Go で自作したリバースプロキシ + CDN キャッシュ。`Vary` ヘッダーに対応したキャッシュキー生成が特徴。

- ポート: 8084
- キャッシュストレージ: プロセス内メモリ (`map[string]*cacheEntry`)
- TTL管理: `Cache-Control: max-age` から取得
- Vary対応: `CACHE_VARY_HEADERS` 環境変数でホワイトリスト指定

## リクエストフロー

```mermaid
sequenceDiagram
    participant C as Client
    participant G as cdn-go :8084
    participant B as Backend (1 or 2)

    C->>G: GET /heavy?n=30<br/>Accept-Language: ja

    G->>G: キャッシュキー生成<br/>GET:/heavy?n=30<br/>[Accept-Language:ja]

    alt Cache HIT (TTL内)
        G->>G: メモリからエントリ取得
        G-->>C: 200 OK<br/>X-Cache: HIT<br/>X-Cache-Key: ...
    else Cache MISS
        G->>B: GET /heavy?n=30 (ReverseProxy)
        Note right of G: ModifyResponse で<br/>レスポンスをキャプチャ
        B-->>G: 200 OK<br/>Cache-Control: public, max-age=10<br/>X-Backend-Instance: backend-1
        G->>G: Cache-Control 解析
        alt キャッシュ可能
            G->>G: メモリに保存 (TTL=max-age)
            Note right of G: ログ: CACHE STORE<br/>TTL, backend 情報を記録
        end
        G-->>C: 200 OK<br/>X-Cache: MISS<br/>X-Cache-Key: ...<br/>X-Backend-Instance: backend-1
    end
```

## コンポーネント構成

```mermaid
flowchart TD
    REQ[HTTP Request] --> KEYGEN[キャッシュキー生成]

    subgraph キャッシュキー構成
        METHOD["METHOD"]
        PATH["PATH?sortedQuery"]
        VARY["Varyヘッダー値<br/>(CACHE_VARY_HEADERS で指定)"]
        FORMAT["KEY = METHOD:PATH [header:value,...]"]
        METHOD --> FORMAT
        PATH --> FORMAT
        VARY --> FORMAT
    end

    KEYGEN --> FORMAT
    FORMAT --> LOOKUP{キャッシュ検索<br/>sync.RWMutex RLock}

    LOOKUP -->|HIT & TTL有効| RESP_HIT[レスポンス返却<br/>X-Cache: HIT<br/>X-Cache-Key: key]
    LOOKUP -->|MISS or TTL切れ| PROXY[バックエンドへプロキシ<br/>httputil.ReverseProxy]

    PROXY --> BACKEND[Backend :8080]
    BACKEND -->|ModifyResponse| CC_PARSE[Cache-Control 解析]

    CC_PARSE --> CHECK{キャッシュ可否判定}
    CHECK -->|public + max-age有効<br/>no-cache/no-store/private なし| STORE[メモリに保存<br/>sync.RWMutex Lock]
    CHECK -->|キャッシュ不可| SKIP[保存スキップ]

    STORE --> LOG[詳細ログ出力<br/>CACHE STORE: TTL, backend]
    LOG --> RESP_MISS[レスポンス返却<br/>X-Cache: MISS]
    SKIP --> RESP_MISS

    subgraph メモリキャッシュストレージ
        MAP["map[string]*cacheEntry"]
        ENTRY["cacheEntry {<br/>statusCode int<br/>header http.Header<br/>body []byte<br/>cachedAt time.Time<br/>expiresAt time.Time<br/>}"]
        MAP --- ENTRY
    end

    STORE --> MAP
    LOOKUP -.->|読み取り| MAP
```
