package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"gitlab-mr-conformity-bot/pkg/logger"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

// WebhookJob represents a webhook job in the queue.
type WebhookJob struct {
	ID              string
	ProjectID       string
	MergeRequestIID string
	WebhookType     string
	Payload         *gitlabapi.MergeEvent
	CreatedAt       int64
	Attempts        int
	MaxAttempts     int
}

// JobProcessor defines the interface for processing webhook jobs.
type JobProcessor interface {
	ProcessJob(c context.Context, job *WebhookJob) error
}

// QueueManager manages Redis queues for GitLab MR webhooks.
type QueueManager struct {
	redis              *redis.Client
	queuePrefix        string
	lockPrefix         string
	processingPrefix   string
	activeQueuesKey    string
	defaultLockTTL     time.Duration
	maxRetries         int
	processingInterval time.Duration
	workerPoolSize     int
	isProcessing       bool
	stopChan           chan struct{}
	log                *logger.Logger
}

// Config holds configuration for the queue manager.
type Config struct {
	RedisHost          string
	RedisPassword      string
	RedisDB            int
	QueuePrefix        string
	LockPrefix         string
	ProcessingPrefix   string
	ActiveQueuesKey    string
	DefaultLockTTL     time.Duration
	MaxRetries         int
	ProcessingInterval time.Duration
	WorkerPoolSize     int
}

// NewRedisClient creates a Redis client configured for queue usage.
func NewRedisClient(host, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       db,
	})
}

// NewQueueManager creates a new queue manager instance.
func NewQueueManager(config *Config, log *logger.Logger) *QueueManager {
	if config == nil {
		config = &Config{}
	}

	if config.QueuePrefix == "" {
		config.QueuePrefix = "gitlab:mr:queue"
	}
	if config.LockPrefix == "" {
		config.LockPrefix = "gitlab:mr:lock"
	}
	if config.ProcessingPrefix == "" {
		config.ProcessingPrefix = "gitlab:mr:processing"
	}
	if config.ActiveQueuesKey == "" {
		config.ActiveQueuesKey = "gitlab:mr:active-queues"
	}
	if config.DefaultLockTTL == 0 {
		config.DefaultLockTTL = 5 * time.Minute
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.ProcessingInterval == 0 {
		config.ProcessingInterval = time.Second
	}
	if config.WorkerPoolSize == 0 {
		config.WorkerPoolSize = 10
	}

	return &QueueManager{
		redis:              NewRedisClient(config.RedisHost, config.RedisPassword, config.RedisDB),
		queuePrefix:        config.QueuePrefix,
		lockPrefix:         config.LockPrefix,
		processingPrefix:   config.ProcessingPrefix,
		activeQueuesKey:    config.ActiveQueuesKey,
		defaultLockTTL:     config.DefaultLockTTL,
		maxRetries:         config.MaxRetries,
		processingInterval: config.ProcessingInterval,
		workerPoolSize:     config.WorkerPoolSize,
		stopChan:           make(chan struct{}),
		log:                log,
	}
}

func (qm *QueueManager) ActiveQueuesKey() string {
	return qm.activeQueuesKey
}

// EnqueueWebhook adds a webhook job to the queue for a specific MR.
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
	pipe := qm.redis.TxPipeline()
	pipe.Del(c, queueKey)
	pipe.LPush(c, queueKey, jobData)
	pipe.SAdd(c, qm.activeQueuesKey, queueKey)
	pipe.Expire(c, queueKey, 24*time.Hour)
	if _, err := pipe.Exec(c); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}

	qm.log.Info("Enqueued webhook job", "jobId", jobID, "projectId", projectID, "mrId", mergeRequestIID)
	return jobID, nil
}

