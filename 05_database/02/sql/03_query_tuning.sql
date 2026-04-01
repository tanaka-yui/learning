-- ============================================================
-- 03_query_tuning.sql: クエリチューニング解説で使用するクエリ例
-- 対象DB: chapter02
-- 注意: 00_schema.sql 実行後に使うこと
-- ============================================================

-- ----------------------------------------------------------------
-- 例1: IN vs EXISTS（集合メンバーシップの確認）
-- ----------------------------------------------------------------
-- IN: サブクエリ結果を全部取得してからマッチング
EXPLAIN ANALYZE
SELECT * FROM posts
WHERE user_id IN (SELECT follow_user_id FROM follows WHERE user_id = 1);

-- EXISTS: 1件見つかったら即終了（大きい集合に強い）
EXPLAIN ANALYZE
SELECT * FROM posts p
WHERE EXISTS (
    SELECT 1 FROM follows f
    WHERE f.user_id = 1 AND f.follow_user_id = p.user_id
);

-- ----------------------------------------------------------------
-- 例2: NOT EXISTS によるブロック/ミュート除外
-- ----------------------------------------------------------------
-- 「自分（user_id=1）がブロックしていない かつ ブロックされていない」投稿
EXPLAIN ANALYZE
SELECT p.*
FROM posts p
WHERE NOT EXISTS (
    -- 自分がブロックした
    SELECT 1 FROM user_blocks b
    WHERE b.user_id = 1 AND b.block_user_id = p.user_id
)
AND NOT EXISTS (
    -- 自分がブロックされた
    SELECT 1 FROM user_blocks b
    WHERE b.user_id = p.user_id AND b.block_user_id = 1
);

-- ----------------------------------------------------------------
-- 例3: CTE（WITH句）の使い方
-- ----------------------------------------------------------------
-- フォロー中ユーザーの投稿を CTE で整理
EXPLAIN ANALYZE
WITH followed_users AS (
    SELECT follow_user_id FROM follows WHERE user_id = 1
)
SELECT p.*
FROM posts p
JOIN followed_users fu ON fu.follow_user_id = p.user_id
ORDER BY p.created_at DESC
LIMIT 20;

-- MATERIALIZED ヒントで強制的に一時テーブル化（副作用の隔離に使う）
EXPLAIN ANALYZE
WITH followed_users AS MATERIALIZED (
    SELECT follow_user_id FROM follows WHERE user_id = 1
)
SELECT p.*
FROM posts p
JOIN followed_users fu ON fu.follow_user_id = p.user_id
ORDER BY p.created_at DESC
LIMIT 20;

-- ----------------------------------------------------------------
-- 例4: 相関サブクエリ vs LATERAL JOIN
-- ----------------------------------------------------------------
-- 遅い: 相関サブクエリ（投稿ごとにサブクエリが実行される）
EXPLAIN ANALYZE
SELECT
    p.id,
    p.content,
    (SELECT COUNT(*) FROM post_favorites pf WHERE pf.post_id = p.id) AS like_count
FROM posts p
WHERE p.user_id = 42;

-- 速い: LATERAL JOIN（集約をJOINとして処理できる場合あり）
EXPLAIN ANALYZE
SELECT p.id, p.content, agg.like_count
FROM posts p
LEFT JOIN LATERAL (
    SELECT COUNT(*) AS like_count
    FROM post_favorites pf
    WHERE pf.post_id = p.id
) agg ON TRUE
WHERE p.user_id = 42;

-- ----------------------------------------------------------------
-- 例5: ページネーション（OFFSET vs キーセット）
-- ----------------------------------------------------------------
-- 遅い: OFFSET（深いページほど大量の行を読み捨てる）
EXPLAIN ANALYZE
SELECT * FROM posts ORDER BY created_at DESC LIMIT 20 OFFSET 10000;

-- 速い: キーセットページネーション（前ページ最後の created_at を使う）
EXPLAIN ANALYZE
SELECT * FROM posts
WHERE created_at < '2025-06-01 00:00:00+09'
ORDER BY created_at DESC
LIMIT 20;
