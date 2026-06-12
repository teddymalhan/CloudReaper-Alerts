// Package reactor is the event-driven front-end of orphan-watch. Instead of polling on a schedule,
// it reacts to EC2 "AWS API Call via CloudTrail" events delivered by EventBridge the moment a
// resource may have become orphaned, runs a targeted check, and — if the resource really is
// orphaned now — enqueues the same OrphanAlert the scheduled scanner would, so the downstream
// SQS → notifier Lambda → Slack tail is unchanged.
package reactor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"

	"github.com/teddymalhan/aws-play/internal/alert"
	"github.com/teddymalhan/aws-play/internal/detector"
	"github.com/teddymalhan/aws-play/internal/sender"
)

// Detail is the subset of a CloudTrail event record (the `detail` of an EventBridge event) we read.
type Detail struct {
	EventName         string          `json:"eventName"`
	EventSource       string          `json:"eventSource"`
	AWSRegion         string          `json:"awsRegion"`
	RequestParameters json.RawMessage `json:"requestParameters"`
}

// Check routes a single CloudTrail event to the targeted orphan check for its action and returns
// only resources that are *currently* orphaned. Events for unhandled actions yield no findings.
//
// Of the orphan-creating EC2 actions, only DetachVolume carries a directly re-checkable id; for
// DisassociateAddress and TerminateInstances the affected id is gone by the time the event lands
// (the association/instance no longer exists), so those fall back to a focused re-scan of the
// resource type the action can orphan.
func Check(ctx context.Context, c detector.EC2API, d Detail) ([]detector.Finding, error) {
	switch d.EventName {
	case "DetachVolume":
		var rp struct {
			VolumeID string `json:"volumeId"`
		}
		if err := json.Unmarshal(d.RequestParameters, &rp); err != nil {
			return nil, fmt.Errorf("parse DetachVolume requestParameters: %w", err)
		}
		if rp.VolumeID == "" {
			return nil, fmt.Errorf("DetachVolume event missing volumeId")
		}
		return detector.CheckVolume(ctx, c, rp.VolumeID)

	case "DisassociateAddress":
		// associationId in the request no longer resolves to an allocation; re-scan EIPs instead.
		return detector.FindUnassociatedEIPs(ctx, c)

	case "TerminateInstances":
		// Termination can leave volumes (DeleteOnTermination=false) and disassociate EIPs.
		vols, err := detector.FindUnattachedEBS(ctx, c)
		if err != nil {
			return nil, err
		}
		eips, err := detector.FindUnassociatedEIPs(ctx, c)
		if err != nil {
			return nil, err
		}
		return append(vols, eips...), nil

	default:
		return nil, nil
	}
}

// Handler is the Lambda entry point. It mirrors notifier.Handler: dependencies are interfaces so
// the handler is unit-testable without AWS.
type Handler struct {
	EC2      detector.EC2API
	SQS      sender.SQSAPI
	QueueURL string
}

// Handle processes one EventBridge CloudTrail event. A non-orphaned result is a no-op (the common
// case — most detaches/terminations are immediately followed by cleanup). A confirmed orphan is
// enqueued to SQS for the notifier. An enqueue failure is returned so Lambda retries.
func (h Handler) Handle(ctx context.Context, ev events.CloudWatchEvent) error {
	var d Detail
	if err := json.Unmarshal(ev.Detail, &d); err != nil {
		return fmt.Errorf("parse event detail: %w", err)
	}

	findings, err := Check(ctx, h.EC2, d)
	if err != nil {
		return fmt.Errorf("targeted check for %s: %w", d.EventName, err)
	}
	if len(findings) == 0 {
		log.Printf("%s on resource is not orphaned — no alert", d.EventName)
		return nil
	}

	report := detector.NewReport(findings, ev.Region, ev.AccountID)
	a := alert.FromReport(report, "")
	a.Source = "orphan-watch:reactor" // distinguish event-driven alerts from scheduled scans in Slack

	if err := sender.Enqueue(ctx, h.SQS, h.QueueURL, a); err != nil {
		return fmt.Errorf("enqueue alert for %s: %w", d.EventName, err)
	}
	log.Printf("enqueued event-driven alert for %s (%d orphan(s), $%.2f/mo)", d.EventName, a.TotalOrphans, a.EstimatedMonthlyWasteUSD)
	return nil
}
