# クエリ最適化演習

> **演習前の準備**: psql で接続後、`exercise.sql` を実行すると自動的に演習ユーザーの UUID が `my_id` 変数に設定されます（`\gset` を使用）。以降のクエリでは `:'my_id'` で参照してください。

## 演習の概要

SNSアプリのタイムライン機能を題材に、6ステップでクエリを段階的に最適化していきます。

- **主人公**: 演習ユーザー（`:'my_id'`）
- 各ステップは前のステップの結果を引き継ぐため、**必ず順番に進めること**
- 解答を見る前に、自分で考えてみること

## 前提

- `chapter02` データベースに接続していること
- `\i /sql/02/exercise.sql` で演習用の出発点クエリを確認できること（実行すると `my_id` 変数が自動的に設定される）

---

## Step 1: ベースクエリを理解する

**Goal**: 「フォローしているユーザーの最新投稿20件」を取得するクエリを書く

**Task:**

- `exercise.sql` のクエリを実行して、結果を確認する
- 結果の件数・表示項目（`id`, `content`, `created_at`, `display_name`）を確認する

**考えてみよう:**

- このクエリはどのような処理をしているか？
- サブクエリ（`IN` 句の中身）と外側のクエリの役割は？

---

## Step 2: EXPLAIN ANALYZE で実行計画を読む

**Goal**: Step 1 のクエリに `EXPLAIN ANALYZE` を付けて実行計画を読む

**Task:**

- 先頭に `EXPLAIN ANALYZE` を付けて実行する
- 出力を読んで以下を特定する:
  - Seq Scan が発生しているテーブル
  - 各ノードの `cost` と `actual time`

**考えてみよう:**

- `follows` テーブルはどのように読まれているか？（Seq Scan？ Index Scan？）
- `posts` テーブルはどのように読まれているか？
- どのステップが一番時間がかかっているか？
- `Execution Time:` の値をメモしておこう（Step 6 で最終結果と比較する）

---

## Step 3: インデックスを追加して改善する

**Goal**: Step 2 で発見した Seq Scan を解消するインデックスを追加する

**Task:**

- `follows` テーブルの `user_id` カラムにインデックスを追加する
- `posts` テーブルの `(user_id, created_at DESC)` に複合インデックスを追加する
- 追加後に `EXPLAIN ANALYZE` を再実行して、変化を確認する

**考えてみよう:**

- Seq Scan が Index Scan に変わったか？
- 実行時間はどれくらい改善したか？
- `posts` への複合インデックスはなぜ `(user_id, created_at DESC)` が効果的か？

---

## Step 4: タイムラインを拡張する

**Goal**: ブロック/ミュート除外 + フォロータグの投稿も含めた完全なタイムラインクエリを書く

**Task:** 以下の要件を満たすクエリを作成する

1. フォローしているユーザーの投稿を含める
2. フォローしているタグの投稿も含める（`UNION` で結合）
3. ブロック/ミュートしているユーザーの投稿を除外する（`NOT EXISTS`、4方向）
4. `ORDER BY created_at DESC LIMIT 20` でページング

**ヒント:**

- CTE を使って整理すると読みやすくなる
- ブロック除外は `user_blocks` で自分がブロックした & ブロックされたの2条件
- ミュート除外は `user_mutes` で自分がミュートした & ミュートされたの2条件
- タグフォローの投稿は `hashtag_posts` と `hashtag_follows` を使う

---

## Step 5: Step 4 のクエリを最適化する

**Goal**: Step 4 のクエリに `EXPLAIN ANALYZE` をかけ、追加でインデックスが必要な箇所を特定して追加する

**Task:**

- Step 4 のクエリに `EXPLAIN ANALYZE` を付けて実行する
- 新しく Seq Scan が発生しているテーブルを特定する
- 必要なインデックスを追加する
- 追加後に `EXPLAIN ANALYZE` を再実行して確認する

**ヒント:** 追加が必要なインデックスの候補

- `hashtag_follows(user_id, hashtag_id)` — タグフォロー検索
- `hashtag_posts(hashtag_id, post_id)` — タグ付き投稿検索
- `user_blocks(user_id, block_user_id)` と `(block_user_id, user_id)` — 双方向ブロック
- `user_mutes(user_id, mute_user_id)` と `(mute_user_id, user_id)` — 双方向ミュート

---

## Step 6: 非正規化で最終改善する

**Goal**: `post_stats` テーブルと `posts.hashtags` JSON カラムを活用して、タイムラインを完成させる

**Task:**

1. 以下のコマンドで `04_denormalization.sql` を実行して、`post_stats` テーブルと `posts.hashtags` カラムを用意する（第2章の理論解説ドキュメントを既に読んだ場合は実行済みかもしれない）
   ```sql
   \i /sql/02/04_denormalization.sql
   ```
2. Step 4/5 のクエリを拡張して、以下の情報もタイムラインに表示する
   - いいね数（`post_stats.like_count`）
   - コメント数（`post_stats.reply_count`）
   - スタンプ数（`post_stats.stamp_count`）
   - タグ一覧（`posts.hashtags`、COUNT クエリ不要）
3. 最終版の `EXPLAIN ANALYZE` を実行して、Step 2 で記録した実行時間と比較する（Step 2 実行時にかかった時間をメモしておこう）

**考えてみよう:**

- `post_stats` を使うことで何のサブクエリが不要になったか？
- `posts.hashtags` を使うことで何の JOIN が不要になったか？

---

## 最後に

[演習の解答を見る](answer.md)
