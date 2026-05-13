# 06_microservie Plan 4: Explanation Docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Write the 10 explanation Markdown files under `06_microservie/docs/` and link them from the chapter README, completing the chapter per parent spec §8 item 2.

**Architecture:** Each doc is a standalone Markdown file that follows a shared 5-section template. Nine docs (01-09) are written in parallel by independent subagents; the index doc (10) is written last because it references symbols across the whole codebase. A unified vocabulary table and a code-link dictionary are passed to every subagent so wording stays consistent without serialization.

**Tech Stack:** Markdown, Mermaid (GitHub-rendered), git.

**Reference spec:** `docs/superpowers/specs/2026-05-13-microservices-docs-design.md`
**Parent chapter spec:** `docs/superpowers/specs/2026-05-12-microservices-chapter-design.md`

---

## Phase 0: Branch context

Continue on branch `feat/microservices`. No worktree split. Phases 1–3 of the chapter are already merged into this branch.

---

## Phase 1: Write 9 explanation docs in parallel

Each task (1.1 through 1.9) is a single-doc subagent dispatch. They are independent and can run concurrently — dispatch all 9 in one message.

The template, vocabulary table, and code-link dictionary embedded in each task come straight from the spec (`docs/superpowers/specs/2026-05-13-microservices-docs-design.md` §4-§7) and are repeated in each task for self-containment.

### Common prompt elements (shared across all 9 tasks)

Every Phase 1 task uses the same shared elements. When dispatching, paste them along with the task-specific parts:

**Vocabulary table — DO NOT VIOLATE (spec §4):**

| 概念 | 採用表記 | NG 表記 |
|---|---|---|
| microservice | マイクロサービス | マイクロ・サービス / μサービス |
| monolith | モノリス | モノリシック（名詞として使うとき）|
| modular monolith | モジュラーモノリス | モジュール式モノリス |
| Conway's law | コンウェイの法則 | コンウェイ法則 / Conway 則 |
| Team Topologies | Team Topologies（原語） | チームトポロジー |
| Bounded Context | 境界づけられたコンテキスト | バウンデッドコンテキスト |
| Saga | Saga（原語） | サーガ |
| Circuit Breaker | サーキットブレーカー | 回路遮断器 |
| timeout | タイムアウト | 時間切れ |
| retry | リトライ | 再試行 |
| idempotency | 冪等性 | べき等性 |
| observability | 観測性 | オブザーバビリティ / 可観測性 |
| trace_id | trace_id | トレース ID |
| span | span | スパン / 区間 |
| service mesh | サービスメッシュ | サービス網 |
| sidecar | サイドカー | サイドカー proxy |

**Code-link dictionary (spec §5) — file::symbol form, NO line numbers:**

| 概念 | ファイル | シンボル |
|---|---|---|
| Saga オーケストレーション | `services/order/internal/saga/checkout.go` | `Run`, `Step` |
| Inventory Reserve | `services/inventory/internal/server/grpc.go` | `Reserve` |
| Inventory Commit | 同上 | `Commit` |
| Inventory Release | 同上 | `Release` |
| Payment 擬似失敗 | `services/payment/internal/flake/flake.go` | `ShouldFail` |
| Circuit Breaker | `services/order/internal/saga/checkout.go` | `gobreaker.CircuitBreaker` の初期化 |
| Retry + 指数バックオフ | 同上 | `backoff.Retry` 呼び出し |
| BFF Checkout 集約 | `bff/internal/handler/checkout.go` | `Checkout.Post` |
| BFF Auth middleware | `bff/internal/middleware/auth.go` | `Auth` |
| JWT 発行・検証 | `services/user-auth/internal/jwt/manager.go` | `Issue`, `Verify` |
| trace_id レスポンス header | `bff/internal/middleware/traceid.go` | `TraceID` |
| エラー JSON 統一 | `bff/internal/httpx/error.go` | `WriteError` |
| OTel SDK 初期化 | `bff/internal/obs/tracing.go` | `InitTracing` |
| GetUser RPC | `services/user-auth/internal/server/grpc.go` | `GetUser` |
| Auth.Me ハンドラ | `bff/internal/handler/auth.go` | `Auth.Me` |
| Frontend trace_id 表示 | `frontend/src/components/TraceIdChip.tsx` | `TraceIdChip` |
| Frontend API ラッパ | `frontend/src/api/http.ts` | `apiFetch`, `ApiError` |

