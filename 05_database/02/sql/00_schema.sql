-- ============================================================
-- 00_schema.sql: SNS アプリケーションのスキーマ + テストデータ
-- 対象DB: chapter02
--
-- テーブル構成（3NF）:
--   users, posts, post_replies, post_favorites,
--   stamps, post_stamps,
--   follows, hashtags, hashtag_posts, hashtag_follows,
--   user_blocks, user_mutes
--
-- データ量（EXPLAIN の差が出るよう大量に投入）:
--   users: 1,000件
--   posts: 100,000件
--   follows: 50,000件
--   post_favorites: 200,000件
--   post_replies: 30,000件
--   hashtags: 100件
--   hashtag_posts: 300,000件
--   hashtag_follows: 10,000件
--   user_blocks: 5,000件
--   user_mutes: 5,000件
-- ============================================================

-- ============================================================
-- テーブル定義
-- ============================================================

DROP TABLE IF EXISTS user_mutes       CASCADE;
DROP TABLE IF EXISTS user_blocks      CASCADE;
DROP TABLE IF EXISTS hashtag_follows  CASCADE;
DROP TABLE IF EXISTS hashtag_posts    CASCADE;
DROP TABLE IF EXISTS hashtags         CASCADE;
DROP TABLE IF EXISTS post_stamps      CASCADE;
DROP TABLE IF EXISTS stamps           CASCADE;
DROP TABLE IF EXISTS post_favorites   CASCADE;
DROP TABLE IF EXISTS post_replies     CASCADE;
DROP TABLE IF EXISTS posts            CASCADE;
DROP TABLE IF EXISTS follows          CASCADE;
DROP TABLE IF EXISTS users            CASCADE;

-- ユーザー
CREATE TABLE users (
    id           SERIAL PRIMARY KEY,
    display_name VARCHAR(100) NOT NULL,
    bio          TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- フォロー関係
CREATE TABLE follows (
    user_id        INT NOT NULL REFERENCES users(id),
    follow_user_id INT NOT NULL REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, follow_user_id),
    CHECK (user_id <> follow_user_id)
);

