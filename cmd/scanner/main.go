// Command scanner is the scheduled Lambda that scans AWS for orphaned resources and enqueues alerts.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/teddymalhan/CloudReaper-Alerts/internal/scanner"
)

func main() {
	ctx := context.Background()

	queueURL := os.Getenv("QUEUE_URL")
	if queueURL == "" {
		log.Fatal("QUEUE_URL env var not set")
	}
	region := envOr("AWS_REGION", "us-east-1")
	accountID := os.Getenv("ACCOUNT_ID")
	if accountID == "" {
		log.Fatal("ACCOUNT_ID env var not set")
	}

	cfg, err := loadAWSConfig(ctx, region)
	if err != nil {
		log.Fatalf("load aws config: %v", err)
	}
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

	handler := scanner.Handler{EC2: ec2Client, SQS: sqsClient, QueueURL: queueURL, Region: region, AccountID: accountID}
	lambda.Start(handler.Handle)
}

func loadAWSConfig(ctx context.Context, region string) (aws.Config, error) {
	if endpoint := os.Getenv("AWS_ENDPOINT_URL"); endpoint != "" {
		return config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
		)
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return aws.Config{}, err
	}
	if cfg.Region == "" {
		return aws.Config{}, fmt.Errorf("AWS region not configured")
	}
	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
