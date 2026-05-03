package jobs

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"example.com/haohao/backend/internal/service"
)

type DataPipelineScheduleRunner interface {
	RunDueSchedules(ctx context.Context, now time.Time, batchSize int32) (service.DataPipelineScheduleRunSummary, error)
}

type DataPipelineScheduleMetrics interface {
	ObserveDataPipelineScheduleRun(trigger string, duration time.Duration, err error)
	IncDataPipelineScheduleItems(kind string, count int64)
}

type DataPipelineScheduleConfig struct {
	Enabled      bool
	Interval     time.Duration
	Timeout      time.Duration
	BatchSize    int32
	RunOnStartup bool
}

type DataPipelineScheduleJob struct {
	runner  DataPipelineScheduleRunner
	config  DataPipelineScheduleConfig
	logger  *slog.Logger
	metrics DataPipelineScheduleMetrics
	running atomic.Bool
}

func NewDataPipelineScheduleJob(runner DataPipelineScheduleRunner, config DataPipelineScheduleConfig, logger *slog.Logger, metrics DataPipelineScheduleMetrics) *DataPipelineScheduleJob {
	if logger == nil {
		logger = slog.Default()
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 20
	}
	return &DataPipelineScheduleJob{runner: runner, config: config, logger: logger, metrics: metrics}
}

func (j *DataPipelineScheduleJob) Start(ctx context.Context) {
	if j == nil || j.runner == nil || !j.config.Enabled {
		return
	}
	if j.config.Interval <= 0 || j.config.Timeout <= 0 {
		j.logger.ErrorContext(ctx, "data pipeline schedule job disabled because interval or timeout is not positive")
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

func (j *DataPipelineScheduleJob) runOnce(parent context.Context, trigger string) {
	if !j.running.CompareAndSwap(false, true) {
		j.logger.DebugContext(parent, "data pipeline schedule job skipped because previous run is still active", "trigger", trigger)
		return
	}
	defer j.running.Store(false)

	ctx, cancel := context.WithTimeout(parent, j.config.Timeout)
	defer cancel()

	startedAt := time.Now()
	summary, err := j.runner.RunDueSchedules(ctx, startedAt, j.config.BatchSize)
	duration := time.Since(startedAt)
	if j.metrics != nil {
		j.metrics.ObserveDataPipelineScheduleRun(trigger, duration, err)
		if err == nil {
			j.metrics.IncDataPipelineScheduleItems("claimed", int64(summary.Claimed))
			j.metrics.IncDataPipelineScheduleItems("created", int64(summary.Created))
			j.metrics.IncDataPipelineScheduleItems("skipped", int64(summary.Skipped))
			j.metrics.IncDataPipelineScheduleItems("failed", int64(summary.Failed))
			j.metrics.IncDataPipelineScheduleItems("disabled", int64(summary.Disabled))
		}
	}
	if err != nil {
		j.logger.ErrorContext(ctx, "data pipeline schedule job failed", "trigger", trigger, "duration_ms", float64(duration.Microseconds())/1000, "error", err.Error())
		return
	}
	if trigger == "startup" || summary.Claimed > 0 || summary.Created > 0 || summary.Skipped > 0 || summary.Failed > 0 || summary.Disabled > 0 {
		j.logger.InfoContext(ctx, "data pipeline schedule job completed", append([]any{"trigger", trigger, "duration_ms", float64(duration.Microseconds()) / 1000}, dataPipelineSummaryAttrs(summary)...)...)
		return
	}
	j.logger.DebugContext(ctx, "data pipeline schedule job completed", "trigger", trigger, "duration_ms", float64(duration.Microseconds())/1000)
}

func dataPipelineSummaryAttrs(summary service.DataPipelineScheduleRunSummary) []any {
	return []any{"claimed", summary.Claimed, "created", summary.Created, "skipped", summary.Skipped, "failed", summary.Failed, "disabled", summary.Disabled}
}
