package handler

import (
	"context"
	"encoding/json"
	"net/http"

	catalogv1 "microservie/proto/gen/go/catalog/v1"
)

type CatalogClient interface {
	ListProducts(ctx context.Context) ([]*catalogv1.Product, error)
}

type Products struct {
	c CatalogClient
}

func NewProducts(c CatalogClient) *Products {
	return &Products{c: c}
}

type productDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int32  `json:"price_cents"`
}

type listResponse struct {
	Products []productDTO `json:"products"`
}

func (p *Products) List(w http.ResponseWriter, r *http.Request) {
	ps, err := p.c.ListProducts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	out := listResponse{Products: make([]productDTO, 0, len(ps))}
	for _, x := range ps {
		out.Products = append(out.Products, productDTO{
			ID: x.Id, Name: x.Name, Description: x.Description, PriceCents: x.PriceCents,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
