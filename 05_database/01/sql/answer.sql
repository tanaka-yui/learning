-- ============================================================
-- answer.md の段階的変換テーブル
-- 対象DB: chapter01
-- ============================================================

-- ----------------------------------------------------------------
-- Step 1: 第1正規形
-- receipts_bad の繰り返しグループを行に展開した状態
-- ----------------------------------------------------------------
DROP TABLE IF EXISTS receipts_1nf CASCADE;

CREATE TABLE receipts_1nf (
    receipt_id   INT,
    receipt_date DATE,
    customer_id  INT,
    customer_name VARCHAR(100),
    staff_id     INT,
    staff_name   VARCHAR(100),
    product_id   INT,
    product_name VARCHAR(100),
    category     VARCHAR(100),
    price        NUMERIC,
    quantity     INT,
    PRIMARY KEY (receipt_id, product_id)
);

INSERT INTO receipts_1nf VALUES
    (1001, '2024-01-15', 42,   '田中 花子', 5, '佐藤 太郎', 101, '牛乳',   '乳製品', 198, 2),
    (1001, '2024-01-15', 42,   '田中 花子', 5, '佐藤 太郎', 202, '食パン', 'パン',   248, 1),
    (1002, '2024-01-15', NULL, NULL,         5, '佐藤 太郎', 303, 'バター', '乳製品', 398, 1),
    (1003, '2024-01-16', 17,   '鈴木 一郎', 8, '田中 次郎', 101, '牛乳',   '乳製品', 198, 1),
    (1003, '2024-01-16', 17,   '鈴木 一郎', 8, '田中 次郎', 404, '卵',     '卵類',   298, 2);

-- ----------------------------------------------------------------
-- Step 2: 第2正規形
-- 部分関数従属を排除し、マスターテーブルを分離した状態
-- ----------------------------------------------------------------
DROP TABLE IF EXISTS receipt_items_2nf CASCADE;
DROP TABLE IF EXISTS receipts_2nf      CASCADE;
DROP TABLE IF EXISTS products_2nf      CASCADE;
DROP TABLE IF EXISTS customers_2nf     CASCADE;
DROP TABLE IF EXISTS staff_2nf         CASCADE;

CREATE TABLE customers_2nf (
    customer_id   INT PRIMARY KEY,
    customer_name VARCHAR(100)
);

CREATE TABLE products_2nf (
    product_id   INT PRIMARY KEY,
    product_name VARCHAR(100),
    category     VARCHAR(100),
    price        NUMERIC
);

CREATE TABLE staff_2nf (
    staff_id   INT PRIMARY KEY,
    staff_name VARCHAR(100)
);

CREATE TABLE receipts_2nf (
    receipt_id   INT PRIMARY KEY,
    receipt_date DATE,
    customer_id  INT REFERENCES customers_2nf (customer_id),
    staff_id     INT NOT NULL REFERENCES staff_2nf (staff_id)
);

CREATE TABLE receipt_items_2nf (
    receipt_id INT REFERENCES receipts_2nf (receipt_id),
    product_id INT REFERENCES products_2nf (product_id),
    quantity   INT,
    PRIMARY KEY (receipt_id, product_id)
);

INSERT INTO customers_2nf VALUES
    (17, '鈴木 一郎'),
    (31, '山田 美咲'),
    (42, '田中 花子'),
    (55, '高橋 健太'),
    (63, '渡辺 恵子');

INSERT INTO products_2nf VALUES
    (101, '牛乳',   '乳製品', 198),
    (202, '食パン', 'パン',   248),
    (303, 'バター', '乳製品', 398),
    (404, '卵',     '卵類',   298),
    (505, 'チーズ', '乳製品', 348);

INSERT INTO staff_2nf VALUES
    (5,  '佐藤 太郎'),
    (8,  '田中 次郎'),
    (12, '山本 花子'),
    (15, '中村 健一'),
    (20, '小林 美咲');

INSERT INTO receipts_2nf VALUES
    (1001, '2024-01-15', 42,   5),
    (1002, '2024-01-15', NULL, 5),
    (1003, '2024-01-16', 17,   8),
    (1004, '2024-01-16', 42,   8),
    (1005, '2024-01-17', 31,   5);

INSERT INTO receipt_items_2nf VALUES
    (1001, 101, 2),
    (1001, 202, 1),
    (1002, 303, 1),
    (1003, 101, 1),
    (1003, 404, 2);

-- ----------------------------------------------------------------
-- Step 3: 第3正規形（最終形）
-- カテゴリを独立テーブルに分離し、推移関数従属を排除した状態
-- ----------------------------------------------------------------
DROP TABLE IF EXISTS receipt_items CASCADE;
DROP TABLE IF EXISTS receipts       CASCADE;
DROP TABLE IF EXISTS products       CASCADE;
DROP TABLE IF EXISTS categories     CASCADE;
DROP TABLE IF EXISTS customers      CASCADE;
DROP TABLE IF EXISTS staff          CASCADE;

CREATE TABLE categories (
    category_id   INT PRIMARY KEY,
    category_name VARCHAR(100)
);

CREATE TABLE products (
    product_id   INT PRIMARY KEY,
    product_name VARCHAR(100),
    category_id  INT REFERENCES categories (category_id),
    price        NUMERIC
);

CREATE TABLE customers (
    customer_id   INT PRIMARY KEY,
    customer_name VARCHAR(100)
);

CREATE TABLE staff (
    staff_id   INT PRIMARY KEY,
    staff_name VARCHAR(100)
);

CREATE TABLE receipts (
    receipt_id   INT PRIMARY KEY,
    receipt_date DATE,
    customer_id  INT REFERENCES customers (customer_id),
    staff_id     INT NOT NULL REFERENCES staff (staff_id)
);

CREATE TABLE receipt_items (
    receipt_id INT REFERENCES receipts (receipt_id),
    product_id INT REFERENCES products (product_id),
    quantity   INT,
    PRIMARY KEY (receipt_id, product_id)
);

INSERT INTO categories VALUES
    (1, '乳製品'),
    (2, 'パン'),
    (3, '卵類'),
    (4, '野菜'),
    (5, '飲料');

INSERT INTO products VALUES
    (101, '牛乳',   1, 198),
    (202, '食パン', 2, 248),
    (303, 'バター', 1, 398),
    (404, '卵',     3, 298),
    (505, 'チーズ', 1, 348);

INSERT INTO customers VALUES
    (17, '鈴木 一郎'),
    (31, '山田 美咲'),
    (42, '田中 花子'),
    (55, '高橋 健太'),
    (63, '渡辺 恵子');

INSERT INTO staff VALUES
    (5,  '佐藤 太郎'),
    (8,  '田中 次郎'),
    (12, '山本 花子'),
    (15, '中村 健一'),
    (20, '小林 美咲');

INSERT INTO receipts VALUES
    (1001, '2024-01-15', 42,   5),
    (1002, '2024-01-15', NULL, 5),
    (1003, '2024-01-16', 17,   8),
    (1004, '2024-01-16', 42,   8),
    (1005, '2024-01-17', 31,   5);

INSERT INTO receipt_items VALUES
    (1001, 101, 2),
    (1001, 202, 1),
    (1002, 303, 1),
    (1003, 101, 1),
    (1003, 404, 2),
    (1003, 505, 1),
    (1004, 202, 3),
    (1005, 101, 1),
    (1005, 303, 1);
