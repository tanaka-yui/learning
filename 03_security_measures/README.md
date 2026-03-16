# 03_security_measures: Webアプリケーションセキュリティ学習

OWASP Top 10を中心とした学習環境。脆弱コードと対策コードを用意。

## 攻撃タイプ一覧

| 攻撃タイプ | バックエンド (脆弱/対策) | フロントエンド (脆弱/対策) |
|-----------|------------------------|-------------------------|
| sql-injection | 8080 / 8081 | 3000 / 3001 |
| xss | 8082 / 8083 | 3002 / 3003 |
| csrf | 8084 / 8085 | 3004 / 3005 |
| auth-bypass | 8086 / 8087 | 3006 / 3007 |
| path-traversal | 8088 / 8089 | 3008 / 3009 |
| command-injection | 8090 / 8091 | 3010 / 3011 |

## 前提条件

- Docker / Docker Compose
- (任意) Go 1.22+
- (任意) Node.js 20+

## 使い方

```bash
# 個別の攻撃タイプを起動
make sql-injection
make xss
make csrf
make auth-bypass
make path-traversal
make command-injection

# 全デモ起動
make all

# 全サービス停止
make down
```

## ドキュメント

- [概要・学習ガイド](docs/overview.md)
