package rules

import (
	"context"
	"fmt"
	"strings"

	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity/helper/codeowners"
	"gitlab-mr-conformity-bot/internal/conformity/helper/common"
	"gitlab-mr-conformity-bot/internal/conformity/validator"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type DescriptionRule struct {
	config           config.DescriptionConfig
	ticketValidators *validator.ValidatorManager
}

func NewDescriptionRule(cfg interface{}) *DescriptionRule {
	descCfg, ok := cfg.(config.DescriptionConfig)
	if !ok {
		descCfg = config.DescriptionConfig{
			Required:  true,
			MinLength: 20,
		}
	}
	return &DescriptionRule{
		config:           descCfg,
		ticketValidators: validator.BuildTicketValidators(descCfg.Jira, descCfg.Asana),
	}
}

func (r *DescriptionRule) Name() string {
	return "Description Validation"
}

func (r *DescriptionRule) Severity() Severity {
	return SeverityWarning
}

func (r *DescriptionRule) Check(mr *gitlabapi.MergeRequest, commits []*gitlabapi.Commit, approvals *common.Approvals, cos []*codeowners.PatternGroup, members []*gitlabapi.ProjectMember) (*RuleResult, error) {
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

	// Ticket validation (Jira, Asana, etc.)
	if r.ticketValidators.HasValidators() && description != "" {
		ctx := context.Background()
		result := r.ticketValidators.ValidateMessage(ctx, description)

		if result.AllMissing {
			ruleResult.Error = append(ruleResult.Error, "No ticket reference found in description")
			ruleResult.Suggestion = append(ruleResult.Suggestion, "Include a ticket reference (e.g., Jira: [ABC-123], Asana: https://app.asana.com/.../1234567890123456)")
		} else if !result.AnyValid {
			for name, valResult := range result.Results {
				if !valResult.Valid {
					ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("%s: %s", name, valResult.Error))
					ruleResult.Suggestion = append(ruleResult.Suggestion, "Use a valid ticket reference from one of the configured systems")
				}
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
