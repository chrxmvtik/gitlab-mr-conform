package rules

import (
	"fmt"

	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity/helper/codeowners"
	"gitlab-mr-conformity-bot/internal/conformity/helper/common"

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
func (r *ApprovalsRule) Check(mr *gitlabapi.MergeRequest, commits []*gitlabapi.Commit, approvals *common.Approvals, cos []*codeowners.PatternGroup, members []*gitlabapi.ProjectMember) (*RuleResult, error) {
	ruleResult := &RuleResult{}

	if !r.config.UseCodeowners {
		if approvals.ApprovalsCount < r.config.MinCount {
			ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Insufficient approvals (need %d, have %d)", r.config.MinCount, approvals.ApprovalsCount))
			ruleResult.Suggestion = append(ruleResult.Suggestion, "Wait for required approvals before merging")
		}
	} else {
		if len(cos) == 0 {
			ruleResult.Error = append(ruleResult.Error, "CODEOWNERS enabled, but could not process owners.")
			ruleResult.Suggestion = append(ruleResult.Suggestion, "Check .gitlab/CODEOWNERS file for validation errors.")
		} else {
			summary := codeowners.CreateCodeOwnersSummary(cos, approvals, members)
			if summary.AllPatternsApproved {
				return &RuleResult{Passed: true}, nil
			}

			aggregatedError, suggestion := summary.GenerateAggregatedOutput()

			ruleResult.Error = append(ruleResult.Error, aggregatedError)
			if suggestion != "" {
				ruleResult.Suggestion = append(ruleResult.Suggestion, suggestion)
			}
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
