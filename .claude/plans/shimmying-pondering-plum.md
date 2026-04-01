# 第2章改善: UUID化 + Go シードツール + テストデータ増量

## Context

第2章の全ファイルは実装済み。3つの改善が必要:
1. **データ量不足**: 1K users / 100K posts では EXPLAIN で差が体感できない
2. **FK が不正確**: `random() * N` では実在レコードとの紐づきが不正確
3. **PK が SERIAL**: UUID に変更（Go ツールで UUIDv7 を生成し、PK だけで時系列ソート可能にする）

## データ量

| テーブル | 現在 | 変更後 |
|---------|------|--------|
| users | 1,000 | 10,000 |
| posts | 100,000 | 1,000,000 |
| follows | ~50,000 | ~300,000 |
| post_favorites | ~200,000 | ~1,500,000 |
| post_replies | 30,000 | 200,000 |
| stamps | 10 | 10 |
| post_stamps | ~50,000 | ~500,000 |
| hashtags | 100 | 500 |
| hashtag_posts | ~300,000 | ~2,000,000 |
| hashtag_follows | ~10,000 | ~50,000 |
| user_blocks | ~5,000 | ~30,000 |
| user_mutes | ~5,000 | ~30,000 |

## UUID + UUIDv7 設計

### スキーマ側
- 全 5 テーブルの PK: `SERIAL` → `UUID PRIMARY KEY DEFAULT gen_random_uuid()`（PG17 ネイティブ）
- 全 18 FK カラム: `INT` → `UUID`

### Go シードツール側
- `github.com/google/uuid` の `uuid.NewV7()` で UUIDv7 を生成
- UUIDv7 は時刻情報を含むため、Go 側で `created_at` と同期した UUIDv7 を生成 → `ORDER BY id` ≒ `ORDER BY created_at`
- PostgreSQL スキーマの DEFAULT は `gen_random_uuid()`（v4）だが、シードツールが UUIDv7 を明示的に投入

### UUIDv7 の教材的メリット（docs に記載）
- `ORDER BY id DESC` で時系列ソート可能
- キーセットページネーションで `WHERE id < :'last_id'` が使える
- 分散システムでの ID 衝突回避

### ハードコード ID の対処
`user_id = 1` / `user_id = 42` → psql `\gset` で変数化:
```sql
SELECT id AS my_id FROM users ORDER BY id LIMIT 1 \gset
SELECT id AS other_id FROM users ORDER BY id LIMIT 1 OFFSET 41 \gset
-- 以降 :'my_id' / :'other_id' で参照
```

## Go シードツール

### 構成
```
05_database/02/tools/seed/
├── Dockerfile
├── main.go
├── go.mod
└── go.sum
```

### Docker化
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /seed .

FROM alpine:3.21
COPY --from=builder /seed /seed
ENTRYPOINT ["/seed"]
```

docker-compose.yml にサービス追加（`profiles: [tools]` で通常起動時は除外）:
```yaml
seed:
  build: ./02/tools/seed
  depends_on:
    postgres:
      condition: service_healthy
  environment:
    DATABASE_URL: postgres://learning:learning@postgres:5432/chapter02
  profiles:
    - tools
```

### 依存
- `github.com/google/uuid` — UUIDv7 生成
- `github.com/jackc/pgx/v5` — PostgreSQL ドライバ（`CopyFrom` で高速バルクインサート）

### 生成ロジック（依存順）
1. **users** (10K) — UUIDv7 生成、`[]uuid.UUID` にID保持
2. **stamps** (10) — 固定名リスト、ID保持
3. **hashtags** (500) — ID保持
4. **posts** (1M) — `userIDs[rand.Intn(len)]` で実在ユーザー参照、UUIDv7 は `created_at` と同期、ID保持
5. **follows** (300K) — ユーザーペア生成、`u1 != u2` 保証、重複排除
6. **post_favorites** (1.5M) — `postIDs[rand.Intn(len)]` + `userIDs[rand.Intn(len)]` で実在レコード参照
7. **post_replies** (200K) — 同上
8. **post_stamps** (500K) — postID + stampID + userID すべて実在参照
9. **hashtag_posts** (2M) — hashtagID + postID 実在参照
10. **hashtag_follows** (50K) — hashtagID + userID 実在参照
11. **user_blocks** (30K) — ユーザーペア、`u1 != u2` 保証
12. **user_mutes** (30K) — 同上
13. **ANALYZE** 実行

### パフォーマンス
- `pgx.CopyFrom` でバルクインサート（1M 行でも数十秒）
- 重複排除: Go 側で `map` を使い ON CONFLICT 前に除去
- メモリ: postIDs 1M × 16bytes = 16MB、十分許容範囲

### 接続情報
デフォルト: `postgres://learning:learning@localhost:5432/chapter02`
環境変数 `DATABASE_URL` で上書き可能

## init/00_init.sh 変更

