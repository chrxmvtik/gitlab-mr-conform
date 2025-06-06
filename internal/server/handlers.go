package server

import (
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

type Webhook struct {
	Secret         string
	EventsToAccept []gitlabapi.EventType
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "gitlab-mr-conformity-bot",
	})
}

func (s *Server) handleWebhook(c *gin.Context) {
	wh := Webhook{
		Secret:         s.config.GitLab.SecretToken,
		EventsToAccept: []gitlabapi.EventType{gitlabapi.EventTypeMergeRequest, gitlabapi.EventTypeNote},
	}

	// If we have a secret set, we should check if the request matches it.
	if len(s.config.GitLab.SecretToken) > 0 {
		signature := c.Request.Header.Get("X-Gitlab-Token")
		if signature != wh.Secret {
			s.logger.Error("missing X-Gitlab-Event Header", "error")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Secret token validation failed"})
			return
		}
	}

	event := c.Request.Header.Get("X-Gitlab-Event")
	if strings.TrimSpace(event) == "" {
		s.logger.Error("missing X-Gitlab-Event Header", "error")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil || len(payload) == 0 {
		s.logger.Error("Failed to read webhook payload", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request payload"})
		return
	}

	eventType := gitlabapi.EventType(event)
	if !isEventSubscribed(eventType, wh.EventsToAccept) {
		s.logger.Error("Event not defined to be parsed", "error", eventType)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Event not defined to be parsed"})
		return
	}

	// Parse webhook event
	parsedEvent, err := gitlabapi.ParseWebhook(eventType, payload)
	if err != nil {
		s.logger.Error("Failed to parse webhook event", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook payload"})
		return
	}

	switch parsedEvent := parsedEvent.(type) {
	case *gitlabapi.MergeEvent:
		s.logger.Info("Processing merge request event",
			"project_id", parsedEvent.Project.ID,
			"mr_id", parsedEvent.ObjectAttributes.IID,
			"action", parsedEvent.ObjectAttributes.Action)

		// Check merge request conformity
		result, err := s.checker.CheckMergeRequest(parsedEvent.Project.ID, parsedEvent.ObjectAttributes.IID)
		if err != nil {
			s.logger.Error("Failed to check merge request",
				"project_id", parsedEvent.Project.ID,
				"mr_id", parsedEvent.ObjectAttributes.IID,
				"error", err)
			c.JSON(http.StatusOK, gin.H{"error": "Check failed"})
			return
		}

		// Post discussion with results
		if err := s.gitlabClient.CreateUpdateMergeRequestDiscussion(parsedEvent.Project.ID, parsedEvent.ObjectAttributes.IID, result.Summary, result.Passed); err != nil {
			s.logger.Error("Failed to post discussion", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to post discussion"})
			return
		}

		// Set commit status
		status := "success"
		if !result.Passed {
			status = "failed"
		}

		if err := s.gitlabClient.SetCommitStatus(parsedEvent.Project.ID, parsedEvent.ObjectAttributes.LastCommit.ID, status, "MR Conformity Check"); err != nil {
			s.logger.Error("Failed to set commit status", "error", err)
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "Processed successfully",
			"passed":   result.Passed,
			"failures": len(result.Failures),
		})
	}
}

func (s *Server) handleStatus(c *gin.Context) {
	projectID := c.Param("project_id")
	mrIDStr := c.Param("mr_id")

	mrID, err := strconv.Atoi(mrIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid MR ID"})
		return
	}

	result, err := s.checker.CheckMergeRequest(projectID, mrID)
	if err != nil {
		s.logger.Error("Failed to check merge request", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Check failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"passed":   result.Passed,
		"failures": result.Failures,
		"summary":  result.Summary,
	})
}

func isEventSubscribed(event gitlabapi.EventType, events []gitlabapi.EventType) bool {
	return slices.Contains(events, event)
}
