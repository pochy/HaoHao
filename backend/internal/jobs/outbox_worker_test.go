package jobs

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	db "example.com/haohao/backend/internal/db"
	"example.com/haohao/backend/internal/service"
)

type fakeOutboxQueue struct {
	events           []db.OutboxEvent
	claimErr         error
	markSentErr      error
	markFailedErr    error
	markSentCtxErr   error
	markFailedCtxErr error
	sent             int
	failed           int
}

func (q *fakeOutboxQueue) Claim(context.Context, string, int) ([]db.OutboxEvent, error) {
	if q.claimErr != nil {
		return nil, q.claimErr
	}
	return q.events, nil
}

func (q *fakeOutboxQueue) MarkSent(ctx context.Context, event db.OutboxEvent) error {
	q.markSentCtxErr = ctx.Err()
	if q.markSentErr != nil {
		return q.markSentErr
	}
	q.sent++
	return nil
}

func (q *fakeOutboxQueue) MarkFailed(ctx context.Context, event db.OutboxEvent, cause error) error {
	q.markFailedCtxErr = ctx.Err()
	if q.markFailedErr != nil {
		return q.markFailedErr
	}
	q.failed++
	return nil
}

type fakeOutboxHandler struct {
	err error
	fn  func(context.Context, db.OutboxEvent) error
}

func (h fakeOutboxHandler) HandleOutboxEvent(ctx context.Context, event db.OutboxEvent) error {
	if h.fn != nil {
		return h.fn(ctx, event)
	}
	return h.err
}

func newOutboxWorkerForLogTest(queue *fakeOutboxQueue, handler service.OutboxHandler, logs *bytes.Buffer) *OutboxWorker {
	return &OutboxWorker{
		outbox:  queue,
		handler: handler,
		config: OutboxWorkerConfig{
			Timeout:   time.Second,
			BatchSize: 20,
			WorkerID:  "test-worker",
		},
		logger: newInfoBufferLogger(logs),
	}
}

func TestOutboxWorkerRunOnceNoopIntervalDoesNotInfoLog(t *testing.T) {
	var logs bytes.Buffer
	worker := newOutboxWorkerForLogTest(&fakeOutboxQueue{}, fakeOutboxHandler{}, &logs)

	worker.runOnce(context.Background(), "interval")

	if strings.Contains(logs.String(), "outbox worker completed") {
		t.Fatalf("expected no info completion log for noop interval, got %s", logs.String())
	}
}

func TestOutboxWorkerRunOnceStartupInfoLogs(t *testing.T) {
	var logs bytes.Buffer
	worker := newOutboxWorkerForLogTest(&fakeOutboxQueue{}, fakeOutboxHandler{}, &logs)

	worker.runOnce(context.Background(), "startup")

	got := logs.String()
	if !strings.Contains(got, "outbox worker completed") {
		t.Fatalf("expected startup completion log, got %s", got)
	}
	if !strings.Contains(got, `"claimed":0`) {
		t.Fatalf("expected startup completion log with counts, got %s", got)
	}
}

func TestOutboxWorkerRunOnceWithSentEventsInfoLogsCounts(t *testing.T) {
	var logs bytes.Buffer
	queue := &fakeOutboxQueue{
		events: []db.OutboxEvent{
			{ID: 1, EventType: "tenant.invited", MaxAttempts: 8},
			{ID: 2, EventType: "notification.created", MaxAttempts: 8},
		},
	}
	worker := newOutboxWorkerForLogTest(queue, fakeOutboxHandler{}, &logs)

	worker.runOnce(context.Background(), "interval")

	got := logs.String()
	if !strings.Contains(got, "outbox worker completed") {
		t.Fatalf("expected info completion log, got %s", got)
	}
	if !strings.Contains(got, `"claimed":2`) || !strings.Contains(got, `"sent":2`) {
		t.Fatalf("expected completion log with sent counts, got %s", got)
	}
	if queue.sent != 2 {
		t.Fatalf("sent count = %d, want 2", queue.sent)
	}
}