**Template (spec §6) — 01-09 must follow this structure:**

```markdown
# <タイトル>

> <1-2 文の概要。この doc で答える問い>

---

## 1. なぜ <概念> を扱うのか

<why セクション>

## 2. <概念> とは

<定義。本章サンプルとの紐付けを意識>
Mermaid 図は概念が視覚化で明確になる場合に 1 枚入れる。

## 3. 実例: 本章のサンプルではどう現れるか

<具体例。コード位置は file::symbol で参照>

## 4. 落とし穴 / よくある誤解

<2〜3 個>

## 5. スコープ外 — この章で扱わないこと

<親仕様 §7 の該当部分>

---

**次に読む:** [N+1: <次のタイトル>](0(N+1)_<slug>.md)
**章の入口に戻る:** [README](../README.md)
```

**Style rules:**
- 日本語
- 2,000〜4,000 字（許容 1,500〜4,500）
- Mermaid 中心、必要なときだけ
- why → how の順
- コード参照は file::symbol 形式、行番号禁止
- 辞書外のシンボル参照禁止

**Self-check before commit (each subagent runs these locally):**
```bash
wc -m 06_microservie/docs/0X_<slug>.md
# count must be 1500-4500

# vocabulary check
for w in 'マイクロ・サービス' 'モノリシック' 'コンウェイ法則' 'バウンデッドコンテキスト' \
         'サーガ' 'べき等性' 'オブザーバビリティ' '可観測性' 'サービス網' 'トレース ID' 'トレースID'; do
  grep -n "$w" 06_microservie/docs/0X_<slug>.md && echo "VOCAB VIOLATION: $w"
done

# template section check (expect at least 5 ## headings)
grep -c '^## ' 06_microservie/docs/0X_<slug>.md

# line-number ban
grep -nE '\.go:[0-9]+|\.tsx?:[0-9]+' 06_microservie/docs/0X_<slug>.md && echo "LINE NUMBER FOUND"
```

**Commit (each subagent):**
```bash
git add 06_microservie/docs/0X_<slug>.md
git commit -m "microservices(docs): add 0X_<slug>.md"
```

---

### Task 1.1: Write `01_concepts.md`

**File to create:** `06_microservie/docs/01_concepts.md`

**Role (from spec §2):** マイクロサービスとは / モノリス・モジュラーモノリスとの違い

**Required topics (cover all):**
- マイクロサービスの定義（独立デプロイ可能 / 自分の DB を持つ / 小さい責務 / プロセス境界）
- モノリス・モジュラーモノリスとの対比（同じプロセス / 同じ DB / 境界がコードレベルか配備レベルか）
- 本章サンプルが「最小のマイクロサービス」になっている事実（5 services + BFF + frontend + 5 Postgres）
- 「サービス」と「ライブラリ」の違い
- 何が「マイクロ」なのか（規模ではなく「独立にライフサイクル管理できる単位」）

**Individual params (spec §7):**
- §3「実例」で Plan 1 のディレクトリ構成（`services/`, `bff/`, `frontend/`, `proto/`）と章全体図に触れる
- 親仕様 §3.1 の全体図を Mermaid で簡略化して掲載

**§5 スコープ外で触れる内容:**
- マイクロサービスの「サイズの基準」議論（ピザ 2 枚等）には踏み込まない
- 本章ではコードを書くが、組織論は §03 に任せる

**Next link:** 02_pros_cons.md

**Steps:**

- [ ] **Step 1: Read the parent chapter spec sections referenced**

Read `docs/superpowers/specs/2026-05-12-microservices-chapter-design.md` §1.1, §3.1, §3.2 for context.

- [ ] **Step 2: Write the file with the template**