// ProcessMRQueue processes all queued jobs for a specific MR.
func (qm *QueueManager) ProcessMRQueue(c context.Context, projectID, mergeRequestIID string, processor JobProcessor) error {
	queueKey := qm.getQueueKey(projectID, mergeRequestIID)
	lockKey := qm.getLockKey(projectID, mergeRequestIID)

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
		}
	}()

	for {
		job, err := qm.dequeueJob(c, queueKey)
		if err != nil {
			return fmt.Errorf("failed to dequeue job: %w", err)
		}
		if job == nil {
			break
		}

		qm.log.Info("Processing job", "jobId", job.ID, "projectId", projectID, "mrId", mergeRequestIID)

		if err := qm.markJobAsProcessing(c, job); err != nil {
			qm.log.Warn("Failed to mark job as processing", "jobId", job.ID, "projectId", projectID, "mrId", mergeRequestIID, "error", err)
		}

		if err := processor.ProcessJob(c, job); err != nil {
			qm.log.Error("Error processing job", "jobId", job.ID, "projectId", projectID, "mrId", mergeRequestIID, "error", err)
			if err := qm.handleJobFailure(c, job, queueKey, err); err != nil {
				qm.log.Error("Error handling job", "jobId", job.ID, "projectId", projectID, "mrId", mergeRequestIID, "error", err)
			}
		} else {
			qm.log.Info("Successfully processed job", "jobId", job.ID, "projectId", projectID, "mrId", mergeRequestIID)
			if err := qm.removeJobFromProcessing(c, job); err != nil {
				qm.log.Warn("Failed to remove job from processing", "jobId", job.ID, "projectId", projectID, "mrId", mergeRequestIID, "error", err)
			}
		}
	}

	if err := qm.cleanupActiveQueue(c, queueKey); err != nil {
		qm.log.Warn("Failed to cleanup active queue", "key", queueKey, "error", err)
	}

	return nil
}

// StartProcessor starts the queue processor that continuously processes jobs.
func (qm *QueueManager) StartProcessor(c context.Context, processor JobProcessor) {
	if qm.isProcessing {
		qm.log.Info("Queue processor is already running")
		return
	}

	qm.isProcessing = true
	qm.log.Info("Starting GitLab MR queue processor")

	go func() {
		defer func() {
			qm.isProcessing = false
			qm.log.Info("Queue processor stopped")
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
					qm.log.Error("Error processing queues", "error", err)
				}
			}
		}
	}()
}

// StopProcessor stops the queue processor.
func (qm *QueueManager) StopProcessor() {
	if !qm.isProcessing {
		return
	}

	qm.log.Info("Stopping GitLab MR queue processor")
	close(qm.stopChan)
}

// ProcessAllQueues processes every active MR queue.
func (qm *QueueManager) ProcessAllQueues(c context.Context, processor JobProcessor) error {
	return qm.processAllQueues(c, processor)
}

// GetQueueStats returns statistics about the queues.
func (qm *QueueManager) GetQueueStats(c context.Context) (*QueueStats, error) {
	queueKeys, err := qm.scanActiveQueues(c)
	if err != nil {
		return nil, fmt.Errorf("failed to get active queue keys: %w", err)
	}

	processingKeys, err := qm.scanKeys(c, qm.processingPrefix+":*")
	if err != nil {
		return nil, fmt.Errorf("failed to get processing keys: %w", err)
	}

	var totalJobs int64
	queueDetails := make([]QueueDetail, 0, len(queueKeys))
	totalQueues := 0

	for _, queueKey := range queueKeys {
		projectID, mergeRequestIID, ok := qm.parseQueueKey(queueKey)
		if !ok {
			continue
		}

		jobCount, err := qm.redis.LLen(c, queueKey).Result()
		if err != nil {
			qm.log.Warn("Failed to get queue length", "key", queueKey, "error", err)
			continue
		}
		if jobCount == 0 {
			_ = qm.redis.SRem(c, qm.activeQueuesKey, queueKey).Err()
			continue
		}

		totalQueues++
		totalJobs += jobCount
		queueDetails = append(queueDetails, QueueDetail{
			ProjectID:       projectID,
			MergeRequestIID: mergeRequestIID,
			JobCount:        int(jobCount),
		})
	}

	return &QueueStats{
		TotalQueues:    totalQueues,
		TotalJobs:      int(totalJobs),
		ProcessingJobs: len(processingKeys),
		QueueDetails:   queueDetails,
	}, nil
}

// QueueStats represents queue statistics.
type QueueStats struct {
	TotalQueues    int           `json:"total_queues"`
	TotalJobs      int           `json:"total_jobs"`
	ProcessingJobs int           `json:"processing_jobs"`
	QueueDetails   []QueueDetail `json:"queue_details"`
}

// QueueDetail represents details about a specific queue.
type QueueDetail struct {
	ProjectID       string `json:"project_id"`
	MergeRequestIID string `json:"merge_request_iid"`
	JobCount        int    `json:"job_count"`
}

