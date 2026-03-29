# 05_database Chapter 1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** `05_database/01/` にデータベース正規化の学習資料（README + 解説 + 演習）を作成する。

**Architecture:** Markdownドキュメント3ファイル構成。`docs/normalization.md` に正規化の理論解説、`docs/exercise.md` にスーパーのレジシステムを題材にした段階的設計演習、`README.md` に目次と学習ガイドを配置する。

**Tech Stack:** Markdown, SQL（PostgreSQL構文）

---

### Task 1: README.md の作成

**Files:**
- Create: `05_database/01/README.md`

**Step 1: ファイルを作成する**

```markdown
# 第1章: データベースの基礎 — 正規化

## この章で学ぶこと

- 正規化の目的と必要性
- 第1〜第3正規形（実務でよく使う範囲）
- 第4〜第7正規形（理論的な上位正規形）
- 非正規化とその使いどころ
- 実践: スーパーのレジシステムのDB設計

## 読む順番

1. [正規化の解説](docs/normalization.md) — 理論と各正規形の定義
2. [設計演習](docs/exercise.md) — スーパーのレジシステムを段階的に正規化する

## 前提知識

- SQLの基本（SELECT / CREATE TABLE）
- 主キー・外部キーの概念
```

**Step 2: 内容を確認する**

ファイルが存在し、3セクション（学ぶこと・読む順番・前提知識）が揃っていることを確認。

**Step 3: コミット**

```bash
git add 05_database/01/README.md
git commit -m "docs(database): add chapter 1 README"
```

---

### Task 2: normalization.md — セクション1〜3（正規化とは、一覧、1NF）

**Files:**
- Create: `05_database/01/docs/normalization.md`

**Step 1: ファイルを作成し、セクション1〜3を書く**

```markdown
# データベース正規化

## 1. 正規化とは

正規化とは、データベースのテーブル設計を体系的に整理し、**データの冗長性を排除**し、**更新異常を防ぐ**プロセスです。

### 目的

- **冗長性の排除**: 同じデータを複数箇所に持たない
- **更新異常の防止**: データを1箇所だけ更新すれば済む状態にする
- **整合性の確保**: データの矛盾が起きない構造にする

### 更新異常の種類

| 異常の種類 | 説明 | 例 |
|-----------|------|-----|
| 更新異常 | 同じデータが複数行にあり、一部だけ更新されると矛盾が生じる | 商品の価格が複数行にあり、片方だけ更新された |
| 挿入異常 | 必要な情報が揃わないとレコードを挿入できない | 注文なしでは顧客情報を登録できない |
| 削除異常 | あるデータを削除すると、他の必要な情報も失われる | 最後の注文を削除すると顧客情報も消える |

### メリット・デメリット

| | メリット | デメリット |
|---|---------|-----------|
| 正規化 | 冗長性排除、更新異常防止、整合性確保 | JOINが増えてクエリが複雑になる |
| 非正規化 | クエリがシンプル、読み取り性能が高い | 冗長性あり、更新異常のリスク |

---

## 2. 正規形の一覧

| 正規形 | 英語名 | 主な条件 |
|--------|--------|---------|
| 1NF | First Normal Form | 繰り返しグループの排除、原子値 |
| 2NF | Second Normal Form | 部分関数従属の排除 |
| 3NF | Third Normal Form | 推移関数従属の排除 |
| BCNF | Boyce-Codd Normal Form | すべての決定項が候補キー |
| 4NF | Fourth Normal Form | 多値従属性の排除 |
| 5NF | Fifth Normal Form | 結合従属性の排除 |
| 6NF / DKNF / 7NF | 上位正規形 | 時制データ、ドメイン・キー制約 |

実務では **3NF または BCNF** まで適用することがほとんどです。

---

## 3. 第1正規形（1NF）

### 定義

テーブルのすべての列が**原子値**（これ以上分割できない値）を持ち、**繰り返しグループが存在しない**状態。

### 違反例

```sql
-- 1NF違反: 1つのセルに複数の値が入っている
CREATE TABLE orders_bad (
    order_id    INT PRIMARY KEY,
    customer    VARCHAR(100),
    items       VARCHAR(500)  -- 例: '牛乳,卵,パン' （複数値）
);

