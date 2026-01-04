package testutil

import (
	"strings"
	"testing"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

// AssertMRHasDiscussion checks if a merge request has at least one discussion
func AssertMRHasDiscussion(t *testing.T, discussions []*gitlabapi.Discussion) {
	t.Helper()

	if len(discussions) == 0 {
		t.Error("Expected at least one discussion on the merge request")
	}
}

// AssertMRDiscussionContains checks if any discussion contains the expected text
func AssertMRDiscussionContains(t *testing.T, discussions []*gitlabapi.Discussion, expectedText string) {
	t.Helper()

	for _, discussion := range discussions {
		for _, note := range discussion.Notes {
			if strings.Contains(note.Body, expectedText) {
				return
			}
		}
	}

	t.Errorf("Expected to find '%s' in merge request discussions, but it was not found", expectedText)
}

// AssertNoErrors checks that there are no errors
func AssertNoErrors(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

// AssertError checks that an error occurred
func AssertError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Error("Expected an error, but got none")
	}
}

// AssertEqual checks if two values are equal
func AssertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()

	if expected != actual {
		t.Errorf("Expected %v, but got %v", expected, actual)
	}
}

// AssertNotNil checks that a value is not nil
func AssertNotNil(t *testing.T, value interface{}) {
	t.Helper()

	if value == nil {
		t.Error("Expected value to be non-nil, but got nil")
	}
}
