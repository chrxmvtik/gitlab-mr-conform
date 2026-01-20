package testutil

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

// TestConfig holds configuration for integration tests
type TestConfig struct {
	GitLabURL   string
	GitLabToken string
	Client      *TestClient
	Project     *gitlabapi.Project
}

// SetupTestEnvironment initializes the test environment
func SetupTestEnvironment(t *testing.T) *TestConfig {
	t.Helper()

	// Get GitLab URL and token from environment or files
	gitlabURL := getEnvOrFile(t, "GITLAB_URL", "../../test/docker/gitlab_url.txt")
	gitlabToken := getEnvOrFile(t, "GITLAB_TOKEN", "../../test/docker/gitlab_token.txt")

	if gitlabURL == "" || gitlabToken == "" {
		t.Fatal("GitLab URL and token must be set. Run test/docker/run_gitlab.sh first.")
	}

	// Create GitLab test client
	client, err := NewTestClient(gitlabToken, gitlabURL, false)
	if err != nil {
		t.Fatalf("Failed to create GitLab client: %v", err)
	}

	// Create a project for testing
	project := CreateTestProject(t, client, "integration")
	t.Logf("✓ Created test project: %s (ID: %d)", project.Name, project.ID)

	// Create a Project Webhook for gitlab-mr-conform bot to use
	webhookURL := "http://bot.local:8081/webhook"
	hook := CreateProjectWebhook(t, client, project.ID, webhookURL)
	t.Logf("✓ Created project webhook: %s", hook.URL)

	return &TestConfig{
		GitLabURL:   gitlabURL,
		GitLabToken: gitlabToken,
		Client:      client,
		Project:     project,
	}
}

// getEnvOrFile tries to get value from environment variable first, then from file
func getEnvOrFile(t *testing.T, envKey, filePath string) string {
	t.Helper()

	// Try environment variable first
	if val := os.Getenv(envKey); val != "" {
		return strings.TrimSpace(val)
	}

	// Try reading from file
	if data, err := os.ReadFile(filePath); err == nil {
		return strings.TrimSpace(string(data))
	}

	return ""
}

// GetRandomName generates a random name for test resources
func GetRandomName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().Unix())
}

// CreateTestProject creates a test project in GitLab
func CreateTestProject(t *testing.T, client *TestClient, name string) *gitlabapi.Project {
	t.Helper()

	projectName := fmt.Sprintf("test-%s-%d", name, time.Now().Unix())

	project, err := client.CreateProject(projectName, &gitlabapi.CreateProjectOptions{
		Description:          gitlabapi.Ptr("Test project for MR conformity bot"),
		InitializeWithReadme: gitlabapi.Ptr(true),
		Visibility:           gitlabapi.Ptr(gitlabapi.PublicVisibility),
	})
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	t.Cleanup(func() {
		_ = client.DeleteProject(project.ID)
	})

	return project
}

// CreateTestWebhook creates a project webhook triggered by merge request events
func CreateProjectWebhook(t *testing.T, client *TestClient, projectID interface{}, webhookURL string) *gitlabapi.ProjectHook {
	t.Helper()

	hookOptions := &gitlabapi.AddProjectHookOptions{
		Name:                  gitlabapi.Ptr("gitlab-mr-conform"),
		URL:                   &webhookURL,
		PushEvents:            gitlabapi.Ptr(false),
		MergeRequestsEvents:   gitlabapi.Ptr(true),
		EnableSSLVerification: gitlabapi.Ptr(false),
	}

	hook, _, err := client.api.Projects.AddProjectHook(projectID, hookOptions)
	if err != nil {
		t.Fatalf("Failed to create project webhook: %v", err)
	}

	var event gitlabapi.ProjectHookEvent

	event = "merge_requests_events"

	testHook, err := client.api.Projects.TriggerTestProjectHook(projectID, hook.ID, event)
	// if err != nil {
	// 	t.Fatalf("Failed to test project webhook: %v\n", err)
	// }

	fmt.Printf("Webhook test result: %d", testHook.StatusCode)

	t.Cleanup(func() {
		_, _ = client.api.Projects.DeleteProjectHook(projectID, hook.ID)
	})

	return hook
}

// CreateTestBranch creates a test branch in a project
func CreateTestBranch(t *testing.T, client *TestClient, projectID interface{}, branchName, ref string) error {
	t.Helper()

	_, err := client.CreateBranch(projectID, branchName, ref)
	return err
}

// CreateTestFile creates a test file in a project
func CreateTestFile(t *testing.T, client *TestClient, projectID interface{}, branch, filePath, content string) error {
	t.Helper()

	return client.CreateFile(projectID, filePath, branch, content, "Test commit")
}

// CreateTestMergeRequest creates a test merge request
func CreateTestMergeRequest(t *testing.T, client *TestClient, projectID interface{}, sourceBranch, targetBranch, title, description string) (*gitlabapi.MergeRequest, error) {
	t.Helper()

	mr, err := client.CreateMergeRequest(projectID, &gitlabapi.CreateMergeRequestOptions{
		Title:        gitlabapi.Ptr(title),
		Description:  gitlabapi.Ptr(description),
		SourceBranch: gitlabapi.Ptr(sourceBranch),
		TargetBranch: gitlabapi.Ptr(targetBranch),
	})
	if err != nil {
		return nil, err
	}

	t.Cleanup(func() {
		// Close the MR on cleanup
		_ = client.UpdateMergeRequest(projectID, mr.IID, &gitlabapi.UpdateMergeRequestOptions{
			StateEvent: gitlabapi.Ptr("close"),
		})
	})
	return mr, nil
}

// WaitForMergeRequest waits for a merge request to be fully created
func WaitForMergeRequest(t *testing.T, client *TestClient, projectID interface{}, mrIID int, timeout time.Duration) (*gitlabapi.MergeRequest, error) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mr, err := client.GetMergeRequest(projectID, mrIID)
		if err == nil && mr.ID > 0 {
			return mr, nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("timeout waiting for merge request to be ready")
}

// WaitForBotDiscussion waits for the bot to create a discussion on the merge request
func WaitForBotDiscussion(t *testing.T, client *TestClient, projectID interface{}, mrIID int, timeout time.Duration) (*gitlabapi.Discussion, error) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		discussion, err := client.GetMRConformDiscussion(projectID, mrIID)
		if err == nil && discussion != nil && discussion.Notes != nil && len(discussion.Notes) > 0 {
			return discussion, nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("timeout waiting for bot discussion")
}
