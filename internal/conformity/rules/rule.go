package rules

import (
	"gitlab-mr-conformity-bot/internal/conformity/helper/codeowners"
	"gitlab-mr-conformity-bot/internal/conformity/helper/common"

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
	Check(mr *gitlabapi.MergeRequest, commits []*gitlabapi.Commit, approvals *common.Approvals, cos []*codeowners.PatternGroup, members []*gitlabapi.ProjectMember) (*RuleResult, error)
}

type RuleResult struct {
	Passed     bool
	Error      []string
	Suggestion []string
}
