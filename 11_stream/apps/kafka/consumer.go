package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func runConsume(args []string) error {
	fs := flag.NewFlagSet("consume", flag.ExitOnError)
	max := fs.Int("max", 5, "expected number of messages")
	group := fs.String("group", "demo", "consumer group id")
	fromBeginning := fs.Bool("from-beginning", false, "start from the earliest offset when the group has no committed offset")
	timeout := fs.Duration("timeout", 15*time.Second, "overall deadline")
	fs.Parse(args)

	opts := []kgo.Opt{
		kgo.SeedBrokers("localhost:9092"),
		kgo.ConsumerGroup(*group),
		kgo.ConsumeTopics("orders"),
	}
	if *fromBeginning {
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return err
	}
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	received := 0
	for received < *max {
		fetches := cl.PollFetches(ctx)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			break
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			return connectHint(fmt.Errorf("fetch errors: %v", errs))
		}
		iter := fetches.RecordIter()
		for !iter.Done() {
			rec := iter.Next()
			received++
			fmt.Printf("received: group=%s partition=%d offset=%d key=%s %s\n",
				*group, rec.Partition, rec.Offset, rec.Key, rec.Value)
		}
	}
	// group の committed offset を確定させてから終了（次回は続きから読む）
	if err := cl.CommitUncommittedOffsets(context.Background()); err != nil {
		return err
	}
	if received < *max {
		return fmt.Errorf("expected %d messages, got %d", *max, received)
	}
	return nil
}
