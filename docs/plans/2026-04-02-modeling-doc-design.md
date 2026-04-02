# テーブル設計モデル比較ドキュメント 設計書

**日付**: 2026-04-02  
**対象ファイル**: `05_database/02/docs/05_modeling.md`

---

## 概要

第1章で正規化モデルを学んだ前提で、実務でよく登場する4つのテーブル設計モデル（イミュータブル・Theory of Models・アンカー・スター型）を実例付きで解説するドキュメントに書き換える。

---

## ドキュメント構成

### タイトル

`# テーブル設計モデルの比較`

### 全体骨格

```
# テーブル設計モデルの比較
（イントロ: 第1章で正規化を学んだ前提として、それ以外の4つの設計思想を紹介）

## 1. イミュータブルモデル
## 2. Theory of Models (TM)
## 3. アンカーモデル
## 4. スター型スキーマ
## 5. モデル選択の判断基準
```

各セクションは `04_denormalization.md` と同じ小見出し構成:

```
### 概念
### 適用例
  - テーブル定義表（カラム名/型/制約/説明）
  - サンプルデータ表
  - Mermaid ER図
### クエリ例
### トレードオフ
```

---

## 各セクション詳細

### 1. イミュータブルモデル

**題材**: ユーザープロフィール変更の履歴管理

**テーブル**:
- `users(id, created_at)` — 不変のコア情報のみ保持
- `user_profile_history(user_id, display_name, bio, effective_at)` — プロフィール変更を INSERT で積み上げ

**Mermaid ER図**: `users ||--o{ user_profile_history`

**クエリ例**:
- 「現在のプロフィールを取得」— `DISTINCT ON (user_id) ORDER BY effective_at DESC`
- 「特定時点のプロフィールを取得」— `WHERE effective_at <= '2025-10-01'`

**トレードオフ表**: 過去再現性 / 読み取り複雑度 / ストレージコスト

---

### 2. Theory of Models (TM)

**題材**: SNSスキーマをリソース（永続的な実体）とイベント（発生した出来事）に分類

**説明方針**:
- 現行SNSスキーマの各テーブルをリソース/イベントに分類した対応表を提示
- リソーステーブル: users, posts, stamps, hashtags
- イベントテーブル: follows, post_favorites, post_stamps, post_replies, hashtag_posts, user_blocks, user_mutes

**テーブル定義**: 既存スキーマから代表例として `users`（リソース）と `follows`（イベント）のテーブル定義を掲載

**Mermaid ER図**: リソースとイベントをグループで示したER図

**クエリ例**:
- 「通知一覧の生成」— イベントテーブル（post_favorites, post_stamps, post_replies）のみから通知を生成するクエリ
- イベントテーブルをクロスJOINするだけで完結する設計を示す

**トレードオフ表**: 設計明確度 / テーブル分割コスト / クエリの複雑度

---

### 3. アンカーモデル

**題材**: `users` の属性を超正規化（1属性1テーブル）

**テーブル**:
- `user_anchor(id, created_at)` — アンカー本体（主キーのみ）
- `user_display_name(user_id, display_name, updated_at)` — 表示名属性テーブル
- `user_bio(user_id, bio, updated_at)` — プロフィール文属性テーブル

**Mermaid ER図**: `user_anchor ||--o| user_display_name`, `user_anchor ||--o| user_bio`

**クエリ例**:
- 「ユーザー情報をフル取得」— `LEFT JOIN` を連結するクエリ
- 「属性追加時にALTER TABLE不要」— `CREATE TABLE user_website(...)` の追加例

**トレードオフ表**: カラム追加影響 / JOIN複雑度 / 可読性 / 既存行への影響

---

### 4. スター型スキーマ

**題材**: SNS投稿の分析ダッシュボード（OLAP用途）

**テーブル**:
- `fact_post_events(post_id, user_id, date_id, hashtag_count, like_count, reply_count, stamp_count)` — ファクトテーブル
- `dim_user(user_id, display_name, created_month)` — ユーザーディメンション
- `dim_date(date_id, year, month, day, day_of_week)` — 日付ディメンション

**Mermaid ER図**: ファクト中心の星形ER図

**クエリ例**:
- 「月別・ユーザー別の投稿数・いいね数集計」— GROUP BY with dimension JOIN
- 「特定月のアクティブユーザーランキング」

**トレードオフ表**: 集計速度 / データ鮮度 / 書き込みコスト / トランザクション整合性

---

### 5. モデル選択の判断基準

**構成**: 要件×モデルのマトリクス表

| 要件 | イミュータブル | TM | アンカー | スター型 |
|------|--------------|-----|---------|---------|
| 変更履歴の保存が必要 | ◎ | △ | ○ | × |
| ビジネスルールをDB構造に落としたい | △ | ◎ | △ | × |
| 頻繁なカラム追加が予想される | △ | △ | ◎ | × |
| 大量データの集計・分析が主目的 | × | × | × | ◎ |

---

## 実装方針

- フォーマットは `04_denormalization.md` に完全準拠
- テーブル定義: カラム名/型/制約/備考の4列形式
- サンプルデータ: UUID省略形式（`01905a3b-...`）を統一使用
- Mermaid ER図: 全4モデルに追加
- 既存スキーマ（`00_schema.md`）への参照リンクを適宜挿入
