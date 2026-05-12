# 06_microservie Frontend (Plan 3) 設計書

作成日: 2026-05-13
対象: `06_microservie/frontend/` の React アプリと、それに伴う BFF / user-auth の小規模拡張
親仕様: `docs/superpowers/specs/2026-05-12-microservices-chapter-design.md`（章設計書）の 5 節を実装に落とす

---

## 1. ゴールとスコープ

### 1.1 ゴール

1. 学習者が `make up` 後にブラウザから商品閲覧・カート操作・注文・履歴閲覧をひと通り体験できる
2. 注文確定/エラー画面で trace_id を取得し、Jaeger に直接遷移して checkout の trace を観察できる
3. フロントエンド構造そのものが「層が見える/単機能テスト可能/読みやすい」ものとして教材になる

### 1.2 スコープ内

- Vite + React 18 + TypeScript + React Router v6 による SPA
- 軽量カスタム hook と純粋関数の fetch ラッパ
- HttpOnly Cookie ベースの認証フロー（probe + signout を含む）
- BFF への追加: `GET /api/products/:id`, `GET /api/auth/me`, `POST /api/auth/signout`, エラー JSON 形式の統一, `X-Trace-Id` ヘッダ
- user-auth gRPC への追加: `GetUser`
- docker-compose 統合（vite dev server）
- Vitest による軽量ユニットテスト

### 1.3 スコープ外

- E2E ツール（Cypress / Playwright）
- React Testing Library を使ったコンポーネント描画テスト
- 本番ビルド用 Dockerfile（教材なので開発用のみ）
- Tailwind / 既製 UI ライブラリ
- レスポンシブ / アクセシビリティの作り込み
- 別タブ間のカート同期、ストア再構築（Zustand 等）
- サインアウト以外のセッション管理（リフレッシュトークン等）

---

## 2. ディレクトリ構成

```
06_microservie/frontend/
├── Dockerfile.dev
├── index.html
├── package.json
├── tsconfig.json
├── vite.config.ts
├── public/
└── src/
    ├── main.tsx                      # createRoot、AuthProvider、BrowserRouter
    ├── App.tsx                       # ルーティング表
    ├── api/                          # fetch ラッパ層（純粋関数）
    │   ├── http.ts                   # apiFetch, ApiError
    │   ├── http.test.ts
    │   ├── products.ts               # listProducts, getProduct
    │   ├── auth.ts                   # signIn, signUp, signOut, me
    │   ├── checkout.ts               # postCheckout
    │   └── orders.ts                 # listOrders, getOrder
    ├── hooks/
    │   ├── useAuth.tsx               # AuthProvider + useAuth
    │   ├── useCart.ts
    │   └── useCart.test.ts
    ├── components/
    │   ├── Layout.tsx                # 共通ヘッダ + <Outlet>
    │   ├── RequireAuth.tsx           # 認証必須ルート用ラッパ
    │   ├── ErrorBanner.tsx           # エラー表示 + trace_id チップ
    │   ├── TraceIdChip.tsx           # trace_id コピー + Jaeger リンク
    │   └── ProductCard.tsx
    ├── pages/
    │   ├── Products.tsx
    │   ├── ProductDetail.tsx
    │   ├── Cart.tsx
    │   ├── Checkout.tsx              # 注文確定画面 + 完了表示
    │   ├── Orders.tsx
    │   ├── OrderDetail.tsx
    │   ├── SignIn.tsx
    │   └── SignUp.tsx
    ├── lib/
    │   ├── format.ts                 # formatPrice, shortTraceId
    │   └── format.test.ts
    └── styles.css                    # 単一の軽量 CSS
```

---

## 3. アーキテクチャ

### 3.1 層の責務

```
┌────────────────────────────────────────────────────────┐
│  Browser (Vite Dev Server :5173)                       │
│                                                        │
│  pages/         ← ルート単位の画面コンポーネント         │
│   └─ uses ──→  hooks/      ← useFetch / useAuth / useCart
│                  └─ uses ──→ api/  ← fetch ラッパ層    │
│                                 │                      │
│              components/        │                      │
│               (Layout, ErrorBanner, TraceIdChip など)  │
└──────────────────────────────────┼─────────────────────┘
                                   │ credentials: 'include'
                                   ▼
                          BFF :8080 (HttpOnly Cookie)
```

