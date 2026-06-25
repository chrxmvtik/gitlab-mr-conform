package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

// systemHookObjectKind is a minimal struct used to peek at the object_kind field
// of a GitLab system hook payload without performing a full JSON parse.
type systemHookObjectKind struct {
	ObjectKind string `json:"object_kind"`
}

// peekObjectKind extracts only the top-level "object_kind" field from a raw JSON
// system hook payload. It is intentionally lightweight to allow early discard of
// non-relevant events before the more expensive full parse.
func peekObjectKind(data []byte) (string, error) {
	var envelope systemHookObjectKind
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", err
	}
	return envelope.ObjectKind, nil
}

// handleSystemHookNoQueue processes incoming GitLab system hook events directly (no queue).
func (s *Server) handleSystemHookNoQueue(c *gin.Context) {
	payload, ok := s.readSystemHookPayload(c)
	if !ok {
		return
	}

	// Early discard: peek at object_kind before the more expensive full parse.
	// For large GitLab instances with thousands of repos, repository_update, push,
	// tag_push and other non-MR events arrive at high frequency and would otherwise
	// all incur a full JSON unmarshal.
	if objectKind, err := peekObjectKind(payload); err == nil && objectKind != "merge_request" {
		s.logger.Debug("System hook event discarded early (not a merge request)", "object_kind", objectKind)
		c.JSON(http.StatusOK, gin.H{"message": "Event ignored (not a merge request)"})
		return
	}

	parsedEvent, err := gitlabapi.ParseSystemhook(payload)
	if err != nil {
		s.logger.Error("Failed to parse system hook payload", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid system hook payload"})
		return
	}

	mergeEvent, ok := parsedEvent.(*gitlabapi.MergeEvent)
	if !ok {
		// Non-MR system hook events are silently accepted so GitLab does not mark the hook as failed.
		s.logger.Debug("System hook event ignored (not a merge request)", "type", parsedEvent)
		c.JSON(http.StatusOK, gin.H{"message": "Event ignored (not a merge request)"})
		return
	}

	s.logger.Debug("System hook parsed",
		"event_type", mergeEvent.EventType,
		"target_project_id", mergeEvent.ObjectAttributes.TargetProjectID,
		"source_project_id", mergeEvent.ObjectAttributes.SourceProjectID,
		"mr_id", mergeEvent.ObjectAttributes.ID,
		"mr_iid", mergeEvent.ObjectAttributes.IID,
		"action", mergeEvent.ObjectAttributes.Action,
		"last_commit_sha", mergeEvent.ObjectAttributes.LastCommit.ID)

	if !isRelevantMergeAction(mergeEvent.ObjectAttributes.Action) {
		s.logger.Debug("System hook merge event ignored", "action", mergeEvent.ObjectAttributes.Action)
		c.JSON(http.StatusOK, gin.H{"message": "Event ignored (action not relevant)"})
		return
	}

	projectID := strconv.FormatInt(int64(mergeEvent.ObjectAttributes.TargetProjectID), 10)
	mrIID := int64(mergeEvent.ObjectAttributes.IID)

	if mrIID == 0 {
		s.logger.Error("System hook MR event has iid=0, cannot process",
			"project_id", projectID,
			"mr_id", mergeEvent.ObjectAttributes.ID,
			"raw_payload_hint", "check that object_attributes.iid is present in the GitLab system hook payload")
		c.JSON(http.StatusOK, gin.H{"error": "Cannot process: MR IID is 0"})
		return
	}

	s.logger.Info("Processing system hook merge request event",
		"project_id", projectID,
		"mr_iid", mrIID,
		"action", mergeEvent.ObjectAttributes.Action)

	result, err := s.checker.CheckMergeRequestForSystemHook(projectID, int(mrIID))
	if err != nil {
		s.logger.Error("Failed to check merge request",
			"project_id", projectID,
			"mr_iid", mrIID,
			"error_detail", err.Error())
		c.JSON(http.StatusOK, gin.H{"error": "Check failed", "detail": err.Error()})
		return
	}

	if result.Skipped {
		s.logger.Info("Skipping repository: no .mr-conform.yaml found", "project_id", projectID, "mr_iid", mrIID)
		c.JSON(http.StatusOK, gin.H{"message": "Skipped: no .mr-conform.yaml in repository"})
		return
	}

	if err := s.gitlabClient.CreateUpdateMergeRequestDiscussion(projectID, mrIID, result.Summary, result.Passed); err != nil {
		s.logger.Error("Failed to post discussion",
			"project_id", projectID,
			"mr_iid", mrIID,
			"error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to post discussion"})
		return
	}

	if commitSHA := mergeEvent.ObjectAttributes.LastCommit.ID; commitSHA != "" {
		status := "success"
		if !result.Passed {
			status = "failed"
		}
		if err := s.gitlabClient.SetCommitStatus(projectID, commitSHA, status, "MR Conformity Check"); err != nil {
			s.logger.Error("Failed to set commit status",
				"project_id", projectID,
				"mr_iid", mrIID,
				"commit_sha", commitSHA,
				"error", err)
		}
	} else {
		s.logger.Debug("Skipping commit status: last_commit.id is empty", "project_id", projectID, "mr_iid", mrIID)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Processed successfully",
		"passed":   result.Passed,
		"failures": len(result.Failures),
	})
}

