# 06_microservie 解説ドキュメント (Plan 4) 設計書

作成日: 2026-05-13
対象: `06_microservie/docs/` 配下の 10 個の解説 Markdown
親仕様: `docs/superpowers/specs/2026-05-12-microservices-chapter-design.md` の §2.1, §2.2, §8

---

## 1. 目的とスコープ

### 1.1 目的

章設計書 §8「完了条件」項目 2 を満たし、読者が章の意図を「読む → 動かす → 読み直す」というサイクルで吸収できる解説層を完成させる。コードは既に Plan 1〜3 で揃っているため、本 Plan はコードを書かず Markdown のみを書く。

### 1.2 スコープ内

- `06_microservie/docs/01_concepts.md` 〜 `06_microservie/docs/10_patterns_in_code.md` の 10 ファイルの新規作成
- 統一語彙とコードリンク辞書の整備
- Mermaid 図を中心とした視覚化
- 章間相互リンクの整備（README ↔ docs、doc ↔ doc）

### 1.3 スコープ外

- 多言語化（翻訳）
- 画像ファイルへの書き出し（Mermaid は GitHub のレンダリングに任せる）
- リンクチェッカー導入
- 既存章（02_cache, 05_database 等）の docs の改修
- 新しいサンプルコードの追加

---

## 2. ドキュメント一覧と各役割

章設計書 §2.1 の表を踏襲する。

| ファイル | 役割 |
|---|---|
| `01_concepts.md` | マイクロサービスとは / モノリス・モジュラーモノリスとの違い |
| `02_pros_cons.md` | メリット・デメリット、選定チェックリスト |
| `03_conway.md` | Conway's law, Inverse Conway Maneuver, Team Topologies |
| `04_decomposition.md` | サービス分割（DDD 境界づけられたコンテキスト / ビジネス能力）、EC ドメインへの適用 |
| `05_communication.md` | 同期 vs 非同期、REST vs gRPC、本サンプルでのプロトコル方針 |
| `06_data_ownership.md` | DB-per-service、参照整合性が効かない世界、結果整合性 |
| `07_resilience.md` | timeout / retry / circuit breaker / Saga（補償）、idempotency |
| `08_observability.md` | 構造ログ・trace_id 伝搬・分散トレース、OpenTelemetry の基本 |
| `09_scaling_istio.md` | 大規模化の課題、サービスメッシュ概念、Istio 構成要素 |
| `10_patterns_in_code.md` | 各パターン → 実装ファイル + シンボルのマッピング |

---

## 3. 分量と密度

| 区分 | 字数目安 | Mermaid 図 | コード例 |
|---|---|---|---|
| 01-09（解説型） | 2,000〜4,000 字 | 1〜3 枚 | 0〜2 個（極小） |
| 10（索引型） | 1,500〜3,000 字 | 0 枚 | 0 個（マッピング表中心） |

許容レンジは 1,500〜4,500 字（緩衝込み）。

---

## 4. 統一語彙表

9 並列ライターでも語の揺れが出ないよう、表記を固定する。各サブエージェントのプロンプトにそのまま貼り込む。

| 概念 | 採用表記 | NG 表記 |
|---|---|---|
| microservice | マイクロサービス | マイクロ・サービス / μサービス |
| monolith | モノリス | モノリシック（名詞として使うとき）|
| modular monolith | モジュラーモノリス | モジュール式モノリス |
| Conway's law | コンウェイの法則 | コンウェイ法則 / Conway 則 |
| Team Topologies | Team Topologies（原語） | チームトポロジー |
| Bounded Context | 境界づけられたコンテキスト | バウンデッドコンテキスト |
| Saga | Saga（原語） | サーガ |
| Circuit Breaker | サーキットブレーカー | 回路遮断器 / 略称 CB は本文 2 回目以降のみ可 |
| timeout | タイムアウト | 時間切れ |
| retry | リトライ | 再試行 |
| idempotency | 冪等性 | べき等性 / idempotent 性 |
| observability | 観測性 | オブザーバビリティ / 可観測性 |
| trace_id | trace_id（コード文脈・原語） | トレース ID |
| span | span（原語） | スパン / 区間 |
| service mesh | サービスメッシュ | サービス網 |
| sidecar | サイドカー | サイドカー proxy |

### 4.1 検証用 grep

```bash
for word in 'マイクロ・サービス' 'モノリシック' 'コンウェイ法則' 'バウンデッドコンテキスト' 'サーガ' \
            'べき等性' 'オブザーバビリティ' '可観測性' 'サービス網' 'トレース ID' 'トレースID'; do
  hits=$(grep -rn "$word" 06_microservie/docs/ 2>/dev/null)
  if [ -n "$hits" ]; then
    echo "VOCAB VIOLATION: $word"
    echo "$hits"
  fi
done
```

