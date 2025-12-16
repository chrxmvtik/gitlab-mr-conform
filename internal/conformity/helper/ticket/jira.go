package ticket

import (
	"context"
	"gitlab-mr-conformity-bot/internal/config"
	"regexp"
)

var jiraRegex = regexp.MustCompile(`\s\[?([A-Z0-9]+)-([1-9]\d*)\]?`)

type JiraValidator struct {
	config config.JiraConfig
}

func NewJiraValidator(cfg config.JiraConfig) *JiraValidator {
	return &JiraValidator{config: cfg}
}

func (v *JiraValidator) Name() string {
	return "Jira"
}

func (v *JiraValidator) ContainsTicket(message string) bool {
	return jiraRegex.MatchString(message)
}

func (v *JiraValidator) ExtractTicket(message string) *TicketInfo {
	matches := jiraRegex.FindStringSubmatch(message)
	if len(matches) < 3 {
		return nil
	}

	return &TicketInfo{
		ProjectKey: matches[1],
		TicketID:   matches[2],
		FullMatch:  matches[0],
	}
}

func (v *JiraValidator) ValidateTicket(ctx context.Context, info *TicketInfo) ValidationResult {
	if info == nil {
		return ValidationResult{Found: false, Valid: false}
	}

	for _, key := range v.config.Keys {
		if key == info.ProjectKey {
			return ValidationResult{
				Found:      true,
				Valid:      true,
				TicketInfo: info,
			}
		}
	}

	return ValidationResult{
		Found:      true,
		Valid:      false,
		TicketInfo: info,
		Error:      "Invalid Jira project key",
	}
}