// ClearAllQueues clears all queues (useful for testing/debugging).
func (qm *QueueManager) ClearAllQueues(c context.Context) error {
	patterns := []string{
		qm.queuePrefix + ":*",
		qm.lockPrefix + ":*",
		qm.processingPrefix + ":*",
	}

	for _, pattern := range patterns {
		keys, err := qm.scanKeys(c, pattern)
		if err != nil {
			return fmt.Errorf("failed to get keys for pattern %s: %w", pattern, err)
		}
		if len(keys) == 0 {
			continue
		}
		if err := qm.redis.Del(c, keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete keys: %w", err)
		}
	}

	if err := qm.redis.Del(c, qm.activeQueuesKey).Err(); err != nil {
		return fmt.Errorf("failed to delete active queue index: %w", err)
	}

	return nil
}

// Close gracefully shuts down the queue manager.
func (qm *QueueManager) Close() error {
	qm.StopProcessor()
	return qm.redis.Close()
}

// Health checks if the queue manager is healthy.
func (qm *QueueManager) Health(c context.Context) error {
	return qm.redis.Ping(c).Err()
}

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
			return nil, nil
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
		qm.log.Info("Retrying job", "jobId", job.ID, "projectId", job.ProjectID, "mrId", job.MergeRequestIID, "attempt", job.Attempts, "maxAttempts", job.MaxAttempts)
		jobData, err := json.Marshal(job)
		if err != nil {
			return fmt.Errorf("failed to marshal job for retry: %w", err)
		}

		pipe := qm.redis.TxPipeline()
		pipe.LPush(c, queueKey, jobData)
		pipe.SAdd(c, qm.activeQueuesKey, queueKey)
		pipe.Expire(c, queueKey, 24*time.Hour)
		if _, err := pipe.Exec(c); err != nil {
			return err
		}
		return nil
	}

	qm.log.Info("Job failed after max attempts", "jobId", job.ID, "projectId", job.ProjectID, "mrId", job.MergeRequestIID, "maxAttempts", job.MaxAttempts, "error", jobErr)
	return qm.removeJobFromProcessing(c, job)
}

func (qm *QueueManager) processAllQueues(c context.Context, processor JobProcessor) error {
	queueKeys, err := qm.scanActiveQueues(c)
	if err != nil {
		return fmt.Errorf("failed to get active queue keys: %w", err)
	}

	sem := make(chan struct{}, qm.workerPoolSize)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for _, queueKey := range queueKeys {
		projectID, mergeRequestIID, ok := qm.parseQueueKey(queueKey)
		if !ok {
			continue
		}

		queueLength, err := qm.redis.LLen(c, queueKey).Result()
		if err != nil {
			qm.log.Warn("Failed to get queue length", "key", queueKey, "error", err)
			continue
		}
		if queueLength == 0 {
			_ = qm.redis.SRem(c, qm.activeQueuesKey, queueKey).Err()
			continue
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(projectID, mergeRequestIID string) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := qm.ProcessMRQueue(c, projectID, mergeRequestIID, processor); err != nil {
				qm.log.Info("Error processing MR queue", "projectId", projectID, "mrId", mergeRequestIID, "error", err)
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(projectID, mergeRequestIID)
	}

	wg.Wait()
	return firstErr
}

func (qm *QueueManager) cleanupActiveQueue(c context.Context, queueKey string) error {
	queueLength, err := qm.redis.LLen(c, queueKey).Result()
	if err != nil {
		return err
	}
	if queueLength == 0 {
		return qm.redis.SRem(c, qm.activeQueuesKey, queueKey).Err()
	}
	return nil
}

func (qm *QueueManager) scanActiveQueues(c context.Context) ([]string, error) {
	var (
		cursor uint64
		keys   []string
	)

	for {
		batch, next, err := qm.redis.SScan(c, qm.activeQueuesKey, cursor, "", 100).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

func (qm *QueueManager) scanKeys(c context.Context, pattern string) ([]string, error) {
	var (
		cursor uint64
		keys   []string
	)

	for {
		batch, next, err := qm.redis.Scan(c, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

func (qm *QueueManager) parseQueueKey(queueKey string) (string, string, bool) {
	prefix := qm.queuePrefix + ":"
	if !strings.HasPrefix(queueKey, prefix) {
		return "", "", false
	}

	remainder := strings.TrimPrefix(queueKey, prefix)
	parts := strings.SplitN(remainder, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}
