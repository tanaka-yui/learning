package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func runProduce(args []string) error {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	n := fs.Int("n", 5, "number of messages")
	fs.Parse(args)

	cl, err := kgo.NewClient(
		kgo.SeedBrokers("localhost:9092"),
		kgo.RecordDeliveryTimeout(10*time.Second), // 未起動時に素早く失敗させる
	)
	if err != nil {
		return err
	}
	defer cl.Close()

	ctx := context.Background()
	for i := 1; i <= *n; i++ {
		o := newOrder(i)
		body, _ := json.Marshal(o)
		// key に注文 ID を使う → 同じ key は必ず同じ partition = key 単位の順序保証
		rec := &kgo.Record{Topic: "orders", Key: []byte(o.ID), Value: body}
		if err := cl.ProduceSync(ctx, rec).FirstErr(); err != nil {
			return connectHint(err)
		}
		fmt.Printf("sent: key=%s partition=%d offset=%d %s\n", o.ID, rec.Partition, rec.Offset, body)
	}
	return nil
}