```bash
# chapter02: DDL のみ（データは Go シードツールで投入）
psql ... -c "CREATE DATABASE chapter02;"
psql ... -d chapter02 -f /sql/02/00_schema.sql   # DDL のみ
```

理論 SQL（01〜04）と exercise.sql は学習者が手動で実行するもの。init からは除外。

## Makefile

`05_database/Makefile` を新規作成:

```makefile
.PHONY: up down reset seed psql

up:                              ## DB起動 + DDL
	docker compose up -d

down:                            ## DB停止
	docker compose down

reset:                           ## データ全削除 + 再起動 + シード
	docker compose down -v
	docker compose up -d
	@echo "Waiting for PostgreSQL..."
	@sleep 3
	docker compose run --rm seed

seed:                            ## テストデータ投入
	docker compose run --rm seed

psql:                            ## chapter02 に接続
	docker compose exec postgres psql -U learning -d chapter02
```

## セットアップフロー

```
1. cd 05_database
2. make up      # DB起動 + DDL
3. make seed    # テストデータ投入（Docker で Go ツール実行、数分）
4. make psql    # 演習開始
```

リセットしたいとき: `make reset`

## 変更対象ファイル

### 新規（5 ファイル）
- `05_database/02/tools/seed/Dockerfile`
- `05_database/02/tools/seed/main.go`
- `05_database/02/tools/seed/go.mod`
- `05_database/02/tools/seed/go.sum`
- `05_database/Makefile`

### SQL（7 ファイル）

**`00_schema.sql`** — DDL のみに縮小
- テストデータ INSERT を全削除
- 全 PK を `UUID DEFAULT gen_random_uuid()` に変更
- 全 FK を `UUID` 型に変更

**`01_explain.sql`** — `\gset` + UUID 対応
- 冒頭に `\gset` で `demo_user` 設定
- `user_id = 42` → `:'demo_user'`（5箇所）

**`02_indexing.sql`** — `\gset` + UUID 対応
- `user_id = 42` → `:'demo_user'`、`user_id = 1` → `:'my_id'`（6箇所）

**`03_query_tuning.sql`** — `\gset` + UUID 対応
- `user_id = 1` → `:'my_id'`（8箇所）、`user_id = 42` → `:'demo_user'`（2箇所）
- キーセットページネーション: UUIDv7 `WHERE id < :'last_id'` 例を追加

**`04_denormalization.sql`** — UUID 型対応
- `post_user_id` → `UUID`、`post_stats.post_id` → `UUID`
- `user_id = 42` → `:'demo_user'`、`user_id = 1` → `:'my_id'`

**`exercise.sql`** — `\gset` 導入
- 冒頭に `\gset` + 使い方コメント
- `user_id = 1` → `:'my_id'`

**`answer.sql`** — `\gset` 導入
- `user_id = 1` → `:'my_id'`（22箇所）
- `block_user_id = 1` / `mute_user_id = 1` → `:'my_id'`（各3箇所）

### ドキュメント（7 ファイル）

**`00_schema.md`** — UUID セクション追加 + 件数更新
- UUIDv7 メリット解説セクション追加
- テーブル定義の型更新（SERIAL→UUID、INT FK→UUID）
- テストデータ概要テーブル件数更新
- サンプルデータ ID を UUID 形式に

**`01_explain.md`** — 件数更新（100,000→1,000,000）3箇所

**`02_indexing.md`** — 件数更新（50,000→300,000、100,000→1,000,000）

**`03_query_tuning.md`** — UUIDv7 ページネーション例追加

**`04_denormalization.md`** — ID 記述更新

**`exercise.md`** — `\gset` 使い方追加、件数更新

**`answer.md`** — 件数更新、SQL の UUID 対応

### インフラ（3 ファイル）

**`docker-compose.yml`** — seed サービス追加（profiles: tools）

**`init/00_init.sh`** — chapter02 を DDL のみに簡略化

**`docs/plans/2026-04-01-database-chapter2-design.md`** — UUID + データ量更新

## 実装順序

1. `00_schema.sql` を DDL のみに書き換え（UUID 化）
2. Go シードツール作成（`Dockerfile` + `main.go` + `go.mod`）
3. `docker-compose.yml` に seed サービス追加
4. `Makefile` 作成
5. `init/00_init.sh` を DDL のみに修正
6. `04_denormalization.sql` UUID 型対応
7. `01_explain.sql` → `02_indexing.sql` → `03_query_tuning.sql` — `\gset` + UUID
8. `exercise.sql` + `answer.sql` — `\gset` + UUID
9. docs 全 7 ファイル更新
10. 設計ドキュメント更新

## 検証
1. `cd 05_database && make reset` → DB起動 + シードデータ投入完了
2. `make psql`
3. `SELECT count(*) FROM posts;` → 1,000,000
4. `SELECT id, created_at FROM posts ORDER BY id LIMIT 5;` → ID 順 ≒ 時系列順
5. 演習 Step 1〜6 が `\gset` で正常動作
6. `EXPLAIN ANALYZE` で Seq Scan が明確に遅い（数百ms〜）
