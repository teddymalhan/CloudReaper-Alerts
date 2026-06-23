// Command reactor is the event-driven Lambda. EventBridge invokes it with an EC2 "AWS API Call via
// CloudTrail" event (DetachVolume / DisassociateAddress / TerminateInstances); the reactor confirms
// the resource is actually orphaned now and, if so, enqueues an OrphanAlert to SQS — the same queue
// the scheduled scanner uses, so SQS → notifier → Slack is identical downstream.
package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/teddymalhan/CloudReaper-Alerts/internal/reactor"
)

func main() {
	ctx := context.Background()

	queueURL := os.Getenv("QUEUE_URL")
	if queueURL == "" {
		log.Fatal("QUEUE_URL env var not set")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("load aws config: %v", err)
	}

	// Floci/LocalStack: terraform injects AWS_ENDPOINT_URL so the SDK targets the local gateway.
	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	ec2Client := ec2.NewFromConfig(cfg, func(o *ec2.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	sqsClient := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	handler := reactor.Handler{EC2: ec2Client, SQS: sqsClient, QueueURL: queueURL}
	lambda.Start(handler.Handle)
}
