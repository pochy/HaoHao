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

type OutboxWorkerConfig struct {
	Enabled   bool
	Interval  time.Duration
	Timeout   time.Duration
	BatchSize int
	WorkerID  string
}

type OutboxWorker struct {
	outbox  *service.OutboxService
	handler service.OutboxHandler
	config  OutboxWorkerConfig
	logger  *slog.Logger
	metrics OutboxMetrics
	running atomic.Bool
}

func NewOutboxWorker(outbox *service.OutboxService, handler service.OutboxHandler, config OutboxWorkerConfig, logger *slog.Logger, metrics OutboxMetrics) *OutboxWorker {
	if logger == nil {
		logger = slog.Default()
	}
	if config.WorkerID == "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "local"
		}
		config.WorkerID = fmt.Sprintf("%s:%d", host, os.Getpid())
	}
	return &OutboxWorker{
		outbox:  outbox,
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
			go w.runOnce(ctx, "interval")
		}
	}
}

func (w *OutboxWorker) runOnce(parent context.Context, trigger string) {
	if !w.running.CompareAndSwap(false, true) {
		w.logger.WarnContext(parent, "outbox worker skipped because previous run is still active", "trigger", trigger)
		return
	}
	defer w.running.Store(false)

	ctx, cancel := context.WithTimeout(parent, w.config.Timeout)
	defer cancel()

	startedAt := time.Now()
	err := w.runBatch(ctx)
	duration := time.Since(startedAt)
	if w.metrics != nil {
		w.metrics.ObserveOutboxRun(trigger, duration, err)
	}
	if err != nil {
		w.logger.ErrorContext(ctx, "outbox worker failed", "trigger", trigger, "duration_ms", float64(duration.Microseconds())/1000, "error", err.Error())
		return
	}
	w.logger.InfoContext(ctx, "outbox worker completed", "trigger", trigger, "duration_ms", float64(duration.Microseconds())/1000)
}

func (w *OutboxWorker) runBatch(ctx context.Context) error {
	events, err := w.outbox.Claim(ctx, w.config.WorkerID, w.config.BatchSize)
	if err != nil {
		return fmt.Errorf("claim outbox events: %w", err)
	}
	for _, event := range events {
		handleErr := w.handler.HandleOutboxEvent(ctx, event)
		if handleErr == nil {
			if err := w.outbox.MarkSent(ctx, event); err != nil {
				return fmt.Errorf("mark outbox event sent: %w", err)
			}
			if w.metrics != nil {
				w.metrics.IncOutboxEvent(event.EventType, "sent")
			}
			continue
		}
		if errors.Is(handleErr, service.ErrUnknownOutboxEvent) {
			if err := w.outbox.MarkFailed(ctx, eventWithMaxAttempts(event), handleErr); err != nil {
				return fmt.Errorf("mark unknown outbox event dead: %w", err)
			}
			if w.metrics != nil {
				w.metrics.IncOutboxEvent(event.EventType, "dead")
			}
			continue
		}
		if err := w.outbox.MarkFailed(ctx, event, handleErr); err != nil {
			return fmt.Errorf("mark outbox event failed: %w", err)
		}
		if w.metrics != nil {
			w.metrics.IncOutboxEvent(event.EventType, "failed")
		}
	}
	return nil
}

func eventWithMaxAttempts(event db.OutboxEvent) db.OutboxEvent {
	event.Attempts = event.MaxAttempts
	return event
}
