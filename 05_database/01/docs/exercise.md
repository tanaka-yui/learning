# 設計演習: スーパーのレジシステム

## 問題

以下の要件を満たすデータベースを設計してください。

**要件:**
1. レジで商品を購入すると、レシートが発行される
2. レシートには複数の商品が含まれる
3. 商品にはカテゴリがある（例: 乳製品、野菜、飲料）
4. 会員カードを持つ顧客は識別される（非会員は NULL）
5. レジを担当したスタッフが記録される

**考えてみましょう:**
どのようなテーブル構成にしますか？どんな問題が起きそうですか？

---

## Step 0: 非正規形（最初の悪い設計）

すべてのデータを1つのテーブルに詰め込んだ状態から始めます。

```sql
CREATE TABLE receipts_bad (
    receipt_id       INT,
    receipt_date     DATE,
    customer_id      INT,          -- NULL = 非会員
    customer_name    VARCHAR(100), -- NULL = 非会員
    staff_id         INT,
    staff_name       VARCHAR(100),
    product_id_1     INT,
    product_name_1   VARCHAR(100),
    category_1       VARCHAR(100),
    price_1          NUMERIC,
    qty_1            INT,
    product_id_2     INT,          -- 2品目（存在しない場合はNULL）
    product_name_2   VARCHAR(100),
    category_2       VARCHAR(100),
    price_2          NUMERIC,
    qty_2            INT,
    product_id_3     INT,          -- 3品目（存在しない場合はNULL）
    product_name_3   VARCHAR(100),
    category_3       VARCHAR(100),
    price_3          NUMERIC,
    qty_3            INT
    -- ... 4品目以降は？
);

INSERT INTO receipts_bad VALUES (
    1001, '2024-01-15',
    42, '田中 花子',
    5, '佐藤 太郎',
    101, '牛乳', '乳製品', 198, 2,
    202, '食パン', 'パン', 248, 1,
    NULL, NULL, NULL, NULL, NULL
);
```

### 発生する問題

**1. 繰り返しグループ（1NF違反）**
- 商品列が `product_id_1`, `product_id_2`, `product_id_3` と横に並んでいる
- 4品目以上を買うとテーブル定義を変えなければならない
- 「牛乳を買った全レシートを検索する」クエリが極めて複雑になる

**2. 更新異常**
- 商品名が変わったとき → `product_name_1` / `product_name_2` / `product_name_3` すべてを更新
- スタッフ名が変わったとき → そのスタッフが担当した全レシートを更新

**3. 挿入異常**
- 新しい商品を登録するだけでは（購入されるまで）データが存在できない

**4. 削除異常**
- レシートを削除すると、そのレシートにしか存在しない商品情報も消える

---

次のステップでこれを段階的に修正していきます。
