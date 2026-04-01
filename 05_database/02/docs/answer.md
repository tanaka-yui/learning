# クエリ最適化演習 — 解答

> 解答を見る前に自分で解いてみましたか？
> 以下の EXPLAIN ANALYZE 出力は代表的な例です。実際の値は環境・データによって異なります。

---

## Step 1: ベースクエリ

**クエリの目的:** 演習ユーザー（`:'my_id'`）がフォローしているユーザーの投稿を新着順に20件取得する

```sql
SELECT p.id, p.content, p.created_at, u.display_name
FROM posts p
JOIN users u ON u.id = p.user_id
JOIN follows f ON f.follow_user_id = p.user_id
WHERE f.user_id = :'my_id'
ORDER BY p.created_at DESC
LIMIT 20;
```

**各部の役割:**

| 部分 | 役割 |
|------|------|
| `JOIN follows f ON f.follow_user_id = p.user_id` | 投稿者がフォロー対象かどうかを結合で確認 |
| `WHERE f.user_id = :'my_id'` | 演習ユーザーのフォロー関係だけに絞り込む |
| `ORDER BY p.created_at DESC` | 新着順に並び替える |
| `LIMIT 20` | 先頭20件のみ取得する |

**問題点:** `follows` テーブルを全件スキャンしてから `user_id = :'my_id'` で絞り込んでいるため、フォロー数が増えるほど遅くなる。

---

## Step 2: EXPLAIN ANALYZE で実行計画を読む

```
Hash Join  (cost=3335.00..36231.00 rows=2000 width=114) (actual time=32.3..245.2 loops=1)
  Hash Cond: (p.user_id = f.follow_user_id)
  -> Seq Scan on posts  (cost=0.00..28431.00 rows=1000000 width=80)
       (actual time=0.024..183.0 rows=1000000 loops=1)
  -> Hash  (cost=3285.00..3285.00 rows=4000 width=16) (actual time=31.8..31.8 loops=1)
       -> Seq Scan on follows  (cost=0.00..3085.00 rows=300000 width=32)
            (actual time=0.018..17.2 rows=300000 loops=1)
            Filter: (user_id = '01905a3b-7c10-7000-8000-000000000001')
            Rows Removed by Filter: 299970
Execution Time: 245.8 ms
```

**問題点の読み取り方:**

| 注目箇所 | 意味 |
|---------|------|
| `Seq Scan on posts (rows=1000000)` | posts テーブルを全100万件スキャンしている |
| `Seq Scan on follows (rows=300000)` | follows テーブルを全30万件スキャンしている |
| `Rows Removed by Filter: 299970` | 30万件スキャンして実際に使うのは約30件のみ（99.99%が無駄） |
| `Execution Time: 245.8 ms` | インデックスなしでは約246msかかっている |

---

## Step 3: インデックスを追加して改善する

**追加するインデックス:**

```sql
-- follows テーブル: user_id でフォロー先を素早く引く
CREATE INDEX IF NOT EXISTS idx_follows_user_id
    ON follows(user_id);

-- posts テーブル: user_id で絞り込み + created_at DESC でソート
CREATE INDEX IF NOT EXISTS idx_posts_user_created
    ON posts(user_id, created_at DESC);
```

**EXISTS を使ったクエリの書き換え:**

`JOIN` から `EXISTS` へ書き換えることで、同じユーザーが複数のフォロー経路で一致した場合の重複を防ぎ、必要な行が見つかった時点で探索を打ち切れる。

```sql
SELECT p.id, p.content, p.created_at, u.display_name
FROM posts p
JOIN users u ON u.id = p.user_id
WHERE EXISTS (
    SELECT 1 FROM follows f
    WHERE f.user_id = :'my_id' AND f.follow_user_id = p.user_id
)
ORDER BY p.created_at DESC
LIMIT 20;
```

**インデックス追加後の EXPLAIN:**

```
Nested Loop  (cost=0.56..845.20 rows=100 width=114) (actual time=0.04..1.8 loops=1)
  -> Index Scan using idx_follows_user_id on follows  (cost=0.42..54.30 rows=30 ...)
       Index Cond: (user_id = '01905a3b-7c10-7000-8000-000000000001')
  -> Index Scan using idx_posts_user_created on posts  (cost=0.56..15.30 rows=2 ...)
       Index Cond: (user_id = f.follow_user_id)
Execution Time: 1.9 ms
```

**改善効果:**

| 指標 | 最適化前 | 最適化後 |
|------|---------|---------|
| follows スキャン | Seq Scan（全300,000件） | Index Scan（~30件） |
| posts スキャン | Seq Scan（全1,000,000件） | Index Scan（絞り込み済み） |
| 実行時間 | ~246ms | ~2ms |

