# 第1章: データベースの基礎 — 正規化

## この章で学ぶこと

- 正規化の目的と必要性
- 第1〜第3正規形（実務でよく使う範囲）
- 第4〜第7正規形（理論的な上位正規形）
- 非正規化とその使いどころ
- 実践: スーパーのレジシステムのDB設計

## 読む順番

1. [正規化の解説](01/docs/normalization.md) — 理論と各正規形の定義
2. [設計演習](01/docs/exercise.md) — スーパーのレジシステムを段階的に正規化する
3. [設計演習の解答](01/docs/answer.md)

## 前提知識

- SQLの基本（SELECT / CREATE TABLE）
- 主キー・外部キーの概念

---

## 第2章: クエリ最適化 — EXPLAIN・インデックス・非正規化

### この章で学ぶこと

- EXPLAIN ANALYZE による実行計画の読み方
- インデックスの種類と選び方（B-tree・複合・部分・カバリング）
- クエリの書き換えによる最適化（EXISTS・CTE・LATERAL JOIN・ページネーション）
- パフォーマンスのための非正規化パターン（冗長FK・JSON集約・集約キャッシュ）

### 読む順番

1. [SNSスキーマ設計](02/docs/00_schema.md) — 題材となるSNSのDB構造
2. [EXPLAINの読み方](02/docs/01_explain.md) — 実行計画を読んでボトルネックを発見する
3. [インデックス](02/docs/02_indexing.md) — インデックスの仕組みと選び方
4. [クエリチューニング](02/docs/03_query_tuning.md) — クエリの書き方で速くする
5. [非正規化パターン](02/docs/04_denormalization.md) — 意図的な冗長化でさらに速くする
6. [クエリ最適化演習](02/docs/exercise.md) — タイムラインクエリを段階的に改善する
7. [演習の解答](02/docs/answer.md)

### 前提知識

- 第1章の内容（正規化の理解）
- 基本的なJOINクエリ（INNER JOIN・LEFT JOIN）
- CREATE TABLE・INSERT
