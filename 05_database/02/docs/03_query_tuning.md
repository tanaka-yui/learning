# クエリチューニング

## 1. EXISTS vs IN vs JOIN

### 動作の違い

それぞれの演算子は内部的に異なる処理を行い、得意なケースが異なります。

| 演算子 | 動作 | 向いているケース |
|---|---|---|
| EXISTS | 1件見つかったら即終了 | 「存在するか？」の確認。大きい集合 |
| IN | サブクエリ全件を取得してリスト比較 | 小さい確定リスト（数十件以下） |
| JOIN | 全件突合 | 集計や、JOINした側のカラムも必要な場合 |

### EXISTS

**1件マッチした時点でサブクエリを打ち切る**ため、大きい集合の存在確認に強い。サブクエリ内は `SELECT 1` で十分（戻り値は使わないため）。

```sql
-- フォロー中のユーザーの投稿を取得（EXISTS）
SELECT * FROM posts p
WHERE EXISTS (
    SELECT 1 FROM follows f
    WHERE f.user_id = 1 AND f.follow_user_id = p.user_id
);
```

### IN

サブクエリの**全件を取得してリストを構築**し、そのリストと比較する。件数が少ない確定リストに向く。

```sql
-- フォロー中のユーザーの投稿を取得（IN）
SELECT * FROM posts
WHERE user_id IN (SELECT follow_user_id FROM follows WHERE user_id = 1);
```

### JOIN

**全件を突合**するため、集計（`COUNT`、`SUM` など）やJOINした側のカラムも取得したい場合に使う。

```sql
-- フォロー中のユーザーの投稿を取得（JOIN）
SELECT p.*
FROM posts p
JOIN follows f ON f.follow_user_id = p.user_id
WHERE f.user_id = 1;
```

### 使い分けの指針

- **「〜が存在するか」の確認** → EXISTS
- **固定の小さいリストと照合** → IN（例: `WHERE status IN ('active', 'pending')`）
- **結合先のカラムも必要・集計が必要** → JOIN

---

## 2. NOT EXISTS によるブロック/ミュート除外

### SNS での典型パターン

タイムラインを表示する際、ブロックやミュートしているユーザーの投稿を除外する必要があります。実際のSNSアプリでは以下の4条件をすべて満たす投稿のみ表示します。

```sql
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
)
AND NOT EXISTS (
    -- 自分がミュートした
    SELECT 1 FROM user_mutes m
    WHERE m.user_id = 1 AND m.mute_user_id = p.user_id
)
AND NOT EXISTS (
    -- 自分がミュートされた
    SELECT 1 FROM user_mutes m
    WHERE m.user_id = p.user_id AND m.mute_user_id = 1
);
```

### NOT EXISTS vs LEFT JOIN IS NULL

同じ結果を `LEFT JOIN ... IS NULL` でも書けますが、NOT EXISTS の方が多くの場合で速い。

```sql
-- LEFT JOIN IS NULL による除外（参考）
SELECT p.*
FROM posts p
LEFT JOIN user_blocks b1 ON b1.user_id = 1      AND b1.block_user_id = p.user_id
LEFT JOIN user_blocks b2 ON b2.user_id = p.user_id AND b2.block_user_id = 1
WHERE b1.user_id IS NULL
  AND b2.user_id IS NULL;
```

| 方式 | 動作 | 特徴 |
|---|---|---|
| NOT EXISTS | 条件に合う行が1件見つかった時点で即終了（短絡評価） | 除外対象が多いほど速くなりやすい |
| LEFT JOIN IS NULL | 全行をJOINしてからNULLフィルタリング | 結合結果が大きくなりやすい |

NOT EXISTS は**短絡評価**（short-circuit evaluation）が働くため、ブロック・ミュートの除外のように「マッチしたら即除外」という用途に適しています。

---

## 3. CTE（WITH句）

### 基本的な使い方

CTE（Common Table Expression）は `WITH` 句で定義する名前付きサブクエリです。クエリの可読性向上や、同じサブクエリを複数回参照する場合の重複排除に役立ちます。

```sql
WITH followed_users AS (
    SELECT follow_user_id FROM follows WHERE user_id = 1
)
SELECT p.*
FROM posts p
JOIN followed_users fu ON fu.follow_user_id = p.user_id
ORDER BY p.created_at DESC
LIMIT 20;
```

### PostgreSQL 12 以降の振る舞い

| バージョン | デフォルト動作 |
|---|---|
| PostgreSQL 11 以前 | CTE は常に一時テーブルとして具体化（MATERIALIZED） |
| PostgreSQL 12 以降 | CTE はデフォルトでインライン展開（NOT MATERIALIZED）。最適化器が貫通して最適化できる |

PostgreSQL 12 以降では、オプティマイザが CTE をインライン展開し、メインクエリと合わせて最適なプランを選択します。

### MATERIALIZED ヒント

