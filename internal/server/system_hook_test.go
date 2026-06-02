package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gitlab-mr-conformity-bot/internal/config"
	"gitlab-mr-conformity-bot/internal/conformity"
	"gitlab-mr-conformity-bot/internal/gitlab"
	"gitlab-mr-conformity-bot/pkg/logger"
)

// systemHookMRPayload returns a minimal but valid system hook merge_request JSON payload.
func systemHookMRPayload(projectID int, mrIID int, action string, lastCommitSHA string) []byte {
	payload := map[string]interface{}{
		"object_kind": "merge_request",
		"event_type":  "merge_request",
		"user": map[string]interface{}{
			"name":     "Test User",
			"username": "testuser",
		},
		"project": map[string]interface{}{
			"name":               "Test Project",
			"path_with_namespace": "group/test-project",
			"web_url":            "http://gitlab.example.com/group/test-project",
		},
		"object_attributes": map[string]interface{}{
			"id":                mrIID * 10,
			"iid":               mrIID,
			"title":             "Test MR",
			"state":             "opened",
			"source_branch":     "feature/test",
			"target_branch":     "main",
			"source_project_id": projectID,
			"target_project_id": projectID,
			"action":            action,
			"last_commit": map[string]interface{}{
				"id":      lastCommitSHA,
				"message": "test commit",
				"author": map[string]interface{}{
					"name":  "Test User",
					"email": "test@example.com",
				},
			},
			"work_in_progress": false,
		},
	}
	data, _ := json.Marshal(payload)
	return data
}

// pushSystemHookPayload returns a valid system hook push event payload (non-MR).
func pushSystemHookPayload() []byte {
	payload := map[string]interface{}{
		"object_kind": "push",
		"event_name":  "push",
		"user_id":     1,
		"user_name":   "Test User",
		"project_id":  1,
	}
	data, _ := json.Marshal(payload)
	return data
}

// newTestServerWithMockGitLab creates a Server backed by a real gitlab.Client pointed at
// a mock GitLab HTTP server. It returns the Server and a teardown function.
//
// mockHandler is the http.HandlerFunc that responds to GitLab API calls.
// When mockHandler is nil a default handler returning minimal valid JSON is used.
func newTestServerWithMockGitLab(t *testing.T, cfg *config.Config, mockHandler http.HandlerFunc) (*Server, func()) {
	t.Helper()

	if mockHandler == nil {
		mockHandler = defaultMockGitLabHandler()
	}

	mockGitLab := httptest.NewServer(mockHandler)

	cfg.GitLab.BaseURL = mockGitLab.URL
	cfg.GitLab.Token = "test-token"

	gitlabClient, err := gitlab.NewClient(cfg.GitLab.Token, cfg.GitLab.BaseURL, false)
	if err != nil {
		t.Fatalf("failed to create test GitLab client: %v", err)
	}

	log := logger.New()
	checker := conformity.NewChecker(cfg.Rules, gitlabClient, log, cfg.Integrations)

	srv := &Server{
		config:       cfg,
		gitlabClient: gitlabClient,
		checker:      checker,
		logger:       log,
	}

	return srv, mockGitLab.Close
}