INSERT INTO orders_bad VALUES (1, '田中 花子', '牛乳,卵,パン');
INSERT INTO orders_bad VALUES (2, '鈴木 一郎', '卵,バター');
```

**問題点:**
- `items` に複数の商品が詰め込まれている
- 「牛乳を買った人を検索する」クエリが書きにくい（LIKE検索に頼ることになる）
- 商品を追加・削除するたびに文字列を加工する必要がある

### 1NFに変換後

```sql
-- 1NF準拠: 繰り返しグループを行に展開する
CREATE TABLE orders (
    order_id    INT,
    customer    VARCHAR(100),
    item        VARCHAR(100),
    PRIMARY KEY (order_id, item)
);

INSERT INTO orders VALUES (1, '田中 花子', '牛乳');
INSERT INTO orders VALUES (1, '田中 花子', '卵');
INSERT INTO orders VALUES (1, '田中 花子', 'パン');
INSERT INTO orders VALUES (2, '鈴木 一郎', '卵');
INSERT INTO orders VALUES (2, '鈴木 一郎', 'バター');
```

**まだ残っている問題点:**
- `customer` が `order_id` だけでなく `item` にも依存しているように見える（実際は `order_id` だけで決まる）
- → **部分関数従属**が存在する（2NF違反）
```

**Step 2: 内容を確認する**

SQLが正しい構文であること、違反例と修正後の対比が明確であることを確認。

**Step 3: コミット**

```bash
git add 05_database/01/docs/normalization.md
git commit -m "docs(database): add normalization sections 1-3 (1NF)"
```

---

### Task 3: normalization.md — セクション4〜5（2NF、3NF）

**Files:**
- Modify: `05_database/01/docs/normalization.md`

**Step 1: セクション4〜5を追記する**

```markdown
## 4. 第2正規形（2NF）

### 定義

1NFを満たし、かつ**部分関数従属が存在しない**状態。
すべての非キー列が**主キー全体**に依存している。

> **部分関数従属**: 複合主キーの一部だけで非キー列が決まること。

### 違反例

```sql
-- 2NF違反: 複合主キー (order_id, product_id) のうち
-- product_id だけで product_name が決まる（部分関数従属）
CREATE TABLE order_items_bad (
    order_id     INT,
    product_id   INT,
    product_name VARCHAR(100),  -- product_id だけで決まる → 部分従属
    quantity     INT,
    PRIMARY KEY (order_id, product_id)
);
```

**問題点:**
- 商品名を変更するとき、その商品を含む全行を更新する必要がある（更新異常）
- 商品情報だけを登録することができない（挿入異常）

### 2NFに変換後

```sql
-- 商品テーブルを分離する
CREATE TABLE products (
    product_id   INT PRIMARY KEY,
    product_name VARCHAR(100)
);

CREATE TABLE order_items (
    order_id    INT,
    product_id  INT REFERENCES products(product_id),
    quantity    INT,
    PRIMARY KEY (order_id, product_id)
);
```

**まだ残っている問題点:**
- 非キー列が他の非キー列に依存している場合（推移関数従属）はまだ残っている
- → **推移関数従属**が存在する可能性（3NF違反）

---

## 5. 第3正規形（3NF）

### 定義

2NFを満たし、かつ**推移関数従属が存在しない**状態。
すべての非キー列が**主キーにのみ**直接依存している。

> **推移関数従属**: 非キー列Aが非キー列Bを通じて主キーに依存すること（主キー → B → A）。

### 違反例

```sql
-- 3NF違反: order_id → staff_id → staff_name という推移従属
CREATE TABLE orders_bad (
    order_id    INT PRIMARY KEY,
    customer_id INT,
    staff_id    INT,
    staff_name  VARCHAR(100),  -- staff_id で決まる → 推移従属
    order_date  DATE
);
```

**問題点:**
- スタッフ名が変わったとき、そのスタッフが担当した全注文を更新する必要がある
- スタッフ情報だけを登録できない

### 3NFに変換後

```sql
CREATE TABLE staff (
    staff_id   INT PRIMARY KEY,
    staff_name VARCHAR(100)
);

CREATE TABLE orders (
    order_id    INT PRIMARY KEY,
    customer_id INT,
    staff_id    INT REFERENCES staff(staff_id),
    order_date  DATE
);
```

これで更新異常・挿入異常・削除異常がすべて解消されます。
```

**Step 2: 内容を確認する**

2NF・3NFの定義と違反例が明確に対比されていることを確認。

