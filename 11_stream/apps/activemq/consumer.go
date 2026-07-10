package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/go-stomp/stomp/v3"
)

func runConsume(args []string) error {
	fs := flag.NewFlagSet("consume", flag.ExitOnError)
	max := fs.Int("max", 5, "expected number of messages")
	topic := fs.Bool("topic", false, "subscribe /topic/orders instead of /queue/orders")
	timeout := fs.Duration("timeout", 15*time.Second, "overall deadline")
	fs.Parse(args)

	conn, err := dial()
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	dest := destination(*topic)
	sub, err := conn.Subscribe(dest, stomp.AckAuto)
	if err != nil {
		return err
	}

	received := 0
	deadline := time.After(*timeout)
	for received < *max {
		select {
		case msg := <-sub.C:
			if msg.Err != nil {
				return msg.Err
			}
			received++
			fmt.Printf("received from %s: %s\n", dest, msg.Body)
		case <-deadline:
			return fmt.Errorf("expected %d messages, got %d (timeout)", *max, received)
		}
	}
	return nil
}
