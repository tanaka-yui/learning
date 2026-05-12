package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"microservie/bff/internal/httpx"
	"microservie/bff/internal/middleware"
	orderv1 "microservie/proto/gen/go/order/v1"
)

type OrdersClient interface {
	GetOrder(ctx context.Context, orderID, userID string) (*orderv1.Order, error)
	ListOrders(ctx context.Context, userID string) ([]*orderv1.Order, error)
}

type Orders struct{ c OrdersClient }

func NewOrders(c OrdersClient) *Orders { return &Orders{c: c} }

type orderItemDTO struct {
	ProductID      string `json:"product_id"`
	Quantity       int32  `json:"quantity"`
	UnitPriceCents int32  `json:"unit_price_cents"`
}
type orderDTO struct {
	ID         string         `json:"id"`
	UserID     string         `json:"user_id"`
	Status     string         `json:"status"`
	TotalCents int32          `json:"total_cents"`
	Items      []orderItemDTO `json:"items"`
}

func toDTO(o *orderv1.Order) orderDTO {
	items := make([]orderItemDTO, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, orderItemDTO{
			ProductID: it.ProductId, Quantity: it.Quantity, UnitPriceCents: it.UnitPriceCents,
		})
	}
	return orderDTO{ID: o.Id, UserID: o.UserId, Status: o.Status, TotalCents: o.TotalCents, Items: items}
}

func (h *Orders) List(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r.Context())
	os, err := h.c.ListOrders(r.Context(), uid)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadGateway, "UPSTREAM_FAILED", err.Error())
		return
	}
	out := make([]orderDTO, 0, len(os))
	for _, o := range os {
		out = append(out, toDTO(o))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"orders": out})
}

func (h *Orders) Get(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r.Context())
	orderID := chi.URLParam(r, "id")
	o, err := h.c.GetOrder(r.Context(), orderID, uid)
	if err != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toDTO(o))
}
