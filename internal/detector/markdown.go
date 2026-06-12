package detector

import (
	"fmt"
	"strings"
)

// Markdown renders a human-readable report, mirroring dummy-infra's report.md.
func (r Report) Markdown() string {
	var b strings.Builder
	b.WriteString("# Cost Janitor Report\n\n")
	fmt.Fprintf(&b, "**Scanned:** %s  \n", r.ScanTimestamp)
	fmt.Fprintf(&b, "**Region:** %s  \n", r.Region)
	fmt.Fprintf(&b, "**Total orphans:** %d  \n", r.Summary.TotalOrphans)
	fmt.Fprintf(&b, "**Estimated monthly waste:** $%.2f\n\n", r.Summary.EstimatedMonthlyWasteUSD)

	if len(r.Findings) == 0 {
		b.WriteString("No orphaned resources found.\n")
		return b.String()
	}

	b.WriteString("## Findings\n\n")
	b.WriteString("| Resource ID | Type | Reason | Age (days) | Est. Cost/month |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "| %s | %s | %s | %d | $%.2f |\n",
			f.ResourceID, f.ResourceType, f.Reason, f.AgeDays, f.EstimatedMonthlyCostUSD)
	}
	return b.String()
}
