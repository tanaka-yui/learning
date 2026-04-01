-- ============================================================
-- exercise.md の Step 0: 非正規形テーブル
-- 対象DB: chapter01
-- ============================================================

DROP TABLE IF EXISTS receipts_bad CASCADE;

CREATE TABLE receipts_bad (
    receipt_id     INT PRIMARY KEY,
    receipt_date   DATE,
    customer_id    INT,
    customer_name  VARCHAR(100),
    staff_id       INT,
    staff_name     VARCHAR(100),
    product_id_1   INT,
    product_name_1 VARCHAR(100),
    category_1     VARCHAR(100),
    price_1        NUMERIC,
    qty_1          INT,
    product_id_2   INT,
    product_name_2 VARCHAR(100),
    category_2     VARCHAR(100),
    price_2        NUMERIC,
    qty_2          INT,
    product_id_3   INT,
    product_name_3 VARCHAR(100),
    category_3     VARCHAR(100),
    price_3        NUMERIC,
    qty_3          INT
);

INSERT INTO receipts_bad VALUES
    (1001, '2024-01-15', 42,   '田中 花子', 5, '佐藤 太郎', 101, '牛乳',   '乳製品', 198, 2, 202, '食パン', 'パン',   248, 1,  NULL, NULL,     NULL,   NULL, NULL),
    (1002, '2024-01-15', NULL, NULL,         5, '佐藤 太郎', 303, 'バター', '乳製品', 398, 1, NULL, NULL,    NULL,   NULL, NULL, NULL, NULL,     NULL,   NULL, NULL),
    (1003, '2024-01-16', 17,   '鈴木 一郎', 8, '田中 次郎', 101, '牛乳',   '乳製品', 198, 1,  404, '卵',    '卵類',  298, 2,   505, 'チーズ', '乳製品', 348,  1),
    (1004, '2024-01-16', 42,   '田中 花子', 8, '田中 次郎', 202, '食パン', 'パン',   248, 3, NULL, NULL,    NULL,   NULL, NULL, NULL, NULL,     NULL,   NULL, NULL),
    (1005, '2024-01-17', 31,   '山田 美咲', 5, '佐藤 太郎', 101, '牛乳',   '乳製品', 198, 1,  303, 'バター', '乳製品', 398, 1, NULL, NULL,     NULL,   NULL, NULL);
