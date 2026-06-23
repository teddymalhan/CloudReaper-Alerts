package slack

import (
	"strings"
	"testing"

	goslack "github.com/slack-go/slack"

	"github.com/teddymalhan/CloudReaper-Alerts/internal/alert"
)

// flatten extracts all rendered text from a set of blocks so tests can assert on content.
func flatten(blocks []goslack.Block) string {
	var b strings.Builder
	for _, blk := range blocks {
		switch v := blk.(type) {
		case *goslack.HeaderBlock:
			b.WriteString(v.Text.Text + "\n")
		case *goslack.SectionBlock:
			if v.Text != nil {
				b.WriteString(v.Text.Text + "\n")
			}
		case *goslack.ContextBlock:
			for _, el := range v.ContextElements.Elements {
				if t, ok := el.(*goslack.TextBlockObject); ok {
					b.WriteString(t.Text + "\n")
				}
			}
		}
	}
	return b.String()
}

func TestBuildBlocksWithOrphans(t *testing.T) {
	a := alert.OrphanAlert{
		Source: "orphan-watch", Region: "us-east-1", AccountID: "000000000000",
		ScanTimestamp: "2026-06-11T00:00:00Z", TotalOrphans: 2, EstimatedMonthlyWasteUSD: 4.29,
		BuildURL: "https://ci/42",
		Findings: []alert.AlertFinding{
			{ResourceID: "vol-1", ResourceType: "ebs_volume", Reason: "unattached", EstimatedMonthlyCostUSD: 0.64},
			{ResourceID: "eipalloc-1", ResourceType: "elastic_ip", Reason: "unassociated", EstimatedMonthlyCostUSD: 3.65},
		},
	}

	text := flatten(BuildBlocks(a))
	for _, want := range []string{
		"Orphaned Resources Detected", "*2*", "$4.29/month", "us-east-1",
		"vol-1", "eipalloc-1", "unattached", "$3.65/mo", "source: orphan-watch", "build",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("blocks missing %q\n---\n%s", want, text)
		}
	}
}

func TestBuildBlocksTruncates(t *testing.T) {
	a := alert.OrphanAlert{TotalOrphans: 12, Region: "us-east-1"}
	for i := 0; i < 12; i++ {
		a.Findings = append(a.Findings, alert.AlertFinding{ResourceID: "vol", ResourceType: "ebs_volume", Reason: "unattached"})
	}
	text := flatten(BuildBlocks(a))
	if !strings.Contains(text, "and 2 more") {
		t.Errorf("expected truncation note, got:\n%s", text)
	}
}

func TestBuildBlocksClean(t *testing.T) {
	a := alert.OrphanAlert{Source: "orphan-watch", Region: "us-east-1", AccountID: "0", TotalOrphans: 0}
	text := flatten(BuildBlocks(a))
	if !strings.Contains(text, "No Orphaned Resources") {
		t.Errorf("expected clean header, got:\n%s", text)
	}
}