Write `06_microservie/docs/01_concepts.md` following the shared template. Cover all required topics. Include a Mermaid diagram for the chapter architecture (services + BFF + frontend + postgres).

- [ ] **Step 3: Run self-checks**

Run the four self-check commands listed in "Common prompt elements" with `01_concepts.md` substituted. All must pass.

- [ ] **Step 4: Commit**

```bash
git add 06_microservie/docs/01_concepts.md
git commit -m "microservices(docs): add 01_concepts.md"
```

---

### Task 1.2: Write `02_pros_cons.md`

**File to create:** `06_microservie/docs/02_pros_cons.md`

**Role:** メリット・デメリット、選定チェックリスト

**Required topics:**
- メリット 4 つ: 独立デプロイ / 独立スケール / 障害分離 / 技術スタック自由度
- デメリット 4 つ: 運用複雑性 / 分散トランザクション / 観測性の困難 / ネットワーク遅延と失敗
- 選定チェックリスト（YES の方が多ければマイクロサービスを検討するクラス）
- 本章サンプルでのトレードオフ実例

**Individual params:**
- §3「実例」で親仕様 §3.4 主要フロー（Saga）と payment のフレーク注入（`services/payment/internal/flake/flake.go::ShouldFail`）に触れる
- 「分散トランザクション」のデメリットを Saga 補償の図で示す

**§5 スコープ外:**
- 具体的な人月見積もりや組織サイズの数値基準
- Mesos/Kubernetes 導入コストの試算

**Next link:** 03_conway.md

**Steps:**

- [ ] **Step 1: Read parent spec sections**

Read parent chapter spec §3.4, §4.

- [ ] **Step 2: Write the file**

Write `06_microservie/docs/02_pros_cons.md`. Include a Mermaid sequence diagram showing where Saga compensation fires.

- [ ] **Step 3: Run self-checks**

- [ ] **Step 4: Commit**

```bash
git add 06_microservie/docs/02_pros_cons.md
git commit -m "microservices(docs): add 02_pros_cons.md"
```

---

### Task 1.3: Write `03_conway.md`

**File to create:** `06_microservie/docs/03_conway.md`

**Role:** Conway's law, Inverse Conway Maneuver, Team Topologies

**Required topics:**
- コンウェイの法則の主張と原典（1968年 Conway）
- 「組織が先、アーキテクチャが後」になりがちな仕組み
- Inverse Conway Maneuver — 望むアーキテクチャに合わせて組織を作り直す
- Team Topologies の 4 種のチーム: Stream-aligned / Enabling / Platform / Complicated subsystem
- Team Topologies の 3 種のインタラクション: Collaboration / X-as-a-Service / Facilitating
- 本章サンプルを 5 つの仮想 Stream-aligned チームに割り当てる思考実験

**Individual params:**
- §3「実例」で services 境界に対応する仮想チームを定義（catalog 担当 / inventory 担当 / order 担当 / payment 担当 / user-auth 担当）
- frontend は「BFF を X-as-a-Service として消費する Stream-aligned」と説明

**§5 スコープ外:**
- 実組織での導入手順
- Spotify モデル等の派生

**Next link:** 04_decomposition.md

**Steps:**

- [ ] **Step 1: Read parent spec §1.1 item 3 and §2.1 table row for 03**

- [ ] **Step 2: Write the file**

Write `06_microservie/docs/03_conway.md`. Optional: a Mermaid diagram of the 5 virtual teams aligned to the 5 services.

- [ ] **Step 3: Run self-checks**

- [ ] **Step 4: Commit**

```bash
git add 06_microservie/docs/03_conway.md
git commit -m "microservices(docs): add 03_conway.md"
```

---

### Task 1.4: Write `04_decomposition.md`

**File to create:** `06_microservie/docs/04_decomposition.md`

**Role:** サービス分割（DDD 境界づけられたコンテキスト / ビジネス能力）、EC ドメインへの適用

