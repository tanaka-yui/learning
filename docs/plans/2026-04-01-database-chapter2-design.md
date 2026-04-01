# データベース第2章 設計ドキュメント: SQLクエリ最適化

## 概要

第1章（正規化理論）の続編として、SNSアプリケーションを題材にEXPLAIN、インデックス、クエリチューニング、非正規化を学ぶ実践的な教材。

## 対象者

- 第1章修了者（正規化理論を理解し、基本的なSELECT/JOIN/CREATE TABLEが書ける）

## アプローチ: 二段階型（理論トピック分割 + 段階的演習）

理論ドキュメントはトピックごとに分割して参照しやすくし、演習は1つの段階的ストーリーで全トピックを横断する。

## ファイル構成

```
02/
├── docs/
│   ├── 00_schema.md              # SNSスキーマ設計 + ER図 + データ概要
│   ├── 01_explain.md             # EXPLAIN理論（実行計画の読み方）
│   ├── 02_indexing.md            # インデックス理論
│   ├── 03_query_tuning.md        # クエリチューニング
│   ├── 04_denormalization.md     # 非正規化パターン
│   ├── exercise.md               # 段階的演習（全トピック横断）
│   └── answer.md                 # ステップごとの解答
└── sql/
    ├── 00_schema.sql             # 3NFスキーマ + テストデータ生成
    ├── 01_explain.sql            # EXPLAIN理論の実行例
    ├── 02_indexing.sql           # インデックス理論の実行例
    ├── 03_query_tuning.sql       # クエリチューニングの実行例
    ├── 04_denormalization.sql    # 非正規化の実行例
    ├── exercise.sql              # 演習セットアップ
    └── answer.sql                # 解答SQL
```

## SNSスキーマ（12テーブル、3NF）

| テーブル | 用途 | 主キー |
|---------|------|--------|
| `users` | ユーザープロフィール | id (SERIAL) |
| `posts` | 投稿 | id (SERIAL) |
| `post_replies` | コメント | id (SERIAL) |
| `post_favorites` | いいね | (post_id, user_id) |
| `stamps` | スタンプマスタ | id (SERIAL) |
| `post_stamps` | 投稿へのスタンプ | (post_id, stamp_id, user_id) |
| `follows` | フォロー関係 | (user_id, follow_user_id) |
| `hashtags` | タグマスタ | id (SERIAL) |
| `hashtag_posts` | タグ-投稿紐付け | (hashtag_id, post_id) |
| `hashtag_follows` | タグフォロー | (hashtag_id, user_id) |
| `user_blocks` | ブロック | (user_id, block_user_id) |
| `user_mutes` | ミュート | (user_id, mute_user_id) |

### テストデータ量

`generate_series` + `random()` で生成:
- 1,000ユーザー、100,000投稿、50,000フォロー、200,000いいね、30,000コメント

## 理論ドキュメント内容

### 00_schema.md — SNSスキーマ設計
- 全12テーブルの設計意図とカラム定義
- Mermaid ER図
- テストデータ生成の説明

### 01_explain.md — 実行計画の読み方
- `EXPLAIN` vs `EXPLAIN ANALYZE`
- ノードタイプ: Seq Scan, Index Scan, Index Only Scan, Bitmap Index/Heap Scan
- JOIN戦略: Nested Loop, Hash Join, Merge Join
- コストモデル: startup cost, total cost, rows, width
- 実測値: actual time, loops, buffers
- SNSの`posts`テーブルでの具体例

### 02_indexing.md — インデックス
- B-treeの概念的な仕組み
- 単一カラム vs 複合インデックス（左端プレフィックスルール）
- 部分インデックス（`WHERE deleted_at IS NULL`）
- カバリングインデックス（`INCLUDE`）
- インデックスが逆効果になるケース

### 03_query_tuning.md — クエリチューニング
- `EXISTS` vs `IN` vs `JOIN` の使い分け
- `NOT EXISTS` でブロック/ミュート除外
- CTE（WITH句）の振る舞い（PG12+の自動最適化、`MATERIALIZED`ヒント）
- 相関サブクエリ vs LATERAL JOIN
- キーセットページネーション vs OFFSET

### 04_denormalization.md — 非正規化パターン
- 冗長FK: `post_favorites.post_user_id`
- JSON集約カラム: `posts.hashtags`
- 集約キャッシュテーブル: `post_stats(post_id, like_count, comment_count, stamp_count)`
- トレードオフ分析: 読み取り頻度 vs 書き込み頻度、整合性コスト

## 演習構成（6ステップの段階的改善）

| Step | 学習内容 | タスク |
|------|---------|-------|
| Step 1 | ベースクエリ | フォロー中ユーザーの投稿一覧を取得するSELECTを書く |
| Step 2 | EXPLAIN読解 | Step 1にEXPLAIN ANALYZEをかけ、Seq Scan箇所とコストを特定 |
| Step 3 | インデックス | 適切なインデックスを追加し、Seq Scan→Index Scanの変化を確認 |
| Step 4 | タイムライン拡張 | ブロック/ミュート除外 + タグフォロー投稿追加の完全タイムラインクエリ |
| Step 5 | 複合最適化 | Step 4のEXPLAIN→追加インデックス→CTE書き換え→再EXPLAIN |
| Step 6 | 非正規化 | post_stats追加、hashtags JSON化、最終タイムラインクエリのBefore/After比較 |

### answer.md の各ステップ構成
1. 完成SQL
2. EXPLAIN ANALYZE出力（注釈付き）
3. 「何が改善されたか・なぜか」の解説

## 参考リポジトリ（piamy-backend）から取り入れるパターン

- タイムラインCTEチェーン構造（簡略化版）
- ブロック/ミュートの4方向NOT EXISTS除外
- `post_favorites.post_user_id` の冗長FK
- `posts.hashtags` のJSON集約カラム
- フォローユーザー + フォロータグのUNIONによるフィード構築

## インフラ変更

- `docker-compose.yml`: `./02/sql:/sql/02:ro` ボリュームマウントのコメント解除
- `init/00_init.sh`: chapter02データベース作成セクションのコメント解除、SQLファイル読み込み順序設定
