package detector

// Finding describes a single orphaned (or non-compliant) resource. The JSON shape mirrors
// dummy-infra's report.json so existing tooling/expectations carry over.
type Finding struct {
	ResourceID              string             `json:"resource_id"`
	ResourceType            string             `json:"resource_type"`
	Reason                  string             `json:"reason"`
	AgeDays                 int                `json:"age_days"`
	EstimatedMonthlyCostUSD float64            `json:"estimated_monthly_cost_usd"`
	Tags                    map[string]*string `json:"tags"`
	SuggestedAction         string             `json:"suggested_action"`
	SafeToAutoDelete        bool               `json:"safe_to_auto_delete"`
}

// Summary is the roll-up over all findings.
type Summary struct {
	TotalOrphans             int     `json:"total_orphans"`
	EstimatedMonthlyWasteUSD float64 `json:"estimated_monthly_waste_usd"`
}

// Report is the full scan result written to report.json.
type Report struct {
	ScanTimestamp string    `json:"scan_timestamp"`
	AccountID     string    `json:"account_id"`
	Region        string    `json:"region"`
	Summary       Summary   `json:"summary"`
	Findings      []Finding `json:"findings"`
}
