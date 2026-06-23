package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	goslack "github.com/slack-go/slack"

	"github.com/teddymalhan/CloudReaper-Alerts/internal/alert"
)

type fakePoster struct {
	calls   int
	channel string
	blocks  []goslack.Block
	err     error
}

func (f *fakePoster) Post(_ context.Context, channelID string, blocks []goslack.Block) error {
	f.calls++
	f.channel = channelID
	f.blocks = blocks
	return f.err
}

func sqsRecord(t *testing.T, a alert.OrphanAlert) events.SQSMessage {
	t.Helper()
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	return events.SQSMessage{MessageId: "m-1", Body: string(body)}
}

func TestHandleDelivers(t *testing.T) {
	poster := &fakePoster{}
	h := Handler{Poster: poster, ChannelID: "C123"}
	a := alert.OrphanAlert{Source: "orphan-watch", TotalOrphans: 1, Region: "us-east-1",
		Findings: []alert.AlertFinding{{ResourceID: "vol-1", ResourceType: "ebs_volume", Reason: "unattached"}}}

	if err := h.Handle(context.Background(), events.SQSEvent{Records: []events.SQSMessage{sqsRecord(t, a)}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if poster.calls != 1 || poster.channel != "C123" {
		t.Errorf("expected 1 call to C123, got %d to %q", poster.calls, poster.channel)
	}
	if len(poster.blocks) == 0 {
		t.Error("expected blocks to be built and passed")
	}
}

func TestHandleSkipsMalformed(t *testing.T) {
	poster := &fakePoster{}
	h := Handler{Poster: poster, ChannelID: "C123"}
	ev := events.SQSEvent{Records: []events.SQSMessage{{MessageId: "bad", Body: "{not json"}}}

	if err := h.Handle(context.Background(), ev); err != nil {
		t.Fatalf("malformed message should not error the batch: %v", err)
	}
	if poster.calls != 0 {
		t.Errorf("expected no delivery for malformed message, got %d", poster.calls)
	}
}

func TestHandleReturnsErrorOnDeliveryFailure(t *testing.T) {
	poster := &fakePoster{err: errors.New("slack down")}
	h := Handler{Poster: poster, ChannelID: "C123"}
	ev := events.SQSEvent{Records: []events.SQSMessage{sqsRecord(t, alert.OrphanAlert{TotalOrphans: 1})}}

	if err := h.Handle(context.Background(), ev); err == nil {
		t.Fatal("expected error so SQS retries the batch")
	}
}
