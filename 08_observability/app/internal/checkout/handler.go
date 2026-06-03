package checkout

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
		return
	}
	res, err := h.svc.Checkout(r.Context(), req)
	switch {
	case errors.Is(err, ErrInvalidRequest):
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	case errors.Is(err, ErrChargeFailed):
		http.Error(w, `{"error":"charge failed"}`, http.StatusBadGateway)
		return
	case err != nil:
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}
