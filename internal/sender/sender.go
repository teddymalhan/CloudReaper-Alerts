// Package sender reads a detector report and POSTs an OrphanAlert to the pipeline's HTTP
// endpoint (API Gateway), which forwards it to SQS for the notifier Lambda.
package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/teddymalhan/aws-play/internal/alert"
	"github.com/teddymalhan/aws-play/internal/detector"
)

// LoadReport reads a detector.Report from a JSON file.
func LoadReport(path string) (detector.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return detector.Report{}, fmt.Errorf("read report %s: %w", path, err)
	}
	var r detector.Report
	if err := json.Unmarshal(data, &r); err != nil {
		return detector.Report{}, fmt.Errorf("parse report %s: %w", path, err)
	}
	return r, nil
}

// Post sends the alert as JSON to endpoint using client. A non-2xx response is an error.
func Post(ctx context.Context, client *http.Client, endpoint string, a alert.OrphanAlert) error {
	body, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post to %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("endpoint returned %s: %s", resp.Status, string(respBody))
	}
	return nil
}
