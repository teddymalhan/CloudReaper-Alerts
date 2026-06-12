package detector

import (
	"strings"
	"testing"
)

func TestMarkdownWithFindings(t *testing.T) {
	r := Report{
		ScanTimestamp: "2026-06-11T00:00:00Z",
		Region:        "us-east-1",
		Summary:       Summary{TotalOrphans: 1, EstimatedMonthlyWasteUSD: 0.64},
		Findings: []Finding{{
			ResourceID: "vol-1", ResourceType: "ebs_volume", Reason: "unattached",
			AgeDays: 3, EstimatedMonthlyCostUSD: 0.64,
		}},
	}
	md := r.Markdown()
	for _, want := range []string{"# Cost Janitor Report", "**Total orphans:** 1", "$0.64", "vol-1", "## Findings"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n---\n%s", want, md)
		}
	}
}

func TestMarkdownClean(t *testing.T) {
	r := Report{Summary: Summary{TotalOrphans: 0}}
	if md := r.Markdown(); !strings.Contains(md, "No orphaned resources found.") {
		t.Errorf("expected clean message, got:\n%s", md)
	}
}
