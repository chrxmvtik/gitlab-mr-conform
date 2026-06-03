package conformity

import (
	"encoding/base64"
	"fmt"
	"log"
	"sort"
	"strings"

	"gitlab-mr-conformity-bot/internal/cache"
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
	Skipped  bool
	Failures []RuleFailure
	Summary  string
}

type RuleFailure struct {
	RuleName   string
	Severity   rules.Severity
	Error      []string
	Suggestion []string
}

func NewChecker(defaultConfig config.RulesConfig, client *gitlab.Client, log *logger.Logger, integrations config.IntegrationsConfig) *Checker {
	return NewCheckerWithCache(defaultConfig, client, log, integrations, nil)
}

func NewCheckerWithCache(defaultConfig config.RulesConfig, client *gitlab.Client, log *logger.Logger, integrations config.IntegrationsConfig, c cache.Cache) *Checker {
	return &Checker{
		configLoader:     config.NewConfigLoaderWithCache(defaultConfig, client, log, c),
		ruleBuilder:      NewRuleBuilder(integrations),
		summaryGenerator: NewSummaryGenerator(),
		gitlabClient:     client,
		logger:           log,
	}
}

func (c *Checker) CheckMergeRequest(projectID interface{}, mrID int) (*CheckResult, error) {
	finalConfig, presence, err := c.configLoader.LoadConfig(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	if presence == config.ConfigNotFound {
		c.logger.Info("Skipping repository: no .mr-conform.yaml found", "project_id", projectID)
		return &CheckResult{Skipped: true}, nil
	}

	rulesList := c.ruleBuilder.BuildRules(finalConfig)
	mr, commits, approvals, err := c.fetchMergeRequestData(projectID, mrID, finalConfig)
	if err != nil {
		return nil, err
	}

	var co []*codeowners.PatternGroup
	var members []*gitlabapi.ProjectMember

	if finalConfig.Approvals.UseCodeowners {
		members, err = c.gitlabClient.ListProjectMembers(projectID)
		if err != nil {
			c.logger.Info("Failed to list project members", "error", err)
		}
		co, err = c.getCodeowners(projectID, mrID, members)
		if err != nil {
			c.logger.Info("No CODEOWNERS file found in repository, skipping", "error", err)
		}
	}

	failures := c.executeRuleChecks(rulesList, mr, commits, approvals, co, members)
	passed := len(failures) == 0
	summary := c.summaryGenerator.GenerateSummary(failures)

	return &CheckResult{
		Passed:   passed,
		Failures: failures,
		Summary:  summary,
	}, nil
}

func (c *Checker) fetchMergeRequestData(projectID interface{}, mrID int, finalConfig config.RulesConfig) (*gitlabapi.MergeRequest, []*gitlabapi.Commit, *common.Approvals, error) {
	mr, err := c.gitlabClient.GetMergeRequest(projectID, mrID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get merge request: %w", err)
	}

	approvals, err := c.gitlabClient.ListMergeRequestApprovals(projectID, mrID, mr.Author.ID, finalConfig.Approvals.ExcludeCreatorFromCount)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get merge request: %w", err)
	}

	commits, err := c.gitlabClient.ListMergeRequestCommits(projectID, mrID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get commits: %w", err)
	}

	return mr, commits, approvals, nil
}

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
	co, err := c.gitlabClient.GetCodeownersFile(projectID)
	if err != nil {
		c.logger.Debug("No CODEOWNERS file found in repository, skipping", "error", err)
		return nil, err
	}

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
