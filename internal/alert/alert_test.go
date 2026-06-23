package alert

import (
	"testing"

	"github.com/teddymalhan/CloudReaper-Alerts/internal/detector"
)

func TestFromReport(t *testing.T) {
	r := detector.Report{
		Region:        "us-east-1",
		AccountID:     "000000000000",
		ScanTimestamp: "2026-06-11T00:00:00Z",
		Summary:       detector.Summary{TotalOrphans: 2, EstimatedMonthlyWasteUSD: 4.29},
		Findings: []detector.Finding{
			{ResourceID: "vol-1", ResourceType: "ebs_volume", Reason: "unattached", EstimatedMonthlyCostUSD: 0.64},
			{ResourceID: "eipalloc-1", ResourceType: "elastic_ip", Reason: "unassociated", EstimatedMonthlyCostUSD: 3.65},
		},
	}

	a := FromReport(r, "https://ci/build/42")

	if a.Source != "orphan-watch" {
		t.Errorf("source: got %q", a.Source)
	}
	if a.TotalOrphans != 2 || a.EstimatedMonthlyWasteUSD != 4.29 {
		t.Errorf("summary not carried: %+v", a)
	}
	if a.Region != "us-east-1" || a.AccountID != "000000000000" || a.BuildURL != "https://ci/build/42" {
		t.Errorf("metadata not carried: %+v", a)
	}
	if len(a.Findings) != 2 || a.Findings[0].ResourceID != "vol-1" || a.Findings[1].EstimatedMonthlyCostUSD != 3.65 {
		t.Errorf("findings not carried: %+v", a.Findings)
	}
}
