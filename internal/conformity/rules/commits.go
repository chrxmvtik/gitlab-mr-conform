package rules

import (
	"context"
	"fmt"
	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity/helper/codeowners"
	"gitlab-mr-conformity-bot/internal/conformity/helper/common"
	"gitlab-mr-conformity-bot/internal/conformity/validator"
	"regexp"
	"strings"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type CommitsRule struct {
	config           config.CommitsConfig
	ticketValidators *validator.ValidatorManager
}

func NewCommitsRule(cfg interface{}) *CommitsRule {
	commitsCfg, ok := cfg.(config.CommitsConfig)
	if !ok {
		commitsCfg = config.CommitsConfig{
			MaxLength: 72,
			Conventional: config.ConventionalConfig{
				Types:  []string{"feat"},
				Scopes: []string{".*"},
			},
			Jira: config.JiraConfig{
				Keys: []string{""},
			},
		}
	}
	return &CommitsRule{
		config:           commitsCfg,
		ticketValidators: validator.BuildTicketValidators(commitsCfg.Jira, commitsCfg.Asana),
	}
}

func (r *CommitsRule) Name() string {
	return "Commit Messages"
}

func (r *CommitsRule) Severity() Severity {
	return SeverityWarning
}

func (r *CommitsRule) Check(mr *gitlabapi.MergeRequest, commits []*gitlabapi.Commit, approvals *common.Approvals, cos []*codeowners.PatternGroup, members []*gitlabapi.ProjectMember) (*RuleResult, error) {
	// Aggregation structures - store commit info instead of just strings
	var tooLongCommits []*gitlabapi.Commit
	var invalidFormatCommits []*gitlabapi.Commit
	invalidTypes := make(map[string][]*gitlabapi.Commit)
	invalidScopes := make(map[string][]*gitlabapi.Commit)
	var missingJiraCommits []*gitlabapi.Commit
	invalidJiraProjects := make(map[string][]*gitlabapi.Commit)
	var missingTicketCommits []*gitlabapi.Commit
	invalidTickets := make(map[string][]*gitlabapi.Commit)

	for _, commit := range commits {
		lines := strings.Split(commit.Message, "\n")
		firstLine := strings.TrimSpace(lines[0])

		// Check message length
		if len(firstLine) > r.config.MaxLength {
			tooLongCommits = append(tooLongCommits, commit)
		}

		// Conventional Commit Check
		groups := common.ParseHeader(commit.Message)
		if len(groups) != 7 {
			invalidFormatCommits = append(invalidFormatCommits, commit)
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
				if invalidTypes[ccType] == nil {
					invalidTypes[ccType] = []*gitlabapi.Commit{}
				}
				invalidTypes[ccType] = append(invalidTypes[ccType], commit)
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
					if invalidScopes[ccScope] == nil {
						invalidScopes[ccScope] = []*gitlabapi.Commit{}
					}
					invalidScopes[ccScope] = append(invalidScopes[ccScope], commit)
				}
			}
		}

		// Ticket validation (Jira, Asana, etc.)
		if r.ticketValidators.HasValidators() {
			ctx := context.Background()
			result := r.ticketValidators.ValidateMessage(ctx, commit.Message)

			if result.AllMissing {
				missingTicketCommits = append(missingTicketCommits, commit)
			} else if !result.AnyValid {
				for name, valResult := range result.Results {
					if !valResult.Valid {
						errorKey := fmt.Sprintf("%s: %s", name, valResult.Error)
						if invalidTickets[errorKey] == nil {
							invalidTickets[errorKey] = []*gitlabapi.Commit{}
						}
						invalidTickets[errorKey] = append(invalidTickets[errorKey], commit)
					}
				}
			}
		}

		// Legacy Jira validation for backward compatibility
		if len(r.config.Jira.Keys) > 0 && !r.ticketValidators.HasValidators() {
			if !common.JiraRegex.MatchString(commit.Message) {
				missingJiraCommits = append(missingJiraCommits, commit)
			} else {
				submatch := common.JiraRegex.FindStringSubmatch(commit.Message)
				jiraProject := submatch[1]
				if !common.Contains(r.config.Jira.Keys, jiraProject) {
					if invalidJiraProjects[jiraProject] == nil {
						invalidJiraProjects[jiraProject] = []*gitlabapi.Commit{}
					}
					invalidJiraProjects[jiraProject] = append(invalidJiraProjects[jiraProject], commit)
				}
			}
		}
	}

	// Build aggregated results
	ruleResult := &RuleResult{}

	// Aggregate too long commits
	if len(tooLongCommits) > 0 {
		errorMsg := fmt.Sprintf("%d commit(s) exceed max length of %d chars:", len(tooLongCommits), r.config.MaxLength)
		for _, commit := range tooLongCommits {
			commitTitle := common.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
			errorMsg += fmt.Sprintf("\n  - %s ([%s](%s))", commitTitle, commit.ShortID, commit.WebURL)
		}
		ruleResult.Error = append(ruleResult.Error, errorMsg)
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Keep commit messages concise and under the character limit")
	}

	// Aggregate invalid format commits
	if len(invalidFormatCommits) > 0 {
		errorMsg := fmt.Sprintf("%d commit(s) have invalid Conventional Commit format:", len(invalidFormatCommits))
		for _, commit := range invalidFormatCommits {
			commitTitle := common.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
			errorMsg += fmt.Sprintf("\n  - %s ([%s](%s))", commitTitle, commit.ShortID, commit.WebURL)
		}
		ruleResult.Error = append(ruleResult.Error, errorMsg)
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Use format: \n> ``` \n> type(scope?): description \n> ```\n> Example: \n`feat(auth): add login retry mechanism`\n\n")
	}

	// Aggregate invalid types
	for invalidType, commits := range invalidTypes {
		errorMsg := fmt.Sprintf("%d commit(s) use invalid type '%s':", len(commits), invalidType)
		for _, commit := range commits {
			commitTitle := common.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
			errorMsg += fmt.Sprintf("\n  - %s ([%s](%s))", commitTitle, commit.ShortID, commit.WebURL)
		}
		ruleResult.Error = append(ruleResult.Error, errorMsg)
		ruleResult.Suggestion = append(ruleResult.Suggestion,
			fmt.Sprintf("Use one of the allowed types: %s", strings.Join(r.config.Conventional.Types, ", ")))
	}

	// Aggregate invalid scopes
	for invalidScope, commits := range invalidScopes {
		errorMsg := fmt.Sprintf("%d commit(s) use invalid scope '%s':", len(commits), invalidScope)
		for _, commit := range commits {
			commitTitle := common.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
			errorMsg += fmt.Sprintf("\n  - %s ([%s](%s))", commitTitle, commit.ShortID, commit.WebURL)
		}
		ruleResult.Error = append(ruleResult.Error, errorMsg)
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Use a valid scope or omit it")
	}

	// Aggregate missing ticket commits (new validator system)
	if len(missingTicketCommits) > 0 {
		errorMsg := fmt.Sprintf("%d commit(s) missing ticket reference:", len(missingTicketCommits))
		for _, commit := range missingTicketCommits {
			commitTitle := common.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
			errorMsg += fmt.Sprintf("\n  - %s ([%s](%s))", commitTitle, commit.ShortID, commit.WebURL)
		}
		ruleResult.Error = append(ruleResult.Error, errorMsg)
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Include a ticket reference (e.g., Jira: [ABC-123], Asana: PROJ-1234567890123456) \n> **Example**: \n> `fix(token): handle expired JWT refresh logic [SEC-456]`")
	}

	// Aggregate invalid tickets (new validator system)
	for errorKey, commits := range invalidTickets {
		errorMsg := fmt.Sprintf("%d commit(s) with %s:", len(commits), errorKey)
		for _, commit := range commits {
			commitTitle := common.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
			errorMsg += fmt.Sprintf("\n  - %s ([%s](%s))", commitTitle, commit.ShortID, commit.WebURL)
		}
		ruleResult.Error = append(ruleResult.Error, errorMsg)
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Use a valid ticket reference from one of the configured systems")
	}

	// Aggregate missing Jira commits (legacy)
	if len(missingJiraCommits) > 0 {
		errorMsg := fmt.Sprintf("%d commit(s) missing Jira issue tag:", len(missingJiraCommits))
		for _, commit := range missingJiraCommits {
			commitTitle := common.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
			errorMsg += fmt.Sprintf("\n  - %s ([%s](%s))", commitTitle, commit.ShortID, commit.WebURL)
		}
		ruleResult.Error = append(ruleResult.Error, errorMsg)
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Include a Jira tag like [ABC-123] or ABC-123 \n> **Example**: \n> `fix(token): handle expired JWT refresh logic [SEC-456] `")
	}

	// Aggregate invalid Jira projects (legacy)
	for invalidProject, commits := range invalidJiraProjects {
		errorMsg := fmt.Sprintf("* %d commit(s) use invalid Jira project '%s':", len(commits), invalidProject)
		for _, commit := range commits {
			commitTitle := common.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
			errorMsg += fmt.Sprintf("\n  - %s ([%s](%s))", commitTitle, commit.ShortID, commit.WebURL)
		}
		ruleResult.Error = append(ruleResult.Error, errorMsg)
		ruleResult.Suggestion = append(ruleResult.Suggestion,
			fmt.Sprintf("Use a valid Jira key such as %s", r.config.Jira.Keys[0]))
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
