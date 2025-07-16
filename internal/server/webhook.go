package server

import (
	"context"
	"fmt"
	"gitlab-mr-conformity-bot/internal/queue"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

// HandleWebhook processes incoming webhook and enqueues it for processing
func (s *Server) HandleWebhook(c *gin.Context) {
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
			"projectId", parsedEvent.Project.ID,
			"mrId", parsedEvent.ObjectAttributes.IID,
			"action", parsedEvent.ObjectAttributes.Action)

		pID := strconv.Itoa(parsedEvent.Project.ID)
		mrID := strconv.Itoa(parsedEvent.ObjectAttributes.IID)
		// Enqueue the webhook for processing
		jobID, err := s.queueManager.EnqueueWebhook(c, pID, mrID, parsedEvent.EventType, parsedEvent)
		if err != nil {
			s.logger.Error("Failed to enqueue webhook event", "error", err)
			return
		}
		//log.Printf("Webhook enqueued successfully with job ID: %s", jobID)
		s.logger.Info("Webhook enqueued successfully", "jobId", jobID)
		return
	}

}

// ProcessJob implements the JobProcessor interface
func (s *Server) ProcessJob(c context.Context, job *queue.WebhookJob) error {
	s.logger.Info("Processing webhook for MR", "jobId", job.ID, "webhookType", job.WebhookType, "projectId", job.ProjectID, "mrId", job.MergeRequestIID)

	mrID, err := strconv.Atoi(job.MergeRequestIID)
	if err != nil {
		fmt.Println("Error converting string to int:", err)
	}

	// Check merge request conformity
	result, err := s.checker.CheckMergeRequest(job.ProjectID, mrID)
	if err != nil {
		s.logger.Error("Failed to check merge request",
			"jobId", job.ID,
			"projectId", job.ProjectID,
			"mrId", job.MergeRequestIID,
			"error", err)
		return err
	}

	// Post discussion with results
	if err := s.gitlabClient.CreateUpdateMergeRequestDiscussion(job.ProjectID, mrID, result.Summary, result.Passed); err != nil {
		s.logger.Error("Failed to post discussion",
			"jobId", job.ID,
			"projectId", job.ProjectID,
			"mrId", job.MergeRequestIID,
			"error", err)
		return err
	}

	// Set commit status
	status := "success"
	if !result.Passed {
		status = "failed"
	}

	if err := s.gitlabClient.SetCommitStatus(job.ProjectID, job.Payload.ObjectAttributes.LastCommit.ID, status, "MR Conformity Check"); err != nil {
		s.logger.Error("Failed to set commit status",
			"jobId", job.ID,
			"projectId", job.ProjectID,
			"mrId", job.MergeRequestIID,
			"error", err)
		return err
	}

	return nil
}

// StartProcessor starts the background job processor
func (s *Server) StartProcessor(c context.Context) {
	s.logger.Info("Starting webhook processor")
	s.queueManager.StartProcessor(c, s)
}

// StopProcessor stops the background job processor
func (s *Server) StopProcessor() {
	s.logger.Info("Stopping webhook processor...")
	s.queueManager.StopProcessor()
}

// Health check methods

func (s *Server) Health(c context.Context) error {
	return s.queueManager.Health(c)
}

func (s *Server) GetStats(c context.Context) (*queue.QueueStats, error) {
	return s.queueManager.GetQueueStats(c)
}

func isEventSubscribed(event gitlabapi.EventType, events []gitlabapi.EventType) bool {
	return slices.Contains(events, event)
}