**Step 3: コミット**

```bash
git add 05_database/01/docs/normalization.md
git commit -m "docs(database): add normalization sections 4-5 (2NF, 3NF)"
```

---

### Task 4: normalization.md — セクション6〜8（BCNF、4NF〜7NF、非正規化）

**Files:**
- Modify: `05_database/01/docs/normalization.md`

**Step 1: セクション6〜8を追記する**

```markdown
## 6. ボイス・コッド正規形（BCNF）

### 定義

3NFの強化版。すべての**決定項（関数従属の左辺）が候補キー**である状態。

3NFを満たしていてもBCNFを満たさないケースが稀にある。

### 3NFを満たすがBCNF違反の例

```sql
-- 前提: 生徒は複数の科目を履修し、各科目は1人の教師が担当
-- 候補キー: (student_id, subject) または (student_id, teacher_id)
-- 関数従属: teacher_id → subject（教師が決まると科目が決まる）
--           しかし teacher_id は候補キーではない → BCNF違反

CREATE TABLE enrollments_bad (
    student_id INT,
    subject    VARCHAR(100),
    teacher_id INT,
    -- teacher_id → subject だが teacher_id は候補キーでない
    PRIMARY KEY (student_id, subject)
);
```

### BCNFに変換後

```sql
CREATE TABLE teacher_subjects (
    teacher_id INT PRIMARY KEY,
    subject    VARCHAR(100)
);

CREATE TABLE enrollments (
    student_id INT,
    teacher_id INT REFERENCES teacher_subjects(teacher_id),
    PRIMARY KEY (student_id, teacher_id)
);
```

---

## 7. 第4〜第7正規形

これらは理論的な上位正規形です。実務で意識することは稀ですが、概念として知っておくと役立ちます。

### 第4正規形（4NF）

**多値従属性**を排除した状態。

> 多値従属性: 主キーAに対して、BとCが互いに独立して複数の値を持つ場合（A →→ B かつ A →→ C）。

```sql
-- 違反例: 人が複数の趣味と複数のスキルを持つ場合
-- hobby と skill は独立しているのに1テーブルに混在している
CREATE TABLE person_hobbies_skills_bad (
    person_id INT,
    hobby     VARCHAR(100),
    skill     VARCHAR(100),
    PRIMARY KEY (person_id, hobby, skill)
);

-- 4NF準拠: テーブルを分割する
CREATE TABLE person_hobbies (
    person_id INT,
    hobby     VARCHAR(100),
    PRIMARY KEY (person_id, hobby)
);

CREATE TABLE person_skills (
    person_id INT,
    skill     VARCHAR(100),
    PRIMARY KEY (person_id, skill)
);
```

### 第5正規形（5NF）

**結合従属性**を排除した状態。テーブルをどのように分割しても、JOINで元に戻せる状態。
4NFより制約が厳しく、実務での適用は非常にまれ。

### 第6正規形（6NF）

**時制データ**（有効期間を持つデータ）を扱うための正規形。
例: 商品の価格が期間ごとに変わる場合に、時間軸を独立した軸として扱う。

```sql
-- 6NF的な考え方: 期間を明示的に管理する
CREATE TABLE product_prices (
    product_id  INT,
    valid_from  DATE,
    valid_to    DATE,
    price       NUMERIC,
    PRIMARY KEY (product_id, valid_from)
);
```

### DKNF / 第7正規形

- **DKNF（ドメイン・キー正規形）**: すべての制約がドメイン制約またはキー制約から導出できる状態。理論的な到達点。
- **7NF**: 6NFを時制データの観点でさらに拡張したもの。学術的な概念。

---

## 8. 非正規化

### 目的

意図的に正規化を崩し、**クエリのパフォーマンスを向上させる**テクニック。

### メリット・デメリット

| | 内容 |
|---|------|
| **メリット** | JOINが減り、クエリがシンプルになる |
| | 読み取りパフォーマンスが向上する |
| **デメリット** | データの冗長性が生まれる |
| | 更新時に複数箇所を同期する必要がある |
| | 整合性の維持がアプリケーション側の責任になる |

### 適用場面

- **分析系クエリ（OLAP）**: 集計・レポート用のテーブルで読み取りを最優先にする場合
- **キャッシュテーブル**: 計算済みの集計値を別テーブルに保持する
- **高トラフィックな読み取り**: 大量アクセスに対してJOINのコストを下げたい場合

```sql
-- 非正規化の例: 注文テーブルに顧客名を直接持つ
-- （正規化では customers テーブルをJOINするところを、コピーして持つ）
CREATE TABLE orders_denormalized (
    order_id       INT PRIMARY KEY,
    customer_id    INT,
    customer_name  VARCHAR(100),  -- customers テーブルからコピー（冗長）
    order_total    NUMERIC,       -- 明細の合計を事前計算（冗長）
    order_date     DATE
);
```

> **実務の指針**: まず正規化して設計し、パフォーマンス問題が実際に発生したときに限り、計測しながら非正規化を検討する。
```

