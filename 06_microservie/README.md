# 06_microservie: マイクロサービス学習プロジェクト

ECサイトを題材に、小規模マイクロサービスの実装パターンを学ぶ章。

## 学習動線

1. [01: マイクロサービスとは](docs/01_concepts.md)
2. [02: メリット・デメリット](docs/02_pros_cons.md)
3. [03: コンウェイの法則と Team Topologies](docs/03_conway.md)
4. [04: サービス分割](docs/04_decomposition.md)
5. [05: 通信プロトコル](docs/05_communication.md)
6. [06: データ所有](docs/06_data_ownership.md)
7. [07: レジリエンス](docs/07_resilience.md)
8. [08: 観測性](docs/08_observability.md)
9. [09: 大規模化と Istio](docs/09_scaling_istio.md)
10. [10: コード上のパターン索引](docs/10_patterns_in_code.md)

## 起動

````bash
make up      # 全サービス起動
make down    # 停止
make logs    # ログ
````

## アクセス先

| URL | 用途 |
|---|---|
| http://localhost:8080/api/products | BFF REST API |
| http://localhost:16686 | Jaeger UI（分散トレース） |

## クイックスタート

````bash
make up      # 全コンテナ起動（frontend 含む）
make seed    # 商品10件・ユーザ2件投入
````

その後:
- `http://localhost:5173` — React UI
- `http://localhost:16686` — Jaeger UI

サインインに使えるダミーユーザ:
- `alice@example.com` / `password`
- `bob@example.com` / `password`
