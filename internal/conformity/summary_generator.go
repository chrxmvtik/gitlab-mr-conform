package conformity

import (
	"fmt"
	"sort"

	"gitlab-mr-conformity-bot/internal/conformity/rules"
)

// SummaryGenerator handles generating summaries for check results
type SummaryGenerator struct{}

// NewSummaryGenerator creates a new summary generator
func NewSummaryGenerator() *SummaryGenerator {
	return &SummaryGenerator{}
}

// GenerateSummary creates a formatted summary from rule failures
func (sg *SummaryGenerator) GenerateSummary(failures []RuleFailure) string {
	if len(failures) == 0 {
		return sg.generateSuccessSummary()
	}

	return sg.generateFailureSummary(failures)
}

// generateSuccessSummary creates a summary for when all checks pass
func (sg *SummaryGenerator) generateSuccessSummary() string {
	return "## 🧾 **Merge Request Compliance Report**\n\n✅ **All conformity checks passed!**"
}

// generateFailureSummary creates a summary for when checks fail
func (sg *SummaryGenerator) generateFailureSummary(failures []RuleFailure) string {
	summary := fmt.Sprintf("## 🧾 **Merge Request Compliance Report**\n\n### ❌ %d conformity check(s) failed:\n\n---\n\n", len(failures))

	// Sort failures by severity (higher severity first)
	sortedFailures := sg.sortFailuresBySeverity(failures)

	for _, failure := range sortedFailures {
		summary += sg.formatFailure(failure)
	}

	return summary
}

// sortFailuresBySeverity sorts failures with higher severity first
func (sg *SummaryGenerator) sortFailuresBySeverity(failures []RuleFailure) []RuleFailure {
	sortedFailures := make([]RuleFailure, len(failures))
	copy(sortedFailures, failures)

	sort.Slice(sortedFailures, func(i, j int) bool {
		return sortedFailures[i].Severity > sortedFailures[j].Severity
	})

	return sortedFailures
}

// formatFailure formats a single rule failure
func (sg *SummaryGenerator) formatFailure(failure RuleFailure) string {
	emoji := sg.getSeverityEmoji(failure.Severity)

	summary := fmt.Sprintf("#### %s **%s**\n\n", emoji, failure.RuleName)

	for count, e := range failure.Error {
		summary += fmt.Sprintf("📄 **Issue %d**: %s\n", count+1, e)
		if count < len(failure.Suggestion) {
			summary += fmt.Sprintf(">💡 **Tip**: %s", failure.Suggestion[count])
		}
		summary += "\n---\n\n"
	}

	summary += "\n---\n\n"
	return summary
}

// getSeverityEmoji returns the appropriate emoji for a given severity
func (sg *SummaryGenerator) getSeverityEmoji(severity rules.Severity) string {
	if severity == rules.SeverityError {
		return "❌"
	}
	return "⚠️"
}
