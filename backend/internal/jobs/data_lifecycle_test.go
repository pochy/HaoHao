package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/jackc/pgx/v5/pgtype"
)

type fakeLifecycleQueries struct {
	err error
}

func (q fakeLifecycleQueries) DeleteExpiredIdempotencyKeys(context.Context) (int64, error) {
	return 1, q.err
}

func (q fakeLifecycleQueries) ExpireTenantInvitations(context.Context) (int64, error) {
	return 2, nil
}

func (q fakeLifecycleQueries) DeleteProcessedOutboxEventsBefore(context.Context, pgtype.Timestamptz) (int64, error) {
	return 3, nil
}

func (q fakeLifecycleQueries) DeleteReadNotificationsBefore(context.Context, pgtype.Timestamptz) (int64, error) {
	return 4, nil
}

func (q fakeLifecycleQueries) SoftDeleteExpiredTenantDataExports(context.Context) (int64, error) {
	return 5, nil
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
