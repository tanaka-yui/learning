-- ============================================================
-- exercise.sql: クエリ最適化演習
-- 対象DB: chapter02
-- 注意: 00_schema.sql 実行後に使うこと
-- ============================================================

-- ================================================================
-- 演習の出発点クエリ（最適化なし）
-- 各 Step で EXPLAIN ANALYZE を使って問題を発見し、改善していく
-- ================================================================

-- ユーザーID=1 がフォローしているユーザーの最新投稿20件
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
    WHERE user_id = 1
)
ORDER BY p.created_at DESC
LIMIT 20;
