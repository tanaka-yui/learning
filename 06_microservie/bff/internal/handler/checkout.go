package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"microservie/bff/internal/httpx"
	"microservie/bff/internal/middleware"
	catalogv1 "microservie/proto/gen/go/catalog/v1"
	orderv1 "microservie/proto/gen/go/order/v1"
)

type CatalogProductGetter interface {
	GetProduct(ctx context.Context, id string) (*catalogv1.Product, error)
}

type OrderPlacer interface {
	PlaceOrder(ctx context.Context, userID string, items []*orderv1.PlaceOrderItem) (*orderv1.PlaceOrderResponse, error)
}

type Checkout struct {
	cat CatalogProductGetter
	ord OrderPlacer
}

func NewCheckout(cat CatalogProductGetter, ord OrderPlacer) *Checkout {
	return &Checkout{cat: cat, ord: ord}
}

type checkoutItemReq struct {
	ProductID string `json:"product_id"`
	Quantity  int32  `json:"quantity"`
}

type checkoutReq struct {
	Items []checkoutItemReq `json:"items"`
}

type checkoutResp struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

func (h *Checkout) Post(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r.Context())
	if uid == "" {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}

	var req checkoutReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	if len(req.Items) == 0 {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", "items required")
		return
	}

	items := make([]*orderv1.PlaceOrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		p, err := h.cat.GetProduct(r.Context(), it.ProductID)
		if err != nil {
			httpx.WriteError(w, r, http.StatusBadGateway, "UPSTREAM_FAILED", "product lookup failed: "+err.Error())
			return
		}
		items = append(items, &orderv1.PlaceOrderItem{
			ProductId: it.ProductID, Quantity: it.Quantity, UnitPriceCents: p.PriceCents,
		})
	}

	res, err := h.ord.PlaceOrder(r.Context(), uid, items)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadGateway, "UPSTREAM_FAILED", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(checkoutResp{OrderID: res.OrderId, Status: res.Status})
}
