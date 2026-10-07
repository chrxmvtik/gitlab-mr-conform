package rules

import (
	"strings"
	"testing"

	"gitlab-mr-conformity-bot/internal/config"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

func TestTitleRuleAllowsConfiguredRegexAsAlternativeToConventionalCommit(t *testing.T) {
	rule := NewTitleRule(config.TitleConfig{
		MaxLength:    100,
		Conventional: config.ConventionalConfig{Types: []string{"feat"}},
		AllowedRegex: []string{`^Release/v\d+\.\d+\.\d+$`},
	}, config.IntegrationsConfig{})

	for _, title := range []string{"feat: add title validation", "Release/v1.2.3"} {
		result, err := rule.Check(mergeRequestWithTitle(title), nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("Check(%q) returned an error: %v", title, err)
		}
		if !result.Passed {
			t.Fatalf("Check(%q) failed: %v", title, result.Error)
		}
	}
}

func TestTitleRuleRejectsTitleThatMatchesNeitherFormat(t *testing.T) {
	rule := NewTitleRule(config.TitleConfig{
		MaxLength:    100,
		Conventional: config.ConventionalConfig{Types: []string{"feat"}},
		AllowedRegex: []string{`^Release/v\d+\.\d+\.\d+$`},
	}, config.IntegrationsConfig{})

	result, err := rule.Check(mergeRequestWithTitle("Release 1.2.3"), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Check returned an error: %v", err)
	}
	if result.Passed || !strings.Contains(strings.Join(result.Error, "\n"), "Invalid Conventional Commit format") {
		t.Fatalf("expected a conventional-format failure, got %+v", result)
	}
}

func TestTitleRuleReturnsConfigurationErrorForInvalidAllowedRegex(t *testing.T) {
	rule := NewTitleRule(config.TitleConfig{AllowedRegex: []string{"["}}, config.IntegrationsConfig{})

	_, err := rule.Check(mergeRequestWithTitle("Release/v1.2.3"), nil, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid title allowed_regex") {
		t.Fatalf("expected invalid allowed_regex error, got %v", err)
	}
}

func mergeRequestWithTitle(title string) *gitlabapi.MergeRequest {
	return &gitlabapi.MergeRequest{
		BasicMergeRequest: gitlabapi.BasicMergeRequest{Title: title},
	}
}
