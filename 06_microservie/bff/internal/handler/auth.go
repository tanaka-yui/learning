package handler

import (
	"context"
	"encoding/json"
	"net/http"
)

type AuthClient interface {
	SignUp(ctx context.Context, email, password string) (string, error)
	SignIn(ctx context.Context, email, password string) (string, error)
}

type Auth struct{ c AuthClient }

func NewAuth(c AuthClient) *Auth { return &Auth{c: c} }

func (a *Auth) SignUp(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	uid, err := a.c.SignUp(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"user_id": uid})
}

func (a *Auth) SignIn(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	token, err := a.c.SignIn(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, "invalid credentials", 401)
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
