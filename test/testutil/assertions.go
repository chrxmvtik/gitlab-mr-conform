package testutil

import (
	"strings"
	"testing"
)

// AssertHasMRConformDiscussion checks if the MR has a discussion from the MR Conformity Bot
func AssertHasMRConformDiscussion(t *testing.T, client *TestClient, projectID int, mrIID int) {
	t.Helper()

	mrConformDiscussion, err := client.GetMRConformDiscussion(projectID, mrIID)
	AssertNoErrors(t, err)
	AssertNotNil(t, mrConformDiscussion)
}

// AssertMRConformDiscussionContains checks if the MR Conformity Bot discussion contains the expected text
func AssertMRConformDiscussionContains(t *testing.T, client *TestClient, projectID int, mrIID int, expectedText string) {
	t.Helper()

	mrConformDiscussion, err := client.GetMRConformDiscussion(projectID, mrIID)
	AssertNoErrors(t, err)
	AssertNotNil(t, mrConformDiscussion)

	// Check if Notes is nil or empty
	if mrConformDiscussion.Notes == nil || len(mrConformDiscussion.Notes) == 0 {
		t.Error("Expected MR Conformity Bot discussion to have notes, but found none")
		return
	}

	found := false
	for _, note := range mrConformDiscussion.Notes {
		if strings.Contains(note.Body, expectedText) {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected MR Conformity Bot discussion to contain text: %q", expectedText)
	}
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
