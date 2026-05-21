package testutil

import (
	"fmt"
	"strings"

	"gitlab-mr-conformity-bot/internal/gitlab"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

// TestClient wraps the GitLab client with additional test-specific methods
type TestClient struct {
	*gitlab.Client
	api *gitlabapi.Client
}

// NewTestClient creates a new test client wrapper
func NewTestClient(token, baseURL string, insecure bool) (*TestClient, error) {
	client, err := gitlab.NewClient(token, baseURL, insecure)
	if err != nil {
		return nil, err
	}

	// Create a direct GitLab API client for test operations
	apiClient, err := gitlabapi.NewClient(token, gitlabapi.WithBaseURL(baseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab API client: %w", err)
	}

	return &TestClient{
		Client: client,
		api:    apiClient,
	}, nil
}

// CreateProject creates a new project
func (c *TestClient) CreateProject(name string, opts *gitlabapi.CreateProjectOptions) (*gitlabapi.Project, error) {
	if opts == nil {
		opts = &gitlabapi.CreateProjectOptions{}
	}
	// Always set the Name field
	opts.Name = gitlabapi.Ptr(name)
	project, _, err := c.api.Projects.CreateProject(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}
	return project, nil
}

// DeleteProject deletes a project
func (c *TestClient) DeleteProject(projectID interface{}) error {
	_, err := c.api.Projects.DeleteProject(projectID, nil)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	return nil
}

// CreateBranch creates a new branch
func (c *TestClient) CreateBranch(projectID interface{}, branch, ref string) (*gitlabapi.Branch, error) {
	b, _, err := c.api.Branches.CreateBranch(projectID, &gitlabapi.CreateBranchOptions{
		Branch: &branch,
		Ref:    &ref,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create branch: %w", err)
	}
	return b, nil
}

// CreateFile creates a new file in repository
func (c *TestClient) CreateFile(projectID interface{}, filePath, branch, content, commitMessage string) error {
	_, _, err := c.api.RepositoryFiles.CreateFile(projectID, filePath, &gitlabapi.CreateFileOptions{
		Branch:        &branch,
		Content:       &content,
		CommitMessage: &commitMessage,
	})
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	return nil
}

// CreateMergeRequest creates a new merge request
func (c *TestClient) CreateMergeRequest(projectID interface{}, opts *gitlabapi.CreateMergeRequestOptions) (*gitlabapi.MergeRequest, error) {
	mr, _, err := c.api.MergeRequests.CreateMergeRequest(projectID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create merge request: %w", err)
	}
	return mr, nil
}

// UpdateMergeRequest updates a merge request
func (c *TestClient) UpdateMergeRequest(projectID interface{}, mrIID int, opts *gitlabapi.UpdateMergeRequestOptions) error {
	_, _, err := c.api.MergeRequests.UpdateMergeRequest(projectID, mrIID, opts)
	if err != nil {
		return fmt.Errorf("failed to update merge request: %w", err)
	}
	return nil
}

// CreateProjectHook creates a webhook for a project
func (c *TestClient) CreateProjectHook(projectID interface{}, opts *gitlabapi.AddProjectHookOptions) (*gitlabapi.ProjectHook, error) {
	hook, _, err := c.api.Projects.AddProjectHook(projectID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create project hook: %w", err)
	}
	return hook, nil
}

// DeleteProjectHook deletes a project webhook
func (c *TestClient) DeleteProjectHook(projectID interface{}, hookID int) error {
	_, err := c.api.Projects.DeleteProjectHook(projectID, hookID)
	if err != nil {
		return fmt.Errorf("failed to delete project hook: %w", err)
	}
	return nil
}

// GetCurrentUser returns the current authenticated user
func (c *TestClient) GetCurrentUser() (*gitlabapi.User, error) {
	user, _, err := c.api.Users.CurrentUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}
	return user, nil
}

// GetMergeRequest gets a merge request by IID
func (c *TestClient) GetMergeRequest(projectID interface{}, mrIID int) (*gitlabapi.MergeRequest, error) {
	mr, _, err := c.api.MergeRequests.GetMergeRequest(projectID, mrIID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request: %w", err)
	}
	return mr, nil
}

// ListMergeRequestDiscussions lists discussions for a merge request
func (c *TestClient) ListMergeRequestDiscussions(projectID interface{}, mrIID int, opts *gitlabapi.ListMergeRequestDiscussionsOptions) ([]*gitlabapi.Discussion, error) {
	discussions, _, err := c.api.Discussions.ListMergeRequestDiscussions(projectID, mrIID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list merge request discussions: %w", err)
	}
	return discussions, nil
}

// GetMRConformDiscussion retrieves the discussion created by the MR Conformity Bot
func (c *TestClient) GetMRConformDiscussion(projectID interface{}, mrIID int) (*gitlabapi.Discussion, error) {
	discussions, _, err := c.api.Discussions.ListMergeRequestDiscussions(projectID, mrIID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list merge request discussions: %w", err)
	}

	for _, discussion := range discussions {
		// Checks for "🧾 Merge Request Compliance Report"
		for _, note := range discussion.Notes {
			if note.System || note.Body == "" {
				continue
			}
			if strings.Contains(note.Body, "Merge Request Compliance Report") {
				return discussion, nil
			}
		}
	}

	return nil, fmt.Errorf("no discussion from Merge Request Conformity Bot found")
}