**Step 2: 内容を確認する**

全8セクションが揃っていること、SQLが一貫して正しい構文であることを確認。

**Step 3: コミット**

```bash
git add 05_database/01/docs/normalization.md
git commit -m "docs(database): add normalization sections 6-8 (BCNF, 4NF-7NF, denormalization)"
```

---

### Task 5: exercise.md — 問題文と Step 0（非正規形）

**Files:**
- Create: `05_database/01/docs/exercise.md`

**Step 1: ファイルを作成し、問題文とStep 0を書く**

```markdown
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
```

**Step 2: 内容を確認する**

問題文の要件が5つ揃っていること、4種類の異常が説明されていることを確認。

**Step 3: コミット**

```bash
git add 05_database/01/docs/exercise.md
git commit -m "docs(database): add exercise problem and step 0 (unnormalized)"
```

---

### Task 6: exercise.md — Step 1〜3 と解答まとめ

**Files:**
- Modify: `05_database/01/docs/exercise.md`

**Step 1: Step 1〜3と解答まとめを追記する**

```markdown
## Step 1: 1NFへの変換

**解決すること:** 繰り返しグループ（商品列の横並び）を排除する

```sql
-- 商品を行に展開する
CREATE TABLE receipts_1nf (
    receipt_id    INT,
    receipt_date  DATE,
    customer_id   INT,
    customer_name VARCHAR(100),
    staff_id      INT,
    staff_name    VARCHAR(100),
    product_id    INT,
    product_name  VARCHAR(100),
    category      VARCHAR(100),
    price         NUMERIC,
    quantity      INT,
    PRIMARY KEY (receipt_id, product_id)
);

INSERT INTO receipts_1nf VALUES
    (1001, '2024-01-15', 42, '田中 花子', 5, '佐藤 太郎', 101, '牛乳', '乳製品', 198, 2),
    (1001, '2024-01-15', 42, '田中 花子', 5, '佐藤 太郎', 202, '食パン', 'パン', 248, 1);
```

**改善されたこと:**
- 何品買っても行を増やすだけで対応できる
- 「牛乳を買った全レシート」が `WHERE product_name = '牛乳'` で検索できる

**まだ残っている問題点:**
- `customer_name` は `customer_id` だけで決まる（主キー全体ではなく一部で決まる）→ **部分関数従属（2NF違反）**
- `product_name`, `category`, `price` は `product_id` だけで決まる → **部分関数従属（2NF違反）**
- `staff_name` は `staff_id` だけで決まる → **部分関数従属（2NF違反）**

---

## Step 2: 2NFへの変換

**解決すること:** 部分関数従属を排除する（主キーの一部で決まる列を別テーブルへ）

```sql
-- 顧客テーブル（customer_id → customer_name）
CREATE TABLE customers (
    customer_id   INT PRIMARY KEY,
    customer_name VARCHAR(100)
);

-- 商品テーブル（product_id → product_name, category, price）
CREATE TABLE products (
    product_id   INT PRIMARY KEY,
    product_name VARCHAR(100),
    category     VARCHAR(100),
    price        NUMERIC
);

-- スタッフテーブル（staff_id → staff_name）
CREATE TABLE staff (
    staff_id   INT PRIMARY KEY,
    staff_name VARCHAR(100)
);

-- レシートヘッダー（receipt_id → receipt_date, customer_id, staff_id）
CREATE TABLE receipts_2nf (
    receipt_id   INT PRIMARY KEY,
    receipt_date DATE,
    customer_id  INT REFERENCES customers(customer_id),  -- NULL許容（非会員）
    staff_id     INT REFERENCES staff(staff_id)
);

