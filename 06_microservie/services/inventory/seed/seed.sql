TRUNCATE TABLE reservation_items, reservations, stocks RESTART IDENTITY CASCADE;

INSERT INTO stocks (product_id, available) VALUES
  ('p-001', 100), ('p-002', 200), ('p-003', 30),  ('p-004', 150),
  ('p-005', 80),  ('p-006', 40),  ('p-007', 500), ('p-008', 60),
  ('p-009', 120), ('p-010', 250);
