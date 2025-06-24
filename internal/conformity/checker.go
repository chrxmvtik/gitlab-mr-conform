package conformity

import (
	"encoding/base64"
	"fmt"
	"log"
	"sort"
	"strings"

	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity/helper/codeowners"
	"gitlab-mr-conformity-bot/internal/conformity/helper/common"
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
	mr, commits, approvals, err := c.fetchMergeRequestData(projectID, mrID)
	if err != nil {
		return nil, err
	}

	var co []*codeowners.PatternGroup

	var members []*gitlabapi.ProjectMember

	if finalConfig.Approvals.UseCodeowners {
		// Get project members
		members, err = c.gitlabClient.ListProjectMembers(projectID)
		if err != nil {
			c.logger.Info("Failed to list project members", "error", err)
		}
		// Get CODEOWNERS file from repository
		co, err = c.getCodeowners(projectID, mrID, members)
		if err != nil {
			c.logger.Info("No CODEOWNERS file found in repository, skipping", "error", err)
		}

	}

	// Execute rule checks
	failures := c.executeRuleChecks(rulesList, mr, commits, approvals, co, members)

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
func (c *Checker) fetchMergeRequestData(projectID interface{}, mrID int) (*gitlabapi.MergeRequest, []*gitlabapi.Commit, *common.Approvals, error) {
	// Get merge request details
	mr, err := c.gitlabClient.GetMergeRequest(projectID, mrID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get merge request: %w", err)
	}
	// Get mr approvers
	approvals, err := c.gitlabClient.ListMergeRequestApprovals(projectID, mrID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get merge request: %w", err)
	}

	// Get commits for commit-related rules
	commits, err := c.gitlabClient.ListMergeRequestCommits(projectID, mrID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get commits: %w", err)
	}

	return mr, commits, approvals, nil
}

// executeRuleChecks runs all rules and collects failures
func (c *Checker) executeRuleChecks(rulesList []rules.Rule, mr *gitlabapi.MergeRequest, commits []*gitlabapi.Commit, approvals *common.Approvals, codeowners []*codeowners.PatternGroup, members []*gitlabapi.ProjectMember) []RuleFailure {
	var failures []RuleFailure

	for _, rule := range rulesList {
		c.logger.Debug("Checking rule", "rule", rule.Name())

		result, err := rule.Check(mr, commits, approvals, codeowners, members)
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

func (c *Checker) getCodeowners(projectID interface{}, mrID int, members []*gitlabapi.ProjectMember) ([]*codeowners.PatternGroup, error) {
	// Try to get CODEOWNERS file from repository
	co, err := c.gitlabClient.GetCodeownersFile(projectID)
	if err != nil {
		c.logger.Debug("No CODEOWNERS file found in repository, skipping", "error", err)
		return nil, err
	}

	// Decode the base64 content
	decoded, err := base64.StdEncoding.DecodeString(co.Content)
	if err != nil {
		c.logger.Warn("Failed to decode config file from repository, using default config", "error", err)
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	parser := codeowners.NewCodeownersParser(c.logger)
	for _, member := range members {
		parser.AddAccessibleUser(member.Username)
		parser.AddAccessibleRole(int(member.AccessLevel))
		parser.AddAccessibleEmail(member.Email)
	}

	cos, err := parser.Parse(strings.NewReader(string(decoded)))
	if err != nil {
		c.logger.Fatal("Error parsing CODEOWNERS: %v", err)
	}

	paths, err := c.gitlabClient.GetAllDiffsPaths(projectID, mrID)
	if err != nil {
		log.Fatalf("Error obtaining diff paths: %v", err)
	}

	// Get only active patterns (final effective patterns)
	coGrp := codeowners.GetActivePatternAggregation(cos, paths)
	var sortedGroups []*codeowners.PatternGroup
	for _, pg := range coGrp.PatternGroups {
		sortedGroups = append(sortedGroups, pg)
	}

	sort.Slice(sortedGroups, func(i, j int) bool {
		return sortedGroups[i].Pattern < sortedGroups[j].Pattern
	})

	return sortedGroups, nil
}
