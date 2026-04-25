package jobs

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

type ReconcileRunner interface {
	RunOnce(context.Context) error
}

type ReconcileSchedulerConfig struct {
	Enabled      bool
	Interval     time.Duration
	Timeout      time.Duration
	RunOnStartup bool
}

type ReconcileScheduler struct {
	job     ReconcileRunner
	config  ReconcileSchedulerConfig
	logger  *slog.Logger
	running atomic.Bool
}

func NewReconcileScheduler(job ReconcileRunner, config ReconcileSchedulerConfig, logger *slog.Logger) *ReconcileScheduler {
	if logger == nil {
		logger = slog.Default()
	}

	return &ReconcileScheduler{
		job:    job,
		config: config,
		logger: logger,
	}
}

func (s *ReconcileScheduler) Start(ctx context.Context) {
	if s == nil || s.job == nil || !s.config.Enabled {
		return
	}
	if s.config.Interval <= 0 {
		s.logger.ErrorContext(ctx, "provisioning reconcile scheduler disabled because interval is not positive", "interval", s.config.Interval.String())
		return
	}
	if s.config.Timeout <= 0 {
		s.logger.ErrorContext(ctx, "provisioning reconcile scheduler disabled because timeout is not positive", "timeout", s.config.Timeout.String())
		return
	}

	if s.config.RunOnStartup {
		go s.runOnce(ctx, "startup")
	}

	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go s.runOnce(ctx, "interval")
		}
	}
}

func (s *ReconcileScheduler) runOnce(parent context.Context, trigger string) {
	if !s.running.CompareAndSwap(false, true) {
		s.logger.WarnContext(parent, "provisioning reconcile skipped because previous run is still active", "trigger", trigger)
		return
	}
	defer s.running.Store(false)

	ctx, cancel := context.WithTimeout(parent, s.config.Timeout)
	defer cancel()

	startedAt := time.Now()
	err := s.job.RunOnce(ctx)
	duration := time.Since(startedAt)
	attrs := []any{
		"trigger", trigger,
		"duration_ms", float64(duration.Microseconds()) / 1000,
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		s.logger.ErrorContext(ctx, "provisioning reconcile failed", attrs...)
		return
	}

	s.logger.InfoContext(ctx, "provisioning reconcile completed", attrs...)
}