-- 投稿
CREATE TABLE posts (
    id         SERIAL PRIMARY KEY,
    user_id    INT NOT NULL REFERENCES users(id),
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- コメント（投稿へのリプライ）
CREATE TABLE post_replies (
    id         SERIAL PRIMARY KEY,
    post_id    INT NOT NULL REFERENCES posts(id),
    user_id    INT NOT NULL REFERENCES users(id),
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- いいね
CREATE TABLE post_favorites (
    post_id    INT NOT NULL REFERENCES posts(id),
    user_id    INT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (post_id, user_id)
);

-- スタンプマスタ
CREATE TABLE stamps (
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 投稿へのスタンプ
CREATE TABLE post_stamps (
    post_id    INT NOT NULL REFERENCES posts(id),
    stamp_id   INT NOT NULL REFERENCES stamps(id),
    user_id    INT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (post_id, stamp_id, user_id)
);

-- タグマスタ
CREATE TABLE hashtags (
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 投稿-タグ紐付け
CREATE TABLE hashtag_posts (
    hashtag_id INT NOT NULL REFERENCES hashtags(id),
    post_id    INT NOT NULL REFERENCES posts(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (hashtag_id, post_id)
);

-- タグフォロー
CREATE TABLE hashtag_follows (
    hashtag_id INT NOT NULL REFERENCES hashtags(id),
    user_id    INT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (hashtag_id, user_id)
);

-- ブロック
CREATE TABLE user_blocks (
    user_id       INT NOT NULL REFERENCES users(id),
    block_user_id INT NOT NULL REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, block_user_id),
    CHECK (user_id <> block_user_id)
);

-- ミュート
CREATE TABLE user_mutes (
    user_id      INT NOT NULL REFERENCES users(id),
    mute_user_id INT NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, mute_user_id),
    CHECK (user_id <> mute_user_id)
);

-- ============================================================
-- テストデータ生成
-- ============================================================

-- ユーザー: 1,000件
INSERT INTO users (display_name, bio, created_at)
SELECT
    'ユーザー' || i,
    'ユーザー' || i || 'のプロフィールです。',
    NOW() - (random() * INTERVAL '365 days')
FROM generate_series(1, 1000) AS i;

-- スタンプマスタ: 10件
INSERT INTO stamps (name) VALUES
    ('いいね'), ('最高'), ('笑える'), ('驚き'), ('悲しい'),
    ('怒り'), ('応援'), ('感謝'), ('好き'), ('拍手');

-- フォロー: 50,000件（1ユーザーあたり平均50フォロー）
INSERT INTO follows (user_id, follow_user_id, created_at)
SELECT DISTINCT u1, u2, NOW() - (random() * INTERVAL '300 days')
FROM (
    SELECT
        (random() * 999 + 1)::INT AS u1,
        (random() * 999 + 1)::INT AS u2
    FROM generate_series(1, 60000)   -- 重複除去のため多めに生成
) t
WHERE t.u1 <> t.u2
ON CONFLICT DO NOTHING;

-- 投稿: 100,000件
INSERT INTO posts (user_id, content, created_at)
SELECT
    (random() * 999 + 1)::INT,
    '投稿内容 ' || i || ': ' || md5(i::TEXT),
    NOW() - (random() * INTERVAL '180 days')
FROM generate_series(1, 100000) AS i;

-- コメント: 30,000件
INSERT INTO post_replies (post_id, user_id, content, created_at)
SELECT
    (random() * 99999 + 1)::INT,
    (random() * 999 + 1)::INT,
    'コメント ' || i,
    NOW() - (random() * INTERVAL '180 days')
FROM generate_series(1, 30000) AS i;

-- いいね: 200,000件
INSERT INTO post_favorites (post_id, user_id, created_at)
SELECT DISTINCT
    (random() * 99999 + 1)::INT,
    (random() * 999 + 1)::INT,
    NOW() - (random() * INTERVAL '180 days')
FROM generate_series(1, 250000)
ON CONFLICT DO NOTHING;

-- スタンプ: 50,000件
INSERT INTO post_stamps (post_id, stamp_id, user_id, created_at)
SELECT DISTINCT
    (random() * 99999 + 1)::INT,
    (random() * 9 + 1)::INT,
    (random() * 999 + 1)::INT,
    NOW() - (random() * INTERVAL '180 days')
FROM generate_series(1, 70000)
ON CONFLICT DO NOTHING;

-- タグ: 100件
INSERT INTO hashtags (name)
SELECT 'タグ' || i FROM generate_series(1, 100) AS i;

-- タグ-投稿紐付け: 300,000件（1投稿あたり平均3タグ）
INSERT INTO hashtag_posts (hashtag_id, post_id, created_at)
SELECT DISTINCT
    (random() * 99 + 1)::INT,
    (random() * 99999 + 1)::INT,
    NOW() - (random() * INTERVAL '180 days')
FROM generate_series(1, 400000)
ON CONFLICT DO NOTHING;

-- タグフォロー: 10,000件
INSERT INTO hashtag_follows (hashtag_id, user_id, created_at)
SELECT DISTINCT
    (random() * 99 + 1)::INT,
    (random() * 999 + 1)::INT,
    NOW() - (random() * INTERVAL '300 days')
FROM generate_series(1, 15000)
ON CONFLICT DO NOTHING;

-- ブロック: 5,000件
INSERT INTO user_blocks (user_id, block_user_id, created_at)
SELECT DISTINCT u1, u2, NOW() - (random() * INTERVAL '300 days')
FROM (
    SELECT
        (random() * 999 + 1)::INT AS u1,
        (random() * 999 + 1)::INT AS u2
    FROM generate_series(1, 7000)
) t
WHERE t.u1 <> t.u2
ON CONFLICT DO NOTHING;

-- ミュート: 5,000件
INSERT INTO user_mutes (user_id, mute_user_id, created_at)
SELECT DISTINCT u1, u2, NOW() - (random() * INTERVAL '300 days')
FROM (
    SELECT
        (random() * 999 + 1)::INT AS u1,
        (random() * 999 + 1)::INT AS u2
    FROM generate_series(1, 7000)
) t
WHERE t.u1 <> t.u2
ON CONFLICT DO NOTHING;

-- 統計情報を最新化（EXPLAIN の推定を正確にする）
ANALYZE;
