# app-cache アーキテクチャ

アプリケーションプロセス内のインメモリキャッシュ。`sync.RWMutex` で保護された `map` にキャッシュエントリを保存する。

- ポート: 8081
- キャッシュストレージ: プロセス内メモリ (`map[string]*cacheEntry`)
- TTL管理: `Cache-Control: max-age` から取得

## リクエストフロー

```mermaid
sequenceDiagram
    participant C as Client
    participant A as app-cache :8081
    participant B as Backend (1 or 2)

    C->>A: GET /heavy?n=30

    A->>A: キャッシュキー生成<br/>GET:/heavy?n=30

    alt Cache HIT (TTL内)
        A->>A: メモリからエントリ取得
        A-->>C: 200 OK<br/>X-Cache: HIT
    else Cache MISS
        A->>B: GET /heavy?n=30 (プロキシ)
        B-->>A: 200 OK<br/>Cache-Control: public, max-age=10<br/>X-Backend-Instance: backend-1
        A->>A: Cache-Control 解析
        alt キャッシュ可能 (public + max-age)
            A->>A: メモリに保存 (TTL=max-age)
        end
        A-->>C: 200 OK<br/>X-Cache: MISS<br/>X-Backend-Instance: backend-1
    end
```

## コンポーネント構成

```mermaid
flowchart TD
    REQ[HTTP Request] --> KEYGEN[キャッシュキー生成<br/>METHOD:PATH?sortedQuery]
    KEYGEN --> LOOKUP{キャッシュ検索<br/>sync.RWMutex RLock}

    LOOKUP -->|HIT & TTL有効| RESP_HIT[レスポンス返却<br/>X-Cache: HIT]
    LOOKUP -->|MISS or TTL切れ| PROXY[バックエンドへプロキシ<br/>httputil.ReverseProxy]

    PROXY --> BACKEND[Backend :8080]
    BACKEND --> CC_PARSE[Cache-Control 解析]

    CC_PARSE --> CHECK{キャッシュ可否判定}
    CHECK -->|public + max-age有効<br/>no-cache/no-store なし| STORE[メモリに保存<br/>sync.RWMutex Lock]
    CHECK -->|キャッシュ不可| SKIP[保存スキップ]

    STORE --> RESP_MISS[レスポンス返却<br/>X-Cache: MISS]
    SKIP --> RESP_MISS

    subgraph メモリキャッシュストレージ
        MAP["map[string]*cacheEntry"]
        ENTRY["cacheEntry {<br/>statusCode int<br/>header http.Header<br/>body []byte<br/>cachedAt time.Time<br/>expiresAt time.Time<br/>}"]
        MAP --- ENTRY
    end

    STORE --> MAP
    LOOKUP -.->|読み取り| MAP
```