// HandleSystemHook processes incoming GitLab system hook events and enqueues them for processing.
func (s *Server) HandleSystemHook(c *gin.Context) {
	payload, ok := s.readSystemHookPayload(c)
	if !ok {
		return
	}

	// Early discard: same optimisation as handleSystemHookNoQueue — avoid a full
	// JSON parse for the high-volume non-MR events before they ever reach the queue.
	if objectKind, err := peekObjectKind(payload); err == nil && objectKind != "merge_request" {
		s.logger.Debug("System hook event discarded early (not a merge request)", "object_kind", objectKind)
		c.JSON(http.StatusOK, gin.H{"message": "Event ignored (not a merge request)"})
		return
	}

	parsedEvent, err := gitlabapi.ParseSystemhook(payload)
	if err != nil {
		s.logger.Error("Failed to parse system hook payload", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid system hook payload"})
		return
	}

	mergeEvent, ok := parsedEvent.(*gitlabapi.MergeEvent)
	if !ok {
		s.logger.Debug("System hook event ignored (not a merge request)")
		c.JSON(http.StatusOK, gin.H{"message": "Event ignored (not a merge request)"})
		return
	}

	if !isRelevantMergeAction(mergeEvent.ObjectAttributes.Action) {
		s.logger.Debug("System hook merge event ignored", "action", mergeEvent.ObjectAttributes.Action)
		c.JSON(http.StatusOK, gin.H{"message": "Event ignored (action not relevant)"})
		return
	}

	projectID := strconv.FormatInt(int64(mergeEvent.ObjectAttributes.TargetProjectID), 10)
	mrIIDInt := int64(mergeEvent.ObjectAttributes.IID)

	if mrIIDInt == 0 {
		s.logger.Error("System hook MR event has iid=0, cannot enqueue",
			"project_id", projectID,
			"mr_id", mergeEvent.ObjectAttributes.ID)
		c.JSON(http.StatusOK, gin.H{"error": "Cannot process: MR IID is 0"})
		return
	}

	mrIID := strconv.FormatInt(mrIIDInt, 10)

	s.logger.Info("Enqueuing system hook merge request event",
		"project_id", projectID,
		"mr_iid", mrIID,
		"action", mergeEvent.ObjectAttributes.Action)

	jobID, err := s.queueManager.EnqueueWebhook(c, projectID, mrIID, string(mergeEvent.EventType), mergeEvent)
	if err != nil {
		s.logger.Error("Failed to enqueue system hook event", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue event"})
		return
	}

	s.logger.Info("System hook event enqueued successfully", "job_id", jobID)
	c.JSON(http.StatusOK, gin.H{"message": "Enqueued successfully", "job_id": jobID})
}

// readSystemHookPayload validates the system hook token and reads the request body.
// Returns the payload bytes and true on success, or writes an error response and returns false.
func (s *Server) readSystemHookPayload(c *gin.Context) ([]byte, bool) {
	if len(s.config.GitLab.SystemHookSecretToken) > 0 {
		token := c.Request.Header.Get("X-Gitlab-Token")
		if token != s.config.GitLab.SystemHookSecretToken {
			s.logger.Error("System hook token validation failed")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Secret token validation failed"})
			return nil, false
		}
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil || len(payload) == 0 {
		s.logger.Error("Failed to read system hook payload", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request payload"})
		return nil, false
	}

	return payload, true
}

// isRelevantMergeAction returns true for MR actions that should trigger a conformity check.
func isRelevantMergeAction(action string) bool {
	switch action {
	case "open", "reopen", "update":
		return true
	default:
		return false
	}
}

// processSystemHookMergeEvent contains the shared business logic for system hook MR processing.
// It is used by both queue-based and direct processors.
func (s *Server) processSystemHookMergeEvent(ctx context.Context, projectID string, mrIID int, lastCommitSHA string) error {
	result, err := s.checker.CheckMergeRequestForSystemHook(projectID, mrIID)
	if err != nil {
		return err
	}

	if result.Skipped {
		s.logger.Info("Skipping repository: no .mr-conform.yaml found", "project_id", projectID, "mr_iid", mrIID)
		return nil
	}

	if err := s.gitlabClient.CreateUpdateMergeRequestDiscussion(projectID, int64(mrIID), result.Summary, result.Passed); err != nil {
		return err
	}

	status := "success"
	if !result.Passed {
		status = "failed"
	}

	return s.gitlabClient.SetCommitStatus(projectID, lastCommitSHA, status, "MR Conformity Check")
}
