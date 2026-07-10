package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-stomp/stomp/v3"
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

// destination で queue（競合コンシューマ）と topic（fan-out）を切り替えるのが STOMP 流。
func destination(topic bool) string {
	if topic {
		return "/topic/orders"
	}
	return "/queue/orders"
}

func dial() (*stomp.Conn, error) {
	conn, err := stomp.Dial("tcp", "localhost:61613",
		stomp.ConnOpt.Login("admin", "admin"),
	)
	if err != nil && strings.Contains(err.Error(), "connection refused") {
		return nil, fmt.Errorf("ActiveMQ に接続できません。`make up` を先に実行してください: %w", err)
	}
	return conn, err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: activemq <produce|consume> [flags]")
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
