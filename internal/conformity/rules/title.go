package rules

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity/helper/codeowners"
	"gitlab-mr-conformity-bot/internal/conformity/helper/common"
	"gitlab-mr-conformity-bot/internal/conformity/validator"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type TitleRule struct {
	config           config.TitleConfig
	ticketValidators *validator.ValidatorManager
}

func NewTitleRule(cfg interface{}) *TitleRule {
	titleCfg, ok := cfg.(config.TitleConfig)
	if !ok {
		titleCfg = config.TitleConfig{
			MinLength: 10,
			MaxLength: 100,
			Conventional: config.ConventionalConfig{
				Types:  []string{"feat"},
				Scopes: []string{".*"},
			},
			Jira: config.JiraConfig{
				Keys: []string{""},
			},
		}
	}
	return &TitleRule{
		config:           titleCfg,
		ticketValidators: validator.BuildTicketValidators(titleCfg.Jira, titleCfg.Asana),
	}
}

func (r *TitleRule) Name() string {
	return "Title Validation"
}

func (r *TitleRule) Severity() Severity {
	return SeverityError
}

func (r *TitleRule) Check(mr *gitlabapi.MergeRequest, commits []*gitlabapi.Commit, approvals *common.Approvals, cos []*codeowners.PatternGroup, members []*gitlabapi.ProjectMember) (*RuleResult, error) {
	ruleResult := &RuleResult{}

	title := mr.Title

	// Length Checks
	if len(title) < r.config.MinLength {
		ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Title too short (minimum %d characters)", r.config.MinLength))
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Provide a more descriptive title")
	}

	if len(title) > r.config.MaxLength {
		ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Title too long (maximum %d characters)", r.config.MaxLength))
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Shorten the title while keeping it descriptive")
	}

	// Forbidden Words
	titleLower := strings.ToLower(title)
	for _, word := range r.config.ForbiddenWords {
		if strings.Contains(titleLower, strings.ToLower(word)) {
			ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Title contains forbidden word: %s", word))
			ruleResult.Suggestion = append(ruleResult.Suggestion, "Remove or replace the forbidden word")
			break
		}
	}

	// Conventional Commit Check
	groups := common.ParseHeader(title)
	if len(groups) != 7 {
		ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Invalid Conventional Commit format in title: %q", title))
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Use format:  \n> ```  \n> type(scope?): description  \n> ```\n> Example:  \n`feat(auth): add login retry mechanism`\n\n")
	} else if len(groups) == 7 {

		ccType := groups[1]
		ccScope := groups[3]

		// Type Validation
		typeIsValid := false
		for _, t := range r.config.Conventional.Types {
			if t == ccType {
				typeIsValid = true
				break
			}
		}
		if !typeIsValid {
			ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Invalid type %q: allowed types are %v", ccType, r.config.Conventional.Types))
			ruleResult.Suggestion = append(ruleResult.Suggestion, fmt.Sprintf("Use one of the allowed types: %s", strings.Join(r.config.Conventional.Types, ", ")))
		}

		// Scope Validation (optional)
		if ccScope != "" && len(r.config.Conventional.Scopes) > 0 {
			scopeIsValid := false
			for _, scope := range r.config.Conventional.Scopes {
				re := regexp.MustCompile(scope)
				if re.MatchString(ccScope) {
					scopeIsValid = true
					break
				}
			}
			if !scopeIsValid {
				ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Invalid scope %q: allowed scopes are %v", ccScope, r.config.Conventional.Scopes))
				ruleResult.Suggestion = append(ruleResult.Suggestion, "Use a valid scope or omit it")
			}
		}
	}

	// Ticket validation (Jira, Asana, etc.)
	if r.ticketValidators.HasValidators() {
		ctx := context.Background()
		result := r.ticketValidators.ValidateMessage(ctx, title)

		if result.AllMissing {
			ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("No ticket reference found in title: %q", title))
			ruleResult.Suggestion = append(ruleResult.Suggestion, "Include a ticket reference (e.g., Jira: [ABC-123], Asana: PROJ-1234567890123456)  \n> **Example**:  \n> `fix(token): handle expired JWT refresh logic [SEC-456]`")
		} else if !result.AnyValid {
			for name, valResult := range result.Results {
				if !valResult.Valid {
					ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("%s: %s", name, valResult.Error))
					ruleResult.Suggestion = append(ruleResult.Suggestion, "Use a valid ticket reference from one of the configured systems")
				}
			}
		}
	}

	// Legacy Jira validation for backward compatibility
	if len(r.config.Jira.Keys) > 0 && !r.ticketValidators.HasValidators() {
		if !common.JiraRegex.MatchString(title) {
			ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("No Jira issue tag found in title: %q", title))
			ruleResult.Suggestion = append(ruleResult.Suggestion, "Include a Jira tag like [ABC-123] or ABC-123  \n> **Example**:  \n> `fix(token): handle expired JWT refresh logic [SEC-456] `")
		} else {
			submatch := common.JiraRegex.FindStringSubmatch(title)
			jiraProject := submatch[1]

			if !common.Contains(r.config.Jira.Keys, jiraProject) {
				ruleResult.Error = append(ruleResult.Error, fmt.Sprintf("Jira project %q is not valid. Allowed: %v", jiraProject, r.config.Jira.Keys))
				ruleResult.Suggestion = append(ruleResult.Suggestion, fmt.Sprintf("Use a valid Jira key such as %s", r.config.Jira.Keys[0]))
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
