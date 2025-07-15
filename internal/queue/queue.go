package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab-mr-conformity-bot/pkg/logger"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

// WebhookJob represents a webhook job in the queue
type WebhookJob struct {
	//Webhook         *gitlabapi.Event
	ID              string //`json:"id"`
	ProjectID       string //`json:"project_id"`
	MergeRequestIID string //`json:"merge_request_iid"`
	WebhookType     string //`json:"webhook_type"`
	Payload         *gitlabapi.MergeEvent
	CreatedAt       int64 //`json:"created_at"`
	Attempts        int   //`json:"attempts"`
	MaxAttempts     int   //`json:"max_attempts"`
}

// JobProcessor defines the interface for processing webhook jobs
type JobProcessor interface {
	ProcessJob(c context.Context, job *WebhookJob) error
}

// QueueManager manages Redis queues for GitLab MR webhooks
type QueueManager struct {
	redis              *redis.Client
	queuePrefix        string
	lockPrefix         string
	processingPrefix   string
	defaultLockTTL     time.Duration
	maxRetries         int
	processingInterval time.Duration
	isProcessing       bool
	stopChan           chan struct{}
	log                *logger.Logger
}

// Config holds configuration for the queue manager
type Config struct {
	RedisHost          string
	RedisPassword      string
	RedisDB            int
	QueuePrefix        string
	LockPrefix         string
	ProcessingPrefix   string
	DefaultLockTTL     time.Duration
	MaxRetries         int
	ProcessingInterval time.Duration
}

// NewQueueManager creates a new queue manager instance
func NewQueueManager(config *Config, log *logger.Logger) *QueueManager {
	if config == nil {
		config = &Config{}
	}

	// Set defaults
	if config.QueuePrefix == "" {
		config.QueuePrefix = "gitlab:mr:queue"
	}
	if config.LockPrefix == "" {
		config.LockPrefix = "gitlab:mr:lock"
	}
	if config.ProcessingPrefix == "" {
		config.ProcessingPrefix = "gitlab:mr:processing"
	}
	if config.DefaultLockTTL == 0 {
		config.DefaultLockTTL = 5 * time.Minute
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.ProcessingInterval == 0 {
		config.ProcessingInterval = 1 * time.Second
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     config.RedisHost,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	return &QueueManager{
		redis:              rdb,
		queuePrefix:        config.QueuePrefix,
		lockPrefix:         config.LockPrefix,
		processingPrefix:   config.ProcessingPrefix,
		defaultLockTTL:     config.DefaultLockTTL,
		maxRetries:         config.MaxRetries,
		processingInterval: config.ProcessingInterval,
		stopChan:           make(chan struct{}),
		log:                log,
	}
}

// EnqueueWebhook adds a webhook job to the queue for a specific MR
func (qm *QueueManager) EnqueueWebhook(c context.Context, projectID, mergeRequestIID, webhookType string, payload *gitlabapi.MergeEvent) (string, error) {
	jobID := uuid.New().String()
	job := &WebhookJob{
		ID:              jobID,
		ProjectID:       projectID,
		MergeRequestIID: mergeRequestIID,
		WebhookType:     webhookType,
		Payload:         payload,
		CreatedAt:       time.Now().Unix(),
		Attempts:        0,
		MaxAttempts:     qm.maxRetries,
	}

	jobData, err := json.Marshal(job)
	if err != nil {
		return "", fmt.Errorf("failed to marshal job: %w", err)
	}

	queueKey := qm.getQueueKey(projectID, mergeRequestIID)

	// Add job to the MR-specific queue (LPUSH for FIFO with RPOP)
	if err := qm.redis.LPush(c, queueKey, jobData).Err(); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}

	// Set queue expiration (cleanup after 24 hours if not processed)
	if err := qm.redis.Expire(c, queueKey, 24*time.Hour).Err(); err != nil {
		qm.log.Warn("Failed to set queue expiration", "error", err)
		//log.Printf("Warning: failed to set queue expiration: %v", err)
	}

	qm.log.Info("Enqueued webhook job", "jobId", jobID, "projectId", projectID, "mrId", mergeRequestIID)
	//log.Printf("Enqueued webhook job %s for MR %s:%s", jobID, projectID, mergeRequestIID)
	return jobID, nil
}

// ProcessMRQueue processes all queued jobs for a specific MR
func (qm *QueueManager) ProcessMRQueue(c context.Context, projectID, mergeRequestIID string, processor JobProcessor) error {
	queueKey := qm.getQueueKey(projectID, mergeRequestIID)
	lockKey := qm.getLockKey(projectID, mergeRequestIID)

	// Try to acquire lock for this MR
	locked, err := qm.acquireLock(c, lockKey)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		qm.log.Info("MR is already being processed", "projectId", projectID, "mrId", mergeRequestIID)
		return nil
	}

	defer func() {
		if err := qm.releaseLock(c, lockKey); err != nil {
			qm.log.Error("Error releasing lock", "error", err)
			//log.Printf("Error releasing lock: %v", err)
		}
	}()

	// Process jobs one by one from the queue
	for {
		job, err := qm.dequeueJob(c, queueKey)
		if err != nil {
			return fmt.Errorf("failed to dequeue job: %w", err)
		}
		if job == nil {
			break // No more jobs in queue
		}

		qm.log.Info("Processing job", "jobId", job.ID, "projectId", projectID, "mrId", mergeRequestIID)

		// Mark job as processing
		if err := qm.markJobAsProcessing(c, job); err != nil {
			//log.Printf("Warning: failed to mark job as processing: %v", err)
			qm.log.Warn("Failed to mark job as processing", "jobId", job.ID, "error", err)
		}

		// Execute the job
		if err := processor.ProcessJob(c, job); err != nil {
			//log.Printf("Error processing job %s: %v", job.ID, err)
			qm.log.Error("Error processing job", "jobId", job.ID, "error", err)
			if err := qm.handleJobFailure(c, job, queueKey, err); err != nil {
				//log.Printf("Error handling job failure: %v", err)
				qm.log.Error("Error handling job", "error", err)
			}
		} else {
			//log.Printf("Successfully processed job %s", job.ID)
			qm.log.Info("Successfully processed job", "jobId", job.ID)
			// Remove from processing set on success
			if err := qm.removeJobFromProcessing(c, job); err != nil {
				//log.Printf("Warning: failed to remove job from processing: %v", err)
				qm.log.Warn("Failed to remove job from processing", "jobId", job.ID, "error", err)
			}
		}
	}

	return nil
}

