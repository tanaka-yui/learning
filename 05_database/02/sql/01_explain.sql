-- ============================================================
-- 01_explain.sql: EXPLAIN 解説で使用するクエリ例
-- 対象DB: chapter02
-- 注意: 00_schema.sql 実行後に使うこと
-- ============================================================

-- ----------------------------------------------------------------
-- 使用するユーザーID（UUID）を変数に設定
-- ----------------------------------------------------------------
SELECT id AS demo_user FROM users ORDER BY id LIMIT 1 OFFSET 41 \gset

-- ----------------------------------------------------------------
-- 例1: Seq Scan（インデックスなし）
-- ----------------------------------------------------------------
-- posts テーブル全件スキャンが発生する例
EXPLAIN ANALYZE
SELECT * FROM posts WHERE user_id = :'demo_user';

-- ----------------------------------------------------------------
-- 例2: Index Scan（インデックスがある場合の比較用）
-- ----------------------------------------------------------------
-- 以下を先に実行してからもう一度 EXPLAIN ANALYZE すると差がわかる
-- CREATE INDEX idx_posts_user_id ON posts(user_id);
EXPLAIN ANALYZE
SELECT * FROM posts WHERE user_id = :'demo_user';

-- ----------------------------------------------------------------
-- 例3: Nested Loop（単一ユーザーのJOIN）
-- ----------------------------------------------------------------
EXPLAIN ANALYZE
SELECT u.display_name, p.content
FROM users u
JOIN posts p ON p.user_id = u.id
WHERE u.id = :'demo_user';

-- ----------------------------------------------------------------
-- 例4: Hash Join（大きいテーブルの集計JOIN）
-- ----------------------------------------------------------------
EXPLAIN ANALYZE
SELECT u.display_name, COUNT(p.id) AS post_count
FROM users u
JOIN posts p ON p.user_id = u.id
GROUP BY u.id, u.display_name;

-- ----------------------------------------------------------------
-- 例5: Sort + Limit（ORDER BY + LIMIT）
-- ----------------------------------------------------------------
EXPLAIN ANALYZE
SELECT * FROM posts ORDER BY created_at DESC LIMIT 20;
