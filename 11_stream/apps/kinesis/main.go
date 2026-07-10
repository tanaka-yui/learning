package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
)

type Order struct {
	ID        string `json:"id"`
	Item      string `json:"item"`
	Amount    int    `json:"amount"`
	CreatedAt string `json:"created_at"`
}

func newOrder(i int) Order {
	return Order{
		ID:        fmt.Sprintf("order-%04d", i),
		Item:      "book",
		Amount:    1000 + i,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
}

func newKinesisClient(ctx context.Context) (*kinesis.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("ap-northeast-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		return nil, err
	}
	return kinesis.NewFromConfig(cfg, func(o *kinesis.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	}), nil
}

func connectHint(err error) error {
	if err != nil && (strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no such host")) {
		return fmt.Errorf("LocalStack に接続できません。`make up` を先に実行してください: %w", err)
	}
	return err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: kinesis <produce|consume> [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "produce":
		err = runProduce(os.Args[2:])
	case "consume":
		err = runConsume(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