**Required topics:**
- 分割の 2 大アプローチ: DDD の境界づけられたコンテキスト / ビジネス能力（Business Capability）
- 「同じ用語が違う意味を持つ」現象（同一名のエンティティが別サービスで別の意味を持つ）
- EC ドメイン分割の選択肢を列挙し、本章での選択を説明
- 「分割しすぎる」失敗と「分割しなさすぎる」失敗

**Individual params:**
- §3「実例」で 5 サービスの責務表を再掲（catalog: 商品マスタ / inventory: 在庫の真実 / order: 注文ライフサイクル / payment: 決済 / user-auth: ユーザと JWT）
- 「inventory と catalog が同じ product_id を別の意味で扱う」例を紹介（catalog は表示用情報、inventory は在庫の真実）

**§5 スコープ外:**
- Event Storming の手順
- DDD の他要素（Aggregate, Entity, Value Object）の網羅

**Next link:** 05_communication.md

**Steps:**

- [ ] **Step 1: Read parent spec §3.2 and §3.3**

- [ ] **Step 2: Write the file**

Write `06_microservie/docs/04_decomposition.md`. Include the 5-service responsibility table (Markdown table, not Mermaid).

- [ ] **Step 3: Run self-checks**

- [ ] **Step 4: Commit**

```bash
git add 06_microservie/docs/04_decomposition.md
git commit -m "microservices(docs): add 04_decomposition.md"
```

---

### Task 1.5: Write `05_communication.md`

**File to create:** `06_microservie/docs/05_communication.md`

**Role:** 同期 vs 非同期、REST/gRPC の選択基準、本サンプルでのプロトコル方針

**Required topics:**
- 同期通信 vs 非同期通信のトレードオフ（即応性 vs 結合度）
- REST と gRPC の比較（人間可読 vs バイナリ高効率 / OpenAPI vs proto / HTTP/1.1 vs HTTP/2）
- 「BFF と外（ブラウザ）は REST、BFF と内（サービス間）は gRPC」の設計選択の理由
- proto による契約管理（buf でのコード生成）
- 「ネットワークは信頼できない」前提

**Individual params:**
- §3「実例」で `proto/` 配下と `bff/internal/handler/checkout.go::Checkout.Post`（REST→gRPC 集約）を扱う
- Mermaid シーケンス図で「Browser → BFF (REST) → 各サービス (gRPC)」フローを描く

**§5 スコープ外:**
- メッセージング基盤（NATS, Kafka, RabbitMQ）
- GraphQL
- gRPC streaming（本章は unary のみ）

**Next link:** 06_data_ownership.md

**Steps:**

- [ ] **Step 1: Read parent spec §3.1, §3.4**

- [ ] **Step 2: Write the file**

Write `06_microservie/docs/05_communication.md` with a Mermaid sequence diagram for the checkout REST→gRPC fanout.

- [ ] **Step 3: Run self-checks**

- [ ] **Step 4: Commit**

```bash
git add 06_microservie/docs/05_communication.md
git commit -m "microservices(docs): add 05_communication.md"
```

---

### Task 1.6: Write `06_data_ownership.md`

**File to create:** `06_microservie/docs/06_data_ownership.md`

**Role:** DB-per-service、参照整合性が効かない世界、結果整合性

**Required topics:**
- DB-per-service の原則（他サービスの DB を直接見ない）
- なぜ「参照整合性（外部キー制約）」が効かなくなるのか
- 結果整合性（eventual consistency）と読み手の責任
- データの重複は正常（catalog と inventory が同じ product_id を別の意味で持つ例）
- 注文行に price snapshot を保存する理由（catalog の価格変更から独立させる）

**Individual params:**
- §3「実例」で postgres-* の 5 インスタンス構成と、`inventory` と `catalog` が同じ `product_id` を別の意味で扱う設計を扱う
- 親仕様 §3.2 サービス責務表を引用

**§5 スコープ外:**
- Event Sourcing / CQRS
- Outbox / Inbox パターン
- 分散トランザクション（2PC, Saga 補償は §07）

**Next link:** 07_resilience.md

**Steps:**

- [ ] **Step 1: Read parent spec §3.2, §3.3**

- [ ] **Step 2: Write the file**