func TestOutboxWorkerRunOnceWithFailedEventInfoLogsCounts(t *testing.T) {
	var logs bytes.Buffer
	queue := &fakeOutboxQueue{
		events: []db.OutboxEvent{{ID: 1, EventType: "email.send", MaxAttempts: 8}},
	}
	worker := newOutboxWorkerForLogTest(queue, fakeOutboxHandler{err: errors.New("handler failed")}, &logs)

	worker.runOnce(context.Background(), "interval")

	got := logs.String()
	if !strings.Contains(got, "outbox worker completed") {
		t.Fatalf("expected info completion log, got %s", got)
	}
	if !strings.Contains(got, `"claimed":1`) || !strings.Contains(got, `"failed":1`) {
		t.Fatalf("expected completion log with failed counts, got %s", got)
	}
	if queue.failed != 1 {
		t.Fatalf("failed count = %d, want 1", queue.failed)
	}
}

func TestOutboxWorkerRunBatchMarksSentAfterHandlerCancelsContext(t *testing.T) {
	queue := &fakeOutboxQueue{
		events: []db.OutboxEvent{{ID: 1, EventType: "email.send", MaxAttempts: 8}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	worker := newOutboxWorkerForLogTest(queue, fakeOutboxHandler{
		fn: func(context.Context, db.OutboxEvent) error {
			cancel()
			return nil
		},
	}, &bytes.Buffer{})

	summary, err := worker.runBatch(ctx)

	if err != nil {
		t.Fatalf("runBatch returned error: %v", err)
	}
	if summary.Sent != 1 || queue.sent != 1 {
		t.Fatalf("sent count = summary %d queue %d, want 1", summary.Sent, queue.sent)
	}
	if queue.markSentCtxErr != nil {
		t.Fatalf("MarkSent context error = %v, want nil", queue.markSentCtxErr)
	}
}

func TestOutboxWorkerRunBatchMarksFailedAfterHandlerCancelsContext(t *testing.T) {
	queue := &fakeOutboxQueue{
		events: []db.OutboxEvent{{ID: 1, EventType: "email.send", MaxAttempts: 8}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	worker := newOutboxWorkerForLogTest(queue, fakeOutboxHandler{
		fn: func(context.Context, db.OutboxEvent) error {
			cancel()
			return errors.New("handler failed")
		},
	}, &bytes.Buffer{})

	summary, err := worker.runBatch(ctx)

	if err != nil {
		t.Fatalf("runBatch returned error: %v", err)
	}
	if summary.Failed != 1 || queue.failed != 1 {
		t.Fatalf("failed count = summary %d queue %d, want 1", summary.Failed, queue.failed)
	}
	if queue.markFailedCtxErr != nil {
		t.Fatalf("MarkFailed context error = %v, want nil", queue.markFailedCtxErr)
	}
}

func TestOutboxWorkerRunOnceWithDeadEventInfoLogsCounts(t *testing.T) {
	var logs bytes.Buffer
	queue := &fakeOutboxQueue{
		events: []db.OutboxEvent{{ID: 1, EventType: "unknown.event", MaxAttempts: 8}},
	}
	worker := newOutboxWorkerForLogTest(queue, fakeOutboxHandler{err: service.ErrUnknownOutboxEvent}, &logs)

	worker.runOnce(context.Background(), "interval")

	got := logs.String()
	if !strings.Contains(got, "outbox worker completed") {
		t.Fatalf("expected info completion log, got %s", got)
	}
	if !strings.Contains(got, `"claimed":1`) || !strings.Contains(got, `"dead":1`) {
		t.Fatalf("expected completion log with dead counts, got %s", got)
	}
	if queue.failed != 1 {
		t.Fatalf("failed count = %d, want 1", queue.failed)
	}
}

func TestOutboxWorkerRunOnceClaimFailureErrorLogs(t *testing.T) {
	var logs bytes.Buffer
	worker := newOutboxWorkerForLogTest(&fakeOutboxQueue{claimErr: errors.New("claim failed")}, fakeOutboxHandler{}, &logs)

	worker.runOnce(context.Background(), "interval")

	got := logs.String()
	if !strings.Contains(got, `"level":"ERROR"`) || !strings.Contains(got, "outbox worker failed") {
		t.Fatalf("expected error log, got %s", got)
	}
}
