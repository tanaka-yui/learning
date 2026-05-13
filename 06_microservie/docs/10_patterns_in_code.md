# 10: コード上のパターン索引

> 本章で学んだパターンが、どのファイル/シンボルで実装されているかの索引。
> docs を読みながら該当コードを開く動線、もしくはコードを読みながら該当 doc を引く動線、どちらでも使える。

---

## 1. 索引の使い方

この章の doc は概念と判断軸を中心に書かれているため、「結局このパターンはコードのどこにあるのか」を引きたい場面が必ず出てくる。本ページはその逆引き辞書で、§2 は「パターン名 → 実装」、§3 は「ファイル → 関連 doc」の二方向に並べてある。doc を読んでいて気になったパターンがあれば §2 から実装ファイルへ、コードを読んでいて背景や設計理由を知りたければ §3 から該当 doc に戻る、という使い方を想定している。

## 2. パターン一覧

| パターン名 | 解説 doc | 実装ファイル | シンボル | 補足 |
|---|---|---|---|---|
| Saga オーケストレーション | [07](07_resilience.md) | `services/order/internal/saga/checkout.go` | `Run` | Step 1/2/3 はコード内コメントで整理 |
| Saga ステップログ | [07](07_resilience.md) | `services/order/internal/saga/checkout.go` | `OrderStore.LogStep` | reserve/charge/commit/refund/release の状態を記録 |
| Inventory Reserve | [07](07_resilience.md) | `services/inventory/internal/server/grpc.go` | `Reserve` | 在庫予約 |
| Inventory Commit | [07](07_resilience.md) | 同上 | `Commit` | 予約確定 |
| Inventory Release | [07](07_resilience.md) | 同上 | `Release` | 予約解放（補償） |
| Payment 擬似失敗注入 | [07](07_resilience.md) | `services/payment/internal/flake/flake.go` | `ShouldFail` | FLAKE_RATE 環境変数で制御 |
| サーキットブレーカー 初期化 | [07](07_resilience.md) | `services/order/internal/resilience/breaker.go` | `NewBreaker` | 失敗率しきい値・open/half-open 設定 |
| サーキットブレーカー 利用 | [07](07_resilience.md) | `services/order/internal/client/payment.go` | `gobreaker.CircuitBreaker` を `Charge` 呼び出しに巻く箇所 | Payment 側にだけ CB を適用 |
| リトライ + 指数バックオフ | [07](07_resilience.md) | `services/order/internal/client/inventory.go` | `backoff.Retry` 呼び出し | Reserve に最大 3 回のリトライ |
| BFF REST→gRPC 集約 | [05](05_communication.md) | `bff/internal/handler/checkout.go` | `Checkout.Post` | カート → order.PlaceOrder の集約 |
| BFF Auth middleware | [04](04_decomposition.md), [05](05_communication.md) | `bff/internal/middleware/auth.go` | `Auth` | Cookie → user_id 解決 |
| JWT 発行・検証 | [04](04_decomposition.md) | `services/user-auth/internal/jwt/jwt.go` | `Issue`, `Verify` | user-auth 内で完結 |
| trace_id レスポンスヘッダ | [08](08_observability.md) | `bff/internal/middleware/traceid.go` | `TraceID` | 全レスポンスに X-Trace-Id を載せる |
| エラー JSON 統一 | [08](08_observability.md) | `bff/internal/httpx/error.go` | `WriteError` | `{code,message,trace_id}` 形式 |
| OTel SDK 初期化 | [08](08_observability.md) | `bff/internal/obs/otel.go` | `InitTracing` | OTLP/gRPC で Collector へ |
| GetUser RPC | [04](04_decomposition.md) | `services/user-auth/internal/server/grpc.go` | `GetUser` | BFF が email 解決に使う |
| Auth.Me ハンドラ | [04](04_decomposition.md) | `bff/internal/handler/auth.go` | `Auth.Me` | フロント向け認証 probe |
| Frontend trace_id 表示 | [08](08_observability.md) | `frontend/src/components/TraceIdChip.tsx` | `TraceIdChip` | コピー + Jaeger リンク |
| Frontend API ラッパ | [08](08_observability.md) | `frontend/src/api/http.ts` | `apiFetch`, `ApiError` | X-Trace-Id を構造体に持ち上げる |

## 3. ファイル別索引

§2 を反転して同じファイルに複数のシンボルがある場合はまとめてある。コードリーディング中にこの表で関連 doc を引き当てて、設計理由や背景を確認する流れを想定している。

| 実装ファイル | 関連 doc |
|---|---|
| `services/order/internal/saga/checkout.go` | [07](07_resilience.md)（`Run`, `OrderStore.LogStep`） |
| `services/order/internal/resilience/breaker.go` | [07](07_resilience.md)（`NewBreaker`） |
| `services/order/internal/client/payment.go` | [07](07_resilience.md)（サーキットブレーカー適用） |
| `services/order/internal/client/inventory.go` | [07](07_resilience.md)（リトライ + 指数バックオフ） |
| `services/inventory/internal/server/grpc.go` | [07](07_resilience.md)（`Reserve` / `Commit` / `Release`） |
| `services/payment/internal/flake/flake.go` | [07](07_resilience.md)（`ShouldFail`） |
| `services/user-auth/internal/jwt/jwt.go` | [04](04_decomposition.md)（`Issue`, `Verify`） |
| `services/user-auth/internal/server/grpc.go` | [04](04_decomposition.md)（`GetUser`） |
| `bff/internal/handler/checkout.go` | [05](05_communication.md)（`Checkout.Post`） |
| `bff/internal/handler/auth.go` | [04](04_decomposition.md)（`Auth.Me`） |
| `bff/internal/middleware/auth.go` | [04](04_decomposition.md), [05](05_communication.md)（`Auth`） |
| `bff/internal/middleware/traceid.go` | [08](08_observability.md)（`TraceID`） |
| `bff/internal/httpx/error.go` | [08](08_observability.md)（`WriteError`） |
| `bff/internal/obs/otel.go` | [08](08_observability.md)（`InitTracing`） |
| `frontend/src/components/TraceIdChip.tsx` | [08](08_observability.md)（`TraceIdChip`） |
| `frontend/src/api/http.ts` | [08](08_observability.md)（`apiFetch`, `ApiError`） |

## 4. 章全体の読み直し動線

実装で迷ったとき、たとえば「なぜ order サービスだけが Saga を持ち、inventory/payment は持たないのか」のような疑問が出たら、まず [06](06_data_ownership.md)（データ所有）と [07](07_resilience.md)（レジリエンス）を再読すると、所有権と補償の責務がどの境界に張り付いているかを思い出せる。「そもそもなぜサービスをこの 5 つに割ったのか」を再確認したい場合は [04](04_decomposition.md)（分割の判断軸）と [03](03_conway.md)（コンウェイの法則）に戻ると、ドメイン境界とチーム境界の対応が見えてくる。観測性の挙動、たとえば trace_id がどう伝播しているのか・なぜ X-Trace-Id をレスポンスに載せているのかを再確認したいときは [08](08_observability.md) を読みつつ、§2 から `TraceID` / `apiFetch` / `TraceIdChip` を順に辿ると BFF→フロントの一気通貫が掴める。さらに「この構成のまま 10 サービス、20 サービスへ広げたらどうなるか」を考えたいときは [09](09_scaling_istio.md) を読み、サービスメッシュ・サイドカーで横串の関心事をどう外出しするかという次のステップに繋げる。

---

**章の入口に戻る:** [README](../README.md)
