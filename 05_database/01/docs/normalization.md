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