| 層 | 責務 | 依存可能な層 |
|---|---|---|
| `api/` | HTTP 呼び出し。型付き DTO / `ApiError` を返す純粋関数 | （なし） |
| `hooks/` | React 状態管理。data/loading/error を持つ。localStorage 連動 | `api/` |
| `components/` | 横断的 UI 部品（Layout, ErrorBanner, RequireAuth など） | `hooks/`, `lib/` |
| `pages/` | 画面単位。データ取得とフォーム送信のオーケストレーション | `hooks/`, `components/`, `api/`, `lib/` |
| `lib/` | 副作用のないユーティリティ（formatPrice, shortTraceId） | （なし） |

### 3.2 ルーティング

| パス | 画面 | 認証 | BFF 呼び出し |
|---|---|---|---|
| `/` | `Products` | 不要 | `GET /api/products` |
| `/products/:id` | `ProductDetail` | 不要 | `GET /api/products/:id` |
| `/cart` | `Cart` | 不要 | （localStorage のみ） |
| `/checkout` | `Checkout` | 必要 | `POST /api/checkout` |
| `/orders` | `Orders` | 必要 | `GET /api/orders` |
| `/orders/:id` | `OrderDetail` | 必要 | `GET /api/orders/:id` |
| `/signin` | `SignIn` | 不要 | `POST /api/auth/signin` |
| `/signup` | `SignUp` | 不要 | `POST /api/auth/signup` |

`<RequireAuth>` でガード。AuthProvider の状態に応じて:
- `loading` → スピナーだけ出す（リダイレクトしない）
- `unauthenticated` → `/signin?next=<元のパス>` にリダイレクト
- `authenticated` → 子要素を描画

サインイン成功後は `next` クエリの値があればそこへ、なければ `/` に戻す。

---

## 4. 認証フロー

### 4.1 AuthProvider

アプリのマウント時に `GET /api/auth/me` を 1 回叩き、結果を Context に保持する。

```ts
type AuthState =
  | { status: 'loading' }
  | { status: 'authenticated'; user: { id: string; email: string } }
  | { status: 'unauthenticated' };

interface AuthContextValue {
  state: AuthState;
  refresh: () => Promise<void>;     // /api/auth/me を再取得
  signOut: () => Promise<void>;     // /api/auth/signout を叩いて状態更新
}
```

- `loading` 中は `<Layout>` がスピナーだけ出す（ルートは描画しない）
- `signIn` 成功後は `refresh()` を呼んで状態を更新し、`next` クエリがあればその値、なければ `/` に遷移
- `signOut()` 成功後は `/` に遷移

### 4.2 Cookie / CORS

- すべての fetch は `credentials: 'include'`
- BFF 側で `Access-Control-Allow-Credentials: true` と `Access-Control-Allow-Origin: http://localhost:5173` が設定済み（既存）
- Cookie 属性は `HttpOnly; SameSite=Lax; Path=/`（既存）

---

## 5. カート

### 5.1 データ構造

```ts
type CartItem = { productId: string; quantity: number };
type Cart = CartItem[];
```

`localStorage['cart']` に JSON 文字列で保存。

### 5.2 useCart API

```ts
useCart(): {
  items: CartItem[];
  add(productId: string, quantity?: number): void;   // 既存があれば加算、デフォルト1
  setQuantity(productId: string, quantity: number): void; // 0 で削除
  remove(productId: string): void;
  clear(): void;
}
```

- 内部実装は `useState<CartItem[]>` + `useEffect` で localStorage と双方向同期
- 初期化時に `JSON.parse` が失敗したら空カートにフォールバック
- カート小計は `Cart.tsx` 内で products の `price_cents` と合わせて計算
- 注文確定が成功したら `clear()` を呼ぶ

### 5.3 バッジ

`Layout` ナビが `useCart` を参照してアイテム数の合計をバッジ表示。

---

## 6. BFF 拡張面

### 6.1 新規エンドポイント

| メソッド | パス | 認証 | 成功レスポンス |
|---|---|---|---|
| `GET` | `/api/products/:id` | 不要 | `{ id, name, description, price_cents }` |
| `GET` | `/api/auth/me` | 必要 | `{ user_id, email }`、未認証時は 401 |
| `POST` | `/api/auth/signout` | 任意 | 204 + `Set-Cookie: session=; Max-Age=0; Path=/` |

### 6.2 エラーレスポンス JSON

すべての非 2xx レスポンスを以下に統一:

