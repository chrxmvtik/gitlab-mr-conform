package gitlab

import (
	"crypto/tls"
	"fmt"
	"gitlab-mr-conformity-bot/internal/conformity/helper/common"
	"net/http"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type Client struct {
	client *gitlab.Client
}

func NewClient(token, baseURL string, insecure bool) (*Client, error) {
	var client *gitlab.Client
	var err error

	if insecure {
		// Create a custom HTTP client that skips TLS verification
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // ⚠️ Use with caution!
				},
			},
		}

		client, err = gitlab.NewClient(token,
			gitlab.WithBaseURL(baseURL),
			gitlab.WithHTTPClient(httpClient),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitLab client (insecure): %w", err)
		}
	} else {
		client, err = gitlab.NewClient(token, gitlab.WithBaseURL(baseURL))
		if err != nil {
			return nil, fmt.Errorf("failed to create GitLab client: %w", err)
		}
	}

	return &Client{client: client}, nil
}

func (c *Client) GetMergeRequest(projectID interface{}, mrID int) (*gitlab.MergeRequest, error) {
	mr, _, err := c.client.MergeRequests.GetMergeRequest(projectID, mrID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request: %w", err)
	}
	return mr, nil
}

func (c *Client) ListMergeRequestApprovals(projectID interface{}, mrID int) (*common.Approvals, error) {
	// List notes
	notes, err := c.getAllNotes(projectID, mrID)
	if err != nil {
		return nil, fmt.Errorf("failed to list notes: %w", err)
	}
	// Map to store latest approval status for each user
	userApprovals := make(map[int]common.ApprovalInfo)

	for _, note := range notes {
		// Check if note is system-generated and contains approval information
		if !note.System {
			continue
		}

		var status string
		noteBody := strings.ToLower(note.Body)

		// Check for approval patterns in system notes
		if strings.EqualFold(noteBody, "approved this merge request") {
			status = "approved"
		} else if strings.EqualFold(noteBody, "unapproved this merge request") {
			status = "unapproved"
		} else {
			// Skip notes that aren't approval-related
			continue
		}

		// Get existing approval info for this user
		existing, exists := userApprovals[note.Author.ID]

		// Update if this is the first entry for user or if this note is newer
		if !exists || (note.UpdatedAt != nil && (existing.UpdatedAt == nil || note.UpdatedAt.After(*existing.UpdatedAt))) {
			userApprovals[note.Author.ID] = common.ApprovalInfo{
				UserID:    note.Author.ID,
				Username:  note.Author.Username,
				Status:    status,
				UpdatedAt: note.UpdatedAt,
			}
		}
	}
	count := 0
	for _, approval := range userApprovals {
		if approval.Status == "approved" {
			count++
		}
	}

	approvals := common.Approvals{
		ApprovalsCount: count,
		ApprovalsInfo:  userApprovals,
	}

	return &approvals, nil
}

func (c *Client) ListMergeRequestCommits(projectID interface{}, mrID int) ([]*gitlab.Commit, error) {
	commits, _, err := c.client.MergeRequests.GetMergeRequestCommits(projectID, mrID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request commits: %w", err)
	}
	return commits, nil
}

func (c *Client) CreateUpdateMergeRequestDiscussion(projectID interface{}, mrID int, note string, passed bool) error {
	identifier := "Merge Request Compliance Report"

	// List discussions
	discussions, err := c.getAllDiscussions(projectID, mrID)
	if err != nil {
		return fmt.Errorf("failed to list discussions: %w", err)
	}

	for _, d := range discussions {
		for _, n := range d.Notes {
			if n.System || n.Body == "" {
				continue
			}
			if strings.Contains(n.Body, identifier) {
				// Update the existing note
				_, _, err := c.client.Notes.UpdateMergeRequestNote(projectID, mrID, n.ID, &gitlab.UpdateMergeRequestNoteOptions{
					Body: &note,
				})
				if err != nil {
					return fmt.Errorf("failed to update discussion: %w", err)
				}
				if n.Resolved != passed {
					c.client.Discussions.ResolveMergeRequestDiscussion(projectID, mrID, d.ID, &gitlab.ResolveMergeRequestDiscussionOptions{
						Resolved: &passed,
					})
					if err != nil {
						return fmt.Errorf("failed to resolve discussion: %w", err)
					}
				}

				return nil
			}
		}

	}

	cdOpts := &gitlab.CreateMergeRequestDiscussionOptions{
		Body: &note,
	}

	// Create discussion
	cD, _, err := c.client.Discussions.CreateMergeRequestDiscussion(projectID, mrID, cdOpts)
	if err != nil {
		return fmt.Errorf("failed to create merge request discussion: %w", err)
	}
	fmt.Printf("Created discussion, id: %s\n", cD.ID)

	// Set resolve status
	c.client.Discussions.ResolveMergeRequestDiscussion(projectID, mrID, cD.ID, &gitlab.ResolveMergeRequestDiscussionOptions{
		Resolved: &passed,
	})
	if err != nil {
		return fmt.Errorf("failed to set resolve status: %w", err)
	}

	return nil
}

