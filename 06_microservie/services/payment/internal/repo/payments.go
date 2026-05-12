package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrPaymentNotFound        = errors.New("payment not found")
	ErrIdempotencyKeyConflict = errors.New("idempotency key already exists")
)

type Payment struct {
	ID, IdempotencyKey, OrderID, Status string
	AmountCents                         int32
}

type Repo struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) Create(ctx context.Context, idemKey, orderID string, amount int32, statusStr string) (string, error) {
	id := uuid.NewString()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO payments (id, idempotency_key, order_id, amount_cents, status) VALUES ($1,$2,$3,$4,$5)`,
		id, idemKey, orderID, amount, statusStr,
	)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return "", ErrIdempotencyKeyConflict
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *Repo) GetByIdempotencyKey(ctx context.Context, key string) (Payment, error) {
	var p Payment
	err := r.pool.QueryRow(ctx,
		`SELECT id, idempotency_key, order_id, amount_cents, status FROM payments WHERE idempotency_key=$1`,
		key,
	).Scan(&p.ID, &p.IdempotencyKey, &p.OrderID, &p.AmountCents, &p.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, ErrPaymentNotFound
	}
	return p, err
}

func (r *Repo) MarkRefunded(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE payments SET status='refunded' WHERE id=$1`,
		id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPaymentNotFound
	}
	return nil
}
