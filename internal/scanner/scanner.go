// Package scanner runs the scheduled account scan and enqueues Slack alerts.
package scanner

import (
	"context"
	"log"

	"github.com/teddymalhan/CloudReaper-Alerts/internal/alert"
	"github.com/teddymalhan/CloudReaper-Alerts/internal/detector"
	"github.com/teddymalhan/CloudReaper-Alerts/internal/sender"
)

// Event is the optional payload accepted by direct Lambda invokes.
type Event struct {
	// SendClean sends a "no orphaned resources" Slack message when the scan is clean.
	// Scheduled runs leave this false to avoid noisy recurring success messages.
	SendClean bool   `json:"sendClean"`
	BuildURL  string `json:"buildUrl,omitempty"`
}

// Result is returned to direct Lambda invocations for CLI feedback.
type Result struct {
	TotalOrphans             int     `json:"totalOrphans"`
	EstimatedMonthlyWasteUSD float64 `json:"estimatedMonthlyWasteUsd"`
	AlertQueued              bool    `json:"alertQueued"`
}

// Handler is the Lambda entry point. Dependencies are interfaces so scans are unit-testable.
type Handler struct {
	EC2       detector.EC2API
	SQS       sender.SQSAPI
	QueueURL  string
	Region    string
	AccountID string
}

// Handle scans the account and queues an alert when findings exist. Clean scans are silent unless
// SendClean is true, which powers `cloudreaper test-alert`.
func (h Handler) Handle(ctx context.Context, ev Event) (Result, error) {
	report, err := detector.Scan(ctx, h.EC2, h.Region, h.AccountID)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		TotalOrphans:             report.Summary.TotalOrphans,
		EstimatedMonthlyWasteUSD: report.Summary.EstimatedMonthlyWasteUSD,
	}
	if report.Summary.TotalOrphans == 0 && !ev.SendClean {
		log.Printf("scan clean for account %s region %s; alert suppressed", h.AccountID, h.Region)
		return result, nil
	}

	a := alert.FromReport(report, ev.BuildURL)
	if err := sender.Enqueue(ctx, h.SQS, h.QueueURL, a); err != nil {
		return Result{}, err
	}
	result.AlertQueued = true
	log.Printf("queued scan alert (%d orphan(s), $%.2f/mo)", result.TotalOrphans, result.EstimatedMonthlyWasteUSD)
	return result, nil
}
