// Package notifier consumes SQS records carrying OrphanAlerts and posts them to Slack.
// The Slack dependency is an interface so the handler is unit-testable without network access.
package notifier

import (
	"context"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"
	goslack "github.com/slack-go/slack"

	"github.com/teddymalhan/aws-play/internal/alert"
	"github.com/teddymalhan/aws-play/internal/slack"
)

// SlackPoster sends pre-built blocks to a channel. The real implementation wraps slack-go.
type SlackPoster interface {
	Post(ctx context.Context, channelID string, blocks []goslack.Block) error
}

// Handler renders and delivers orphan alerts.
type Handler struct {
	Poster    SlackPoster
	ChannelID string
}

// Handle processes an SQS batch. A malformed record is logged and skipped (retrying it would
// never succeed); a Slack delivery failure returns an error so SQS retries the batch.
func (h Handler) Handle(ctx context.Context, ev events.SQSEvent) error {
	for _, rec := range ev.Records {
		var a alert.OrphanAlert
		if err := json.Unmarshal([]byte(rec.Body), &a); err != nil {
			log.Printf("skipping malformed message %s: %v", rec.MessageId, err)
			continue
		}
		blocks := slack.BuildBlocks(a)
		if err := h.Poster.Post(ctx, h.ChannelID, blocks); err != nil {
			log.Printf("slack delivery failed for %s: %v", rec.MessageId, err)
			return err // trigger SQS retry / eventual DLQ
		}
		log.Printf("delivered orphan alert (%d orphan(s)) for message %s", a.TotalOrphans, rec.MessageId)
	}
	return nil
}