// StartProcessor starts the queue processor that continuously processes jobs
func (qm *QueueManager) StartProcessor(c context.Context, processor JobProcessor) {
	if qm.isProcessing {
		//log.Println("Queue processor is already running")
		qm.log.Info("Queue processor is already running")
		return
	}

	qm.isProcessing = true
	//log.Println("Starting GitLab MR queue processor...")
	qm.log.Info("Starting GitLab MR queue processor")

	go func() {
		defer func() {
			qm.isProcessing = false
			qm.log.Info("Queue processor stopped")
			//log.Println("Queue processor stopped")
		}()

		ticker := time.NewTicker(qm.processingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-c.Done():
				return
			case <-qm.stopChan:
				return
			case <-ticker.C:
				if err := qm.processAllQueues(c, processor); err != nil {
					//log.Printf("Error processing queues: %v", err)
					qm.log.Error("Error processing queues", "error", err)
				}
			}
		}
	}()
}

// StopProcessor stops the queue processor
func (qm *QueueManager) StopProcessor() {
	if !qm.isProcessing {
		return
	}

	//log.Println("Stopping GitLab MR queue processor...")
	qm.log.Info("Stopping GitLab MR queue processor")
	close(qm.stopChan)
}

// GetQueueStats returns statistics about the queues
func (qm *QueueManager) GetQueueStats(c context.Context) (*QueueStats, error) {
	queueKeys, err := qm.redis.Keys(c, qm.queuePrefix+":*").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get queue keys: %w", err)
	}

	processingKeys, err := qm.redis.Keys(c, qm.processingPrefix+":*").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get processing keys: %w", err)
	}

	var totalJobs int64
	var queueDetails []QueueDetail

	for _, queueKey := range queueKeys {
		parts := strings.Split(queueKey, ":")
		if len(parts) >= 5 {
			projectID := parts[3]
			mergeRequestIID := parts[4]

			jobCount, err := qm.redis.LLen(c, queueKey).Result()
			if err != nil {
				//log.Printf("Warning: failed to get queue length for %s: %v", queueKey, err)
				qm.log.Warn("Failed to get queue length", "key", queueKey, "error", err)
				continue
			}

			totalJobs += jobCount

			if jobCount > 0 {
				queueDetails = append(queueDetails, QueueDetail{
					ProjectID:       projectID,
					MergeRequestIID: mergeRequestIID,
					JobCount:        int(jobCount),
				})
			}
		}
	}

	return &QueueStats{
		TotalQueues:    len(queueKeys),
		TotalJobs:      int(totalJobs),
		ProcessingJobs: len(processingKeys),
		QueueDetails:   queueDetails,
	}, nil
}

// QueueStats represents queue statistics
type QueueStats struct {
	TotalQueues    int           `json:"total_queues"`
	TotalJobs      int           `json:"total_jobs"`
	ProcessingJobs int           `json:"processing_jobs"`
	QueueDetails   []QueueDetail `json:"queue_details"`
}

// QueueDetail represents details about a specific queue
type QueueDetail struct {
	ProjectID       string `json:"project_id"`
	MergeRequestIID string `json:"merge_request_iid"`
	JobCount        int    `json:"job_count"`
}

