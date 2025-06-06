package rules

import (
	"fmt"
	"strings"

	"gitlab-mr-conformity-bot/internal/config"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type DescriptionRule struct {
	config config.DescriptionConfig
}

func NewDescriptionRule(cfg interface{}) *DescriptionRule {
	descCfg, ok := cfg.(config.DescriptionConfig)
	if !ok {
		descCfg = config.DescriptionConfig{
			Required:  true,
			MinLength: 20,
		}
	}
	return &DescriptionRule{config: descCfg}
}

func (r *DescriptionRule) Name() string {
	return "Description Validation"
}

func (r *DescriptionRule) Severity() Severity {
	return SeverityWarning
}

func (r *DescriptionRule) Check(mr *gitlabapi.MergeRequest, commits []*gitlabapi.Commit) (*RuleResult, error) {
	description := strings.TrimSpace(mr.Description)
	ruleResult := &RuleResult{}

	if r.config.Required && description == "" {
		ruleResult.Error = append(ruleResult.Error, "Description is required")
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Add a description explaining the changes in this merge request")
	}

	if description != "" && len(description) < r.config.MinLength {
		ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Description too short (minimum %d characters)", r.config.MinLength))
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Provide more details about the changes")
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