---

## Step 4: 完全タイムラインクエリ

ユーザーフォローだけでなく、タグフォロー・ブロック・ミュートも考慮した完全版。

```sql
WITH
    -- フォロー中ユーザーの投稿
    user_feed AS (
        SELECT p.id, p.content, p.created_at, p.user_id
        FROM posts p
        WHERE EXISTS (
            SELECT 1 FROM follows f
            WHERE f.user_id = :'my_id' AND f.follow_user_id = p.user_id
        )
    ),
    -- フォロー中タグの投稿
    tag_feed AS (
        SELECT p.id, p.content, p.created_at, p.user_id
        FROM posts p
        WHERE EXISTS (
            SELECT 1
            FROM hashtag_posts hp
            JOIN hashtag_follows hf ON hf.hashtag_id = hp.hashtag_id
            WHERE hp.post_id = p.id AND hf.user_id = :'my_id'
        )
    ),
    -- 全フィード（重複除去）
    feed AS (
        SELECT * FROM user_feed
        UNION
        SELECT * FROM tag_feed
    )
SELECT f.id, f.content, f.created_at, u.display_name
FROM feed f
JOIN users u ON u.id = f.user_id
-- ブロック/ミュート除外（4方向）
WHERE NOT EXISTS (
    SELECT 1 FROM user_blocks b WHERE b.user_id = :'my_id' AND b.block_user_id = f.user_id
)
AND NOT EXISTS (
    SELECT 1 FROM user_blocks b WHERE b.user_id = f.user_id AND b.block_user_id = :'my_id'
)
AND NOT EXISTS (
    SELECT 1 FROM user_mutes m WHERE m.user_id = :'my_id' AND m.mute_user_id = f.user_id
)
AND NOT EXISTS (
    SELECT 1 FROM user_mutes m WHERE m.user_id = f.user_id AND m.mute_user_id = :'my_id'
)
ORDER BY f.created_at DESC
LIMIT 20;
```

**CTE の構造:**

| CTE 名 | 役割 |
|--------|------|
| `user_feed` | フォロー中ユーザーの投稿を EXISTS で取得 |
| `tag_feed` | フォロー中ハッシュタグが付いた投稿を取得 |
| `feed` | `UNION` で2つのフィードを統合し重複を除去 |
| 最終 SELECT | ブロック・ミュートを4方向の `NOT EXISTS` で除外し、新着順20件を返す |

**4方向の除外が必要な理由:**

- 自分 → 相手のブロック: 自分がブロックした相手の投稿を除外
- 相手 → 自分のブロック: 自分をブロックした相手の投稿を除外
- 自分 → 相手のミュート: 自分がミュートした相手の投稿を除外
- 相手 → 自分のミュート: 自分をミュートした相手の投稿を除外（相互ミュート対応）

---

## Step 5: 追加インデックスと最適化

Step 4 のクエリで新たに登場したテーブル（`hashtag_follows`, `hashtag_posts`, `user_blocks`, `user_mutes`）にインデックスを追加する。

```sql
-- タグフォロー: ユーザーIDでフォロータグを引く
CREATE INDEX IF NOT EXISTS idx_hashtag_follows_user
    ON hashtag_follows(user_id, hashtag_id);

-- タグ付き投稿: タグIDで投稿を引く
CREATE INDEX IF NOT EXISTS idx_hashtag_posts_hashtag
    ON hashtag_posts(hashtag_id, post_id);

-- ブロック: 双方向で引けるよう2つのインデックス
CREATE INDEX IF NOT EXISTS idx_user_blocks_user
    ON user_blocks(user_id, block_user_id);
CREATE INDEX IF NOT EXISTS idx_user_blocks_blocked
    ON user_blocks(block_user_id, user_id);

-- ミュート: 双方向で引けるよう2つのインデックス
CREATE INDEX IF NOT EXISTS idx_user_mutes_user
    ON user_mutes(user_id, mute_user_id);
CREATE INDEX IF NOT EXISTS idx_user_mutes_muted
    ON user_mutes(mute_user_id, user_id);
```

**各インデックスが必要な理由:**

| インデックス | 対応する NOT EXISTS / EXISTS の条件 |
|------------|----------------------------------|
| `idx_hashtag_follows_user` | `hf.user_id = :'my_id'` でタグフォロー一覧を高速取得 |
| `idx_hashtag_posts_hashtag` | `hp.hashtag_id = hf.hashtag_id` でタグ付き投稿を高速取得 |
| `idx_user_blocks_user` | `b.user_id = :'my_id'` の方向（自分がブロック）を高速検索 |
| `idx_user_blocks_blocked` | `b.block_user_id = :'my_id'` の方向（自分がブロックされ）を高速検索 |
| `idx_user_mutes_user` | `m.user_id = :'my_id'` の方向（自分がミュート）を高速検索 |
| `idx_user_mutes_muted` | `m.mute_user_id = :'my_id'` の方向（自分がミュートされ）を高速検索 |

