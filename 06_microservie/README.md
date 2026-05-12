# 06_microservie: マイクロサービス学習プロジェクト

ECサイトを題材に、小規模マイクロサービスの実装パターンを学ぶ章。

> 本章は段階的に構築中です。詳細なドキュメントは `docs/` 配下に追加されます。

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

詳細な学習動線は `docs/` に整備予定。
