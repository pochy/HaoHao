package jobs

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/jackc/pgx/v5/pgtype"
)

type fakeLifecycleQueries struct {
	err                      error
	expiredIDKeys            int64
	expiredTenantInvitations int64
	deletedProcessedOutbox   int64
	deletedReadNotifications int64
	expiredTenantDataExports int64
}

func (q fakeLifecycleQueries) DeleteExpiredIdempotencyKeys(context.Context) (int64, error) {
	return q.expiredIDKeys, q.err
}

func (q fakeLifecycleQueries) ExpireTenantInvitations(context.Context) (int64, error) {
	return q.expiredTenantInvitations, nil
}

func (q fakeLifecycleQueries) DeleteProcessedOutboxEventsBefore(context.Context, pgtype.Timestamptz) (int64, error) {
	return q.deletedProcessedOutbox, nil
}

func (q fakeLifecycleQueries) DeleteReadNotificationsBefore(context.Context, pgtype.Timestamptz) (int64, error) {
	return q.deletedReadNotifications, nil
}

func (q fakeLifecycleQueries) SoftDeleteExpiredTenantDataExports(context.Context) (int64, error) {
	return q.expiredTenantDataExports, nil
}

type fakeDeletedFilePurger struct {
	input  service.FilePurgeInput
	calls  int
	result service.FilePurgeResult
	err    error
}

func (p *fakeDeletedFilePurger) PurgeDeletedBodies(_ context.Context, input service.FilePurgeInput) (service.FilePurgeResult, error) {
	p.calls++
	p.input = input
	return p.result, p.err
}

type fakeDataLifecycleMetrics struct {
	items map[string]int64
}

func (m *fakeDataLifecycleMetrics) IncDataLifecycleRun(string, error) {}

func (m *fakeDataLifecycleMetrics) IncDataLifecycleItems(kind string, count int64) {
	if m.items == nil {
		m.items = map[string]int64{}
	}
	m.items[kind] += count
}

func TestDataLifecycleRunOnceNoopIntervalDoesNotInfoLog(t *testing.T) {
	var logs bytes.Buffer
	job := &DataLifecycleJob{
		queries: fakeLifecycleQueries{},
		config:  normalizeDataLifecycleConfig(DataLifecycleConfig{Timeout: time.Second}),
		logger:  newInfoBufferLogger(&logs),
	}

	job.runOnce(context.Background(), "interval")

	if strings.Contains(logs.String(), "data lifecycle job completed") {
		t.Fatalf("expected no info completion log for noop interval, got %s", logs.String())
	}
}

func TestDataLifecycleRunOnceStartupInfoLogs(t *testing.T) {
	var logs bytes.Buffer
	job := &DataLifecycleJob{
		queries: fakeLifecycleQueries{},
		config:  normalizeDataLifecycleConfig(DataLifecycleConfig{Timeout: time.Second}),
		logger:  newInfoBufferLogger(&logs),
	}

	job.runOnce(context.Background(), "startup")

	got := logs.String()
	if !strings.Contains(got, "data lifecycle job completed") {
		t.Fatalf("expected startup completion log, got %s", got)
	}
	if !strings.Contains(got, `"expired_idempotency_keys_deleted":0`) {
		t.Fatalf("expected startup completion log with counts, got %s", got)
	}
}

func TestDataLifecycleRunOnceWithWorkInfoLogsCounts(t *testing.T) {
	var logs bytes.Buffer
	job := &DataLifecycleJob{
		queries: fakeLifecycleQueries{expiredIDKeys: 3},
		config:  normalizeDataLifecycleConfig(DataLifecycleConfig{Timeout: time.Second}),
		logger:  newInfoBufferLogger(&logs),
	}

	job.runOnce(context.Background(), "interval")

	got := logs.String()
	if !strings.Contains(got, "data lifecycle job completed") {
		t.Fatalf("expected info completion log, got %s", got)
	}
	if !strings.Contains(got, `"expired_idempotency_keys_deleted":3`) {
		t.Fatalf("expected completion log with counts, got %s", got)
	}
}

func TestDataLifecycleRunOnceFailureErrorLogs(t *testing.T) {
	var logs bytes.Buffer
	job := &DataLifecycleJob{
		queries: fakeLifecycleQueries{err: errors.New("delete failed")},
		config:  normalizeDataLifecycleConfig(DataLifecycleConfig{Timeout: time.Second}),
		logger:  newInfoBufferLogger(&logs),
	}

	job.runOnce(context.Background(), "interval")

	got := logs.String()
	if !strings.Contains(got, `"level":"ERROR"`) || !strings.Contains(got, "data lifecycle job failed") {
		t.Fatalf("expected error log, got %s", got)
	}
}

func TestDataLifecycleRunOncePurgesDeletedFileBodies(t *testing.T) {
	retention := 2 * time.Hour
	purger := &fakeDeletedFilePurger{
		result: service.FilePurgeResult{Claimed: 3, Purged: 2, Failed: 1},
	}
	metrics := &fakeDataLifecycleMetrics{}
	job := &DataLifecycleJob{
		queries:    fakeLifecycleQueries{},
		filePurger: purger,
		config: normalizeDataLifecycleConfig(DataLifecycleConfig{
			FileDeletedRetention: retention,
			FilePurgeBatchSize:   25,
			FilePurgeLockTimeout: time.Minute,
			WorkerID:             "test-worker",
		}),
		logger:  discardLogger(),
		metrics: metrics,
	}

	before := time.Now().Add(-retention)
	if err := job.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	after := time.Now().Add(-retention)

	if purger.calls != 1 {
		t.Fatalf("purger calls = %d, want 1", purger.calls)
	}
	if purger.input.BatchSize != 25 {
		t.Fatalf("BatchSize = %d, want 25", purger.input.BatchSize)
	}
	if purger.input.WorkerID != "test-worker" {
		t.Fatalf("WorkerID = %q, want test-worker", purger.input.WorkerID)
	}
	if purger.input.LockTimeout != time.Minute {
		t.Fatalf("LockTimeout = %s, want %s", purger.input.LockTimeout, time.Minute)
	}
	if purger.input.Cutoff.Before(before) || purger.input.Cutoff.After(after) {
		t.Fatalf("Cutoff = %s, want between %s and %s", purger.input.Cutoff, before, after)
	}
	if got := metrics.items["file_objects_body_purged"]; got != 2 {
		t.Fatalf("purged metric = %d, want 2", got)
	}
	if got := metrics.items["file_objects_body_purge_failed"]; got != 1 {
		t.Fatalf("failed metric = %d, want 1", got)
	}
}

func TestDataLifecycleRunOnceAllowsNilFilePurger(t *testing.T) {
	job := &DataLifecycleJob{
		queries: fakeLifecycleQueries{},
		config:  normalizeDataLifecycleConfig(DataLifecycleConfig{}),
		logger:  discardLogger(),
	}

	if err := job.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
}

func TestDataLifecycleRunOnceReturnsFilePurgeError(t *testing.T) {
	want := errors.New("purge failed")
	job := &DataLifecycleJob{
		queries:    fakeLifecycleQueries{},
		filePurger: &fakeDeletedFilePurger{err: want},
		config:     normalizeDataLifecycleConfig(DataLifecycleConfig{}),
		logger:     discardLogger(),
	}

	err := job.RunOnce(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("RunOnce() error = %v, want %v", err, want)
	}
}
