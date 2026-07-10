package main

import (
	"encoding/json"
	"flag"
	"fmt"
)

func runProduce(args []string) error {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	n := fs.Int("n", 5, "number of messages")
	topic := fs.Bool("topic", false, "send to /topic/orders instead of /queue/orders")
	fs.Parse(args)

	conn, err := dial()
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	dest := destination(*topic)
	for i := 1; i <= *n; i++ {
		body, _ := json.Marshal(newOrder(i))
		if err := conn.Send(dest, "application/json", body); err != nil {
			return err
		}
		fmt.Printf("sent to %s: %s\n", dest, body)
	}
	return nil
}
