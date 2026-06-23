// Package slack renders an OrphanAlert into Slack Block Kit blocks. Pure functions, no I/O,
// so they are fully unit-testable.
package slack

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"

	"github.com/teddymalhan/CloudReaper-Alerts/internal/alert"
)

// maxListed caps how many findings we enumerate in the message body.
const maxListed = 10

// BuildBlocks turns an OrphanAlert into Slack Block Kit blocks.
func BuildBlocks(a alert.OrphanAlert) []slack.Block {
	if a.TotalOrphans == 0 {
		return cleanBlocks(a)
	}

	header := slack.NewHeaderBlock(
		slack.NewTextBlockObject(slack.PlainTextType, ":broom: Orphaned Resources Detected", true, false),
	)

	summary := slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf(
			":x: *%d* orphaned resource(s) wasting *$%.2f/month*\n*Region:* %s   *Account:* %s",
			a.TotalOrphans, a.EstimatedMonthlyWasteUSD, a.Region, a.AccountID,
		), false, false),
		nil, nil,
	)

	var b strings.Builder
	listed := a.Findings
	if len(listed) > maxListed {
		listed = listed[:maxListed]
	}
	for _, f := range listed {
		fmt.Fprintf(&b, "• *%s* `%s` — %s ($%.2f/mo)\n",
			f.ResourceType, f.ResourceID, f.Reason, f.EstimatedMonthlyCostUSD)
	}
	if len(a.Findings) > maxListed {
		fmt.Fprintf(&b, "_…and %d more_\n", len(a.Findings)-maxListed)
	}
	details := slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType, b.String(), false, false), nil, nil)

	blocks := []slack.Block{header, summary, slack.NewDividerBlock(), details}
	if ctx := contextBlock(a); ctx != nil {
		blocks = append(blocks, ctx)
	}
	return blocks
}

func cleanBlocks(a alert.OrphanAlert) []slack.Block {
	header := slack.NewHeaderBlock(
		slack.NewTextBlockObject(slack.PlainTextType, ":white_check_mark: No Orphaned Resources", true, false),
	)
	body := slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType,
			fmt.Sprintf("Scan of *%s* (account %s) found no orphaned resources.", a.Region, a.AccountID),
			false, false),
		nil, nil)
	blocks := []slack.Block{header, body}
	if ctx := contextBlock(a); ctx != nil {
		blocks = append(blocks, ctx)
	}
	return blocks
}

func contextBlock(a alert.OrphanAlert) *slack.ContextBlock {
	parts := []string{fmt.Sprintf("source: %s", a.Source)}
	if a.ScanTimestamp != "" {
		parts = append(parts, fmt.Sprintf("scanned: %s", a.ScanTimestamp))
	}
	if a.BuildURL != "" {
		parts = append(parts, fmt.Sprintf("<%s|build>", a.BuildURL))
	}
	if len(parts) == 0 {
		return nil
	}
	return slack.NewContextBlock("",
		slack.NewTextBlockObject(slack.MarkdownType, strings.Join(parts, " · "), false, false))
}
