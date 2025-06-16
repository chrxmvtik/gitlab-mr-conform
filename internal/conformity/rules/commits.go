package rules

import (
	"fmt"
	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity/helper"
	"regexp"
	"strings"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type CommitsRule struct {
	config config.CommitsConfig
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
	return &CommitsRule{config: commitsCfg}
}

func (r *CommitsRule) Name() string {
	return "Commit Messages"
}

func (r *CommitsRule) Severity() Severity {
	return SeverityWarning
}

func (r *CommitsRule) Check(mr *gitlabapi.MergeRequest, commits []*gitlabapi.Commit, approvals *int) (*RuleResult, error) {
	// Aggregation structures - store commit info instead of just strings
	var tooLongCommits []*gitlabapi.Commit
	var invalidFormatCommits []*gitlabapi.Commit
	invalidTypes := make(map[string][]*gitlabapi.Commit)
	invalidScopes := make(map[string][]*gitlabapi.Commit)
	var missingJiraCommits []*gitlabapi.Commit
	invalidJiraProjects := make(map[string][]*gitlabapi.Commit)

	for _, commit := range commits {
		lines := strings.Split(commit.Message, "\n")
		firstLine := strings.TrimSpace(lines[0])

		// Check message length
		if len(firstLine) > r.config.MaxLength {
			tooLongCommits = append(tooLongCommits, commit)
		}

		// Conventional Commit Check
		groups := helper.ParseHeader(commit.Message)
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

		// Jira Issue Check
		if len(r.config.Jira.Keys) > 0 {
			if !helper.JiraRegex.MatchString(commit.Message) {
				missingJiraCommits = append(missingJiraCommits, commit)
			} else {
				submatch := helper.JiraRegex.FindStringSubmatch(commit.Message)
				jiraProject := submatch[1]
				if !helper.Contains(r.config.Jira.Keys, jiraProject) {
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
			commitTitle := helper.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
			errorMsg += fmt.Sprintf("\n  - %s ([%s](%s))", commitTitle, commit.ShortID, commit.WebURL)
		}
		ruleResult.Error = append(ruleResult.Error, errorMsg)
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Keep commit messages concise and under the character limit")
	}

	// Aggregate invalid format commits
	if len(invalidFormatCommits) > 0 {
		errorMsg := fmt.Sprintf("%d commit(s) have invalid Conventional Commit format:", len(invalidFormatCommits))
		for _, commit := range invalidFormatCommits {
			commitTitle := helper.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
			errorMsg += fmt.Sprintf("\n  - %s ([%s](%s))", commitTitle, commit.ShortID, commit.WebURL)
		}
		ruleResult.Error = append(ruleResult.Error, errorMsg)
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Use format: \n> ``` \n> type(scope?): description \n> ```\n> Example: \n`feat(auth): add login retry mechanism`\n\n")
	}

	// Aggregate invalid types
	for invalidType, commits := range invalidTypes {
		errorMsg := fmt.Sprintf("%d commit(s) use invalid type '%s':", len(commits), invalidType)
		for _, commit := range commits {
			commitTitle := helper.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
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
			commitTitle := helper.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
			errorMsg += fmt.Sprintf("\n  - %s ([%s](%s))", commitTitle, commit.ShortID, commit.WebURL)
		}
		ruleResult.Error = append(ruleResult.Error, errorMsg)
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Use a valid scope or omit it")
	}

	// Aggregate missing Jira commits
	if len(missingJiraCommits) > 0 {
		errorMsg := fmt.Sprintf("%d commit(s) missing Jira issue tag:", len(missingJiraCommits))
		for _, commit := range missingJiraCommits {
			commitTitle := helper.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
			errorMsg += fmt.Sprintf("\n  - %s ([%s](%s))", commitTitle, commit.ShortID, commit.WebURL)
		}
		ruleResult.Error = append(ruleResult.Error, errorMsg)
		ruleResult.Suggestion = append(ruleResult.Suggestion, "Include a Jira tag like [ABC-123] or ABC-123 \n> **Example**: \n> `fix(token): handle expired JWT refresh logic [SEC-456] `")
	}

	// Aggregate invalid Jira projects
	for invalidProject, commits := range invalidJiraProjects {
		errorMsg := fmt.Sprintf("* %d commit(s) use invalid Jira project '%s':", len(commits), invalidProject)
		for _, commit := range commits {
			commitTitle := helper.TruncateCommitMessage(strings.Split(commit.Message, "\n")[0], 50)
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
