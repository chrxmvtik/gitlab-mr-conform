package conformity

import (
	"fmt"
	"sort"

	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity/rules"
	"gitlab-mr-conformity-bot/internal/gitlab"
	"gitlab-mr-conformity-bot/pkg/logger"
)

type Checker struct {
	rules        []rules.Rule
	gitlabClient *gitlab.Client
	logger       *logger.Logger
}

type CheckResult struct {
	Passed   bool
	Failures []RuleFailure
	Summary  string
}

type RuleFailure struct {
	RuleName   string
	Severity   rules.Severity
	Error      []string
	Suggestion []string
}

func NewChecker(rulesConfig config.RulesConfig, client *gitlab.Client, log *logger.Logger) *Checker {
	var rulesList []rules.Rule

	// Conditionally initialize rules based on configuration
	if rulesConfig.Title.Enabled {
		rulesList = append(rulesList, rules.NewTitleRule(rulesConfig.Title))
	}
	if rulesConfig.Description.Enabled {
		rulesList = append(rulesList, rules.NewDescriptionRule(rulesConfig.Description))
	}
	if rulesConfig.Branch.Enabled {
		rulesList = append(rulesList, rules.NewBranchRule(rulesConfig.Branch))
	}
	if rulesConfig.Commits.Enabled {
		rulesList = append(rulesList, rules.NewCommitsRule(rulesConfig.Commits))
	}
	if rulesConfig.Approvals.Enabled {
		rulesList = append(rulesList, rules.NewApprovalsRule(rulesConfig.Approvals))
	}
	if rulesConfig.Squash.Enabled {
		rulesList = append(rulesList, rules.NewSquashRule(rulesConfig.Squash))
	}

	return &Checker{
		rules:        rulesList,
		gitlabClient: client,
		logger:       log,
	}
}

func (c *Checker) CheckMergeRequest(projectID interface{}, mrID int) (*CheckResult, error) {
	// Get merge request details
	mr, err := c.gitlabClient.GetMergeRequest(projectID, mrID)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request: %w", err)
	}

	// Get commits for commit-related rules
	commits, err := c.gitlabClient.ListMergeRequestCommits(projectID, mrID)
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	var failures []RuleFailure

	// Check each rule
	for _, rule := range c.rules {
		c.logger.Debug("Checking rule", "rule", rule.Name())

		result, err := rule.Check(mr, commits)
		if err != nil {
			c.logger.Error("Rule check failed", "rule", rule.Name(), "error", err)
			continue
		}

		if !result.Passed {
			failures = append(failures, RuleFailure{
				RuleName:   rule.Name(),
				Severity:   rule.Severity(),
				Error:      result.Error,
				Suggestion: result.Suggestion,
			})
		}
	}

	passed := len(failures) == 0
	summary := c.generateSummary(failures)

	return &CheckResult{
		Passed:   passed,
		Failures: failures,
		Summary:  summary,
	}, nil
}

func (c *Checker) generateSummary(failures []RuleFailure) string {
	if len(failures) == 0 {
		return "## 🧾 **MR Conformity Check Summary**\n\n✅ **All conformity checks passed!**"
	}

	summary := fmt.Sprintf("## 🧾 **MR Conformity Check Summary**\n\n### ❌ %d conformity check(s) failed:\n\n---\n\n", len(failures))

	sort.Slice(failures, func(i, j int) bool {
		// Sort with higher severity first
		return failures[i].Severity > failures[j].Severity
	})

	for _, failure := range failures {
		emoji := "⚠️"
		if failure.Severity == rules.SeverityError {
			emoji = "❌"
		}

		summary += fmt.Sprintf("#### %s **%s**\n\n", emoji, failure.RuleName)
		for count, e := range failure.Error {
			summary += fmt.Sprintf("📄 **Issue %d**: %s\n", count+1, e)
			summary += fmt.Sprintf(">💡 **Tip**: %s", failure.Suggestion[count])
			summary += "\n---\n\n"
		}

		summary += "\n---\n\n"
	}

	return summary
}