-- レシート明細（receipt_id + product_id → quantity）
CREATE TABLE receipt_items_2nf (
    receipt_id INT REFERENCES receipts_2nf(receipt_id),
    product_id INT REFERENCES products(product_id),
    quantity   INT,
    PRIMARY KEY (receipt_id, product_id)
);
```

**改善されたこと:**
- 商品名の変更は `products` テーブルの1行を更新するだけ
- スタッフ名の変更は `staff` テーブルの1行を更新するだけ

**まだ残っている問題点:**
- `products` テーブルで `category` の詳細情報（例: カテゴリの説明）を追加したい場合、`product_name → category` という推移従属が潜在している
- → **推移関数従属（3NF違反）**の可能性

---

## Step 3: 3NFへの変換

**解決すること:** 推移関数従属を排除する（非キー列が他の非キー列を通じて決まる関係を切り離す）

```sql
-- カテゴリテーブルを分離（category_name → category_description など将来の拡張も見据えて）
CREATE TABLE categories (
    category_id   INT PRIMARY KEY,
    category_name VARCHAR(100)
);

-- 商品テーブル（カテゴリIDで参照）
CREATE TABLE products (
    product_id   INT PRIMARY KEY,
    product_name VARCHAR(100),
    category_id  INT REFERENCES categories(category_id),
    price        NUMERIC
);

-- 顧客テーブル（変更なし）
CREATE TABLE customers (
    customer_id   INT PRIMARY KEY,
    customer_name VARCHAR(100)
);

-- スタッフテーブル（変更なし）
CREATE TABLE staff (
    staff_id   INT PRIMARY KEY,
    staff_name VARCHAR(100)
);

-- レシートヘッダー（変更なし）
CREATE TABLE receipts (
    receipt_id   INT PRIMARY KEY,
    receipt_date DATE,
    customer_id  INT REFERENCES customers(customer_id),  -- NULL = 非会員
    staff_id     INT REFERENCES staff(staff_id)
);

-- レシート明細（変更なし）
CREATE TABLE receipt_items (
    receipt_id INT REFERENCES receipts(receipt_id),
    product_id INT REFERENCES products(product_id),
    quantity   INT,
    PRIMARY KEY (receipt_id, product_id)
);
```

---

## 解答まとめ

### 最終テーブル構成

```
categories
├── category_id (PK)
└── category_name

products
├── product_id (PK)
├── product_name
├── category_id (FK → categories)
└── price

customers
├── customer_id (PK)
└── customer_name

staff
├── staff_id (PK)
└── staff_name

receipts
├── receipt_id (PK)
├── receipt_date
├── customer_id (FK → customers, NULL許容)
└── staff_id (FK → staff)

receipt_items
├── receipt_id (FK → receipts) ┐ 複合PK
├── product_id (FK → products) ┘
└── quantity
```

### 各テーブルの役割

| テーブル | 役割 |
|---------|------|
| `categories` | 商品カテゴリのマスター |
| `products` | 商品マスター（名前・価格・カテゴリ） |
| `customers` | 会員顧客マスター |
| `staff` | レジ担当スタッフマスター |
| `receipts` | 購入トランザクションのヘッダー（いつ・誰が・誰に） |
| `receipt_items` | レシートの明細（何を・何個） |

### 正規化の効果

| 問題 | 解決方法 |
|-----|---------|
| 商品名変更 | `products` の1行のみ更新 |
| スタッフ名変更 | `staff` の1行のみ更新 |
| カテゴリ追加 | `categories` に1行追加するだけ |
| 新商品登録 | 購入前から `products` に登録可能 |
| 何品でも対応 | `receipt_items` に行を追加するだけ |
```

**Step 2: 内容を確認する**

Step 0〜3の流れが一貫していること、解答まとめのテーブルが最終SQLと一致していることを確認。

**Step 3: コミット**

```bash
git add 05_database/01/docs/exercise.md
git commit -m "docs(database): add exercise steps 1-3 and final answer"
```

---

## 完了チェックリスト

- [ ] `05_database/01/README.md` — 概要・目次・前提知識
- [ ] `05_database/01/docs/normalization.md` — 全8セクション（1NF〜7NF、非正規化）
- [ ] `05_database/01/docs/exercise.md` — 問題 + Step 0〜3 + 解答まとめ
- [ ] SQLが一貫してPostgreSQL構文
- [ ] 各タスクでコミット済み