```json
{ "code": "INVALID_INPUT", "message": "items required", "trace_id": "abc123def..." }
```

- `Content-Type: application/json`
- 共通ヘルパ `bff/internal/httpx/error.go::WriteError(w, r, status, code, msg)`
- `code` の語彙:
  - `UNAUTHORIZED` (401)
  - `INVALID_INPUT` (400)
  - `NOT_FOUND` (404)
  - `UPSTREAM_FAILED` (502)
  - `INTERNAL` (500)
- `trace_id` は `trace.SpanContextFromContext(r.Context()).TraceID().String()`、無効な場合は空文字
- 成功レスポンス形式は既存のまま変更しない

### 6.3 X-Trace-Id ヘッダ

新規 middleware を `otelhttp.NewHandler` の内側に挟む:

```go
// bff/internal/middleware/traceid.go
func TraceID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sc := trace.SpanContextFromContext(r.Context())
        if sc.IsValid() {
            w.Header().Set("X-Trace-Id", sc.TraceID().String())
        }
        next.ServeHTTP(w, r)
    })
}
```

成功・失敗を問わず全レスポンスに付くようにする。

### 6.4 user-auth gRPC 拡張

`proto/user/v1/user.proto` に `GetUser` を追加:

```proto
service UserAuth {
  // 既存
  rpc SignUp(SignUpRequest) returns (SignUpResponse);
  rpc SignIn(SignInRequest) returns (SignInResponse);
  rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse);
  // 新規
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
}

message GetUserRequest { string user_id = 1; }
message GetUserResponse {
  string user_id = 1;
  string email   = 2;
}
```

- 既存の `users` テーブルから `id, email` を引いて返すだけ
- 失敗は `NotFound`
- `make proto` で再生成

### 6.5 BFF client 拡張

`bff/internal/client/userauth.go` に `GetUser(ctx, id) (*GetUserResponse, error)` を追加し、`/api/auth/me` ハンドラから呼ぶ。

---

## 7. trace_id の UI 露出

### 7.1 fetch ラッパ

```ts
// src/api/http.ts
export class ApiError extends Error {
  constructor(public code: string, message: string, public traceId: string) {
    super(message);
    this.name = 'ApiError';
  }
}

export async function apiFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<{ data: T; traceId: string }> {
  const base = import.meta.env.VITE_API_BASE ?? 'http://localhost:8080';
  const res = await fetch(base + path, { credentials: 'include', ...init });
  const headerTraceId = res.headers.get('X-Trace-Id') ?? '';
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new ApiError(
      body.code ?? 'UNKNOWN',
      body.message ?? res.statusText,
      body.trace_id ?? headerTraceId,
    );
  }
  const data = res.status === 204 ? (undefined as T) : ((await res.json()) as T);
  return { data, traceId: headerTraceId };
}
```

### 7.2 表示コンポーネント

```tsx
// components/TraceIdChip.tsx
export function TraceIdChip({ traceId }: { traceId: string }) {
  if (!traceId) return null;
  const jaeger = import.meta.env.VITE_JAEGER_URL ?? 'http://localhost:16686';
  return (
    <span className="trace-chip">
      trace: <code>{shortTraceId(traceId)}</code>
      <button onClick={() => navigator.clipboard.writeText(traceId)}>📋</button>
      <a href={`${jaeger}/trace/${traceId}`} target="_blank" rel="noreferrer">Jaeger</a>
    </span>
  );
}
```

### 7.3 表示ポリシー

| 状況 | trace_id 表示位置 |
|---|---|
| 通常一覧画面 | 出さない |
| エラー時 | `<ErrorBanner>` 内に必ず |
| `/checkout` 完了表示 | 「注文成功」の直下 |
| `/orders/:id` | 詳細画面の下部 |

---

## 8. テスト戦略

### 8.1 BFF 側（追加分）

| テスト | 場所 | 観点 |
|---|---|---|
| `/api/products/:id` 正常 / NOT_FOUND | `bff/internal/handler/products_test.go` | DTO 形 / エラー JSON 形 |
| `/api/auth/me` 200 / 401 | `bff/internal/handler/auth_test.go` | DTO 形 / Cookie 検証 |
| `/api/auth/signout` | 同上 | `Max-Age=0` Set-Cookie |
| `WriteError` ヘルパ | `bff/internal/httpx/error_test.go` | フォーマット / Content-Type |
| `TraceID` middleware | `bff/internal/middleware/traceid_test.go` | span 付き context で header がセット |

