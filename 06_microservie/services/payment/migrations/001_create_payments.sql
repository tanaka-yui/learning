CREATE TABLE IF NOT EXISTS payments (
    id              TEXT PRIMARY KEY,
    idempotency_key TEXT UNIQUE NOT NULL,
    order_id        TEXT NOT NULL,
    amount_cents    INTEGER NOT NULL CHECK (amount_cents > 0),
    status          TEXT NOT NULL CHECK (status IN ('succeeded','refunded')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