Write `06_microservie/docs/06_data_ownership.md`. Include a Mermaid diagram of 5 separate Postgres instances each owned by one service.

- [ ] **Step 3: Run self-checks**

- [ ] **Step 4: Commit**

```bash
git add 06_microservie/docs/06_data_ownership.md
git commit -m "microservices(docs): add 06_data_ownership.md"
```

---

### Task 1.7: Write `07_resilience.md`

**File to create:** `06_microservie/docs/07_resilience.md`

**Role:** timeout / retry / circuit breaker / Saga（補償）、idempotency

**Required topics:**
- なぜマイクロサービスでレジリエンスが必要か（「ネットワークは信頼できない」+ 「障害は独立しているように見えて連鎖する」）
- タイムアウトの設計（クライアント側 deadline、サーバ側でも `ctx.Err()` チェック）
- リトライ（指数バックオフ、冪等な呼び出しに限る）
- サーキットブレーカー（closed → open → half-open）
- Saga（オーケストレーション型、補償ステップを逆順に実行）
- 冪等性キーがなぜ必須か（リトライの安全性のため）

**Individual params:**
- §3「実例」は本 doc の核心:
  - Saga: `services/order/internal/saga/checkout.go::Run`
  - リトライ: 同ファイル `backoff.Retry`
  - サーキットブレーカー: 同ファイル `gobreaker.CircuitBreaker` の初期化
  - 擬似失敗注入: `services/payment/internal/flake/flake.go::ShouldFail`
  - 冪等性キー: `reservation_id`, `payment_idempotency_key`（親仕様 §3.4）
- 親仕様 §4.1 のレジリエンスパターン表を引用

**§5 スコープ外（親仕様 §4.3 のリスト）:**
- Bulkhead, Rate limiting, Backpressure
- Choreography 型 Saga
- Outbox / Inbox

**Next link:** 08_observability.md

**Steps:**

- [ ] **Step 1: Read parent spec §4.1, §4.2, §4.3 and the actual Saga code at `06_microservie/services/order/internal/saga/checkout.go`**

- [ ] **Step 2: Write the file**

Write `06_microservie/docs/07_resilience.md`. Include 2 Mermaid diagrams:
1. Saga happy path + compensation flow
2. Circuit breaker state machine (closed → open → half-open)

- [ ] **Step 3: Run self-checks**

- [ ] **Step 4: Commit**

```bash
git add 06_microservie/docs/07_resilience.md
git commit -m "microservices(docs): add 07_resilience.md"
```

---

### Task 1.8: Write `08_observability.md`

**File to create:** `06_microservie/docs/08_observability.md`

**Role:** 構造ログ・trace_id 伝搬・分散トレース、OpenTelemetry の基本

**Required topics:**
- 「観測性」の 3 本柱: ログ / メトリクス / トレース（本章はログとトレース中心）
- 構造ログ（JSON で出すと grep + jq で扱える）
- 分散トレースの概念（trace_id / span_id / parent_span_id）
- W3C TraceContext と traceparent ヘッダ
- OpenTelemetry の SDK / Collector / バックエンド（Jaeger）の構造
- 本章サンプルで「checkout 1 回」を Jaeger でどう追えるか
- trace_id を UI に出して Jaeger に飛べる体験

**Individual params:**
- §3「実例」で:
  - OTel SDK 初期化: `bff/internal/obs/tracing.go::InitTracing`
  - trace_id レスポンスヘッダ: `bff/internal/middleware/traceid.go::TraceID`
  - エラー JSON に trace_id 同梱: `bff/internal/httpx/error.go::WriteError`
  - フロントエンドでの表示: `frontend/src/components/TraceIdChip.tsx::TraceIdChip`
  - API ラッパが trace_id を取り出す: `frontend/src/api/http.ts::apiFetch`, `ApiError`
- gRPC interceptor で server/client 両側を自動 instrumentation する仕組み（親仕様 §4.5）
- 各サービスが OTel SDK セットアップをコピペで持つ「意図的な重複」方針（親仕様 §4.7）

