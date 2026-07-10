package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
)

func runProduce(args []string) error {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	n := fs.Int("n", 5, "number of records")
	fs.Parse(args)

	ctx := context.Background()
	client, err := newKinesisClient(ctx)
	if err != nil {
		return err
	}
	for i := 1; i <= *n; i++ {
		o := newOrder(i)
		body, _ := json.Marshal(o)
		// PartitionKey が shard を決める（Kafka の key ≈ PartitionKey）
		out, err := client.PutRecord(ctx, &kinesis.PutRecordInput{
			StreamName:   aws.String("orders"),
			PartitionKey: aws.String(o.ID),
			Data:         body,
		})
		if err != nil {
			return connectHint(err)
		}
		fmt.Printf("sent: shard=%s seq=%s %s\n", *out.ShardId, *out.SequenceNumber, body)
	}
	return nil
}
