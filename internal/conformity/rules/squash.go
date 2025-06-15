package rules

import (
	"fmt"

	"gitlab-mr-conformity-bot/internal/config"

	doublestar "github.com/bmatcuk/doublestar/v4"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type SquashRule struct {
	config config.SquashConfig
}

func NewSquashRule(cfg interface{}) *SquashRule {
	squashCfg, ok := cfg.(config.SquashConfig)
	if !ok {
		squashCfg = config.SquashConfig{
			EnforceBranches:  []string{"feature/*", "fix/*"},
			DisallowBranches: []string{"release/*"},
		}
	}
	return &SquashRule{config: squashCfg}
}

func (r *SquashRule) Name() string {
	return "Squash enforce"
}

func (r *SquashRule) Severity() Severity {
	return SeverityError
}

func (r *SquashRule) Check(mr *gitlabapi.MergeRequest, commits []*gitlabapi.Commit, approvals *gitlabapi.MergeRequestApprovals) (*RuleResult, error) {
	branchName := mr.SourceBranch
	matched := false
	ruleResult := &RuleResult{}

	// Check if squash is enforced for matching patterns
	for _, pattern := range r.config.EnforceBranches {
		match, err := doublestar.PathMatch(pattern, branchName)
		if err != nil {
			return nil, fmt.Errorf("invalid enforce pattern '%s': %v", pattern, err)
		}
		if match {
			matched = true
			if mr.SquashOnMerge {
				return &RuleResult{Passed: true}, nil
			}
			ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Branch '%s' must use squash on merge (matched enforce pattern: %s)", branchName, pattern))
			ruleResult.Suggestion = append(ruleResult.Suggestion, "Enable squash on merge")
			break
		}
	}

	// Check if squash is disallowed for matching patterns
	for _, pattern := range r.config.DisallowBranches {
		match, err := doublestar.PathMatch(pattern, branchName)
		if err != nil {
			return nil, fmt.Errorf("invalid disallow pattern '%s': %v", pattern, err)
		}
		if match {
			matched = true
			if !mr.SquashOnMerge {
				return &RuleResult{Passed: true}, nil
			}
			ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Branch '%s' must not use squash on merge (matched disallow pattern: %s)", branchName, pattern))
			ruleResult.Suggestion = append(ruleResult.Suggestion, "Disable squash on merge")
			break
		}
	}

	// Default behavior for unmatched branches: require squash
	if !matched {
		if mr.SquashOnMerge {
			return &RuleResult{Passed: true}, nil
		}
		ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Branch '%s' is not matched by any rule and must squash on merge by default", branchName))
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Enable squash on merge")
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
