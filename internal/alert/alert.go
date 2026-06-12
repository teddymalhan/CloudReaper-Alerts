// Package alert defines the JSON contract that travels sender → API Gateway → SQS → notifier.
// Keeping it in one place means the producer (sender) and consumer (notifier Lambda) can never
// drift apart.
package alert

import "github.com/teddymalhan/aws-play/internal/detector"

// OrphanAlert is the message body queued for the notifier to render into Slack.
type OrphanAlert struct {
	Source                   string         `json:"source"`
	Region                   string         `json:"region"`
	AccountID                string         `json:"accountId"`
	ScanTimestamp            string         `json:"scanTimestamp"`
	TotalOrphans             int            `json:"totalOrphans"`
	EstimatedMonthlyWasteUSD float64        `json:"estimatedMonthlyWasteUsd"`
	BuildURL                 string         `json:"buildUrl,omitempty"`
	Findings                 []AlertFinding `json:"findings"`
}

// AlertFinding is a trimmed view of detector.Finding — only what the Slack message needs.
type AlertFinding struct {
	ResourceID              string  `json:"resourceId"`
	ResourceType            string  `json:"resourceType"`
	Reason                  string  `json:"reason"`
	EstimatedMonthlyCostUSD float64 `json:"estimatedMonthlyCostUsd"`
}

// FromReport builds an OrphanAlert from a detector Report. buildURL is optional CI context.
func FromReport(r detector.Report, buildURL string) OrphanAlert {
	findings := make([]AlertFinding, 0, len(r.Findings))
	for _, f := range r.Findings {
		findings = append(findings, AlertFinding{
			ResourceID:              f.ResourceID,
			ResourceType:            f.ResourceType,
			Reason:                  f.Reason,
			EstimatedMonthlyCostUSD: f.EstimatedMonthlyCostUSD,
		})
	}
	return OrphanAlert{
		Source:                   "orphan-watch",
		Region:                   r.Region,
		AccountID:                r.AccountID,
		ScanTimestamp:            r.ScanTimestamp,
		TotalOrphans:             r.Summary.TotalOrphans,
		EstimatedMonthlyWasteUSD: r.Summary.EstimatedMonthlyWasteUSD,
		BuildURL:                 buildURL,
		Findings:                 findings,
	}
}