// defaultMockGitLabHandler returns a handler that answers common GitLab API calls with
// minimal valid JSON so that the conformity checker can complete without errors.
// The .mr-conform.yaml file returns 404 (not found), so the repo is skipped.
func defaultMockGitLabHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Next-Page", "")
		w.Header().Set("X-Page", "1")
		w.Header().Set("X-Total-Pages", "1")

		path := r.URL.Path
		method := r.Method

		switch {
		// Sub-resource routes must come before parent routes (most specific first)

		// GET .../merge_requests/:iid/notes
		case method == http.MethodGet && pathContains(path, "/notes"):
			fmt.Fprintln(w, `[]`)

		// GET .../merge_requests/:iid/commits
		case method == http.MethodGet && pathContains(path, "/commits"):
			fmt.Fprintln(w, `[{"id":"abc123","title":"test commit","author_name":"Test","created_at":"2024-01-01T00:00:00Z","message":"test commit"}]`)

		// GET .../merge_requests/:iid/discussions
		// POST .../merge_requests/:iid/discussions
		case (method == http.MethodGet || method == http.MethodPost) && pathContains(path, "/discussions"):
			if method == http.MethodGet {
				fmt.Fprintln(w, `[]`)
			} else {
				fmt.Fprintln(w, `{"id":"disc1","notes":[{"id":1,"body":"test","resolved":false,"system":false}]}`)
			}

		// PUT .../discussions/:did — resolve
		case method == http.MethodPut && pathContains(path, "/discussions/"):
			fmt.Fprintln(w, `{"id":"disc1","resolved":true}`)

		// GET .../merge_requests/:iid/diffs
		case method == http.MethodGet && pathContains(path, "/diffs"):
			fmt.Fprintln(w, `[]`)

		// GET .../merge_requests/:iid — specific MR (no trailing sub-resource)
		case method == http.MethodGet && pathContains(path, "/merge_requests/"):
			fmt.Fprintln(w, `{"id":10,"iid":1,"title":"Test MR","state":"opened","author":{"id":1,"username":"testuser","name":"Test User"},"description":"test description"}`)

		// POST .../statuses/:sha — commit status
		case method == http.MethodPost && pathContains(path, "/statuses/"):
			fmt.Fprintln(w, `{"id":1,"sha":"abc123","status":"success"}`)

		// GET /api/v4/projects/:id/repository/files/... — config file not found
		case method == http.MethodGet && pathContains(path, "/repository/files/"):
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, `{"message":"404 Not Found"}`)

		// GET /api/v4/projects/:id — project info (must come after all sub-resource project routes)
		case method == http.MethodGet && pathContains(path, "/api/v4/projects/"):
			fmt.Fprintln(w, `{"id":1,"name":"Test","default_branch":"main","path_with_namespace":"group/test"}`)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"message":"mock: unhandled %s %s"}`, method, path)
		}
	}
}

// mockGitLabHandlerWithConfigFile returns a handler like defaultMockGitLabHandler but serves
// the given content as the .mr-conform.yaml file (base64-encoded).
// Pass an empty string to simulate an empty (but present) config file.
func mockGitLabHandlerWithConfigFile(content string) http.HandlerFunc {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Next-Page", "")
		w.Header().Set("X-Page", "1")
		w.Header().Set("X-Total-Pages", "1")

		path := r.URL.Path
		method := r.Method

		switch {
		case method == http.MethodGet && pathContains(path, "/notes"):
			fmt.Fprintln(w, `[]`)
		case method == http.MethodGet && pathContains(path, "/commits"):
			fmt.Fprintln(w, `[{"id":"abc123","title":"test commit","author_name":"Test","created_at":"2024-01-01T00:00:00Z","message":"test commit"}]`)
		case (method == http.MethodGet || method == http.MethodPost) && pathContains(path, "/discussions"):
			if method == http.MethodGet {
				fmt.Fprintln(w, `[]`)
			} else {
				fmt.Fprintln(w, `{"id":"disc1","notes":[{"id":1,"body":"test","resolved":false,"system":false}]}`)
			}
		case method == http.MethodPut && pathContains(path, "/discussions/"):
			fmt.Fprintln(w, `{"id":"disc1","resolved":true}`)
		case method == http.MethodGet && pathContains(path, "/diffs"):
			fmt.Fprintln(w, `[]`)
		case method == http.MethodGet && pathContains(path, "/merge_requests/"):
			fmt.Fprintln(w, `{"id":10,"iid":1,"title":"Test MR","state":"opened","author":{"id":1,"username":"testuser","name":"Test User"},"description":"test description"}`)
		case method == http.MethodPost && pathContains(path, "/statuses/"):
			fmt.Fprintln(w, `{"id":1,"sha":"abc123","status":"success"}`)
		// Serve the .mr-conform.yaml config file
		case method == http.MethodGet && pathContains(path, "/repository/files/"):
			fmt.Fprintf(w, `{"file_name":".mr-conform.yaml","content":"%s","encoding":"base64"}`, encoded)
		case method == http.MethodGet && pathContains(path, "/api/v4/projects/"):
			fmt.Fprintln(w, `{"id":1,"name":"Test","default_branch":"main","path_with_namespace":"group/test"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"message":"mock: unhandled %s %s"}`, method, path)
		}
	}
}


func pathContains(path, sub string) bool {
	for i := 0; i <= len(path)-len(sub); i++ {
		if path[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// newTestRequest builds an HTTP request with the system hook headers.
func newTestRequest(payload []byte, token string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/system-hook", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gitlab-Event", "System Hook")
	if token != "" {
		req.Header.Set("X-Gitlab-Token", token)
	}
	return req
}

// invokeHandler runs the handleSystemHookNoQueue handler and returns the recorder.
func invokeHandler(srv *Server, req *http.Request) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	srv.handleSystemHookNoQueue(c)
	return w
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHandleSystemHookNoQueue_ValidMergeRequestEvent(t *testing.T) {
	cfg := &config.Config{}
	// Use a handler that serves a populated .mr-conform.yaml so the request is fully processed.
	srv, teardown := newTestServerWithMockGitLab(t, cfg, mockGitLabHandlerWithConfigFile("rules:\n  title:\n    enabled: false\n"))
	defer teardown()

	payload := systemHookMRPayload(1, 1, "open", "abc123sha")
	req := newTestRequest(payload, "")

	w := invokeHandler(srv, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["message"] != "Processed successfully" {
		t.Errorf("unexpected message: %v", resp["message"])
	}
}

func TestHandleSystemHookNoQueue_InvalidToken(t *testing.T) {
	cfg := &config.Config{}
	cfg.GitLab.SystemHookSecretToken = "correct-secret"
	srv, teardown := newTestServerWithMockGitLab(t, cfg, nil)
	defer teardown()

	payload := systemHookMRPayload(1, 1, "open", "abc123sha")
	req := newTestRequest(payload, "wrong-secret")

	w := invokeHandler(srv, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "Secret token validation failed" {
		t.Errorf("unexpected error: %v", resp["error"])
	}
}

func TestHandleSystemHookNoQueue_ValidTokenAccepted(t *testing.T) {
	cfg := &config.Config{}
	cfg.GitLab.SystemHookSecretToken = "my-secret"
	// Use a handler that serves a populated .mr-conform.yaml so the request is fully processed.
	srv, teardown := newTestServerWithMockGitLab(t, cfg, mockGitLabHandlerWithConfigFile("rules:\n  title:\n    enabled: false\n"))
	defer teardown()

	payload := systemHookMRPayload(1, 1, "open", "abc123sha")
	req := newTestRequest(payload, "my-secret")

	w := invokeHandler(srv, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}
}

func TestHandleSystemHookNoQueue_NonMergeRequestEvent(t *testing.T) {
	cfg := &config.Config{}
	srv, teardown := newTestServerWithMockGitLab(t, cfg, nil)
	defer teardown()

	payload := pushSystemHookPayload()
	req := newTestRequest(payload, "")

	w := invokeHandler(srv, req)

	// Non-MR events should be silently accepted (200) so GitLab doesn't mark the hook as failed.
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["message"] != "Event ignored (not a merge request)" {
		t.Errorf("unexpected message: %v", resp["message"])
	}
}

func TestHandleSystemHookNoQueue_IrrelevantMergeAction(t *testing.T) {
	cfg := &config.Config{}
	srv, teardown := newTestServerWithMockGitLab(t, cfg, nil)
	defer teardown()

	// "close" is not in the relevant actions list
	payload := systemHookMRPayload(1, 1, "close", "abc123sha")
	req := newTestRequest(payload, "")

	w := invokeHandler(srv, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["message"] != "Event ignored (action not relevant)" {
		t.Errorf("unexpected message: %v", resp["message"])
	}
}

func TestHandleSystemHookNoQueue_MalformedPayload(t *testing.T) {
	cfg := &config.Config{}
	srv, teardown := newTestServerWithMockGitLab(t, cfg, nil)
	defer teardown()

	req := newTestRequest([]byte("this is not json"), "")

	w := invokeHandler(srv, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestIsRelevantMergeAction(t *testing.T) {
	tests := []struct {
		action   string
		expected bool
	}{
		{"open", true},
		{"reopen", true},
		{"update", true},
		{"close", false},
		{"merge", false},
		{"approved", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := isRelevantMergeAction(tt.action)
			if got != tt.expected {
				t.Errorf("isRelevantMergeAction(%q) = %v, want %v", tt.action, got, tt.expected)
			}
		})
	}
}

// TestHandleSystemHookNoQueue_NoConfigFile_Skipped verifies that a repository without a
// .mr-conform.yaml file is silently skipped (200, no discussion or commit status posted).
func TestHandleSystemHookNoQueue_NoConfigFile_Skipped(t *testing.T) {
	cfg := &config.Config{}
	// defaultMockGitLabHandler returns 404 for /repository/files/ → repo is skipped.
	srv, teardown := newTestServerWithMockGitLab(t, cfg, nil)
	defer teardown()

	payload := systemHookMRPayload(1, 1, "open", "abc123sha")
	req := newTestRequest(payload, "")

	w := invokeHandler(srv, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["message"] != "Skipped: no .mr-conform.yaml in repository" {
		t.Errorf("expected skip message, got: %v", resp["message"])
	}
}

// TestHandleSystemHookNoQueue_EmptyConfigFile_UsesDefaults verifies that a repository with an
// empty .mr-conform.yaml falls back to the global default configuration and is processed normally.
func TestHandleSystemHookNoQueue_EmptyConfigFile_UsesDefaults(t *testing.T) {
	cfg := &config.Config{}
	// Serve an empty .mr-conform.yaml — should use global defaults and process the MR.
	srv, teardown := newTestServerWithMockGitLab(t, cfg, mockGitLabHandlerWithConfigFile(""))
	defer teardown()

	payload := systemHookMRPayload(1, 1, "open", "abc123sha")
	req := newTestRequest(payload, "")

	w := invokeHandler(srv, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["message"] != "Processed successfully" {
		t.Errorf("expected 'Processed successfully', got: %v", resp["message"])
	}
}
