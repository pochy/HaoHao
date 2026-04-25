package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

type fakeReconcileRunner struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	err     error
}

func (r *fakeReconcileRunner) RunOnce(ctx context.Context) error {
	r.calls.Add(1)
	if r.started != nil {
		select {
		case r.started <- struct{}{}:
		default:
		}
	}
	if r.release != nil {
		select {
		case <-r.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return r.err
}

type fakeReconcileMetrics struct {
	runs    atomic.Int32
	skipped atomic.Int32
	failed  atomic.Bool
}

func (m *fakeReconcileMetrics) ObserveReconcileRun(_ string, _ time.Duration, err error) {
	m.runs.Add(1)
	m.failed.Store(err != nil)
}

func (m *fakeReconcileMetrics) IncReconcileSkipped(_ string) {
	m.skipped.Add(1)
}

func TestReconcileSchedulerRunOnStartup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := &fakeReconcileRunner{started: make(chan struct{}, 1)}
	scheduler := NewReconcileScheduler(runner, ReconcileSchedulerConfig{
		Enabled:      true,
		Interval:     time.Hour,
		Timeout:      time.Second,
		RunOnStartup: true,
	}, discardLogger(), nil)

	go scheduler.Start(ctx)

	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not run on startup")
	}

	if got := runner.calls.Load(); got != 1 {
		t.Fatalf("calls = %d", got)
	}
}

func TestReconcileSchedulerSkipsOverlap(t *testing.T) {
	metrics := &fakeReconcileMetrics{}
	runner := &fakeReconcileRunner{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	scheduler := NewReconcileScheduler(runner, ReconcileSchedulerConfig{
		Enabled:  true,
		Interval: time.Hour,
		Timeout:  time.Second,
	}, discardLogger(), metrics)

	go scheduler.runOnce(context.Background(), "first")
	<-runner.started

	scheduler.runOnce(context.Background(), "second")
	close(runner.release)

	if got := runner.calls.Load(); got != 1 {
		t.Fatalf("calls = %d", got)
	}
	if got := metrics.skipped.Load(); got != 1 {
		t.Fatalf("skipped metrics = %d", got)
	}
}

func TestReconcileSchedulerRecordsRunFailureMetric(t *testing.T) {
	metrics := &fakeReconcileMetrics{}
	runner := &fakeReconcileRunner{err: errors.New("reconcile failed")}
	scheduler := NewReconcileScheduler(runner, ReconcileSchedulerConfig{
		Enabled:  true,
		Interval: time.Hour,
		Timeout:  time.Second,
	}, discardLogger(), metrics)

	scheduler.runOnce(context.Background(), "manual")

	if got := metrics.runs.Load(); got != 1 {
		t.Fatalf("run metrics = %d", got)
	}
	if !metrics.failed.Load() {
		t.Fatal("failed metric flag = false")
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
