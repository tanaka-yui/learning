# 非正規化パターン

## 1. 非正規化とは

第1章では、正規化によって**データの整合性を確保**し、更新異常を防ぐ設計を学びました。
第2章では、その正規化を土台にしつつ、**パフォーマンスのために意図的に正規化を崩す**テクニックを扱います。

### 正規化との関係

| 観点 | 正規化 | 非正規化 |
|------|--------|---------|
| 目的 | 整合性の確保・更新異常の防止 | 読み取り性能の向上 |
| JOINの数 | 多い（テーブルが分割されている） | 少ない（結果が事前に計算されている） |
| 書き込みコスト | 低い（1箇所だけ更新する） | 高い（複数箇所を同期する必要がある） |
| 整合性リスク | 低い | 高い（同期忘れで矛盾が生じる） |

### 重要な原則

> **非正規化は設計の「間違い」ではなく、意図的なトレードオフである。**

読み取りを速くする代わりに、書き込み時の整合性維持コストが増えます。
このトレードオフを理解した上で、**実際に計測してパフォーマンス問題が発生したとき**に限り適用するのが原則です。

---

## 2. パターン1 — 冗長FK

### 概念

あるテーブルの外部キーを別テーブルへコピーし、JOINを1つ削減するパターンです。

### 適用例

`post_favorites.post_user_id`（`posts.user_id` のコピー）

**post_favorites テーブル（非正規化後）**

| 列名 | 日本語名 | 型 | 制約 | 備考 |
|------|---------|-----|------|------|
| post_id | 投稿ID | INT | PK（複合）, FK → posts | |
| user_id | いいねしたユーザーID | INT | PK（複合）, FK → users | |
| post_user_id | 投稿者ID | INT | NOT NULL | posts.user_id のコピー（冗長） |
| created_at | 作成日時 | TIMESTAMPTZ | NOT NULL | |

### ユースケース

「誰が自分の投稿にいいねしたか」を高頻度で取得する通知機能。

### クエリ比較

**Before（JOINが必要）**

```sql
SELECT pf.user_id AS reaction_user, p.user_id AS post_owner
FROM post_favorites pf
JOIN posts p ON p.id = pf.post_id
WHERE p.user_id = 42;
```

**After（JOINなしで直接取得できる）**

```sql
SELECT user_id AS reaction_user, post_user_id AS post_owner
FROM post_favorites
WHERE post_user_id = 42;
```

### トレードオフ

| 観点 | 内容 |
|------|------|
| 読み取り改善 | `posts` との JOIN を1つ削減できる |
| 書き込みコスト | 低い（投稿の `user_id` が変わった場合のみ更新が必要） |
| 整合性リスク | 低い（`user_id` はほぼ不変のため、実際に問題になることは稀） |

---

## 3. パターン2 — JSON集約カラム

### 概念

JOINによって得られる結果をあらかじめ計算し、JSON形式で1カラムに格納するパターンです。

### 適用例

`posts.hashtags`（タグ名の JSON 配列）

**posts テーブル（非正規化後）**

| 列名 | 日本語名 | 型 | 制約 | 備考 |
|------|---------|-----|------|------|
| id | 投稿ID | INT | PK | |
| user_id | ユーザーID | INT | NOT NULL, FK → users | |
| content | 本文 | TEXT | NOT NULL | |
| hashtags | タグ一覧 | JSONB | | `hashtag_posts` + `hashtags` の結合結果（冗長） |
| created_at | 作成日時 | TIMESTAMPTZ | NOT NULL | |

### ユースケース

タイムライン表示でタグを毎回 JOIN 取得するのを避ける。

### クエリ比較

**Before（2回の JOIN が必要）**

```sql
SELECT p.id, p.content, array_agg(h.name) AS tags
FROM posts p
LEFT JOIN hashtag_posts hp ON hp.post_id = p.id
LEFT JOIN hashtags h ON h.id = hp.hashtag_id
WHERE p.user_id = 42
GROUP BY p.id, p.content;
```

**After（JOINなしでタグを取得できる）**

```sql
SELECT id, content, hashtags FROM posts WHERE user_id = 42;
```

### トレードオフ

