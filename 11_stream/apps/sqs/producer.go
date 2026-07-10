package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func runProduce(args []string) error {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	n := fs.Int("n", 5, "number of messages")
	queue := fs.String("queue", "orders", "queue name")
	fs.Parse(args)

	ctx := context.Background()
	client, err := newSQSClient(ctx)
	if err != nil {
		return err
	}
	urlOut, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: queue})
	if err != nil {
		return connectHint(err)
	}
	for i := 1; i <= *n; i++ {
		body, _ := json.Marshal(newOrder(i))
		if _, err := client.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:    urlOut.QueueUrl,
			MessageBody: aws.String(string(body)),
		}); err != nil {
			return connectHint(err)
		}
		fmt.Printf("sent: %s\n", body)
	}
	return nil
}