// ClearAllQueues clears all queues (useful for testing/debugging)
func (qm *QueueManager) ClearAllQueues(c context.Context) error {
	patterns := []string{
		qm.queuePrefix + ":*",
		qm.lockPrefix + ":*",
		qm.processingPrefix + ":*",
	}

	for _, pattern := range patterns {
		keys, err := qm.redis.Keys(c, pattern).Result()
		if err != nil {
			return fmt.Errorf("failed to get keys for pattern %s: %w", pattern, err)
		}

		if len(keys) > 0 {
			if err := qm.redis.Del(c, keys...).Err(); err != nil {
				return fmt.Errorf("failed to delete keys: %w", err)
			}
		}
	}

	return nil
}

// Close gracefully shuts down the queue manager
func (qm *QueueManager) Close() error {
	qm.StopProcessor()
	return qm.redis.Close()
}

// Health checks if the queue manager is healthy
func (qm *QueueManager) Health(c context.Context) error {
	return qm.redis.Ping(c).Err()
}

// Private helper methods

func (qm *QueueManager) getQueueKey(projectID, mergeRequestIID string) string {
	return fmt.Sprintf("%s:%s:%s", qm.queuePrefix, projectID, mergeRequestIID)
}

func (qm *QueueManager) getLockKey(projectID, mergeRequestIID string) string {
	return fmt.Sprintf("%s:%s:%s", qm.lockPrefix, projectID, mergeRequestIID)
}

func (qm *QueueManager) getProcessingKey(jobID string) string {
	return fmt.Sprintf("%s:%s", qm.processingPrefix, jobID)
}

func (qm *QueueManager) acquireLock(c context.Context, lockKey string) (bool, error) {
	result, err := qm.redis.SetNX(c, lockKey, time.Now().Unix(), qm.defaultLockTTL).Result()
	if err != nil {
		return false, err
	}
	return result, nil
}

func (qm *QueueManager) releaseLock(c context.Context, lockKey string) error {
	return qm.redis.Del(c, lockKey).Err()
}

func (qm *QueueManager) dequeueJob(c context.Context, queueKey string) (*WebhookJob, error) {
	jobData, err := qm.redis.RPop(c, queueKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No jobs in queue
		}
		return nil, err
	}

	var job WebhookJob
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	return &job, nil
}

func (qm *QueueManager) markJobAsProcessing(c context.Context, job *WebhookJob) error {
	processingKey := qm.getProcessingKey(job.ID)
	jobData, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return qm.redis.Set(c, processingKey, jobData, qm.defaultLockTTL).Err()
}

func (qm *QueueManager) removeJobFromProcessing(c context.Context, job *WebhookJob) error {
	processingKey := qm.getProcessingKey(job.ID)
	return qm.redis.Del(c, processingKey).Err()
}

func (qm *QueueManager) handleJobFailure(c context.Context, job *WebhookJob, queueKey string, jobErr error) error {
	job.Attempts++

	if job.Attempts < job.MaxAttempts {
		// Requeue the job for retry
		//log.Printf("Retrying job %s (attempt %d/%d)", job.ID, job.Attempts, job.MaxAttempts)
		qm.log.Info("Retrying job", "jobId", job.ID, "attempt", job.Attempts, "maxAttempts", job.MaxAttempts)
		jobData, err := json.Marshal(job)
		if err != nil {
			return fmt.Errorf("failed to marshal job for retry: %w", err)
		}
		return qm.redis.LPush(c, queueKey, jobData).Err()
	}

	// Job has exceeded max attempts, log and remove from processing
	//log.Printf("Job %s failed after %d attempts: %v", job.ID, job.MaxAttempts, jobErr)
	qm.log.Info("Job failed after max attempts", "jobId", job.ID, "maxAttempts", job.MaxAttempts, "error", jobErr)
	return qm.removeJobFromProcessing(c, job)
}

func (qm *QueueManager) processAllQueues(c context.Context, processor JobProcessor) error {
	queueKeys, err := qm.redis.Keys(c, qm.queuePrefix+":*").Result()
	if err != nil {
		return fmt.Errorf("failed to get queue keys: %w", err)
	}

	for _, queueKey := range queueKeys {
		parts := strings.Split(queueKey, ":")
		if len(parts) >= 5 {
			projectID := parts[3]
			mergeRequestIID := parts[4]
			// Check if there are jobs in this queue
			queueLength, err := qm.redis.LLen(c, queueKey).Result()
			if err != nil {
				//log.Printf("Warning: failed to get queue length for %s: %v", queueKey, err)
				qm.log.Warn("Failed to get queue length", "key", queueKey, "error", err)
				continue
			}

			if queueLength > 0 {
				if err := qm.ProcessMRQueue(c, projectID, mergeRequestIID, processor); err != nil {
					//log.Printf("Error processing MR queue %s:%s: %v", projectID, mergeRequestIID, err)
					qm.log.Info("Error processing MR queue", "projectId", projectID, "mrId", mergeRequestIID, "error", err)
				}
			}
		}
	}

	return nil
}
