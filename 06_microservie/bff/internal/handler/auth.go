package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"microservie/bff/internal/httpx"
	"microservie/bff/internal/middleware"
)

type AuthClient interface {
	SignUp(ctx context.Context, email, password string) (string, error)
	SignIn(ctx context.Context, email, password string) (string, error)
	GetUser(ctx context.Context, userID string) (string, error)
}

type Auth struct{ c AuthClient }

func NewAuth(c AuthClient) *Auth { return &Auth{c: c} }

func (a *Auth) SignUp(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	uid, err := a.c.SignUp(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"user_id": uid})
}

func (a *Auth) SignIn(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	token, err := a.c.SignIn(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "invalid credentials")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (a *Auth) Me(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserID(r.Context())
	if uid == "" {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	email, err := a.c.GetUser(r.Context(), uid)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "user not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"user_id": uid, "email": email})
}

func (a *Auth) SignOut(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
	w.WriteHeader(http.StatusNoContent)
}
