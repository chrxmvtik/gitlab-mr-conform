package queue_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	gitlabapi "gitlab.com/gitlab-org/api/client-go"

	"gitlab-mr-conformity-bot/internal/queue"
	"gitlab-mr-conformity-bot/pkg/logger"
)

type mockProcessor struct {
	processedJobs []*queue.WebhookJob
	shouldFail    bool
	mu            sync.Mutex
}

func (m *mockProcessor) ProcessJob(_ context.Context, job *queue.WebhookJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shouldFail {
		return errors.New("mock error")
	}
	m.processedJobs = append(m.processedJobs, job)
	return nil
}

func (m *mockProcessor) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.processedJobs)
}

type blockingProcessor struct {
	started sync.Once
	ready   chan struct{}
	release chan struct{}
	count   int32
}

func (p *blockingProcessor) ProcessJob(_ context.Context, _ *queue.WebhookJob) error {
	atomic.AddInt32(&p.count, 1)
	p.started.Do(func() { close(p.ready) })
	<-p.release
	return nil
}

type concurrentProcessor struct {
	delay     time.Duration
	processed int32
	current   int32
	max       int32
}

func (p *concurrentProcessor) ProcessJob(_ context.Context, _ *queue.WebhookJob) error {
	current := atomic.AddInt32(&p.current, 1)
	for {
		max := atomic.LoadInt32(&p.max)
		if current <= max || atomic.CompareAndSwapInt32(&p.max, max, current) {
			break
		}
	}
	atomic.AddInt32(&p.processed, 1)
	time.Sleep(p.delay)
	atomic.AddInt32(&p.current, -1)
	return nil
}

type retryProcessor struct {
	calls int32
}

func (p *retryProcessor) ProcessJob(_ context.Context, _ *queue.WebhookJob) error {
	if atomic.AddInt32(&p.calls, 1) == 1 {
		return errors.New("retry me")
	}
	return nil
}

func newTestQueueManager(t *testing.T) (*queue.QueueManager, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	cfg := &queue.Config{
		RedisHost:          mr.Addr(),
		QueuePrefix:        "test:mr:queue",
		LockPrefix:         "test:mr:lock",
		ProcessingPrefix:   "test:mr:processing",
		DefaultLockTTL:     5 * time.Second,
		MaxRetries:         2,
		ProcessingInterval: 50 * time.Millisecond,
		WorkerPoolSize:     5,
	}
	log := logger.New()
	qm := queue.NewQueueManager(cfg, log)
	t.Cleanup(func() { _ = qm.Close() })
	return qm, mr
}

func activeQueueKeys(t *testing.T, qm *queue.QueueManager, mr *miniredis.Miniredis) []string {
	t.Helper()
	client := queue.NewRedisClient(mr.Addr(), "", 0)
	defer client.Close()

	keys, err := client.SMembers(context.Background(), qm.ActiveQueuesKey()).Result()
	if err != nil {
		t.Fatalf("SMembers() error = %v", err)
	}
	return keys
}

func TestEnqueueWebhook_AddsToActiveSet(t *testing.T) {
	qm, mr := newTestQueueManager(t)
	ctx := context.Background()

	if _, err := qm.EnqueueWebhook(ctx, "1", "2", string(gitlabapi.EventTypeMergeRequest), nil); err != nil {
		t.Fatalf("EnqueueWebhook() error = %v", err)
	}

	keys := activeQueueKeys(t, qm, mr)
	if len(keys) != 1 {
		t.Fatalf("active queues = %d, want 1", len(keys))
	}
}

func TestEnqueueWebhook_Deduplication(t *testing.T) {
	qm, mr := newTestQueueManager(t)
	ctx := context.Background()
	client := queue.NewRedisClient(mr.Addr(), "", 0)
	defer client.Close()

	for i := 0; i < 3; i++ {
		if _, err := qm.EnqueueWebhook(ctx, "1", "2", string(gitlabapi.EventTypeMergeRequest), nil); err != nil {
			t.Fatalf("EnqueueWebhook() error = %v", err)
		}
	}

	keys := activeQueueKeys(t, qm, mr)
	if len(keys) != 1 {
		t.Fatalf("active queues = %d, want 1", len(keys))
	}

	llen, err := client.LLen(ctx, keys[0]).Result()
	if err != nil {
		t.Fatalf("LLen() error = %v", err)
	}
	if llen != 1 {
		t.Fatalf("queue length = %d, want 1", llen)
	}
}