完了条件: 上記スクリプトが何も出力しない。

---

## 5. コードリンク辞書

10_patterns_in_code.md と本文中の「実例」セクションが参照できるシンボルを事前に固定。**辞書外のシンボル参照は禁止**、**行番号も書かない**。

| 概念 | ファイル | シンボル |
|---|---|---|
| Saga オーケストレーション | `services/order/internal/saga/checkout.go` | `Run`, `Step` 構造体 |
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

### 5.1 参照記法

```
`services/order/internal/saga/checkout.go::Run`
```

- 区切りは `::`
- 行番号は禁止
- 10_patterns_in_code がこの辞書をマスターテーブルとして掲載

### 5.2 実在確認

10_patterns_in_code を書くサブエージェントは、上記の各 file:symbol が現コードに存在するか `grep -rn` で確認してから記載する。存在しないものを発見したら控訴（BLOCKED 報告）する。

---

## 6. ドキュメントの共通テンプレート

01-09 は以下の構造を厳守する。

```markdown
# <タイトル>

> <1-2 文の概要。この doc で答える問い>

---

## 1. なぜ <概念> を扱うのか

<why セクション。問題提起 + 読むモチベーション>

## 2. <概念> とは

<定義。教科書的すぎず、本章サンプルとの紐付けを意識>
Mermaid 図は概念が視覚化で明確になる場合に 1 枚入れる。
（無理して入れる必要は無い）

## 3. 実例: 本章のサンプルではどう現れるか

<具体例。コード位置は file::symbol で参照>

## 4. 落とし穴 / よくある誤解

<実装時に出会う罠を 2〜3 個>

## 5. スコープ外 — この章で扱わないこと

<本章で省略した派生トピックの言及。
親仕様 §7「スコープ外」のうち本 doc に関連するものをここで触れる>

---

**次に読む:** [N+1: <次のタイトル>](0(N+1)_<slug>.md)
**章の入口に戻る:** [README](../README.md)
```

### 6.1 10_patterns_in_code は例外

```markdown
# 10: コード上のパターン索引

<本章で出会うパターンが、どのファイル/シンボルで実装されているかの索引>

## 索引の使い方

<読者向け案内。doc を読みながら該当コードを開く動線>

## パターン一覧

| パターン名 | doc | 実装ファイル | シンボル | 補足 |
|---|---|---|---|---|
| Saga オーケストレーション | [07](07_resilience.md) | services/order/internal/saga/checkout.go | Run, Step | … |
| ... | ... | ... | ... | ... |

## 章全体の読み直し動線

<01〜09 のどこから読み返すと有効かのガイド>

---

**章の入口に戻る:** [README](../README.md)
```

---

## 7. doc ごとの「個別パラメータ」

§3「実例」セクションで参照すべきものを doc 単位で固定。

| doc | §3 で参照する主な事実・ファイル |
|---|---|
| 01_concepts | Plan 1 のディレクトリ構成（services/, bff/, frontend/）、章全体図 |
| 02_pros_cons | 親仕様 §3.4 主要フロー Saga 図、payment のフレーク（`flake.go::ShouldFail`） |
| 03_conway | services 境界に対応する仮想チーム（catalog 担当 / inventory 担当 etc.） |
| 04_decomposition | 5 サービスの責務表（catalog/inventory/order/payment/user-auth）、所有データ |
| 05_communication | `proto/` 配下、`bff/internal/handler/checkout.go::Checkout.Post`（REST→gRPC 集約） |
| 06_data_ownership | postgres-* の 5 インスタンス、inventory と catalog が同じ product_id を別の意味で持つこと |
| 07_resilience | `saga/checkout.go::Run`、`backoff.Retry`、`gobreaker.CircuitBreaker`、`flake.go::ShouldFail` |
| 08_observability | `obs/tracing.go::InitTracing`、`middleware/traceid.go::TraceID`、Jaeger UI 体験、`TraceIdChip` |
| 09_scaling_istio | 概念のみ。本章コードへの直接リンクは最小限。Istio 構成要素（Envoy, Pilot, mTLS, L7 routing） |

---

## 8. 並列化戦略

```
[Phase 1: 9 並列]
  subagent-01 ─→ 01_concepts.md       ┐
  subagent-02 ─→ 02_pros_cons.md      │
  subagent-03 ─→ 03_conway.md         │
  subagent-04 ─→ 04_decomposition.md  │ 互いに独立
  subagent-05 ─→ 05_communication.md  ├─ 同時起動
  subagent-06 ─→ 06_data_ownership.md │
  subagent-07 ─→ 07_resilience.md     │
  subagent-08 ─→ 08_observability.md  │
  subagent-09 ─→ 09_scaling_istio.md  ┘

[Phase 2: 1 直列]
  subagent-10 ─→ 10_patterns_in_code.md
                  ↑ 01-09 完了後にスタート
                  ↑ grep でシンボル実在確認
```