func (c *Client) CreateMergeRequestNote(projectID interface{}, mrID int, note string) error {
	opts := &gitlab.CreateMergeRequestNoteOptions{
		Body: &note,
	}
	_, _, err := c.client.Notes.CreateMergeRequestNote(projectID, mrID, opts)
	if err != nil {
		return fmt.Errorf("failed to create merge request note: %w", err)
	}
	return nil
}

func (c *Client) SetCommitStatus(projectID interface{}, sha, state, description string) error {
	opts := &gitlab.SetCommitStatusOptions{
		State:       gitlab.BuildStateValue(state),
		Description: &description,
	}

	_, _, err := c.client.Commits.SetCommitStatus(projectID, sha, opts)
	if err != nil {
		return fmt.Errorf("failed to set commit status: %w", err)
	}
	return nil
}

func (c *Client) GetConfigFile(projectID interface{}) (*gitlab.File, error) {
	// Check default branch
	cP, _, err := c.client.Projects.GetProject(projectID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}
	cfg, _, err := c.client.RepositoryFiles.GetFile(projectID, ".mr-conform.yaml", &gitlab.GetFileOptions{
		Ref: &cP.DefaultBranch,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to config file: %w", err)
	}
	return cfg, nil
}

func (c *Client) getAllDiscussions(projectID interface{}, mrID int) ([]*gitlab.Discussion, error) {
	var allDiscussions []*gitlab.Discussion
	opt := &gitlab.ListMergeRequestDiscussionsOptions{
		PerPage: 100,
	}

	for {
		discussions, resp, err := c.client.Discussions.ListMergeRequestDiscussions(projectID, mrID, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list discussions: %w", err)
		}

		allDiscussions = append(allDiscussions, discussions...)

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allDiscussions, nil
}

func (c *Client) getAllNotes(projectID interface{}, mrID int) ([]*gitlab.Note, error) {
	var allNotes []*gitlab.Note
	opt := &gitlab.ListMergeRequestNotesOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}

	for {
		notes, resp, err := c.client.Notes.ListMergeRequestNotes(projectID, mrID, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list notes: %w", err)
		}

		allNotes = append(allNotes, notes...)

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allNotes, nil
}

func (c *Client) GetAllDiffsPaths(projectID interface{}, mrID int) ([]string, error) {
	var allDiffs []*gitlab.MergeRequestDiff
	opt := &gitlab.ListMergeRequestDiffsOptions{ListOptions: gitlab.ListOptions{PerPage: 20}}

	for {
		diffs, resp, err := c.client.MergeRequests.ListMergeRequestDiffs(projectID, mrID, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list diffs: %w", err)
		}

		allDiffs = append(allDiffs, diffs...)

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	var allPaths []string

	for _, diff := range allDiffs {
		if diff.DeletedFile {
			allPaths = append(allPaths, diff.OldPath)
		} else {
			allPaths = append(allPaths, diff.NewPath)
		}
	}

	return allPaths, nil
}

func (c *Client) GetCodeownersFile(projectID interface{}) (*gitlab.File, error) {
	// Check default branch
	cP, _, err := c.client.Projects.GetProject(projectID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}
	co, _, err := c.client.RepositoryFiles.GetFile(projectID, ".gitlab/CODEOWNERS", &gitlab.GetFileOptions{
		Ref: &cP.DefaultBranch,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to CODEOWNERS file: %w", err)
	}

	return co, nil
}
func (c *Client) ListProjectMembers(projectID interface{}) ([]*gitlab.ProjectMember, error) {
	var allMembers []*gitlab.ProjectMember
	opt := &gitlab.ListProjectMembersOptions{ListOptions: gitlab.ListOptions{PerPage: 20}}

	for {
		members, resp, err := c.client.ProjectMembers.ListAllProjectMembers(projectID, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list project members: %w", err)
		}

		allMembers = append(allMembers, members...)

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	var activeMembers []*gitlab.ProjectMember
	now := time.Now()

	for _, member := range allMembers {
		if member.State == "active" && (member.ExpiresAt == nil || time.Time(*member.ExpiresAt).After(now)) {
			activeMembers = append(activeMembers, member)
		}
	}

	return activeMembers, nil
}
