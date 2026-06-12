// Command sender reads report.json and, when orphans were found, POSTs an OrphanAlert to the
// pipeline endpoint (API Gateway → SQS → notifier Lambda → Slack).
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/teddymalhan/aws-play/internal/alert"
	"github.com/teddymalhan/aws-play/internal/sender"
)

func main() {
	var (
		reportPath = flag.String("report", envOr("REPORT_PATH", "report.json"), "path to detector report.json")
		endpoint   = flag.String("endpoint", envOr("SLACKED_ENDPOINT", ""), "pipeline HTTP endpoint (API Gateway /send-message)")
		buildURL   = flag.String("build-url", os.Getenv("BUILD_URL"), "optional CI build URL for context")
		always     = flag.Bool("always", false, "send even when no orphans are found")
	)
	flag.Parse()

	if *endpoint == "" {
		log.Fatal("no endpoint: set -endpoint or SLACKED_ENDPOINT")
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
	client := &http.Client{Timeout: 15 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := sender.Post(ctx, client, *endpoint, a); err != nil {
		log.Fatalf("send alert: %v", err)
	}
	log.Printf("sent orphan alert (%d orphan(s), $%.2f/mo) to %s",
		a.TotalOrphans, a.EstimatedMonthlyWasteUSD, *endpoint)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