**§5 スコープ外:**
- メトリクス（Prometheus, Grafana）
- ログ集約基盤（Loki, ELK）
- サンプリング戦略の細部

**Next link:** 09_scaling_istio.md

**Steps:**

- [ ] **Step 1: Read parent spec §4.4, §4.5, §4.6, §4.7**

- [ ] **Step 2: Write the file**

Write `06_microservie/docs/08_observability.md`. Include 1 Mermaid diagram showing the trace flow: Browser → BFF → order → inventory → payment → all to OTel Collector → Jaeger.

- [ ] **Step 3: Run self-checks**

- [ ] **Step 4: Commit**

```bash
git add 06_microservie/docs/08_observability.md
git commit -m "microservices(docs): add 08_observability.md"
```

---

### Task 1.9: Write `09_scaling_istio.md`

**File to create:** `06_microservie/docs/09_scaling_istio.md`

**Role:** 大規模化の課題、サービスメッシュ概念、Istio 構成要素

**Required topics:**
- サービス数が増えるとアプリケーションコードに混じってくる関心事（mTLS / リトライ / サーキットブレーカー / トラフィック分割 / 可観測性）
- それらをアプリから「インフラ層」に追い出す発想 = サービスメッシュ
- サイドカープロキシ（Envoy）の役割
- コントロールプレーン（Istio）とデータプレーン（Envoy）
- Istio 構成要素: Pilot / Citadel（mTLS）/ Galley / Telemetry
- 何を解くか: mTLS / L7 ルーティング / 観測性の標準化 / トラフィック分割（カナリア）

**Individual params:**
- 本 doc は **概念解説のみ**。本章コードへの直接リンクは最小限。
- 「本章サンプルのレジリエンスや trace 伝搬が、サービスメッシュに移動するとアプリから消える」という対比を示す

**§5 スコープ外:**
- 実セットアップ手順
- k8s マニフェスト・Helm
- Linkerd, Consul Connect 等の他メッシュ
- Istio ambient mode の細部

**Next link:** 10_patterns_in_code.md

**Steps:**

- [ ] **Step 1: Read parent spec §1.1 item 5, §3.1 mention of mesh, and the chapter design notes**

- [ ] **Step 2: Write the file**

Write `06_microservie/docs/09_scaling_istio.md`. Include 1 Mermaid diagram of the sidecar topology (Service A + sidecar ↔ Service B + sidecar, control plane separate).

- [ ] **Step 3: Run self-checks**

- [ ] **Step 4: Commit**

```bash
git add 06_microservie/docs/09_scaling_istio.md
git commit -m "microservices(docs): add 09_scaling_istio.md"
```

---

## Phase 2: Write the index doc (10_patterns_in_code.md)

After all 9 docs from Phase 1 are committed, dispatch one more subagent for the index doc.

### Task 2.1: Write `10_patterns_in_code.md`

**File to create:** `06_microservie/docs/10_patterns_in_code.md`

**Role:** 各パターンとサンプルコード箇所のマッピング — 章全体の架け橋

**Layout (template-free — this doc is an index):**

```markdown
# 10: コード上のパターン索引

> 本章で学んだパターンが、どのファイル/シンボルで実装されているかの索引。
> docs を読みながら該当コードを開く動線、もしくはコードを読みながら該当 doc を引く動線、どちらでも使える。

---

## 1. 索引の使い方

<2-3 文。読者が「doc → コード」と「コード → doc」両方向で使える>

## 2. パターン一覧

| パターン名 | 解説 doc | 実装ファイル | シンボル | 補足 |
|---|---|---|---|---|
| <パターン> | [0N](0N_<slug>.md) | <path> | <symbol> | <短い注> |
| ... |

## 3. ファイル別索引

<上の逆引き。どの実装ファイルがどの doc で説明されているか>

| 実装ファイル | 関連 doc |
|---|---|
| services/order/internal/saga/checkout.go | [07](07_resilience.md) |
| ... |

## 4. 章全体の読み直し動線

<3-4 文。01-09 のどこから読み返すと有効か。例えば「実装で迷ったらまず 06、07」「設計判断の理由を再確認したいなら 04」など>

---

**章の入口に戻る:** [README](../README.md)
```

