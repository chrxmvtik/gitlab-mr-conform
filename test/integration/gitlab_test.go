package integration

import (
	"gitlab-mr-conformity-bot/test/testutil"
	"testing"
	"time"
)

// TestMergeRequestWorkflow tests the merge request creation and basic workflow for tests
func TestMergeRequestWorkflow(t *testing.T) {
	cfg := testutil.SetupTestEnvironment(t)

	t.Run("should create a discussion on merge request", func(t *testing.T) {

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

		// Wait for bot to process webhook and create discussion
		_, err = testutil.WaitForBotDiscussion(t, cfg.Client, cfg.Project.ID, mr.IID, 180*time.Second)
		testutil.AssertNoErrors(t, err)

		t.Logf("✓ Created MR: %s (IID: %d)", mr.Title, mr.IID)
	})

	t.Run("should be configurable via .mr-conform.yaml", func(t *testing.T) {
		// This test would implement checks for configuration via .mr-conform.yaml
		// Commit the config file to the default branch
		err := testutil.CreateTestFile(t, cfg.Client, cfg.Project.ID, cfg.Project.DefaultBranch, ".mr-conform.yaml", "rules:\n  title_format: '^PROJ-\\d+: .+'\n")
		testutil.AssertNoErrors(t, err)

		// Create feature branch
		branchName := "feature/config-test"
		err = testutil.CreateTestBranch(t, cfg.Client, cfg.Project.ID, branchName, cfg.Project.DefaultBranch)
		testutil.AssertNoErrors(t, err)

		// Add a file
		err = testutil.CreateTestFile(t, cfg.Client, cfg.Project.ID, branchName, "config_test.txt", "Config test")
		testutil.AssertNoErrors(t, err)

		// Create MR with valid title format
		mrTitle := "PROJ-456: Config test MR"
		mrDescription := "This MR tests configuration.\n\nCloses PROJ-456"

		mr, err := testutil.CreateTestMergeRequest(t, cfg.Client, cfg.Project.ID, branchName, cfg.Project.DefaultBranch, mrTitle, mrDescription)
		testutil.AssertNoErrors(t, err)
		testutil.AssertNotNil(t, mr)
		testutil.AssertEqual(t, mrTitle, mr.Title)

		// Wait for bot to process webhook and create discussion
		_, err = testutil.WaitForBotDiscussion(t, cfg.Client, cfg.Project.ID, mr.IID, 180*time.Second)
		testutil.AssertNoErrors(t, err)

		// Verify that the MR Conformity Bot created a discussion
		testutil.AssertMRConformDiscussionContains(t, cfg.Client, cfg.Project.ID, mr.IID, "All conformity checks passed")

		t.Logf("✓ Created MR with config: %s (IID: %d)", mr.Title, mr.IID)
	})
}