### 8.2 user-auth 側

| テスト | 場所 | 観点 |
|---|---|---|
| `GetUser` repository / gRPC | `services/user-auth/internal/...` | 既存ユーザのメールを返す / NotFound |

### 8.3 Frontend 側（Vitest）

| テスト | 場所 | 観点 |
|---|---|---|
| `apiFetch` 成功 / エラー / 204 | `src/api/http.test.ts` | data・traceId・ApiError |
| `useCart` 追加/数量変更/削除/クリア | `src/hooks/useCart.test.ts` | 状態遷移と localStorage 同期 |
| `useCart` localStorage 壊れ復元 | 同上 | 不正 JSON で空カート fallback |
| `formatPrice` | `src/lib/format.test.ts` | 整数セント → `¥1,234` |
| `shortTraceId` | 同上 | 先頭 4 + 末尾 2 |

`@testing-library/react` は導入しない。

### 8.4 統合検証

仕様 6.4 のチェックリストに追加（下記 11 節参照）。

---

## 9. Docker 統合

### 9.1 docker-compose.yml に追加

```yaml
frontend:
  build: { context: ./frontend, dockerfile: Dockerfile.dev }
  environment:
    VITE_API_BASE: http://localhost:8080
    VITE_JAEGER_URL: http://localhost:16686
  ports: ["5173:5173"]
  volumes:
    - ./frontend:/app
    - /app/node_modules
  depends_on:
    - bff
  command: npm run dev -- --host 0.0.0.0
```

### 9.2 Dockerfile.dev

```dockerfile
FROM node:22-alpine
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci || npm install
COPY . .
EXPOSE 5173
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
```

### 9.3 Makefile 追加ターゲット

```makefile
test/frontend:        ## frontend の Vitest を走らせる
	cd frontend && npm test -- --run

frontend/install:     ## ローカル npm install
	cd frontend && npm install

frontend/dev:         ## ローカルで vite dev（docker を使わない場合）
	cd frontend && npm run dev
```

既存 `test` ターゲットは末尾に `test/frontend` を追加する。

### 9.4 .gitignore 追加

```
frontend/node_modules/
frontend/dist/
frontend/.vite/
```

---

## 10. 学習者の起動シナリオ

```
$ make up                              # frontend 含む全コンテナが起動
$ make seed                            # 商品10件・ユーザ2件投入
$ open http://localhost:5173           # UI が開く
$ open http://localhost:16686          # Jaeger も並走

1. /signin に行き alice@example.com / password でサインイン
2. "/" で商品一覧表示、カートに追加
3. /cart で数量を調整、/checkout で注文確定
4. 完了画面の trace ID をクリックして Jaeger を開く
5. /orders で履歴を確認、/orders/:id で詳細＋trace_id 表示

# レジリエンス観察
$ FLAKE_RATE=0.6 make up               # 60% 失敗
6. 再度 /checkout を試み、エラーバナーの trace_id を Jaeger で追う
```

---

## 11. 完了条件

1. `06_microservie/frontend/` が存在し、`make up` 後に `http://localhost:5173` が開ける
2. BFF に下記が実装され、テストが通る:
   - `GET /api/products/:id`
   - `GET /api/auth/me`
   - `POST /api/auth/signout`
   - すべてのエラーが `{ code, message, trace_id }` JSON 形式
   - すべてのレスポンスに `X-Trace-Id` ヘッダ
3. `user-auth.proto` に `GetUser` が追加され、`make proto` で再生成済み
4. `make test` が exit 0（Go 全モジュール + frontend Vitest）
5. 10 節の学習者シナリオが手動で全通する:
   - サインアップ / サインイン / サインアウトが動く
   - 商品一覧 / 詳細
   - カート: 追加・数量変更・削除・空表示
   - 注文確定が成功した時点でカートが自動的に空になる
   - 注文確定後の完了画面に trace_id が表示され、Jaeger に到達できる
   - 注文履歴一覧と詳細
   - 認証必要ルートに未ログインでアクセスすると `/signin?next=...` に飛ぶ
6. `FLAKE_RATE=0.6` で起動した状態でチェックアウトを失敗させ、エラーバナーの trace_id から Jaeger に到達できる
7. 章設計書（親仕様）の verification checklist に「frontend が `:5173` で動き、注文後の trace_id を Jaeger で開ける」を追記
