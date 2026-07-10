package main

import (
	"fmt"
	"os"
	"strings"
	"time"
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

func connectHint(err error) error {
	if err != nil && (strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "unable to dial")) {
		return fmt.Errorf("Kafka に接続できません。`make up` を先に実行してください: %w", err)
	}
	return err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: kafka <produce|consume> [flags]")
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
