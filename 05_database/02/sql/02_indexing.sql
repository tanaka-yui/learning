-- ============================================================
-- 02_indexing.sql: インデックス解説で使用するクエリ例
-- 対象DB: chapter02
-- 注意: 00_schema.sql 実行後に使うこと
-- ============================================================

-- ----------------------------------------------------------------
-- 例1: 単一カラムインデックスの効果
-- ----------------------------------------------------------------
-- Before: posts.user_id にインデックスがない状態（Seq Scan が発生する）
EXPLAIN ANALYZE SELECT * FROM posts WHERE user_id = 42;

-- インデックス追加
CREATE INDEX idx_posts_user_id ON posts(user_id);

-- After: Index Scan に変わることを確認
EXPLAIN ANALYZE SELECT * FROM posts WHERE user_id = 42;

-- ----------------------------------------------------------------
-- 例2: 複合インデックスと左端プレフィックスルール
-- ----------------------------------------------------------------
-- follows(user_id, follow_user_id) の複合インデックス
CREATE INDEX idx_follows_user ON follows(user_id, follow_user_id);

-- インデックスが使われる: 左端カラム (user_id) だけの条件
EXPLAIN ANALYZE SELECT * FROM follows WHERE user_id = 1;

-- インデックスが使われる: 両方のカラムの条件
EXPLAIN ANALYZE SELECT * FROM follows WHERE user_id = 1 AND follow_user_id = 42;

-- インデックスが使われない: 右側のカラム (follow_user_id) だけの条件
EXPLAIN ANALYZE SELECT * FROM follows WHERE follow_user_id = 42;

-- ----------------------------------------------------------------
-- 例3: カバリングインデックス（INCLUDE）
-- ----------------------------------------------------------------
-- タイムライン用: user_id で絞り込み、id と created_at だけ SELECT する
DROP INDEX IF EXISTS idx_posts_user_id;
CREATE INDEX idx_posts_user_covering ON posts(user_id) INCLUDE (id, created_at);

-- Index Only Scan になることを確認（テーブルへのアクセスが不要）
EXPLAIN ANALYZE
SELECT id, created_at FROM posts WHERE user_id = 42 ORDER BY created_at DESC;

-- ----------------------------------------------------------------
-- クリーンアップ（理論解説用インデックスを削除）
-- ※ 演習・解答ファイルで改めて最適なインデックスを作成する
-- ----------------------------------------------------------------
DROP INDEX IF EXISTS idx_follows_user;
DROP INDEX IF EXISTS idx_posts_user_covering;
