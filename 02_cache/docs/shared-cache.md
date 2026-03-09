# shared-cache アーキテクチャ

Valkey（Redis互換）を外部キャッシュストレージとして使用する共有キャッシュ。複数プロセス間でキャッシュを共有できる。

- ポート: 8082
- キャッシュストレージ: Valkey (Redis互換) :6379
- TTL管理: Valkey の `SetEx` (TTL付き保存)
- シリアライズ: JSON

## リクエストフロー

```mermaid
sequenceDiagram
    participant C as Client
    participant S as shared-cache :8082
    participant V as Valkey :6379
    participant B as Backend (1 or 2)

    C->>S: GET /heavy?n=30

    S->>S: キャッシュキー生成<br/>GET:/heavy?n=30

    S->>V: GET key
    alt Cache HIT (Valkey にデータあり)
        V-->>S: JSON データ返却
        S->>S: JSON デシリアライズ
        S-->>C: 200 OK<br/>X-Cache: HIT
    else Cache MISS (Valkey にデータなし)
        V-->>S: nil
        S->>B: GET /heavy?n=30 (プロキシ)
        B-->>S: 200 OK<br/>Cache-Control: public, max-age=10<br/>X-Backend-Instance: backend-1
        S->>S: Cache-Control 解析
        alt キャッシュ可能 (public + max-age)
            S->>S: レスポンスを JSON シリアライズ
            S->>V: SETEX key TTL json_data
        end
        S-->>C: 200 OK<br/>X-Cache: MISS<br/>X-Backend-Instance: backend-1
    end
```

## コンポーネント構成

```mermaid
flowchart TD
    REQ[HTTP Request] --> KEYGEN[キャッシュキー生成<br/>METHOD:PATH?sortedQuery]
    KEYGEN --> VALKEY_GET{Valkey GET}

    VALKEY_GET -->|HIT| DESER[JSON デシリアライズ<br/>CachedResponse]
    DESER --> RESP_HIT[レスポンス返却<br/>X-Cache: HIT]

    VALKEY_GET -->|MISS| PROXY[バックエンドへプロキシ<br/>httputil.ReverseProxy]
    PROXY --> BACKEND[Backend :8080]
    BACKEND --> CC_PARSE[Cache-Control 解析]

    CC_PARSE --> CHECK{キャッシュ可否判定}
    CHECK -->|public + max-age有効| SER[JSON シリアライズ]
    CHECK -->|キャッシュ不可| SKIP[保存スキップ]

    SER --> VALKEY_SET[Valkey SETEX<br/>TTL = max-age]
    VALKEY_SET --> RESP_MISS[レスポンス返却<br/>X-Cache: MISS]
    SKIP --> RESP_MISS

    subgraph Valkey ストレージ
        VDATA["Key: GET:/heavy?n=30"]
        VSTRUCT["CachedResponse {<br/>StatusCode int<br/>Headers map[string][]string<br/>Body string<br/>}"]
        VTTL["TTL: max-age 秒"]
        VDATA --- VSTRUCT
        VSTRUCT --- VTTL
    end

    VALKEY_SET --> VDATA
    VALKEY_GET -.->|読み取り| VDATA
```
