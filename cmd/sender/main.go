// Command sender reads report.json and, when orphans were found, delivers an OrphanAlert into
// the pipeline. Two modes:
//
//	-endpoint URL   POST to the API Gateway /send-message (real AWS / slacked-style front door)
//	-queue-url URL  SendMessage straight to SQS (local/Floci, where API GW→SQS is unsupported)
//
// Both put the same message on the queue, so SQS → Lambda → Slack is identical downstream.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/teddymalhan/aws-play/internal/alert"
	"github.com/teddymalhan/aws-play/internal/sender"
)

func main() {
	var (
		reportPath  = flag.String("report", envOr("REPORT_PATH", "report.json"), "path to detector report.json")
		endpoint    = flag.String("endpoint", envOr("SLACKED_ENDPOINT", ""), "API Gateway /send-message URL (HTTP mode)")
		queueURL    = flag.String("queue-url", os.Getenv("QUEUE_URL"), "SQS queue URL (direct-SQS mode)")
		awsEndpoint = flag.String("aws-endpoint", envOr("AWS_ENDPOINT_URL", "http://localhost:4566"), "AWS endpoint for SQS mode")
		region      = flag.String("region", envOr("AWS_REGION", "us-east-1"), "AWS region for SQS mode")
		buildURL    = flag.String("build-url", os.Getenv("BUILD_URL"), "optional CI build URL for context")
		always      = flag.Bool("always", false, "send even when no orphans are found")
	)
	flag.Parse()

	if *endpoint == "" && *queueURL == "" {
		log.Fatal("nothing to send to: set -queue-url (SQS) or -endpoint (HTTP)")
	}

	report, err := sender.LoadReport(*reportPath)
	if err != nil {
		log.Fatalf("load report: %v", err)
	}
	if report.Summary.TotalOrphans == 0 && !*always {
		log.Printf("no orphans found — skipping Slack notification (use -always to override)")
		return
	}

	a := alert.FromReport(report, *buildURL)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if *queueURL != "" {
		if err := enqueue(ctx, *awsEndpoint, *region, *queueURL, a); err != nil {
			log.Fatalf("enqueue alert: %v", err)
		}
		log.Printf("queued orphan alert (%d orphan(s), $%.2f/mo) to %s", a.TotalOrphans, a.EstimatedMonthlyWasteUSD, *queueURL)
		return
	}

	client := &http.Client{Timeout: 15 * time.Second}
	if err := sender.Post(ctx, client, *endpoint, a); err != nil {
		log.Fatalf("send alert: %v", err)
	}
	log.Printf("sent orphan alert (%d orphan(s), $%.2f/mo) to %s", a.TotalOrphans, a.EstimatedMonthlyWasteUSD, *endpoint)
}

func enqueue(ctx context.Context, awsEndpoint, region, queueURL string, a alert.OrphanAlert) error {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		return err
	}
	client := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		if awsEndpoint != "" {
			o.BaseEndpoint = aws.String(awsEndpoint)
		}
	})
	return sender.Enqueue(ctx, client, queueURL, a)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
