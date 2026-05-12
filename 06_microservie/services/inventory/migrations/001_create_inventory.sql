CREATE TABLE IF NOT EXISTS stocks (
    product_id TEXT PRIMARY KEY,
    available  INTEGER NOT NULL CHECK (available >= 0),
    reserved   INTEGER NOT NULL DEFAULT 0 CHECK (reserved >= 0)
);

CREATE TABLE IF NOT EXISTS reservations (
    id         TEXT PRIMARY KEY,
    order_id   TEXT NOT NULL,
    status     TEXT NOT NULL CHECK (status IN ('held','committed','released')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reservation_items (
    reservation_id TEXT NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    product_id     TEXT NOT NULL,
    quantity       INTEGER NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (reservation_id, product_id)
);
