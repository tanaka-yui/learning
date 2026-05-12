package server

import (
	"context"
	"errors"

	"microservie/user-auth/internal/jwt"
	"microservie/user-auth/internal/repo"
	userv1 "microservie/proto/gen/go/user/v1"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserRepo interface {
	Create(ctx context.Context, email, hash string) (string, error)
	FindByEmail(ctx context.Context, email string) (repo.User, error)
}

type Server struct {
	r   UserRepo
	jwt *jwt.Manager
}

func New(r UserRepo, j *jwt.Manager) *Server { return &Server{r: r, jwt: j} }

func (s *Server) SignUp(ctx context.Context, req *userv1.SignUpRequest) (*userv1.SignUpResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	id, err := s.r.Create(ctx, req.Email, string(hash))
	if errors.Is(err, repo.ErrDuplicateEmail) {
		return nil, status.Error(codes.AlreadyExists, "email taken")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.SignUpResponse{UserId: id}, nil
}

func (s *Server) SignIn(ctx context.Context, req *userv1.SignInRequest) (*userv1.SignInResponse, error) {
	u, err := s.r.FindByEmail(ctx, req.Email)
	if errors.Is(err, repo.ErrUserNotFound) {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	token, err := s.jwt.Issue(u.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.SignInResponse{Token: token}, nil
}

func (s *Server) ValidateToken(ctx context.Context, req *userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
	uid, err := s.jwt.Verify(req.Token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	return &userv1.ValidateTokenResponse{UserId: uid}, nil
}
