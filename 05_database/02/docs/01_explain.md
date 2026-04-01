# EXPLAIN によるクエリ実行計画の読み方

## 1. EXPLAIN と EXPLAIN ANALYZE の違い

PostgreSQL では、クエリを実際に実行する前に**オプティマイザがどのような実行計画を選ぶか**を確認できます。用途に応じて 3 種類のバリアントを使い分けます。

| コマンド | クエリの実行 | 表示される情報 |
|---------|------------|--------------|
| `EXPLAIN` | しない | 推定コスト・推定行数のみ |
| `EXPLAIN ANALYZE` | する | 推定コスト＋実測値（実際の時間・行数） |
| `EXPLAIN (ANALYZE, BUFFERS)` | する | 推定コスト＋実測値＋バッファI/O（ヒット/ミス数） |

### 使い分けの指針

- **チューニング前の仮説確認**: `EXPLAIN` でインデックスが使われそうか確認する
- **実際のボトルネック調査**: `EXPLAIN ANALYZE` で実測値と推定値のずれを確認する
- **I/O 効率の詳細分析**: `EXPLAIN (ANALYZE, BUFFERS)` でキャッシュヒット率を確認する

> **注意**: `EXPLAIN ANALYZE` はクエリを**実際に実行**するため、`DELETE` や `UPDATE` に使う場合はトランザクションでラップして `ROLLBACK` すること。

---

## 2. 出力の読み方

次のクエリを例に、出力の各要素を解説します。

```sql
EXPLAIN ANALYZE
SELECT * FROM posts WHERE user_id = :'my_id';
```

出力例:

```
Seq Scan on posts  (cost=0.00..28431.00 rows=100 width=72) (actual time=0.024..183.42 rows=100 loops=1)
  Filter: (user_id = '01905a3b-7c10-7000-8000-000000000001')
  Rows Removed by Filter: 999900
Planning Time: 0.123 ms
Execution Time: 184.01 ms
```

### 各要素の意味

| 要素 | 説明 |
|-----|------|
| `cost=0.00..28431.00` | 推定コスト。`起動コスト..総コスト`（単位はページI/Oの相対コスト） |
| `rows=100` | オプティマイザの推定行数 |
| `width=72` | 1行あたりのバイト数（推定） |
| `actual time=0.024..183.42` | 実際の時間。`最初の行が出るまで(ms)..全行処理完了(ms)` |
| `rows=100`（actual側） | 実際に返した行数 |
| `loops=1` | このノードが実行されたループ回数（JOINの内側では1より大きくなる） |
| `Rows Removed by Filter: 999900` | フィルタ条件で除外された行数 |
| `Planning Time` | 実行計画の作成にかかった時間 |
| `Execution Time` | クエリ全体の実行時間 |

### 推定値と実測値のずれに注目する

`rows=100`（推定）と `rows=100`（実測）が大きく乖離している場合、統計情報が古い可能性があります。
`ANALYZE` コマンドで統計を更新することで、オプティマイザの判断精度が上がります。

---

## 3. ノードタイプ一覧

実行計画はツリー構造になっており、各ノードが「どうやってデータを取得するか」を表します。
スキャン系の主なノードタイプは以下の通りです。

| ノードタイプ | 発生条件 | 特徴 |
|------------|---------|------|
| Seq Scan | インデックスなし、または大量取得時 | テーブル全件を順番に読む。小テーブルや多くの行を取得する場合は効率的 |
| Index Scan | インデックスあり、少数行を取得 | インデックスで行の位置を特定し、テーブルにアクセスする |
| Index Only Scan | カバリングインデックスで全カラムが充足 | テーブルにアクセスせずインデックスだけで完結。最速 |
| Bitmap Index/Heap Scan | 複数インデックスの組み合わせ、または中程度の行数 | インデックスでビットマップ作成 → ヒープをまとめてアクセス |

### Seq Scan が選ばれる理由