| 観点 | 内容 |
|------|------|
| 読み取り改善 | `hashtag_posts` + `hashtags` への JOIN を複数削減できる |
| 書き込みコスト | 中（タグの追加・削除のたびに `posts.hashtags` も更新が必要） |
| 整合性リスク | 中（タグ変更と JSON 更新のタイミングがずれると矛盾が生じる） |

---

## 4. パターン3 — 集約キャッシュテーブル

### 概念

毎回計算する集計値（COUNT など）をあらかじめ専用テーブルに保持するパターンです。

### 適用例

`post_stats(post_id, like_count, reply_count, stamp_count)`

**post_stats テーブル**

| 列名 | 日本語名 | 型 | 制約 | 備考 |
|------|---------|-----|------|------|
| post_id | 投稿ID | INT | PK, FK → posts | |
| like_count | いいね数 | INT | NOT NULL DEFAULT 0 | `post_favorites` の COUNT キャッシュ |
| reply_count | リプライ数 | INT | NOT NULL DEFAULT 0 | `post_replies` の COUNT キャッシュ |
| stamp_count | スタンプ数 | INT | NOT NULL DEFAULT 0 | `post_stamps` の COUNT キャッシュ |
| updated_at | 更新日時 | TIMESTAMPTZ | NOT NULL | |

### ユースケース

タイムラインでいいね数・コメント数を毎回 COUNT するのを避ける。

### クエリ比較

**Before（コリレートサブクエリで毎回 COUNT が実行される）**

```sql
SELECT
    p.id, p.content, p.created_at,
    (SELECT COUNT(*) FROM post_favorites pf WHERE pf.post_id = p.id) AS like_count,
    (SELECT COUNT(*) FROM post_replies  pr WHERE pr.post_id = p.id) AS reply_count
FROM posts p
WHERE p.user_id IN (SELECT follow_user_id FROM follows WHERE user_id = 1)
ORDER BY p.created_at DESC
LIMIT 20;
```

**After（post_stats を JOIN するだけで集計値が取得できる）**

```sql
SELECT p.id, p.content, p.created_at, ps.like_count, ps.reply_count
FROM posts p
JOIN post_stats ps ON ps.post_id = p.id
WHERE p.user_id IN (SELECT follow_user_id FROM follows WHERE user_id = 1)
ORDER BY p.created_at DESC
LIMIT 20;
```

### トレードオフ

| 観点 | 内容 |
|------|------|
| 読み取り改善 | 毎回の COUNT 集計を削減できる |
| 書き込みコスト | 高い（いいね・コメント・スタンプのたびに `post_stats` も更新が必要） |
| 整合性リスク | 高い（更新漏れや障害時に集計値が実態と乖離する） |

---

## 5. トレードオフ整理

| パターン | 読み取り改善 | 書き込みコスト増 | 整合性リスク |
|---------|------------|----------------|------------|
| 冗長FK | JOIN 1つ削減 | 低（参照元更新時のみ） | 低 |
| JSON集約 | JOIN 複数削減 | 中（タグ変更時） | 中 |
| 集約キャッシュ | COUNT 集計削減 | 高（毎回の更新が必要） | 高 |

### 適用判断の基準

非正規化を検討すべきかどうかは、次の2つの軸で判断します。

| 判断軸 | 非正規化に向いている | 正規化を維持すべき |
|--------|---------------------|-----------------|
| 読み取り:書き込みの比率 | 読み取りが圧倒的に多い | 書き込みが多い、または均衡している |
| 整合性の許容度 | 多少のずれを許容できる（イベント数の表示など） | 厳密な整合性が必要（残高・在庫など） |

---

## 6. 第1章の非正規化との接続

第1章では、セクション8「非正規化」として「意図的に正規化を崩してクエリのパフォーマンスを向上させる」テクニックと、その実務指針（まず正規化して設計し、パフォーマンス問題が実際に発生したときに限り検討する）を学びました。
第2章では、その具体的なパターン（冗長FK・JSON集約・集約キャッシュ）とトレードオフを実際のクエリで確認しました。

### まとめ

| 章 | 観点 | 非正規化の扱い |
|---|------|--------------|
| 第1章 | 理論 | 整合性を優先した正規化の対比として紹介 |
| 第2章 | 実践 | どの場面でどのパターンを使うかを具体的に検討 |

> **実務の指針**: まず正規化して設計し、パフォーマンス問題が実際に発生したときに限り、計測しながら非正規化を検討する。
