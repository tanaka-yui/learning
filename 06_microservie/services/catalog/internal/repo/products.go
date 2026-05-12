package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("product not found")

type Product struct {
	ID          string
	Name        string
	Description string
	PriceCents  int32
}

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) List(ctx context.Context, limit, offset int32) ([]Product, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, description, price_cents FROM products ORDER BY id LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ps []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.PriceCents); err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	return ps, rows.Err()
}

func (r *Repo) Get(ctx context.Context, id string) (Product, error) {
	var p Product
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, description, price_cents FROM products WHERE id = $1`,
		id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.PriceCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	return p, err
}

func (r *Repo) Insert(ctx context.Context, p Product) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO products(id, name, description, price_cents) VALUES ($1,$2,$3,$4)`,
		p.ID, p.Name, p.Description, p.PriceCents,
	)
	return err
}
