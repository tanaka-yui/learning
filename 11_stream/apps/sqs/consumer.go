package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func runConsume(args []string) error {
	fs := flag.NewFlagSet("consume", flag.ExitOnError)
	max := fs.Int("max", 5, "expected number of messages")
	queue := fs.String("queue", "orders", "queue name")
	noDelete := fs.Bool("no-delete", false, "receive without deleting (visibility timeout demo)")
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

	received, emptyPolls := 0, 0
	for received < *max && emptyPolls < 3 {
		out, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:                    urlOut.QueueUrl,
			MaxNumberOfMessages:         10,
			WaitTimeSeconds:             2, // long polling
			MessageSystemAttributeNames: []types.MessageSystemAttributeName{types.MessageSystemAttributeNameApproximateReceiveCount},
		})
		if err != nil {
			return connectHint(err)
		}
		if len(out.Messages) == 0 {
			emptyPolls++
			continue
		}
		for _, m := range out.Messages {
			received++
			fmt.Printf("received (receiveCount=%s): %s\n", m.Attributes["ApproximateReceiveCount"], *m.Body)
			if !*noDelete {
				if _, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
					QueueUrl:      urlOut.QueueUrl,
					ReceiptHandle: m.ReceiptHandle,
				}); err != nil {
					return connectHint(err)
				}
			}
		}
	}
	if received < *max {
		return fmt.Errorf("expected %d messages, got %d", *max, received)
	}
	return nil
}
