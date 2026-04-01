-- ============================================================
-- exercise.sql: クエリ最適化演習
-- 対象DB: chapter02
-- 注意: 00_schema.sql + make seed 実行後に使うこと
-- ============================================================

-- ================================================================
-- 演習ユーザーの設定（\gset でUUIDを変数に格納）
-- 以降のクエリでは :'my_id' で参照できます
-- ================================================================
SELECT id AS my_id FROM users ORDER BY id LIMIT 1 \gset

-- ================================================================
-- 演習の出発点クエリ（最適化なし）
-- 各 Step で EXPLAIN ANALYZE を使って問題を発見し、改善していく
-- ================================================================

-- 演習ユーザーがフォローしているユーザーの最新投稿20件
SELECT
    p.id,
    p.content,
    p.created_at,
    u.display_name
FROM posts p
JOIN users u ON u.id = p.user_id
WHERE p.user_id IN (
    SELECT follow_user_id
    FROM follows
    WHERE user_id = :'my_id'
)
ORDER BY p.created_at DESC
LIMIT 20;
