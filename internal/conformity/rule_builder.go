package conformity

import (
	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity/rules"
)

// RuleBuilder handles building rules from configuration
type RuleBuilder struct{}

// NewRuleBuilder creates a new rule builder
func NewRuleBuilder() *RuleBuilder {
	return &RuleBuilder{}
}

// BuildRules creates rules based on the provided config
func (rb *RuleBuilder) BuildRules(rulesConfig config.RulesConfig) []rules.Rule {
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

	return rulesList
}
