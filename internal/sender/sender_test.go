package sender

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/teddymalhan/aws-play/internal/alert"
	"github.com/teddymalhan/aws-play/internal/detector"
)

type fakeSQS struct {
	gotQueueURL string
	gotBody     string
	err         error
}

func (f *fakeSQS) SendMessage(_ context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.gotQueueURL = aws.ToString(in.QueueUrl)
	f.gotBody = aws.ToString(in.MessageBody)
	return &sqs.SendMessageOutput{}, f.err
}

func TestEnqueueSendsAlert(t *testing.T) {
	f := &fakeSQS{}
	a := alert.OrphanAlert{Source: "orphan-watch", TotalOrphans: 2}
	if err := Enqueue(context.Background(), f, "http://localhost:4566/000000000000/q", a); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.gotQueueURL != "http://localhost:4566/000000000000/q" {
		t.Errorf("queue url: got %q", f.gotQueueURL)
	}
	var sent alert.OrphanAlert
	if err := json.Unmarshal([]byte(f.gotBody), &sent); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if sent.Source != "orphan-watch" || sent.TotalOrphans != 2 {
		t.Errorf("body not delivered intact: %+v", sent)
	}
}

func TestPostSendsJSON(t *testing.T) {
	var gotMethod, gotCT string
	var gotBody alert.OrphanAlert
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"queued"}`))
	}))
	defer srv.Close()

	a := alert.OrphanAlert{Source: "orphan-watch", TotalOrphans: 2, EstimatedMonthlyWasteUSD: 4.29}
	if err := Post(context.Background(), srv.Client(), srv.URL, a); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: got %s", gotMethod)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type: got %s", gotCT)
	}
	if gotBody.Source != "orphan-watch" || gotBody.TotalOrphans != 2 {
		t.Errorf("body not delivered intact: %+v", gotBody)
	}
}

func TestPostNon2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	if err := Post(context.Background(), srv.Client(), srv.URL, alert.OrphanAlert{}); err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestLoadReport(t *testing.T) {
	r := detector.Report{
		Region:  "us-east-1",
		Summary: detector.Summary{TotalOrphans: 1, EstimatedMonthlyWasteUSD: 0.64},
		Findings: []detector.Finding{
			{ResourceID: "vol-1", ResourceType: "ebs_volume", Reason: "unattached", EstimatedMonthlyCostUSD: 0.64},
		},
	}
	data, _ := json.MarshalIndent(r, "", "  ")
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadReport(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Summary.TotalOrphans != 1 || got.Findings[0].ResourceID != "vol-1" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestLoadReportMissingFile(t *testing.T) {
	if _, err := LoadReport(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
