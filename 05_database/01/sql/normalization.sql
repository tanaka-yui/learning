-- ============================================================
-- normalization.md の例題テーブル
-- 対象DB: chapter01
-- ============================================================

-- ----------------------------------------------------------------
-- 第1正規形 (1NF)
-- ----------------------------------------------------------------
DROP TABLE IF EXISTS nf1_orders CASCADE;

CREATE TABLE nf1_orders (
    order_id INT,
    customer VARCHAR(100),
    item     VARCHAR(100),
    PRIMARY KEY (order_id, item)
);

INSERT INTO nf1_orders VALUES
    (1, '田中 花子', '牛乳'),
    (1, '田中 花子', '卵'),
    (1, '田中 花子', 'パン'),
    (2, '鈴木 一郎', '卵'),
    (2, '鈴木 一郎', 'バター');

-- ----------------------------------------------------------------
-- 第2正規形 (2NF)
-- ----------------------------------------------------------------
DROP TABLE IF EXISTS nf2_order_items CASCADE;
DROP TABLE IF EXISTS nf2_products    CASCADE;

CREATE TABLE nf2_products (
    product_id   INT PRIMARY KEY,
    product_name VARCHAR(100)
);

CREATE TABLE nf2_order_items (
    order_id   INT,
    product_id INT REFERENCES nf2_products (product_id),
    quantity   INT,
    PRIMARY KEY (order_id, product_id)
);

INSERT INTO nf2_products VALUES
    (101, '牛乳'),
    (202, '食パン'),
    (303, 'バター'),
    (404, '卵'),
    (505, 'チーズ');

INSERT INTO nf2_order_items VALUES
    (1, 101, 2),
    (1, 202, 1),
    (2, 101, 1),
    (2, 303, 1),
    (3, 202, 3);

-- ----------------------------------------------------------------
-- 第3正規形 (3NF)
-- ----------------------------------------------------------------
DROP TABLE IF EXISTS nf3_orders CASCADE;
DROP TABLE IF EXISTS nf3_staff  CASCADE;

CREATE TABLE nf3_staff (
    staff_id   INT PRIMARY KEY,
    staff_name VARCHAR(100)
);

CREATE TABLE nf3_orders (
    order_id    INT PRIMARY KEY,
    customer_id INT,
    staff_id    INT REFERENCES nf3_staff (staff_id),
    order_date  DATE
);

INSERT INTO nf3_staff VALUES
    (5,  '佐藤 太郎'),
    (8,  '田中 次郎'),
    (12, '山本 花子'),
    (15, '中村 健一'),
    (20, '小林 美咲');

INSERT INTO nf3_orders VALUES
    (1, 42, 5,  '2024-01-15'),
    (2, 17, 5,  '2024-01-15'),
    (3, 42, 8,  '2024-01-16'),
    (4, 31, 8,  '2024-01-16'),
    (5, 55, 5,  '2024-01-17');

-- ----------------------------------------------------------------
-- ボイス・コッド正規形 (BCNF)
-- ----------------------------------------------------------------
DROP TABLE IF EXISTS bcnf_enrollments      CASCADE;
DROP TABLE IF EXISTS bcnf_teacher_subjects CASCADE;

CREATE TABLE bcnf_teacher_subjects (
    teacher_id INT PRIMARY KEY,
    subject    VARCHAR(100)
);

CREATE TABLE bcnf_enrollments (
    student_id INT,
    teacher_id INT REFERENCES bcnf_teacher_subjects (teacher_id),
    PRIMARY KEY (student_id, teacher_id)
);

INSERT INTO bcnf_teacher_subjects VALUES
    (10, '数学'),
    (20, '英語'),
    (30, '物理'),
    (40, '化学'),
    (50, '国語');

INSERT INTO bcnf_enrollments VALUES
    (1, 10),
    (1, 20),
    (2, 10),
    (2, 30),
    (3, 20);

-- ----------------------------------------------------------------
-- 第4正規形 (4NF)
-- ----------------------------------------------------------------
DROP TABLE IF EXISTS nf4_person_skills  CASCADE;
DROP TABLE IF EXISTS nf4_person_hobbies CASCADE;
DROP TABLE IF EXISTS nf4_persons        CASCADE;

CREATE TABLE nf4_persons (
    person_id INT PRIMARY KEY
);

CREATE TABLE nf4_person_hobbies (
    person_id INT REFERENCES nf4_persons (person_id),
    hobby     VARCHAR(100),
    PRIMARY KEY (person_id, hobby)
);

CREATE TABLE nf4_person_skills (
    person_id INT REFERENCES nf4_persons (person_id),
    skill     VARCHAR(100),
    PRIMARY KEY (person_id, skill)
);

INSERT INTO nf4_persons VALUES (1), (2), (3);

INSERT INTO nf4_person_hobbies VALUES
    (1, '読書'),
    (1, '釣り'),
    (2, '料理'),
    (2, '旅行'),
    (3, '読書');

INSERT INTO nf4_person_skills VALUES
    (1, 'Python'),
    (1, 'SQL'),
    (2, 'Java'),
    (2, 'SQL'),
    (3, 'Go');

-- ----------------------------------------------------------------
-- 第6正規形 (6NF) — 時制データの例
-- ----------------------------------------------------------------
DROP TABLE IF EXISTS nf6_product_prices CASCADE;

CREATE TABLE nf6_product_prices (
    product_id INT,
    valid_from DATE,
    valid_to   DATE,
    price      NUMERIC,
    PRIMARY KEY (product_id, valid_from)
);

INSERT INTO nf6_product_prices VALUES
    (101, '2024-01-01', '2024-03-31', 198),
    (101, '2024-04-01', '2024-12-31', 218),
    (202, '2024-01-01', '2024-06-30', 248),
    (202, '2024-07-01', '2024-12-31', 258),
    (303, '2024-01-01', '2024-12-31', 398);
