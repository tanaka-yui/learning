-- ============================================================
-- 00_schema.sql: SNS アプリケーションのスキーマ定義（DDL のみ）
-- 対象DB: chapter02
--
-- テーブル構成（3NF）:
--   users, posts, post_replies, post_favorites,
--   stamps, post_stamps,
--   follows, hashtags, hashtag_posts, hashtag_follows,
--   user_blocks, user_mutes
--
-- NOTE: テストデータの投入は 02/tools/seed Go ツールで行う。
--       このファイルはスキーマ定義のみを含む。
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
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name VARCHAR(100) NOT NULL,
    bio          TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- フォロー関係
CREATE TABLE follows (
    user_id        UUID NOT NULL REFERENCES users(id),
    follow_user_id UUID NOT NULL REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, follow_user_id),
    CHECK (user_id <> follow_user_id)
);

-- 投稿
CREATE TABLE posts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- コメント（投稿へのリプライ）
CREATE TABLE post_replies (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id    UUID NOT NULL REFERENCES posts(id),
    user_id    UUID NOT NULL REFERENCES users(id),
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- いいね
CREATE TABLE post_favorites (
    post_id    UUID NOT NULL REFERENCES posts(id),
    user_id    UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (post_id, user_id)
);

-- スタンプマスタ
CREATE TABLE stamps (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 投稿へのスタンプ
CREATE TABLE post_stamps (
    post_id    UUID NOT NULL REFERENCES posts(id),
    stamp_id   UUID NOT NULL REFERENCES stamps(id),
    user_id    UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (post_id, stamp_id, user_id)
);

-- タグマスタ
CREATE TABLE hashtags (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 投稿-タグ紐付け
CREATE TABLE hashtag_posts (
    hashtag_id UUID NOT NULL REFERENCES hashtags(id),
    post_id    UUID NOT NULL REFERENCES posts(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (hashtag_id, post_id)
);

-- タグフォロー
CREATE TABLE hashtag_follows (
    hashtag_id UUID NOT NULL REFERENCES hashtags(id),
    user_id    UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (hashtag_id, user_id)
);

-- ブロック
CREATE TABLE user_blocks (
    user_id       UUID NOT NULL REFERENCES users(id),
    block_user_id UUID NOT NULL REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, block_user_id),
    CHECK (user_id <> block_user_id)
);

-- ミュート
CREATE TABLE user_mutes (
    user_id      UUID NOT NULL REFERENCES users(id),
    mute_user_id UUID NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, mute_user_id),
    CHECK (user_id <> mute_user_id)
);

-- 統計情報を最新化（EXPLAIN の推定を正確にする）
-- NOTE: seed ツールでデータを投入した後に実行すること
ANALYZE;