インデックス追加後は `Seq Scan` が `Index Scan` に切り替わり、各 `NOT EXISTS` のサブクエリが定数時間で完了するようになる。

---

## Step 6: 非正規化を活用した最終版

`04_denormalization.sql` で作成済みの `post_stats` テーブルと `posts.hashtags` 列を活用して、集計クエリを排除する。

```sql
WITH
    user_feed AS (
        SELECT p.id, p.content, p.created_at, p.user_id, p.hashtags
        FROM posts p
        WHERE EXISTS (
            SELECT 1 FROM follows f
            WHERE f.user_id = :'my_id' AND f.follow_user_id = p.user_id
        )
    ),
    tag_feed AS (
        SELECT p.id, p.content, p.created_at, p.user_id, p.hashtags
        FROM posts p
        WHERE EXISTS (
            SELECT 1
            FROM hashtag_posts hp
            JOIN hashtag_follows hf ON hf.hashtag_id = hp.hashtag_id
            WHERE hp.post_id = p.id AND hf.user_id = :'my_id'
        )
    ),
    feed AS (
        SELECT * FROM user_feed
        UNION
        SELECT * FROM tag_feed
    )
SELECT
    f.id,
    f.content,
    f.created_at,
    f.hashtags,
    u.display_name,
    ps.like_count,
    ps.reply_count,
    ps.stamp_count
FROM feed f
JOIN users u ON u.id = f.user_id
JOIN post_stats ps ON ps.post_id = f.id
WHERE NOT EXISTS (
    SELECT 1 FROM user_blocks b WHERE b.user_id = :'my_id' AND b.block_user_id = f.user_id
)
AND NOT EXISTS (
    SELECT 1 FROM user_blocks b WHERE b.user_id = f.user_id AND b.block_user_id = :'my_id'
)
AND NOT EXISTS (
    SELECT 1 FROM user_mutes m WHERE m.user_id = :'my_id' AND m.mute_user_id = f.user_id
)
AND NOT EXISTS (
    SELECT 1 FROM user_mutes m WHERE m.user_id = f.user_id AND m.mute_user_id = :'my_id'
)
ORDER BY f.created_at DESC
LIMIT 20;
```

**非正規化による改善点:**

| 取得データ | Step 4 までのアプローチ | Step 6 のアプローチ |
|-----------|----------------------|-------------------|
| いいね数 | `likes` テーブルを `COUNT` 集計 | `post_stats.like_count` を直接参照 |
| リプライ数 | `replies` テーブルを `COUNT` 集計 | `post_stats.reply_count` を直接参照 |
| スタンプ数 | `stamps` テーブルを `COUNT` 集計 | `post_stats.stamp_count` を直接参照 |
| ハッシュタグ | `hashtag_posts` → `hashtags` を JOIN | `posts.hashtags`（JSON列）を直接参照 |

---

## 解答まとめ

### Step 1 → Step 6 の改善比較

| 指標 | Step 1（最適化なし） | Step 6（最適化後） |
|-----|-------------------|--------------------|
| follows スキャン | Seq Scan（全300,000件） | Index Scan（~30件） |
| posts スキャン | Seq Scan（全1,000,000件） | Index Scan（絞り込み済み） |
| ブロック/ミュート除外 | なし | NOT EXISTS 4方向 |
| タグフォロー | なし | hashtag_posts + hashtag_follows |
| いいね数取得 | なし | post_stats.like_count（JOIN） |
| タグ表示 | なし | posts.hashtags（JSON直接参照） |
| 実行時間（目安） | ~246ms | ~2ms以下 |

### インデックス一覧

| インデックス名 | テーブル | 列 |
|-------------|--------|-----|
| `idx_follows_user_id` | follows | user_id |
| `idx_posts_user_created` | posts | (user_id, created_at DESC) |
| `idx_hashtag_follows_user` | hashtag_follows | (user_id, hashtag_id) |
| `idx_hashtag_posts_hashtag` | hashtag_posts | (hashtag_id, post_id) |
| `idx_user_blocks_user` | user_blocks | (user_id, block_user_id) |
| `idx_user_blocks_blocked` | user_blocks | (block_user_id, user_id) |
| `idx_user_mutes_user` | user_mutes | (user_id, mute_user_id) |
| `idx_user_mutes_muted` | user_mutes | (mute_user_id, user_id) |