func TestEnqueueWebhook_DifferentMRs_IndependentQueues(t *testing.T) {
	qm, mr := newTestQueueManager(t)
	ctx := context.Background()
	client := queue.NewRedisClient(mr.Addr(), "", 0)
	defer client.Close()

	if _, err := qm.EnqueueWebhook(ctx, "1", "1", string(gitlabapi.EventTypeMergeRequest), nil); err != nil {
		t.Fatalf("EnqueueWebhook() error = %v", err)
	}
	if _, err := qm.EnqueueWebhook(ctx, "1", "2", string(gitlabapi.EventTypeMergeRequest), nil); err != nil {
		t.Fatalf("EnqueueWebhook() error = %v", err)
	}

	keys := activeQueueKeys(t, qm, mr)
	if len(keys) != 2 {
		t.Fatalf("active queues = %d, want 2", len(keys))
	}
	for _, key := range keys {
		llen, err := client.LLen(ctx, key).Result()
		if err != nil {
			t.Fatalf("LLen() error = %v", err)
		}
		if llen != 1 {
			t.Fatalf("queue %s length = %d, want 1", key, llen)
		}
	}
}

func TestProcessMRQueue_RemovesFromActiveSet_WhenEmpty(t *testing.T) {
	qm, mr := newTestQueueManager(t)
	ctx := context.Background()
	processor := &mockProcessor{}

	if _, err := qm.EnqueueWebhook(ctx, "1", "2", string(gitlabapi.EventTypeMergeRequest), nil); err != nil {
		t.Fatalf("EnqueueWebhook() error = %v", err)
	}
	if err := qm.ProcessMRQueue(ctx, "1", "2", processor); err != nil {
		t.Fatalf("ProcessMRQueue() error = %v", err)
	}
	if processor.count() != 1 {
		t.Fatalf("processed jobs = %d, want 1", processor.count())
	}
	if keys := activeQueueKeys(t, qm, mr); len(keys) != 0 {
		t.Fatalf("active queues = %v, want empty", keys)
	}
}

func TestProcessMRQueue_Lock_PreventsConcurrent(t *testing.T) {
	qm, _ := newTestQueueManager(t)
	ctx := context.Background()

	if _, err := qm.EnqueueWebhook(ctx, "1", "2", string(gitlabapi.EventTypeMergeRequest), nil); err != nil {
		t.Fatalf("EnqueueWebhook() error = %v", err)
	}

	first := &blockingProcessor{ready: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		done <- qm.ProcessMRQueue(ctx, "1", "2", first)
	}()

	select {
	case <-first.ready:
	case <-time.After(2 * time.Second):
		t.Fatal("first processor did not start")
	}

	second := &mockProcessor{}
	if err := qm.ProcessMRQueue(ctx, "1", "2", second); err != nil {
		t.Fatalf("second ProcessMRQueue() error = %v", err)
	}

	close(first.release)
	if err := <-done; err != nil {
		t.Fatalf("first ProcessMRQueue() error = %v", err)
	}

	if atomic.LoadInt32(&first.count) != 1 {
		t.Fatalf("first processed = %d, want 1", atomic.LoadInt32(&first.count))
	}
	if second.count() != 0 {
		t.Fatalf("second processed = %d, want 0", second.count())
	}
}

func TestProcessAllQueues_UsesSSCAN_NotKeys(t *testing.T) {
	qm, mr := newTestQueueManager(t)
	ctx := context.Background()
	client := queue.NewRedisClient(mr.Addr(), "", 0)
	defer client.Close()
	processor := &mockProcessor{}

	if _, err := qm.EnqueueWebhook(ctx, "1", "2", string(gitlabapi.EventTypeMergeRequest), nil); err != nil {
		t.Fatalf("EnqueueWebhook() error = %v", err)
	}

	strayKey := "test:mr:queue:99:99"
	strayJob := `{"ID":"stray","ProjectID":"99","MergeRequestIID":"99","WebhookType":"merge_request","CreatedAt":1,"Attempts":0,"MaxAttempts":2}`
	if err := client.LPush(ctx, strayKey, strayJob).Err(); err != nil {
		t.Fatalf("LPush() error = %v", err)
	}

	if err := qm.ProcessAllQueues(ctx, processor); err != nil {
		t.Fatalf("ProcessAllQueues() error = %v", err)
	}
	if processor.count() != 1 {
		t.Fatalf("processed jobs = %d, want 1", processor.count())
	}
	if len(processor.processedJobs) > 0 && processor.processedJobs[0].ProjectID == "99" {
		t.Fatal("stray queue was processed")
	}
	llen, err := client.LLen(ctx, strayKey).Result()
	if err != nil {
		t.Fatalf("LLen() error = %v", err)
	}
	if llen != 1 {
		t.Fatalf("stray queue length = %d, want 1", llen)
	}
}