`MATERIALIZED` を指定すると、CTE を強制的に一時テーブルとして具体化します。

```sql
-- MATERIALIZED: 一時テーブルとして具体化
WITH followed_users AS MATERIALIZED (
    SELECT follow_user_id FROM follows WHERE user_id = 1
)
SELECT p.*
FROM posts p
JOIN followed_users fu ON fu.follow_user_id = p.user_id
ORDER BY p.created_at DESC
LIMIT 20;
```

**MATERIALIZED を使うケース:**

- CTE を複数箇所で参照するとき、同じサブクエリを1回だけ実行させたい
- 副作用のある処理（`INSERT ... RETURNING` など）を1度だけ実行したい
- オプティマイザのインライン展開を抑制してプランを安定させたい

### NOT MATERIALIZED ヒント

`NOT MATERIALIZED` を指定すると、明示的にインライン展開を指定できます（PostgreSQL 12 以降のデフォルト動作と同じ）。チームのコードベースで意図を明示したい場合に使います。

```sql
WITH followed_users AS NOT MATERIALIZED (
    SELECT follow_user_id FROM follows WHERE user_id = 1
)
SELECT p.*
FROM posts p
JOIN followed_users fu ON fu.follow_user_id = p.user_id
ORDER BY p.created_at DESC
LIMIT 20;
```

---

## 4. 相関サブクエリ vs LATERAL JOIN

### 相関サブクエリ

外側のクエリの各行に対して、サブクエリが**1回ずつ実行**されます。投稿数がN件あればサブクエリもN回実行されるため、大きい結果セットでは遅くなります。

```sql
-- 相関サブクエリ: 投稿ごとにいいね数を取得
SELECT
    p.id,
    p.content,
    (SELECT COUNT(*) FROM post_favorites pf WHERE pf.post_id = p.id) AS like_count
FROM posts p
WHERE p.user_id = 42;
```

**処理イメージ:** 投稿数 N × サブクエリ実行 → N回のサブクエリ

### LATERAL JOIN

`LATERAL` を使うと、サブクエリを JOIN として処理できます。オプティマイザが集約処理を最適化できる可能性があります。

```sql
-- LATERAL JOIN: いいね数をJOINとして集約
SELECT p.id, p.content, agg.like_count
FROM posts p
LEFT JOIN LATERAL (
    SELECT COUNT(*) AS like_count
    FROM post_favorites pf
    WHERE pf.post_id = p.id
) agg ON TRUE
WHERE p.user_id = 42;
```

### 使い分けの指針

| ケース | 推奨 |
|---|---|
| クエリ全体で1回だけ実行（SELECT句の単純な値取得） | 相関サブクエリでも問題ない |
| 大きい結果セットの各行に対して集約が必要 | LATERAL JOIN を検討する |
| サブクエリ結果の複数カラムを参照したい | LATERAL JOIN（複数カラムを返せる） |

> **実務の指針**: まず `EXPLAIN ANALYZE` で実際のプランを確認する。件数が少なければ相関サブクエリで十分なことも多い。

---

## 5. ページネーション

### OFFSET 方式

最もシンプルな実装ですが、**0件目からOFFSET件目まで読み込んでから捨てる**という処理になります。ページが深くなるほど読み捨てる行数が増え、遅くなります。

```sql
-- OFFSET ページネーション（20件ずつ、500ページ目）
SELECT * FROM posts ORDER BY created_at DESC LIMIT 20 OFFSET 10000;
```

### キーセットページネーション

前ページの最後のレコードの値（カーソル）を使って `WHERE` 句で絞り込む方式です。**毎回インデックスの該当箇所から直接開始**するため、ページが深くなっても速度が変わりません。

```sql
-- キーセットページネーション（前ページ最後の created_at を使う）
SELECT * FROM posts
WHERE created_at < '2025-06-01 00:00:00+09'
ORDER BY created_at DESC
LIMIT 20;
```

### パフォーマンス比較

| 方式 | 1ページ目 | 500ページ目 | カーソル |
|---|---|---|---|
| OFFSET | 速い | 非常に遅い（10,000行読み捨て） | ページ番号 |
| キーセット | 速い | 速い（常にインデックスから） | 前ページ最後のID/日時 |

### キーセットの制限事項

キーセットページネーションは高速ですが、以下の制限があります。

- **前後のページのみ移動可能**: 「5ページ目にジャンプ」といったランダムなページへのジャンプが難しい
- **カーソル値の管理が必要**: クライアント側で前ページ最後のID・日時を保持する必要がある
- **ソート順の制約**: カーソルに使うカラムにインデックスが必要

> **実務の指針**: SNSのタイムライン・無限スクロールなど「前後にめくる」UIにはキーセット方式が適している。管理画面など「特定ページにジャンプしたい」UIではOFFSET方式も選択肢になる（ただしデータ量に注意）。