オプティマイザはコストベースで判断するため、取得行数が全体の大きな割合を占める場合は Seq Scan の方が効率的になります。
「インデックスを張ったのに使われない」という場合、取得対象が多すぎてインデックスの恩恵が薄いことが原因です。

---

## 4. JOIN 戦略

複数テーブルを結合する場合、オプティマイザはデータ量や統計情報をもとに 3 種類の JOIN 戦略から最適なものを選びます。

| JOIN 戦略 | 発生条件 | 特徴 |
|----------|---------|------|
| Nested Loop | 片方が少数行（インデックスが効く場合） | 外側の各行に対して内側をインデックスで検索。O(N log M) |
| Hash Join | 両方が大量行 | 小さい方でハッシュテーブルを構築し、大きい方でプローブ。O(N+M) |
| Merge Join | 両方がソート済み（または ORDER BY がある） | ソート済みの2つのリストをマージ。ソートコストが必要 |

### 各戦略の使われどころ

- **Nested Loop**: 1ユーザーの投稿を取得するような「絞り込みが効いている JOIN」に向く
- **Hash Join**: 全ユーザーの投稿数を集計するような「大テーブル同士の JOIN」に向く
- **Merge Join**: ソート済みのデータが揃っている場合、あるいは結果のソートが必要な JOIN に向く

---

## 5. 具体例 — posts.user_id へのインデックス効果

SNS の `posts` テーブル（1,000,000件）で特定ユーザーの投稿を取得するクエリを例に、
インデックスの有無による実行計画の差を見てみます。

```sql
SELECT * FROM posts WHERE user_id = :'my_id';
```

### インデックスなし（Seq Scan）

```
Seq Scan on posts  (cost=0.00..28431.00 rows=100 width=72)
                   (actual time=0.024..183.42 rows=100 loops=1)
  Filter: (user_id = '01905a3b-7c10-7000-8000-000000000001')
  Rows Removed by Filter: 999900
Execution Time: 184.5 ms
```

1,000,000件すべてを読み込み、999,900件を捨てています。

### インデックスあり（Index Scan）

```sql
CREATE INDEX idx_posts_user_id ON posts(user_id);
```

```
Index Scan using idx_posts_user_id on posts  (cost=0.42..152.40 rows=100 width=72)
                                             (actual time=0.025..0.284 rows=100 loops=1)
  Index Cond: (user_id = '01905a3b-7c10-7000-8000-000000000001')
Execution Time: 0.3 ms
```

### 比較まとめ

| 指標 | インデックスなし | インデックスあり | 改善率 |
|-----|--------------|--------------|-------|
| 総コスト（推定） | 28431.00 | 152.40 | 約 187 倍 |
| 実行時間 | 184.5 ms | 0.3 ms | 約 615 倍 |
| 読み込んだ行数 | 1,000,000 件 | 100 件（該当分のみ） | 1/10000 |

`idx_posts_user_id` インデックスによって、PostgreSQL は該当する行の物理位置を直接特定できるようになります。
その結果、不要な 999,900 件の読み込みが完全に排除され、実行時間が大幅に短縮されます。

---

## 6. EXPLAIN をもっと活用するオプション

### BUFFERS オプション

```sql
EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM posts WHERE user_id = :'my_id';
```

出力に `Buffers: shared hit=N read=M` が追加されます。

- `shared hit`: バッファキャッシュ（共有メモリ）から読んだページ数 → I/O なし
- `shared read`: ディスクから読んだページ数 → 実際のI/Oが発生

ページを繰り返し読むクエリで `shared hit` が多ければキャッシュが効いている証拠です。
逆に `shared read` が多い場合は、ディスクI/Oがボトルネックになっている可能性があります。

### FORMAT JSON オプション

```sql
EXPLAIN (ANALYZE, FORMAT JSON) SELECT * FROM posts WHERE user_id = :'my_id';
```

JSON 形式で出力されるため、`pgBadger` などのツールで機械的に解析したい場合に便利です。

### 組み合わせ例

```sql
-- I/O 分析まで含めた完全な情報を取得する
EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM posts WHERE user_id = :'my_id';
```
