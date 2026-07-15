package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type recordedStatusPost struct {
	State      string `json:"state"`
	Name       string `json:"name"`
	PipelineID *int64 `json:"pipeline_id"`
}

func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient("test-token", server.URL, false)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client, server
}

func statusServerHandler(t *testing.T, existingStatuses string, posts *[]recordedStatusPost) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/1/repository/commits/abc123/statuses", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("name"); got != commitStatusName {
			t.Errorf("expected name query %q, got %q", commitStatusName, got)
		}
		fmt.Fprint(w, existingStatuses)
	})
	mux.HandleFunc("/api/v4/projects/1/statuses/abc123", func(w http.ResponseWriter, r *http.Request) {
		var post recordedStatusPost
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			t.Errorf("failed to decode status post: %v", err)
		}
		*posts = append(*posts, post)
		fmt.Fprint(w, `{"id": 1}`)
	})
	return mux
}

func TestSetCommitStatusFirstPostOmitsPipelineID(t *testing.T) {
	var posts []recordedStatusPost
	client, _ := newTestClient(t, statusServerHandler(t, `[]`, &posts))

	if err := client.SetCommitStatus(1, "abc123", "failed", "MR Conformity Check"); err != nil {
		t.Fatalf("SetCommitStatus failed: %v", err)
	}

	if len(posts) != 1 {
		t.Fatalf("expected 1 status post, got %d", len(posts))
	}
	if posts[0].PipelineID != nil {
		t.Errorf("expected no pipeline_id on first post, got %d", *posts[0].PipelineID)
	}
	if posts[0].Name != commitStatusName {
		t.Errorf("expected name %q, got %q", commitStatusName, posts[0].Name)
	}
	if posts[0].State != "failed" {
		t.Errorf("expected state failed, got %q", posts[0].State)
	}
}

func TestSetCommitStatusUpdatesEveryPipelineWithExistingStatus(t *testing.T) {
	existing := fmt.Sprintf(
		`[{"id": 10, "name": %[1]q, "status": "failed", "pipeline_id": 111},
		  {"id": 11, "name": %[1]q, "status": "failed", "pipeline_id": 222},
		  {"id": 12, "name": %[1]q, "status": "failed", "pipeline_id": 111}]`,
		commitStatusName)
	var posts []recordedStatusPost
	client, _ := newTestClient(t, statusServerHandler(t, existing, &posts))

	if err := client.SetCommitStatus(1, "abc123", "success", "MR Conformity Check"); err != nil {
		t.Fatalf("SetCommitStatus failed: %v", err)
	}

	if len(posts) != 2 {
		t.Fatalf("expected 2 status posts (one per distinct pipeline), got %d", len(posts))
	}
	gotPipelines := make(map[int64]bool)
	for _, post := range posts {
		if post.PipelineID == nil {
			t.Fatal("expected pipeline_id to be set on update post")
		}
		gotPipelines[*post.PipelineID] = true
		if post.State != "success" {
			t.Errorf("expected state success, got %q", post.State)
		}
	}
	if !gotPipelines[111] || !gotPipelines[222] {
		t.Errorf("expected posts to pipelines 111 and 222, got %v", gotPipelines)
	}
}

func TestSetCommitStatusFailsWhenListingStatusesFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/1/repository/commits/abc123/statuses", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message": "bad request"}`, http.StatusBadRequest)
	})
	client, _ := newTestClient(t, mux)

	if err := client.SetCommitStatus(1, "abc123", "success", "MR Conformity Check"); err == nil {
		t.Fatal("expected error when listing statuses fails, got nil")
	}
}
