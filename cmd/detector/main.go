// Command detector scans an AWS account (LocalStack by default) for orphaned resources,
// writes report.json + report.md, and exits non-zero when orphans are found so CI can gate on it.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/teddymalhan/CloudReaper-Alerts/internal/detector"
)

func main() {
	var (
		endpoint   = flag.String("endpoint", os.Getenv("AWS_ENDPOINT_URL"), "AWS endpoint override (LocalStack/Floci)")
		region     = flag.String("region", envOr("AWS_REGION", "us-east-1"), "AWS region")
		accountID  = flag.String("account", defaultAccountID(), "AWS account id (defaults to STS for real AWS)")
		reportPath = flag.String("report", envOr("REPORT_PATH", "report.json"), "path to write JSON report")
		mdPath     = flag.String("markdown", envOr("REPORT_MD_PATH", "report.md"), "path to write Markdown report")
	)
	flag.Parse()

	ctx := context.Background()

	cfg, err := loadAWSConfig(ctx, *region, *endpoint)
	if err != nil {
		log.Fatalf("load aws config: %v", err)
	}

	client := ec2.NewFromConfig(cfg, func(o *ec2.Options) {
		if *endpoint != "" {
			o.BaseEndpoint = aws.String(*endpoint)
		}
	})

	reportAccountID := *accountID
	if reportAccountID == "" {
		reportAccountID, err = currentAccountID(ctx, cfg)
		if err != nil {
			log.Fatalf("resolve account id: %v", err)
		}
	}

	report, err := detector.Scan(ctx, client, *region, reportAccountID)
	if err != nil {
		log.Fatalf("scan: %v", err)
	}

	if err := writeJSON(*reportPath, report); err != nil {
		log.Fatalf("write json: %v", err)
	}
	if err := os.WriteFile(*mdPath, []byte(report.Markdown()), 0o644); err != nil {
		log.Fatalf("write markdown: %v", err)
	}

	fmt.Printf("Scan complete — %d orphan(s), $%.2f/month estimated waste\n",
		report.Summary.TotalOrphans, report.Summary.EstimatedMonthlyWasteUSD)
	fmt.Printf("Reports written to %s and %s\n", *reportPath, *mdPath)

	if report.Summary.TotalOrphans > 0 {
		os.Exit(1) // signal "orphans found" to the pipeline
	}
}

func writeJSON(path string, report detector.Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func loadAWSConfig(ctx context.Context, region, endpoint string) (aws.Config, error) {
	if endpoint != "" {
		return config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
		)
	}
	return config.LoadDefaultConfig(ctx, config.WithRegion(region))
}

func currentAccountID(ctx context.Context, cfg aws.Config) (string, error) {
	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.Account), nil
}

func defaultAccountID() string {
	if v := os.Getenv("ACCOUNT_ID"); v != "" {
		return v
	}
	if os.Getenv("AWS_ENDPOINT_URL") != "" {
		return "000000000000"
	}
	return ""
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
