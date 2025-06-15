package rules

import (
	"fmt"

	"gitlab-mr-conformity-bot/internal/config"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type ApprovalsRule struct {
	config config.ApprovalsConfig
}

func NewApprovalsRule(cfg interface{}) *ApprovalsRule {
	approvalsCfg, ok := cfg.(config.ApprovalsConfig)
	if !ok {
		approvalsCfg = config.ApprovalsConfig{
			MinCount: 1,
		}
	}
	return &ApprovalsRule{config: approvalsCfg}
}

func (r *ApprovalsRule) Name() string {
	return "Approvals Required"
}

func (r *ApprovalsRule) Severity() Severity {
	return SeverityError
}

func (r *ApprovalsRule) Check(mr *gitlabapi.MergeRequest, commits []*gitlabapi.Commit, approvals *gitlabapi.MergeRequestApprovals) (*RuleResult, error) {
	ruleResult := &RuleResult{}

	approvalsCount := len(approvals.ApprovedBy)

	if approvalsCount < r.config.MinCount {
		ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Insufficient approvals (need %d, have %d)", r.config.MinCount, approvalsCount))
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Wait for required approvals before merging")
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
