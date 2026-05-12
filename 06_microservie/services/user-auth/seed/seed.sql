TRUNCATE TABLE users;

-- パスワードは bcrypt('password', 10) で生成したハッシュ
-- alice@example.com / password、bob@example.com / password
INSERT INTO users (id, email, password_hash) VALUES
  ('u-001', 'alice@example.com', '$2b$10$OqZmupCG2eVxzcTYc1F0p.JXSpQzsZ6IuhTYbGqKvQrhwK32nFVFm'),
  ('u-002', 'bob@example.com',   '$2b$10$OqZmupCG2eVxzcTYc1F0p.JXSpQzsZ6IuhTYbGqKvQrhwK32nFVFm');
