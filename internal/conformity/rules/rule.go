package rules

import (
	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type Severity int

const (
	SeverityWarning Severity = iota
	SeverityError
)

type Rule interface {
	Name() string
	Severity() Severity
	Check(mr *gitlabapi.MergeRequest, commits []*gitlabapi.Commit) (*RuleResult, error)
}

type RuleResult struct {
	Passed     bool
	Error      []string
	Suggestion []string
}
