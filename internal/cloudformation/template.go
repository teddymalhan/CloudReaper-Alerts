package cloudformation

import _ "embed"

// Template is the AWS-native CloudReaper deployment template.
//
//go:embed cloudreaper-alerts.yaml
var Template string
