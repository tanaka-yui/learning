package server_test

import (
	"context"
	"testing"
	"time"

	"microservie/user-auth/internal/jwt"
	"microservie/user-auth/internal/repo"
	"microservie/user-auth/internal/server"
	userv1 "microservie/proto/gen/go/user/v1"

	"golang.org/x/crypto/bcrypt"
)

type memRepo struct{ users map[string]repo.User }

func (m *memRepo) Create(ctx context.Context, email, hash string) (string, error) {
	if _, ok := m.users[email]; ok {
		return "", repo.ErrDuplicateEmail
	}
	id := "id-" + email
	m.users[email] = repo.User{ID: id, Email: email, PasswordHash: hash}
	return id, nil
}
func (m *memRepo) FindByEmail(ctx context.Context, email string) (repo.User, error) {
	u, ok := m.users[email]
	if !ok {
		return repo.User{}, repo.ErrUserNotFound
	}
	return u, nil
}

func TestSignUp_thenSignIn_returnsToken(t *testing.T) {
	r := &memRepo{users: map[string]repo.User{}}
	j := jwt.New([]byte("test-secret-32-bytes-long-padding"), time.Hour)
	s := server.New(r, j)

	_, err := s.SignUp(context.Background(), &userv1.SignUpRequest{Email: "a@x.com", Password: "pw"})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	res, err := s.SignIn(context.Background(), &userv1.SignInRequest{Email: "a@x.com", Password: "pw"})
	if err != nil || res.Token == "" {
		t.Fatalf("SignIn: token=%q err=%v", res.GetToken(), err)
	}
}

func TestSignIn_wrongPasswordReturnsUnauthenticated(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("right"), bcrypt.DefaultCost)
	r := &memRepo{users: map[string]repo.User{"a@x.com": {ID: "u-1", Email: "a@x.com", PasswordHash: string(hash)}}}
	j := jwt.New([]byte("test-secret-32-bytes-long-padding"), time.Hour)
	s := server.New(r, j)

	_, err := s.SignIn(context.Background(), &userv1.SignInRequest{Email: "a@x.com", Password: "wrong"})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestValidateToken_returnsUserID(t *testing.T) {
	j := jwt.New([]byte("test-secret-32-bytes-long-padding"), time.Hour)
	token, _ := j.Issue("user-42")
	s := server.New(&memRepo{}, j)

	res, err := s.ValidateToken(context.Background(), &userv1.ValidateTokenRequest{Token: token})
	if err != nil || res.UserId != "user-42" {
		t.Fatalf("res=%v err=%v", res, err)
	}
}
