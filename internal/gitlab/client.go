package gitlab

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

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

func (c *Client) ListMergeRequestCommits(projectID interface{}, mrID int) ([]*gitlab.Commit, error) {
	commits, _, err := c.client.MergeRequests.GetMergeRequestCommits(projectID, mrID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request commits: %w", err)
	}
	return commits, nil
}

func (c *Client) CreateUpdateMergeRequestDiscussion(projectID interface{}, mrID int, note string, passed bool) error {
	identifier := "MR Conformity Check Summary"

	// List discussions
	discussions, _, err := c.client.Discussions.ListMergeRequestDiscussions(projectID, mrID, &gitlab.ListMergeRequestDiscussionsOptions{
		Pagination: *gitlab.Ptr("1000"),
	})
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

	cD, _, err := c.client.Discussions.CreateMergeRequestDiscussion(projectID, mrID, cdOpts)
	if err != nil {
		return fmt.Errorf("failed to create merge request discussion: %w", err)
	}
	fmt.Printf("Created discussion, id: %s\n", cD.ID)
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
		State:       gitlab.BuildStateValue(state), //gitlab.BuildState(gitlab.BuildStateValue(state)),
		Description: &description,
		//Context:     gitlab.WithContext(), //gitlab.String("mr-conformity-bot"),
	}

	_, _, err := c.client.Commits.SetCommitStatus(projectID, sha, opts)
	if err != nil {
		return fmt.Errorf("failed to set commit status: %w", err)
	}
	return nil
}
