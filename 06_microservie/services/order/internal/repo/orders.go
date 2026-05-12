package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrOrderNotFound = errors.New("order not found")

type OrderItem struct {
	ProductID      string
	Quantity       int32
	UnitPriceCents int32
}

type Order struct {
	ID         string
	UserID     string
	Status     string
	TotalCents int32
	Items      []OrderItem
}

type Repo struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// Create inserts an orders row (status=PENDING) and order_items in one tx,
// computes total from items, and returns the generated orderID.
func (r *Repo) Create(ctx context.Context, userID string, items []OrderItem) (orderID string, totalCents int32, err error) {
	orderID = uuid.NewString()

	var total int32
	for _, it := range items {
		total += it.Quantity * it.UnitPriceCents
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx,
		`INSERT INTO orders (id, user_id, status, total_cents) VALUES ($1, $2, 'PENDING', $3)`,
		orderID, userID, total,
	)
	if err != nil {
		return "", 0, err
	}

	for _, it := range items {
		itemID := uuid.NewString()
		_, err = tx.Exec(ctx,
			`INSERT INTO order_items (id, order_id, product_id, quantity, unit_price_cents)
			 VALUES ($1, $2, $3, $4, $5)`,
			itemID, orderID, it.ProductID, it.Quantity, it.UnitPriceCents,
		)
		if err != nil {
			return "", 0, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return "", 0, err
	}
	return orderID, total, nil
}

// UpdateStatus changes orders.status. Returns ErrOrderNotFound if no row matched.
func (r *Repo) UpdateStatus(ctx context.Context, orderID, status string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE orders SET status=$1 WHERE id=$2`,
		status, orderID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOrderNotFound
	}
	return nil
}

// Get returns an order with its items. Enforces user_id match — a different
// user receives ErrOrderNotFound.
func (r *Repo) Get(ctx context.Context, orderID, userID string) (Order, error) {
	var o Order
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, status, total_cents FROM orders WHERE id=$1 AND user_id=$2`,
		orderID, userID,
	).Scan(&o.ID, &o.UserID, &o.Status, &o.TotalCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrOrderNotFound
	}
	if err != nil {
		return Order{}, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT product_id, quantity, unit_price_cents FROM order_items WHERE order_id=$1`,
		orderID,
	)
	if err != nil {
		return Order{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.ProductID, &it.Quantity, &it.UnitPriceCents); err != nil {
			return Order{}, err
		}
		o.Items = append(o.Items, it)
	}
	if err := rows.Err(); err != nil {
		return Order{}, err
	}

	return o, nil
}

// List returns all orders for a user (no items — keep it light).
func (r *Repo) List(ctx context.Context, userID string) ([]Order, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, status, total_cents FROM orders WHERE user_id=$1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.TotalCents); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}

// LogStep upserts into saga_log. Idempotent via PRIMARY KEY (order_id, step) DO UPDATE.
func (r *Repo) LogStep(ctx context.Context, orderID, step, status, detail string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO saga_log (order_id, step, status, detail, updated_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (order_id, step) DO UPDATE
		   SET status=EXCLUDED.status, detail=EXCLUDED.detail, updated_at=NOW()`,
		orderID, step, status, detail,
	)
	return err
}