### 8.1 コミット戦略

- 各サブエージェントは「書く + コミット」までやる
- コミットメッセージ: `microservices(docs): add 0X_<slug>.md`
- 9 並列なので順序は git に任せる（衝突しないファイル単位の追加のみ）
- 10 は単独コミット

### 8.2 失敗時のリカバリー

- doc 単位で再ディスパッチ可能（互いに依存しない）
- レビューで NG が出たら該当 doc だけ修正サブエージェントを起動
- 統一語彙違反は grep スクリプトで一括検出 → 該当 doc のみ修正

---

## 9. サブエージェントへの渡し方

各 subagent への入力（プロンプト）に以下を含める。

1. **ドキュメントの役割**: §2 表の該当行
2. **必須カバー項目**: doc ごとの 3〜5 個の箇条書き（コントローラが作成）
3. **統一語彙表**: §4 をそのまま貼り付け
4. **コードリンク辞書**: §5 をそのまま貼り付け
5. **文体ルール**: 日本語 / 2000-4000 字 / Mermaid 中心 / why → how / 行番号禁止
6. **共通テンプレート**: §6 をそのまま貼り付け
7. **個別パラメータ**: §7 表の該当行
8. **やってはいけないこと**: スコープ外への踏み込み、他 doc 役割の侵食、辞書外シンボル
9. **自己レビュー前のチェックリスト**: テンプレ充足 / 字数 / 語彙 grep / 末尾ナビ
10. **コミット**: `microservices(docs): add 0X_<slug>.md`

### 9.1 10 専用追加情報

- 「01-09 が完成済み。コミット SHA は <list>」
- 「grep -rn で各シンボルが実コードに存在することを確認すること」
- 「テンプレ外。マッピング表中心」
- 「コードベース全体を `grep -rn` で確認するのは必須」

---

## 10. レビュー戦略

doc は単一ファイルのテキストなので、コードの 2 段レビューは過剰。代わりに:

### 10.1 個別 doc レビュー（grep ベースの自動チェック）

各 doc コミット後にコントローラが実施:
- §6 テンプレートの 5 セクションが全部あるか（`grep -c '^## ' file.md` >= 5）
- 統一語彙違反 grep
- 行番号付き参照の検出（`grep -nE '\.go:\d+|\.tsx?:\d+' file.md`）
- 字数チェック（`wc -m file.md`）
- 末尾ナビの存在（`grep -c '次に読む' file.md`）

### 10.2 章全体整合性レビュー（10 完了後の 1 回）

コントローラが章設計書 §1.3 学習動線通りに 01 → 10 を通読し、以下を確認:
- 隣接 doc 間で重複説明が無い
- 用語の意味が doc をまたいでブレていない
- 「次に読む」が正しい順序
- 10 のマッピングが他 doc から実際に使われている

### 10.3 NG が見つかった場合

該当 doc を 1 つの修正サブエージェントで修正。再度 grep チェック。

---

## 11. README とのリンク

`06_microservie/README.md` から docs/ への動線を引く。READ ME に以下のセクションを追加:

```markdown
## 学習動線

1. [01: マイクロサービスとは](docs/01_concepts.md)
2. [02: メリット・デメリット](docs/02_pros_cons.md)
3. [03: Conway's law と Team Topologies](docs/03_conway.md)
4. [04: サービス分割](docs/04_decomposition.md)
5. [05: 通信プロトコル](docs/05_communication.md)
6. [06: データ所有](docs/06_data_ownership.md)
7. [07: レジリエンス](docs/07_resilience.md)
8. [08: 観測性](docs/08_observability.md)
9. [09: 大規模化と Istio](docs/09_scaling_istio.md)
10. [10: コード上のパターン索引](docs/10_patterns_in_code.md)
```

このリンク追加も Plan 4 のスコープに含める。

---

## 12. 完了条件

1. `06_microservie/docs/` に 10 ファイルが存在
2. 各 doc が §6 テンプレートに従う（01-09）、10 は索引型
3. 字数が各 1,500〜4,500 字
4. §4.1 の grep が空（NG 表記ゼロ）
5. 行番号付き参照が無い
6. コードリンク辞書外のシンボル参照が無い（マニュアル確認）
7. 各 doc に「次に読む」「章の入口に戻る」ナビあり
8. `06_microservie/README.md` に学習動線リンクが追加
9. コントローラの章全体通読レビューでブロッカー無し
10. 親仕様 §8 完了条件項目 2 が満たされる

### 12.1 親仕様への波及

`docs/superpowers/specs/2026-05-12-microservices-chapter-design.md` §8「完了条件」項目 2 を満たし、章全体としての完了に近づく。
