package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	db "example.com/haohao/backend/internal/db"
	"example.com/haohao/backend/internal/service"
)

type OutboxMetrics interface {
	ObserveOutboxRun(trigger string, duration time.Duration, err error)
	IncOutboxEvent(eventType, status string)
}

type outboxQueue interface {
	Claim(ctx context.Context, workerID string, batchSize int) ([]db.OutboxEvent, error)
	MarkSent(ctx context.Context, event db.OutboxEvent) error
	MarkFailed(ctx context.Context, event db.OutboxEvent, cause error) error
}

type OutboxWorkerConfig struct {
	Enabled   bool
	Interval  time.Duration
	Timeout   time.Duration
	BatchSize int
	WorkerID  string
}

const outboxMarkTimeout = 5 * time.Second

type OutboxWorker struct {
	outbox  outboxQueue
	handler service.OutboxHandler
	config  OutboxWorkerConfig
	logger  *slog.Logger
	metrics OutboxMetrics
	running atomic.Bool
}

type outboxRunSummary struct {
	Claimed int
	Sent    int
	Failed  int
	Dead    int
}

func (s outboxRunSummary) changed() bool {
	return s.Claimed > 0 || s.Sent > 0 || s.Failed > 0 || s.Dead > 0
}

func (s outboxRunSummary) attrs() []any {
	return []any{
		"claimed", s.Claimed,
		"sent", s.Sent,
		"failed", s.Failed,
		"dead", s.Dead,
	}
}

func NewOutboxWorker(outbox *service.OutboxService, handler service.OutboxHandler, config OutboxWorkerConfig, logger *slog.Logger, metrics OutboxMetrics) *OutboxWorker {
	if logger == nil {
		logger = slog.Default()
	}
	var queue outboxQueue
	if outbox != nil {
		queue = outbox
	}
	if config.WorkerID == "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "local"
		}
		config.WorkerID = fmt.Sprintf("%s:%d", host, os.Getpid())
	}
	return &OutboxWorker{
		outbox:  queue,
		handler: handler,
		config:  config,
		logger:  logger,
		metrics: metrics,
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	if w == nil || w.outbox == nil || w.handler == nil || !w.config.Enabled {
		return
	}
	if w.config.Interval <= 0 || w.config.Timeout <= 0 {
		w.logger.ErrorContext(ctx, "outbox worker disabled because interval or timeout is not positive")
		return
	}

	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()

	w.runOnce(ctx, "startup")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx, "interval")
		}
	}
}

func (w *OutboxWorker) runOnce(parent context.Context, trigger string) {
	if !w.running.CompareAndSwap(false, true) {
		w.logger.DebugContext(parent, "outbox worker skipped because previous run is still active", "trigger", trigger)
		return
	}
	defer w.running.Store(false)

	ctx, cancel := context.WithTimeout(parent, w.config.Timeout)
	defer cancel()

	startedAt := time.Now()
	summary, err := w.runBatch(ctx)
	duration := time.Since(startedAt)
	if w.metrics != nil {
		w.metrics.ObserveOutboxRun(trigger, duration, err)
	}
	if err != nil {
		w.logger.ErrorContext(ctx, "outbox worker failed", "trigger", trigger, "duration_ms", float64(duration.Microseconds())/1000, "error", err.Error())
		return
	}
	durationMS := float64(duration.Microseconds()) / 1000
	if trigger == "startup" || summary.changed() {
		attrs := append([]any{
			"trigger", trigger,
			"duration_ms", durationMS,
		}, summary.attrs()...)
		w.logger.InfoContext(ctx, "outbox worker completed", attrs...)
		return
	}
	w.logger.DebugContext(ctx, "outbox worker completed", "trigger", trigger, "duration_ms", durationMS)
}

func (w *OutboxWorker) runBatch(ctx context.Context) (outboxRunSummary, error) {
	var summary outboxRunSummary
	events, err := w.outbox.Claim(ctx, w.config.WorkerID, w.config.BatchSize)
	if err != nil {
		return summary, fmt.Errorf("claim outbox events: %w", err)
	}
	summary.Claimed = len(events)
	for _, event := range events {
		handleErr := w.handler.HandleOutboxEvent(ctx, event)
		if handleErr == nil {
			markCtx, cancel := outboxMarkContext(ctx)
			err := w.outbox.MarkSent(markCtx, event)
			cancel()
			if err != nil {
				return summary, fmt.Errorf("mark outbox event sent: %w", err)
			}
			summary.Sent++
			if w.metrics != nil {
				w.metrics.IncOutboxEvent(event.EventType, "sent")
			}
			continue
		}
		if errors.Is(handleErr, service.ErrUnknownOutboxEvent) {
			markCtx, cancel := outboxMarkContext(ctx)
			err := w.outbox.MarkFailed(markCtx, eventWithMaxAttempts(event), handleErr)
			cancel()
			if err != nil {
				return summary, fmt.Errorf("mark unknown outbox event dead: %w", err)
			}
			summary.Dead++
			if w.metrics != nil {
				w.metrics.IncOutboxEvent(event.EventType, "dead")
			}
			continue
		}
		markCtx, cancel := outboxMarkContext(ctx)
		err := w.outbox.MarkFailed(markCtx, event, handleErr)
		cancel()
		if err != nil {
			return summary, fmt.Errorf("mark outbox event failed: %w", err)
		}
		summary.Failed++
		if w.metrics != nil {
			w.metrics.IncOutboxEvent(event.EventType, "failed")
		}
	}
	return summary, nil
}

func outboxMarkContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), outboxMarkTimeout)
}

func eventWithMaxAttempts(event db.OutboxEvent) db.OutboxEvent {
	event.Attempts = event.MaxAttempts
	return event
}
