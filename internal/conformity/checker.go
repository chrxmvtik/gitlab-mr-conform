package conformity

import (
	"fmt"

	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity/rules"
	"gitlab-mr-conformity-bot/internal/gitlab"
	"gitlab-mr-conformity-bot/pkg/logger"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type Checker struct {
	configLoader     *config.ConfigLoader
	ruleBuilder      *RuleBuilder
	summaryGenerator *SummaryGenerator
	gitlabClient     *gitlab.Client
	logger           *logger.Logger
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

func NewChecker(defaultConfig config.RulesConfig, client *gitlab.Client, log *logger.Logger) *Checker {
	return &Checker{
		configLoader:     config.NewConfigLoader(defaultConfig, client, log),
		ruleBuilder:      NewRuleBuilder(),
		summaryGenerator: NewSummaryGenerator(),
		gitlabClient:     client,
		logger:           log,
	}
}

func (c *Checker) CheckMergeRequest(projectID interface{}, mrID int) (*CheckResult, error) {
	// Load configuration (repository or default)
	finalConfig, err := c.configLoader.LoadConfig(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Build rules based on configuration
	rulesList := c.ruleBuilder.BuildRules(finalConfig)

	// Get merge request and commits
	mr, commits, err := c.fetchMergeRequestData(projectID, mrID)
	if err != nil {
		return nil, err
	}

	// Execute rule checks
	failures := c.executeRuleChecks(rulesList, mr, commits)

	// Generate results
	passed := len(failures) == 0
	summary := c.summaryGenerator.GenerateSummary(failures)

	return &CheckResult{
		Passed:   passed,
		Failures: failures,
		Summary:  summary,
	}, nil
}

// fetchMergeRequestData retrieves merge request and commit data
func (c *Checker) fetchMergeRequestData(projectID interface{}, mrID int) (*gitlabapi.MergeRequest, []*gitlabapi.Commit, error) {
	// Get merge request details
	mr, err := c.gitlabClient.GetMergeRequest(projectID, mrID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get merge request: %w", err)
	}

	// Get commits for commit-related rules
	commits, err := c.gitlabClient.ListMergeRequestCommits(projectID, mrID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get commits: %w", err)
	}

	return mr, commits, nil
}

// executeRuleChecks runs all rules and collects failures
func (c *Checker) executeRuleChecks(rulesList []rules.Rule, mr *gitlabapi.MergeRequest, commits []*gitlabapi.Commit) []RuleFailure {
	var failures []RuleFailure

	for _, rule := range rulesList {
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

	return failures
}
