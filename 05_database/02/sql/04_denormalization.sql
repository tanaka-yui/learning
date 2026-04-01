-- ============================================================
-- 04_denormalization.sql: 非正規化パターンの解説例
-- 対象DB: chapter02
-- 注意: 00_schema.sql 実行後に使うこと
-- ============================================================

-- ================================================================
-- パターン1: 冗長FK（post_favorites に post_user_id を持たせる）
-- ================================================================

-- Before: 「誰かが自分の投稿にいいねした」を取得するのにJOINが必要
EXPLAIN ANALYZE
SELECT pf.user_id AS reaction_user, p.user_id AS post_owner
FROM post_favorites pf
JOIN posts p ON p.id = pf.post_id
WHERE p.user_id = 42;

-- 非正規化: post_favorites に post_user_id カラムを追加
ALTER TABLE post_favorites ADD COLUMN IF NOT EXISTS post_user_id INT;

-- 既存データに値を設定
UPDATE post_favorites pf
SET post_user_id = p.user_id
FROM posts p
WHERE p.id = pf.post_id;

-- NOT NULL 制約を追加
ALTER TABLE post_favorites ALTER COLUMN post_user_id SET NOT NULL;

-- インデックスを追加
CREATE INDEX IF NOT EXISTS idx_post_favorites_post_user
    ON post_favorites(post_user_id);

-- After: JOINなしで直接取得できる
EXPLAIN ANALYZE
SELECT user_id AS reaction_user, post_user_id AS post_owner
FROM post_favorites
WHERE post_user_id = 42;

-- ================================================================
-- パターン2: JSON集約カラム（posts に hashtags を持たせる）
-- ================================================================

-- Before: 投稿のタグ一覧取得に2回のJOINが必要
EXPLAIN ANALYZE
SELECT p.id, p.content, array_agg(h.name) AS tags
FROM posts p
LEFT JOIN hashtag_posts hp ON hp.post_id = p.id
LEFT JOIN hashtags h ON h.id = hp.hashtag_id
WHERE p.user_id = 42
GROUP BY p.id, p.content;

-- 非正規化: posts に JSON配列カラムを追加
ALTER TABLE posts ADD COLUMN IF NOT EXISTS hashtags JSONB;

-- 既存データにタグ情報を投入
UPDATE posts p
SET hashtags = (
    SELECT COALESCE(jsonb_agg(h.name), '[]')
    FROM hashtag_posts hp
    JOIN hashtags h ON h.id = hp.hashtag_id
    WHERE hp.post_id = p.id
);

-- GINインデックスを追加（JSON内検索に対応）
CREATE INDEX IF NOT EXISTS idx_posts_hashtags
    ON posts USING GIN (hashtags);

-- After: JOINなしでタグを取得できる
EXPLAIN ANALYZE
SELECT id, content, hashtags FROM posts WHERE user_id = 42;

-- ================================================================
-- パターン3: 集約キャッシュテーブル（post_stats）
-- ================================================================

-- Before: タイムラインのたびにCOUNTサブクエリが実行される
EXPLAIN ANALYZE
SELECT
    p.id, p.content, p.created_at,
    (SELECT COUNT(*) FROM post_favorites pf WHERE pf.post_id = p.id) AS like_count,
    (SELECT COUNT(*) FROM post_replies  pr WHERE pr.post_id = p.id) AS reply_count
FROM posts p
WHERE p.user_id IN (SELECT follow_user_id FROM follows WHERE user_id = 1)
ORDER BY p.created_at DESC
LIMIT 20;

-- 集約キャッシュテーブルを作成
CREATE TABLE IF NOT EXISTS post_stats (
    post_id     INT PRIMARY KEY REFERENCES posts(id),
    like_count  INT NOT NULL DEFAULT 0,
    reply_count INT NOT NULL DEFAULT 0,
    stamp_count INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 現在の集計値を一括投入
INSERT INTO post_stats (post_id, like_count, reply_count, stamp_count)
SELECT
    p.id,
    COUNT(DISTINCT pf.user_id),
    COUNT(DISTINCT pr.id),
    COUNT(DISTINCT ps.user_id)
FROM posts p
LEFT JOIN post_favorites pf ON pf.post_id = p.id
LEFT JOIN post_replies   pr ON pr.post_id = p.id
LEFT JOIN post_stamps    ps ON ps.post_id = p.id
GROUP BY p.id
ON CONFLICT (post_id) DO UPDATE
    SET like_count  = EXCLUDED.like_count,
        reply_count = EXCLUDED.reply_count,
        stamp_count = EXCLUDED.stamp_count,
        updated_at  = NOW();

-- After: post_stats を JOIN するだけで集計値が取得できる
EXPLAIN ANALYZE
SELECT p.id, p.content, p.created_at, ps.like_count, ps.reply_count
FROM posts p
JOIN post_stats ps ON ps.post_id = p.id
WHERE p.user_id IN (SELECT follow_user_id FROM follows WHERE user_id = 1)
ORDER BY p.created_at DESC
LIMIT 20;
