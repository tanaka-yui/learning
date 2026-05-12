package client

import (
	"context"

	userv1 "microservie/proto/gen/go/user/v1"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type UserAuth struct{ c userv1.UserServiceClient }

func DialUserAuth(addr string) (*UserAuth, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, err
	}
	return &UserAuth{c: userv1.NewUserServiceClient(conn)}, nil
}

func (u *UserAuth) SignUp(ctx context.Context, email, password string) (string, error) {
	r, err := u.c.SignUp(ctx, &userv1.SignUpRequest{Email: email, Password: password})
	if err != nil {
		return "", err
	}
	return r.UserId, nil
}

func (u *UserAuth) SignIn(ctx context.Context, email, password string) (string, error) {
	r, err := u.c.SignIn(ctx, &userv1.SignInRequest{Email: email, Password: password})
	if err != nil {
		return "", err
	}
	return r.Token, nil
}

func (u *UserAuth) ValidateToken(ctx context.Context, token string) (string, error) {
	r, err := u.c.ValidateToken(ctx, &userv1.ValidateTokenRequest{Token: token})
	if err != nil {
		return "", err
	}
	return r.UserId, nil
}
