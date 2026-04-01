-- ============================================================
-- answer.sql: クエリ最適化演習 解答SQL
-- 対象DB: chapter02
-- 注意: 00_schema.sql と 04_denormalization.sql 実行後に使うこと
-- ============================================================

-- ================================================================
-- Step 3: インデックスを追加して改善する
-- ================================================================

-- follows テーブル: user_id でフォロー先を素早く引く
CREATE INDEX IF NOT EXISTS idx_follows_user_id
    ON follows(user_id);

-- posts テーブル: user_id で絞り込み + created_at DESC でソート
CREATE INDEX IF NOT EXISTS idx_posts_user_created
    ON posts(user_id, created_at DESC);

-- Step 3: 改善後のクエリ（EXISTS を使った書き換え）
EXPLAIN ANALYZE
SELECT p.id, p.content, p.created_at, u.display_name
FROM posts p
JOIN users u ON u.id = p.user_id
WHERE EXISTS (
    SELECT 1 FROM follows f
    WHERE f.user_id = 1 AND f.follow_user_id = p.user_id
)
ORDER BY p.created_at DESC
LIMIT 20;

-- ================================================================
-- Step 4: 完全タイムラインクエリ
-- ================================================================
WITH
    -- フォロー中ユーザーの投稿
    user_feed AS (
        SELECT p.id, p.content, p.created_at, p.user_id
        FROM posts p
        WHERE EXISTS (
            SELECT 1 FROM follows f
            WHERE f.user_id = 1 AND f.follow_user_id = p.user_id
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
            WHERE hp.post_id = p.id AND hf.user_id = 1
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
    SELECT 1 FROM user_blocks b WHERE b.user_id = 1 AND b.block_user_id = f.user_id
)
AND NOT EXISTS (
    SELECT 1 FROM user_blocks b WHERE b.user_id = f.user_id AND b.block_user_id = 1
)
AND NOT EXISTS (
    SELECT 1 FROM user_mutes m WHERE m.user_id = 1 AND m.mute_user_id = f.user_id
)
AND NOT EXISTS (
    SELECT 1 FROM user_mutes m WHERE m.user_id = f.user_id AND m.mute_user_id = 1
)
ORDER BY f.created_at DESC
LIMIT 20;

-- ================================================================
-- Step 5: 追加インデックス
-- ================================================================

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

-- Step 5: インデックス追加後のタイムラインクエリ（Step 4 と同一、確認用）
EXPLAIN ANALYZE
WITH
    user_feed AS (
        SELECT p.id, p.content, p.created_at, p.user_id
        FROM posts p
        WHERE EXISTS (
            SELECT 1 FROM follows f
            WHERE f.user_id = 1 AND f.follow_user_id = p.user_id
        )
    ),
    tag_feed AS (
        SELECT p.id, p.content, p.created_at, p.user_id
        FROM posts p
        WHERE EXISTS (
            SELECT 1
            FROM hashtag_posts hp
            JOIN hashtag_follows hf ON hf.hashtag_id = hp.hashtag_id
            WHERE hp.post_id = p.id AND hf.user_id = 1
        )
    ),
    feed AS (
        SELECT * FROM user_feed
        UNION
        SELECT * FROM tag_feed
    )
SELECT f.id, f.content, f.created_at, u.display_name
FROM feed f
JOIN users u ON u.id = f.user_id
WHERE NOT EXISTS (
    SELECT 1 FROM user_blocks b WHERE b.user_id = 1 AND b.block_user_id = f.user_id
)
AND NOT EXISTS (
    SELECT 1 FROM user_blocks b WHERE b.user_id = f.user_id AND b.block_user_id = 1
)
AND NOT EXISTS (
    SELECT 1 FROM user_mutes m WHERE m.user_id = 1 AND m.mute_user_id = f.user_id
)
AND NOT EXISTS (
    SELECT 1 FROM user_mutes m WHERE m.user_id = f.user_id AND m.mute_user_id = 1
)
ORDER BY f.created_at DESC
LIMIT 20;

-- ================================================================
-- Step 6: 非正規化を活用した最終版タイムライン
-- ================================================================
-- post_stats と posts.hashtags は 04_denormalization.sql で作成済み

EXPLAIN ANALYZE
WITH
    user_feed AS (
        SELECT p.id, p.content, p.created_at, p.user_id, p.hashtags
        FROM posts p
        WHERE EXISTS (
            SELECT 1 FROM follows f
            WHERE f.user_id = 1 AND f.follow_user_id = p.user_id
        )
    ),
    tag_feed AS (
        SELECT p.id, p.content, p.created_at, p.user_id, p.hashtags
        FROM posts p
        WHERE EXISTS (
            SELECT 1
            FROM hashtag_posts hp
            JOIN hashtag_follows hf ON hf.hashtag_id = hp.hashtag_id
            WHERE hp.post_id = p.id AND hf.user_id = 1
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
    SELECT 1 FROM user_blocks b WHERE b.user_id = 1 AND b.block_user_id = f.user_id
)
AND NOT EXISTS (
    SELECT 1 FROM user_blocks b WHERE b.user_id = f.user_id AND b.block_user_id = 1
)
AND NOT EXISTS (
    SELECT 1 FROM user_mutes m WHERE m.user_id = 1 AND m.mute_user_id = f.user_id
)
AND NOT EXISTS (
    SELECT 1 FROM user_mutes m WHERE m.user_id = f.user_id AND m.mute_user_id = 1
)
ORDER BY f.created_at DESC
LIMIT 20;
