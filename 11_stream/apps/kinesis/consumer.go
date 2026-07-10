package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
)

func runConsume(args []string) error {
	fs := flag.NewFlagSet("consume", flag.ExitOnError)
	max := fs.Int("max", 5, "expected number of records")
	iterType := fs.String("iterator", "TRIM_HORIZON", "shard iterator type: TRIM_HORIZON|LATEST")
	timeout := fs.Duration("timeout", 15*time.Second, "overall deadline")
	fs.Parse(args)

	ctx := context.Background()
	client, err := newKinesisClient(ctx)
	if err != nil {
		return err
	}

	// Kafka と違い consumer group はない。shard を列挙し iterator を自分で管理する
	ls, err := client.ListShards(ctx, &kinesis.ListShardsInput{StreamName: aws.String("orders")})
	if err != nil {
		return connectHint(err)
	}
	itOut, err := client.GetShardIterator(ctx, &kinesis.GetShardIteratorInput{
		StreamName:        aws.String("orders"),
		ShardId:           ls.Shards[0].ShardId,
		ShardIteratorType: types.ShardIteratorType(*iterType),
	})
	if err != nil {
		return err
	}

	iter := itOut.ShardIterator
	received := 0
	deadline := time.Now().Add(*timeout)
	for received < *max && time.Now().Before(deadline) {
		out, err := client.GetRecords(ctx, &kinesis.GetRecordsInput{
			ShardIterator: iter,
			Limit:         aws.Int32(100),
		})
		if err != nil {
			return connectHint(err)
		}
		for _, r := range out.Records {
			received++
			fmt.Printf("received: seq=%s %s\n", *r.SequenceNumber, r.Data)
		}
		iter = out.NextShardIterator
		if len(out.Records) == 0 {
			// 実 AWS では GetRecords は shard あたり 5 回/秒まで。ポーリング間隔を空ける
			time.Sleep(500 * time.Millisecond)
		}
	}
	if received < *max {
		return fmt.Errorf("expected %d records, got %d", *max, received)
	}
	return nil
}
