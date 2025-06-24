package rules

import (
	"fmt"
	"strings"

	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity/helper/codeowners"
	"gitlab-mr-conformity-bot/internal/conformity/helper/common"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type BranchRule struct {
	config config.BranchConfig
}

func NewBranchRule(cfg interface{}) *BranchRule {
	branchCfg, ok := cfg.(config.BranchConfig)
	if !ok {
		branchCfg = config.BranchConfig{
			AllowedPrefixes: []string{"feature/", "bugfix/", "hotfix/"},
		}
	}
	return &BranchRule{config: branchCfg}
}

func (r *BranchRule) Name() string {
	return "Branch Naming"
}

func (r *BranchRule) Severity() Severity {
	return SeverityWarning
}

func (r *BranchRule) Check(mr *gitlabapi.MergeRequest, commits []*gitlabapi.Commit, approvals *common.Approvals, cos []*codeowners.PatternGroup, members []*gitlabapi.ProjectMember) (*RuleResult, error) {
	ruleResult := &RuleResult{}

	branchName := mr.SourceBranch

	// Check forbidden names
	for _, forbidden := range r.config.ForbiddenNames {
		if strings.EqualFold(branchName, forbidden) {
			ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Branch name '%s' is not allowed", branchName))
			ruleResult.Suggestion = append(ruleResult.Suggestion, "Use a more descriptive branch name")
			break
		}
	}

	// Check allowed prefixes
	if len(r.config.AllowedPrefixes) > 0 {
		hasValidPrefix := false
		for _, prefix := range r.config.AllowedPrefixes {
			if strings.HasPrefix(branchName, prefix) {
				hasValidPrefix = true
				break
			}
		}

		if !hasValidPrefix {
			ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Branch should start with: %s", strings.Join(r.config.AllowedPrefixes, ", ")))
			ruleResult.Suggestion = append(ruleResult.Suggestion, fmt.Sprintf("Rename branch to start with '%s'", r.config.AllowedPrefixes[0]))
		}
	}

	if len(ruleResult.Error) != 0 {
		return &RuleResult{
			Passed:     false,
			Error:      ruleResult.Error,
			Suggestion: ruleResult.Suggestion,
		}, nil
	}

	return &RuleResult{Passed: true}, nil
}
