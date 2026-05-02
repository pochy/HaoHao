package jobs

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"example.com/haohao/backend/internal/service"
)

type WorkTableExportScheduleRunner interface {
	RunDueWorkTableExportSchedules(ctx context.Context, now time.Time, batchSize int32) (service.WorkTableExportScheduleRunSummary, error)
}

type WorkTableExportScheduleMetrics interface {
	ObserveWorkTableExportScheduleRun(trigger string, duration time.Duration, err error)
	IncWorkTableExportScheduleItems(kind string, count int64)
}

type WorkTableExportScheduleConfig struct {
	Enabled      bool
	Interval     time.Duration
	Timeout      time.Duration
	BatchSize    int32
	RunOnStartup bool
}

type WorkTableExportScheduleJob struct {
	runner  WorkTableExportScheduleRunner
	config  WorkTableExportScheduleConfig
	logger  *slog.Logger
	metrics WorkTableExportScheduleMetrics
	running atomic.Bool
}

func NewWorkTableExportScheduleJob(runner WorkTableExportScheduleRunner, config WorkTableExportScheduleConfig, logger *slog.Logger, metrics WorkTableExportScheduleMetrics) *WorkTableExportScheduleJob {
	if logger == nil {
		logger = slog.Default()
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 20
	}
	return &WorkTableExportScheduleJob{
		runner:  runner,
		config:  config,
		logger:  logger,
		metrics: metrics,
	}
}

func (j *WorkTableExportScheduleJob) Start(ctx context.Context) {
	if j == nil || j.runner == nil || !j.config.Enabled {
		return
	}
	if j.config.Interval <= 0 || j.config.Timeout <= 0 {
		j.logger.ErrorContext(ctx, "work table export schedule job disabled because interval or timeout is not positive")
		return
	}
	if j.config.RunOnStartup {
		j.runOnce(ctx, "startup")
	}

	ticker := time.NewTicker(j.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.runOnce(ctx, "interval")
		}
	}
}

func (j *WorkTableExportScheduleJob) runOnce(parent context.Context, trigger string) {
	if !j.running.CompareAndSwap(false, true) {
		j.logger.DebugContext(parent, "work table export schedule job skipped because previous run is still active", "trigger", trigger)
		return
	}
	defer j.running.Store(false)

	ctx, cancel := context.WithTimeout(parent, j.config.Timeout)
	defer cancel()

	startedAt := time.Now()
	summary, err := j.runner.RunDueWorkTableExportSchedules(ctx, startedAt, j.config.BatchSize)
	duration := time.Since(startedAt)
	if j.metrics != nil {
		j.metrics.ObserveWorkTableExportScheduleRun(trigger, duration, err)
		if err == nil {
			j.metrics.IncWorkTableExportScheduleItems("claimed", int64(summary.Claimed))
			j.metrics.IncWorkTableExportScheduleItems("created", int64(summary.Created))
			j.metrics.IncWorkTableExportScheduleItems("skipped", int64(summary.Skipped))
			j.metrics.IncWorkTableExportScheduleItems("failed", int64(summary.Failed))
			j.metrics.IncWorkTableExportScheduleItems("disabled", int64(summary.Disabled))
		}
	}
	if err != nil {
		j.logger.ErrorContext(ctx, "work table export schedule job failed", "trigger", trigger, "duration_ms", float64(duration.Microseconds())/1000, "error", err.Error())
		return
	}
	if trigger == "startup" || summary.Claimed > 0 || summary.Created > 0 || summary.Skipped > 0 || summary.Failed > 0 || summary.Disabled > 0 {
		j.logger.InfoContext(ctx, "work table export schedule job completed", append([]any{
			"trigger", trigger,
			"duration_ms", float64(duration.Microseconds()) / 1000,
		}, summaryAttrs(summary)...)...)
		return
	}
	j.logger.DebugContext(ctx, "work table export schedule job completed", "trigger", trigger, "duration_ms", float64(duration.Microseconds())/1000)
}

func summaryAttrs(summary service.WorkTableExportScheduleRunSummary) []any {
	return []any{
		"claimed", summary.Claimed,
		"created", summary.Created,
		"skipped", summary.Skipped,
		"failed", summary.Failed,
		"disabled", summary.Disabled,
	}
}