**Required content for §2 パターン一覧 table:**

The table must include **at minimum** these rows, but the subagent must verify each `file::symbol` actually exists via `grep -rn` before adding it (BLOCKED if any missing):

| パターン名 | 解説 doc | 実装ファイル | シンボル |
|---|---|---|---|
| Saga オーケストレーション | 07 | `services/order/internal/saga/checkout.go` | `Run`, `Step` |
| Inventory Reserve | 07 | `services/inventory/internal/server/grpc.go` | `Reserve` |
| Inventory Commit | 07 | 同上 | `Commit` |
| Inventory Release | 07 | 同上 | `Release` |
| Payment 擬似失敗注入 | 07 | `services/payment/internal/flake/flake.go` | `ShouldFail` |
| Circuit Breaker | 07 | `services/order/internal/saga/checkout.go` | `gobreaker.CircuitBreaker` の初期化 |
| Retry + 指数バックオフ | 07 | 同上 | `backoff.Retry` 呼び出し |
| BFF REST→gRPC 集約 | 05 | `bff/internal/handler/checkout.go` | `Checkout.Post` |
| BFF Auth middleware | 04, 05 | `bff/internal/middleware/auth.go` | `Auth` |
| JWT 発行・検証 | 04 | `services/user-auth/internal/jwt/manager.go` | `Issue`, `Verify` |
| trace_id レスポンスヘッダ | 08 | `bff/internal/middleware/traceid.go` | `TraceID` |
| エラー JSON 統一 | 08 | `bff/internal/httpx/error.go` | `WriteError` |
| OTel SDK 初期化 | 08 | `bff/internal/obs/tracing.go` | `InitTracing` |
| GetUser RPC | 04 | `services/user-auth/internal/server/grpc.go` | `GetUser` |
| Auth.Me ハンドラ | 04 | `bff/internal/handler/auth.go` | `Auth.Me` |
| Frontend trace_id 表示 | 08 | `frontend/src/components/TraceIdChip.tsx` | `TraceIdChip` |
| Frontend API ラッパ | 08 | `frontend/src/api/http.ts` | `apiFetch`, `ApiError` |

**§3 ファイル別索引 table** — derive from §2 by inverting the mapping.

**Steps:**

- [ ] **Step 1: Verify every `file::symbol` actually exists**

For each row in the table above, run:
```bash
grep -rn '<symbol>' 06_microservie/<path>
```
If any symbol is missing, BLOCKED — report which one.

- [ ] **Step 2: Write the file**

Write `06_microservie/docs/10_patterns_in_code.md` following the layout above. Fill in the §2 table verbatim from the spec (rows verified in Step 1). Build §3 ファイル別索引 by inverting.

- [ ] **Step 3: Self-checks**

```bash
wc -m 06_microservie/docs/10_patterns_in_code.md   # 1500-3000 字
grep -nE '\.go:[0-9]+|\.tsx?:[0-9]+' 06_microservie/docs/10_patterns_in_code.md && echo "LINE NUMBER FOUND"
# vocab grep same as Phase 1
```

- [ ] **Step 4: Commit**

```bash
git add 06_microservie/docs/10_patterns_in_code.md
git commit -m "microservices(docs): add 10_patterns_in_code.md"
```

---

## Phase 3: README link and final integration

### Task 3.1: Add learning path links to README

**File to modify:** `06_microservie/README.md`

- [ ] **Step 1: Read the current README**

```bash
cat 06_microservie/README.md
```

- [ ] **Step 2: Append the learning path section**

Append this section after the existing クイックスタート section (or at the end if クイックスタート is missing):

```markdown

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
```

- [ ] **Step 3: Confirm all 10 links resolve**

```bash
for n in 01_concepts 02_pros_cons 03_conway 04_decomposition 05_communication \
         06_data_ownership 07_resilience 08_observability 09_scaling_istio 10_patterns_in_code; do
  test -f 06_microservie/docs/${n}.md || echo "MISSING: docs/${n}.md"
done
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add 06_microservie/README.md
git commit -m "microservices(docs): link learning path from README"
```

