package integration

import (
	"gitlab-mr-conformity-bot/test/testutil"
	"testing"
)

// TestMergeRequestWorkflow tests the merge request creation and basic workflow for tests
func TestMergeRequestWorkflow(t *testing.T) {
	cfg := testutil.SetupTestEnvironment(t)

	t.Run("create merge request", func(t *testing.T) {

		// Create feature branch
		branchName := "feature/test-feature"
		err := testutil.CreateTestBranch(t, cfg.Client, cfg.Project.ID, branchName, cfg.Project.DefaultBranch)
		testutil.AssertNoErrors(t, err)

		// Add a file
		err = testutil.CreateTestFile(t, cfg.Client, cfg.Project.ID, branchName, "feature.txt", "New feature")
		testutil.AssertNoErrors(t, err)

		// Create MR with valid title format
		mrTitle := "PROJ-123: Add new feature"
		mrDescription := "This MR adds a new feature.\n\nCloses PROJ-123"

		mr, err := testutil.CreateTestMergeRequest(t, cfg.Client, cfg.Project.ID, branchName, cfg.Project.DefaultBranch, mrTitle, mrDescription)
		testutil.AssertNoErrors(t, err)
		testutil.AssertNotNil(t, mr)
		testutil.AssertEqual(t, mrTitle, mr.Title)

		// Get the created MR and verify it has been created correctly
		retrievedMR, err := cfg.Client.GetMergeRequest(cfg.Project.ID, mr.IID)
		testutil.AssertNoErrors(t, err)
		testutil.AssertNotNil(t, retrievedMR)
		testutil.AssertEqual(t, mr.Title, retrievedMR.Title)

		t.Logf("✓ Created MR: %s (IID: %d)", mr.Title, mr.IID)
	})

}