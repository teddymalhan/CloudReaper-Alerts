package detector

// Pricing + policy constants, ported from dummy-infra's janitor so cost estimates match.
const (
	EBSGp3PerGBMonth     = 0.08   // $/GB-month for gp3 volumes
	EC2T3MicroPerHour    = 0.0104 // $/hour on-demand Linux t3.micro
	EIPPerHour           = 0.005  // $/hour for an unassociated Elastic IP
	HoursPerMonth        = 730    // AWS convention
	StoppedDaysThreshold = 14     // an instance stopped longer than this is considered orphaned
)

// RequiredTags must be present on every taggable resource; missing ones are flagged.
var RequiredTags = []string{"Project", "Environment", "Owner"}