---

## Phase 4: Whole-chapter consistency review

This is done by the controller, not a subagent. After all 10 docs and the README link are committed.

### Task 4.1: Run grep-based checks across all docs

- [ ] **Step 1: Vocabulary violations**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
for w in 'マイクロ・サービス' 'モノリシック' 'コンウェイ法則' 'バウンデッドコンテキスト' \
         'サーガ' 'べき等性' 'オブザーバビリティ' '可観測性' 'サービス網' 'トレース ID' 'トレースID'; do
  hits=$(grep -rn "$w" 06_microservie/docs/ 2>/dev/null)
  if [ -n "$hits" ]; then
    echo "VOCAB VIOLATION: $w"
    echo "$hits"
  fi
done
```

Expected: no output. If a violation is found, dispatch a fix subagent for that specific doc.

- [ ] **Step 2: Line-number bans**

```bash
grep -rnE '\.go:[0-9]+|\.tsx?:[0-9]+' 06_microservie/docs/
```

Expected: no output (or only matches inside code-block illustrations like SQL `WHERE id = 42`).

- [ ] **Step 3: Word counts in range**

```bash
for f in 06_microservie/docs/*.md; do
  count=$(wc -m < "$f")
  echo "$f: $count chars"
done
```

Verify every file is between 1,500 and 4,500 characters. If a doc is way off, dispatch a fix subagent.

- [ ] **Step 4: Template section presence (docs 01-09)**

```bash
for n in 01_concepts 02_pros_cons 03_conway 04_decomposition 05_communication \
         06_data_ownership 07_resilience 08_observability 09_scaling_istio; do
  count=$(grep -c '^## ' 06_microservie/docs/${n}.md)
  echo "${n}: ${count} h2 sections"
done
```

Expected: each at least 5 (matching the §1-§5 template).

- [ ] **Step 5: Footer navigation present**

```bash
for n in 01_concepts 02_pros_cons 03_conway 04_decomposition 05_communication \
         06_data_ownership 07_resilience 08_observability 09_scaling_istio; do
  grep -c '次に読む' 06_microservie/docs/${n}.md
done
```

Expected: each = 1.

### Task 4.2: Read-through review

- [ ] **Step 1: Read all 10 docs in order**

Read 01 → 10 sequentially. Note:
- Any contradictory claims between docs
- Repeated explanations (one doc covering another's job)
- Order issues — does each doc set up the next?
- Tone / register inconsistencies

- [ ] **Step 2: Decide remediation**

If issues found, dispatch fix subagent(s) for the affected docs. If clean, nothing to do.

### Task 4.3: Update parent spec to mark completion

**File to modify:** `docs/superpowers/specs/2026-05-12-microservices-chapter-design.md`

- [ ] **Step 1: Add note that docs are complete**

This is informational — the §8 verification checklist already exists. No code change here unless a section needs an explicit note. **Skip this task if no edit needed**; the chapter is functionally complete once Phase 3 is done.

---

## Self-Review Notes

After writing the plan I checked it against the spec:

- **Spec §2 (10 files):** ✅ Phase 1 covers 9, Phase 2 covers the 10th, Phase 3 wires the README.
- **Spec §4 (vocabulary):** ✅ embedded in every Phase 1 task; verified in Phase 4 grep.
- **Spec §5 (code links):** ✅ dictionary embedded; verified in Task 2.1 Step 1.
- **Spec §6 (template):** ✅ embedded; verified by section count in Phase 4 grep.
- **Spec §7 (individual params):** ✅ each Phase 1 task lists its individual params.
- **Spec §8 (parallelism):** ✅ Phase 1 = 9 parallel, Phase 2 = sequential 10th.
- **Spec §10 (review):** ✅ Phase 4 covers grep-based + read-through review.
- **Spec §11 (README link):** ✅ Phase 3 Task 3.1.
- **Spec §12 (completion):** ✅ Phase 4 closes the loop.

No placeholders. All task steps have concrete content. No "implement later" or "similar to other tasks" references.
