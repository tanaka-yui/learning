# 03_security_measures 設計ドキュメント

## 概要

OWASP Top 10を中心としたWebアプリケーションセキュリティの学習モジュール。各攻撃手法について、脆弱なコード（vulnerable）と対策済みコード（secure）を用意し、実際に攻撃を再現しながら学べる環境を構築する。

## 対象攻撃手法

1. **SQLインジェクション** (`sql-injection/`)
2. **XSS（クロスサイトスクリプティング）** (`xss/`)
3. **CSRF（クロスサイトリクエストフォージェリ）** (`csrf/`)
4. **認証の不備** (`auth-bypass/`)
5. **パストラバーサル** (`path-traversal/`)
6. **コマンドインジェクション** (`command-injection/`)

## ディレクトリ構成

```
03_security_measures/
├── README.md
├── Makefile
├── docker-compose.yml
├── docs/
│   ├── overview.md
│   ├── sql-injection.md
│   ├── xss.md
│   ├── csrf.md
│   ├── auth-bypass.md
│   ├── path-traversal.md
│   └── command-injection.md
├── sql-injection/
│   ├── vulnerable/
│   │   ├── backend/        # Go
│   │   │   ├── main.go
│   │   │   ├── go.mod
│   │   │   └── Dockerfile
│   │   └── frontend/       # React
│   │       ├── src/
│   │       ├── package.json
│   │       └── Dockerfile
│   └── secure/
│       ├── backend/
│       └── frontend/
├── xss/
│   ├── vulnerable/
│   └── secure/
├── csrf/
│   ├── vulnerable/
│   └── secure/
├── auth-bypass/
│   ├── vulnerable/
│   └── secure/
├── path-traversal/
│   ├── vulnerable/
│   └── secure/
└── command-injection/
    ├── vulnerable/
    └── secure/
```

各攻撃手法ディレクトリは `vulnerable/` と `secure/` を持ち、それぞれにGoバックエンド + Reactフロントエンドを含む。

## 各攻撃手法のデモ内容

### 1. SQLインジェクション (`sql-injection/`)

- **バックエンド:** ユーザー検索API。脆弱版は文字列結合でSQL構築、対策版はプリペアドステートメント使用
- **フロントエンド:** ユーザー検索フォーム
- **DB:** SQLite
- **攻撃例:** `' OR '1'='1` で全ユーザー取得、`'; DROP TABLE users;--` でテーブル削除

### 2. XSS (`xss/`)

- **バックエンド:** 掲示板API。脆弱版は投稿内容を未サニタイズでレスポンス、対策版はエスケープ処理
- **フロントエンド:** 脆弱版は `dangerouslySetInnerHTML` で描画、対策版はテキストノードで描画
- **攻撃例:** `<script>alert('XSS')</script>` や `<img onerror="..." src="x">` の投稿

### 3. CSRF (`csrf/`)

- **バックエンド:** パスワード変更API。脆弱版はCSRFトークンなし、対策版はCSRFトークン検証 + SameSite Cookie
- **フロントエンド:** パスワード変更フォーム + 攻撃者サイトを模したHTMLページ
- **攻撃例:** 罠サイトからの自動POST送信で、ログイン中ユーザーのパスワードを変更

### 4. 認証の不備 (`auth-bypass/`)

- **バックエンド:** ログインAPI。脆弱版はレート制限なし・平文パスワード保存・セッション固定、対策版はbcrypt + レート制限 + セッション再生成
- **フロントエンド:** ログインフォーム + 管理画面
- **攻撃例:** ブルートフォース攻撃、セッションID固定攻撃

### 5. パストラバーサル (`path-traversal/`)

- **バックエンド:** ファイルダウンロードAPI。脆弱版はユーザー入力のパスをそのまま使用、対策版は `filepath.Clean` + ベースディレクトリ検証
- **フロントエンド:** ファイル一覧・ダウンロードUI
- **攻撃例:** `../../etc/passwd` でシステムファイル取得

### 6. コマンドインジェクション (`command-injection/`)

- **バックエンド:** DNS lookup / pingツールAPI。脆弱版は `exec.Command("sh", "-c", input)`、対策版は引数を分離して渡す
- **フロントエンド:** ホスト名入力フォーム
- **攻撃例:** `example.com; cat /etc/passwd` でコマンド連結実行

## 技術スタック

### バックエンド (Go)

- **HTTPルーター:** 標準ライブラリ `net/http`（学習目的のためフレームワークなし）
- **DB:** SQLite（`github.com/mattn/go-sqlite3`）— SQLインジェクション・認証で使用
- **ポート:** 攻撃手法ごとに分離（脆弱版: 偶数ポート, 対策版: 奇数ポート）

### フロントエンド (React)

- **ビルドツール:** Vite
- **UIライブラリ:** 素のCSS（学習に集中するためUIライブラリは使わない）

### Docker Compose

- Composeプロファイルで攻撃手法ごとに分離
- 起動例: `docker compose --profile sql-injection up`

### ポート割り当て

| 攻撃手法 | Backend (vulnerable) | Backend (secure) | Frontend (vulnerable) | Frontend (secure) |
|---|---|---|---|---|
| sql-injection | 8080 | 8081 | 3000 | 3001 |
| xss | 8082 | 8083 | 3002 | 3003 |
| csrf | 8084 | 8085 | 3004 | 3005 |
| auth-bypass | 8086 | 8087 | 3006 | 3007 |
| path-traversal | 8088 | 8089 | 3008 | 3009 |
| command-injection | 8090 | 8091 | 3010 | 3011 |

## ドキュメント構成 (各 `docs/*.md`)

1. **概要** — 攻撃の仕組みを図解付きで説明
2. **脆弱なコードの解説** — なぜ脆弱かをコード参照で解説
3. **攻撃手順** — ステップバイステップで攻撃を再現する方法
4. **対策コードの解説** — 何をどう修正したかを差分で解説
5. **まとめ** — ベストプラクティスとチェックリスト

## Makefile

- `make sql-injection` — 個別起動
- `make all` — 全デモ起動
- `make down` — 全停止