func TestWorkerPool_ProcessesConcurrently(t *testing.T) {
	qm, _ := newTestQueueManager(t)
	ctx := context.Background()
	processor := &concurrentProcessor{delay: 100 * time.Millisecond}

	for i := 0; i < 5; i++ {
		mrID := fmt.Sprintf("%d", i+1)
		if _, err := qm.EnqueueWebhook(ctx, "1", mrID, string(gitlabapi.EventTypeMergeRequest), nil); err != nil {
			t.Fatalf("EnqueueWebhook() error = %v", err)
		}
	}

	if err := qm.ProcessAllQueues(ctx, processor); err != nil {
		t.Fatalf("ProcessAllQueues() error = %v", err)
	}
	if atomic.LoadInt32(&processor.processed) != 5 {
		t.Fatalf("processed jobs = %d, want 5", atomic.LoadInt32(&processor.processed))
	}
	if atomic.LoadInt32(&processor.max) <= 1 {
		t.Fatalf("max concurrency = %d, want > 1", atomic.LoadInt32(&processor.max))
	}
}

func TestGetQueueStats_UsesActiveSet(t *testing.T) {
	qm, mr := newTestQueueManager(t)
	ctx := context.Background()
	client := queue.NewRedisClient(mr.Addr(), "", 0)
	defer client.Close()

	if _, err := qm.EnqueueWebhook(ctx, "1", "1", string(gitlabapi.EventTypeMergeRequest), nil); err != nil {
		t.Fatalf("EnqueueWebhook() error = %v", err)
	}
	if _, err := qm.EnqueueWebhook(ctx, "1", "2", string(gitlabapi.EventTypeMergeRequest), nil); err != nil {
		t.Fatalf("EnqueueWebhook() error = %v", err)
	}
	if err := client.LPush(ctx, "test:mr:queue:99:99", `{"ID":"stray"}`).Err(); err != nil {
		t.Fatalf("LPush() error = %v", err)
	}

	stats, err := qm.GetQueueStats(ctx)
	if err != nil {
		t.Fatalf("GetQueueStats() error = %v", err)
	}
	if stats.TotalQueues != 2 || stats.TotalJobs != 2 {
		t.Fatalf("stats = %+v, want 2 queues and 2 jobs", stats)
	}
}

func TestJobRetry_OnFailure(t *testing.T) {
	qm, mr := newTestQueueManager(t)
	ctx := context.Background()
	processor := &retryProcessor{}
	client := queue.NewRedisClient(mr.Addr(), "", 0)
	defer client.Close()

	if _, err := qm.EnqueueWebhook(ctx, "1", "2", string(gitlabapi.EventTypeMergeRequest), nil); err != nil {
		t.Fatalf("EnqueueWebhook() error = %v", err)
	}
	if err := qm.ProcessMRQueue(ctx, "1", "2", processor); err != nil {
		t.Fatalf("ProcessMRQueue() error = %v", err)
	}
	if atomic.LoadInt32(&processor.calls) != 2 {
		t.Fatalf("processor calls = %d, want 2", atomic.LoadInt32(&processor.calls))
	}
	if keys := activeQueueKeys(t, qm, mr); len(keys) != 0 {
		t.Fatalf("active queues = %v, want empty", keys)
	}
	llen, err := client.LLen(ctx, "test:mr:queue:1:2").Result()
	if err != nil {
		t.Fatalf("LLen() error = %v", err)
	}
	if llen != 0 {
		t.Fatalf("queue length = %d, want 0", llen)
	}
}

func TestHealth_RedisConnected(t *testing.T) {
	qm, _ := newTestQueueManager(t)
	if err := qm.Health(context.Background()); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
}
