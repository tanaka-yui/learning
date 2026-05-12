package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"microservie/bff/internal/httpx"
	catalogv1 "microservie/proto/gen/go/catalog/v1"
)

type CatalogClient interface {
	ListProducts(ctx context.Context) ([]*catalogv1.Product, error)
	GetProduct(ctx context.Context, id string) (*catalogv1.Product, error)
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
		httpx.WriteError(w, r, http.StatusBadGateway, "UPSTREAM_FAILED", err.Error())
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

func (p *Products) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", "id required")
		return
	}
	prod, err := p.c.GetProduct(r.Context(), id)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		httpx.WriteError(w, r, http.StatusBadGateway, "UPSTREAM_FAILED", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(productDTO{
		ID: prod.Id, Name: prod.Name, Description: prod.Description, PriceCents: prod.PriceCents,
	})
}
