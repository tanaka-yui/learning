# shared-cache アーキテクチャ

Valkey（Redis互換）を外部キャッシュストレージとして使用する共有キャッシュ。複数プロセス間でキャッシュを共有できる。
自身でエンドポイントを持ち、キャッシュMISS時はfibonacci計算を直接実行し、結果をValkeyに保存する。

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

    C->>S: GET /heavy?n=30

    S->>S: キャッシュキー生成<br/>GET:/heavy?n=30

    S->>V: GET key
    alt Cache HIT (Valkey にデータあり)
        V-->>S: JSON データ返却
        S->>S: JSON デシリアライズ
        S-->>C: 200 OK<br/>X-Cache: HIT
    else Cache MISS (Valkey にデータなし)
        V-->>S: nil
        S->>S: Fibonacci計算実行
        S->>S: レスポンスを JSON シリアライズ
        S->>V: SETEX key TTL json_data
        S-->>C: 200 OK<br/>X-Cache: MISS<br/>X-Backend-Instance: shared-cache
    end
```

## コンポーネント構成

```mermaid
flowchart TD
    REQ[HTTP Request] --> KEYGEN[キャッシュキー生成<br/>METHOD:PATH?sortedQuery]
    KEYGEN --> VALKEY_GET{Valkey GET}

    VALKEY_GET -->|HIT| DESER[JSON デシリアライズ<br/>CachedResponse]
    DESER --> RESP_HIT[レスポンス返却<br/>X-Cache: HIT]

    VALKEY_GET -->|MISS| CALC[Fibonacci計算実行]
    CALC --> CC_GEN[Cache-Control ヘッダー生成<br/>public, max-age=TTL]

    CC_GEN --> SER[JSON シリアライズ]
    SER --> VALKEY_SET[Valkey SETEX<br/>TTL = max-age]
    VALKEY_SET --> RESP_MISS[レスポンス返却<br/>X-Cache: MISS]

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
