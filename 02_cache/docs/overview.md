# キャッシュパターン全体比較

## 全体アーキテクチャ

```mermaid
flowchart LR
    C[Client<br/>curl] --> A["app-cache<br/>:8081"]
    C --> S["shared-cache<br/>:8082"]
    C --> N["cdn-nginx<br/>:8083"]
    C --> G["cdn-go<br/>:8084"]

    A -->|プロキシ| B1[backend-1<br/>:8080]
    A -->|プロキシ| B2[backend-2<br/>:8080]

    S -->|プロキシ| B1
    S -->|プロキシ| B2
    S <-->|GET/SETEX| V[(Valkey<br/>:6379)]

    N -->|proxy_pass| B1
    N -->|proxy_pass| B2

    G -->|プロキシ| B1
    G -->|プロキシ| B2

    subgraph キャッシュストレージ
        A -.-|メモリ| MA["map + RWMutex"]
        S -.-|外部| V
        N -.-|ディスク| DN["/var/cache/nginx"]
        G -.-|メモリ| MG["map + RWMutex"]
    end
```

## パターン比較

| 特性 | app-cache | shared-cache | cdn-nginx | cdn-go |
|------|-----------|-------------|-----------|--------|
| ポート | 8081 | 8082 | 8083 | 8084 |
| ストレージ | プロセス内メモリ | Valkey (Redis互換) | ディスク + 共有メモリ | プロセス内メモリ |
| 永続化 | なし (再起動で消失) | あり (Valkey保存) | あり (ディスク保存) | なし (再起動で消失) |
| 複数プロセス共有 | 不可 | 可能 | 不可 (単一nginx内) | 不可 |
| TTL制御 | Cache-Control max-age | Cache-Control max-age | proxy_cache_valid | Cache-Control max-age |
| Vary対応 | なし | なし | なし | あり (ホワイトリスト) |
| キャッシュキー | METHOD:PATH?query | METHOD:PATH?query | scheme+method+host+uri | METHOD:PATH?query+headers |
| 実装言語 | Go | Go + Valkey | nginx設定 | Go |

## リクエストフロー比較

```mermaid
flowchart TD
    REQ[HTTP Request] --> PATTERN{キャッシュパターン}

    PATTERN -->|:8081| APP[app-cache]
    PATTERN -->|:8082| SHARED[shared-cache]
    PATTERN -->|:8083| NGINX[cdn-nginx]
    PATTERN -->|:8084| CDN_GO[cdn-go]

    APP --> APP_STORE["メモリ検索<br/>map[string]*cacheEntry<br/>sync.RWMutex"]
    SHARED --> SHARED_STORE["Valkey 検索<br/>go-redis GET<br/>JSON デシリアライズ"]
    NGINX --> NGINX_STORE["ディスク検索<br/>proxy_cache<br/>keys_zone メタデータ"]
    CDN_GO --> CDN_STORE["メモリ検索<br/>map[string]*cacheEntry<br/>sync.RWMutex + Vary"]

    APP_STORE -->|MISS| BACKEND[Backend<br/>Fibonacci計算]
    SHARED_STORE -->|MISS| BACKEND
    NGINX_STORE -->|MISS| BACKEND
    CDN_STORE -->|MISS| BACKEND

    APP_STORE -->|HIT| RESP[レスポンス<br/>X-Cache: HIT]
    SHARED_STORE -->|HIT| RESP
    NGINX_STORE -->|HIT| RESP
    CDN_STORE -->|HIT| RESP
```

## 詳細ドキュメント

- [app-cache アーキテクチャ](app-cache.md)
- [shared-cache アーキテクチャ](shared-cache.md)
- [cdn-nginx アーキテクチャ](cdn-nginx.md)
- [cdn-go アーキテクチャ](cdn-go.md)
