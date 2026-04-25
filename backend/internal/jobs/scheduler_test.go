package jobs

import (
	"context"
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
	return nil
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
	}, discardLogger())

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
	runner := &fakeReconcileRunner{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	scheduler := NewReconcileScheduler(runner, ReconcileSchedulerConfig{
		Enabled:  true,
		Interval: time.Hour,
		Timeout:  time.Second,
	}, discardLogger())

	go scheduler.runOnce(context.Background(), "first")
	<-runner.started

	scheduler.runOnce(context.Background(), "second")
	close(runner.release)

	if got := runner.calls.Load(); got != 1 {
		t.Fatalf("calls = %d", got)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
