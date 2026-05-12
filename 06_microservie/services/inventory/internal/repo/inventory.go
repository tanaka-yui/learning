package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInsufficientStock = errors.New("insufficient stock")
var ErrReservationNotFound = errors.New("reservation not found")

type Item struct {
	ProductID string
	Quantity  int32
}

type Stock struct {
	Available int32
	Reserved  int32
}

type Repo struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// Reserve は items 分を available から差し引き、reserved に振り替える。
// 1つでも不足していれば失敗（全体ロールバック）。reservation_id を返す。
func (r *Repo) Reserve(ctx context.Context, orderID string, items []Item) (string, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	resID := uuid.NewString()
	if _, err := tx.Exec(ctx,
		`INSERT INTO reservations (id, order_id, status) VALUES ($1, $2, 'held')`,
		resID, orderID,
	); err != nil {
		return "", err
	}

	for _, it := range items {
		ct, err := tx.Exec(ctx,
			`UPDATE stocks SET available = available - $1, reserved = reserved + $1
			 WHERE product_id = $2 AND available >= $1`,
			it.Quantity, it.ProductID,
		)
		if err != nil {
			return "", err
		}
		if ct.RowsAffected() == 0 {
			return "", ErrInsufficientStock
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO reservation_items (reservation_id, product_id, quantity)
			 VALUES ($1, $2, $3)`,
			resID, it.ProductID, it.Quantity,
		); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return resID, nil
}

// Commit は reserved を確定し、reservations を committed にする。冪等。
func (r *Repo) Commit(ctx context.Context, reservationID string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM reservations WHERE id = $1 FOR UPDATE`, reservationID,
	).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReservationNotFound
		}
		return err
	}
	if status == "committed" {
		return tx.Commit(ctx) // 冪等
	}
	if status == "released" {
		return errors.New("reservation already released")
	}

	if _, err := tx.Exec(ctx,
		`UPDATE stocks s SET reserved = s.reserved - ri.quantity
		 FROM reservation_items ri
		 WHERE ri.reservation_id = $1 AND ri.product_id = s.product_id`,
		reservationID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE reservations SET status = 'committed' WHERE id = $1`, reservationID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Release は reserved を available に戻し、reservations を released にする。冪等。
func (r *Repo) Release(ctx context.Context, reservationID string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM reservations WHERE id = $1 FOR UPDATE`, reservationID,
	).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReservationNotFound
		}
		return err
	}
	if status == "released" {
		return tx.Commit(ctx) // 冪等
	}
	if status == "committed" {
		return errors.New("reservation already committed")
	}

	if _, err := tx.Exec(ctx,
		`UPDATE stocks s SET available = s.available + ri.quantity, reserved = s.reserved - ri.quantity
		 FROM reservation_items ri
		 WHERE ri.reservation_id = $1 AND ri.product_id = s.product_id`,
		reservationID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE reservations SET status = 'released' WHERE id = $1`, reservationID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repo) GetStock(ctx context.Context, productID string) (Stock, error) {
	var s Stock
	err := r.pool.QueryRow(ctx,
		`SELECT available, reserved FROM stocks WHERE product_id = $1`, productID,
	).Scan(&s.Available, &s.Reserved)
	return s, err
}
