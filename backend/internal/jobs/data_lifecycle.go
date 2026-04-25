package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

type DataLifecycleMetrics interface {
	IncDataLifecycleRun(trigger string, err error)
	IncDataLifecycleItems(kind string, count int64)
}

type DataLifecycleConfig struct {
	Enabled               bool
	Interval              time.Duration
	Timeout               time.Duration
	RunOnStartup          bool
	OutboxRetention       time.Duration
	NotificationRetention time.Duration
	FileDeletedRetention  time.Duration
}

type DataLifecycleJob struct {
	queries *db.Queries
	config  DataLifecycleConfig
	logger  *slog.Logger
	metrics DataLifecycleMetrics
	running atomic.Bool
}

func NewDataLifecycleJob(queries *db.Queries, config DataLifecycleConfig, logger *slog.Logger, metrics DataLifecycleMetrics) *DataLifecycleJob {
	if logger == nil {
		logger = slog.Default()
	}
	return &DataLifecycleJob{
		queries: queries,
		config:  config,
		logger:  logger,
		metrics: metrics,
	}
}

func (j *DataLifecycleJob) Start(ctx context.Context) {
	if j == nil || j.queries == nil || !j.config.Enabled {
		return
	}
	if j.config.Interval <= 0 || j.config.Timeout <= 0 {
		j.logger.ErrorContext(ctx, "data lifecycle job disabled because interval or timeout is not positive")
		return
	}
	if j.config.RunOnStartup {
		go j.runOnce(ctx, "startup")
	}

	ticker := time.NewTicker(j.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go j.runOnce(ctx, "interval")
		}
	}
}

func (j *DataLifecycleJob) runOnce(parent context.Context, trigger string) {
	if !j.running.CompareAndSwap(false, true) {
		j.logger.WarnContext(parent, "data lifecycle job skipped because previous run is still active", "trigger", trigger)
		return
	}
	defer j.running.Store(false)

	ctx, cancel := context.WithTimeout(parent, j.config.Timeout)
	defer cancel()

	err := j.RunOnce(ctx)
	if j.metrics != nil {
		j.metrics.IncDataLifecycleRun(trigger, err)
	}
	if err != nil {
		j.logger.ErrorContext(ctx, "data lifecycle job failed", "trigger", trigger, "error", err.Error())
		return
	}
	j.logger.InfoContext(ctx, "data lifecycle job completed", "trigger", trigger)
}

func (j *DataLifecycleJob) RunOnce(ctx context.Context) error {
	if j == nil || j.queries == nil {
		return nil
	}
	expiredIDKeys, err := j.queries.DeleteExpiredIdempotencyKeys(ctx)
	if err != nil {
		return fmt.Errorf("delete expired idempotency keys: %w", err)
	}
	j.addItems("idempotency_keys", expiredIDKeys)

	expiredInvitations, err := j.queries.ExpireTenantInvitations(ctx)
	if err != nil {
		return fmt.Errorf("expire tenant invitations: %w", err)
	}
	j.addItems("tenant_invitations", expiredInvitations)

	outboxBefore := time.Now().Add(-j.config.OutboxRetention)
	outboxDeleted, err := j.queries.DeleteProcessedOutboxEventsBefore(ctx, pgtype.Timestamptz{Time: outboxBefore, Valid: true})
	if err != nil {
		return fmt.Errorf("delete processed outbox events: %w", err)
	}
	j.addItems("outbox_events", outboxDeleted)

	notificationBefore := time.Now().Add(-j.config.NotificationRetention)
	notificationDeleted, err := j.queries.DeleteReadNotificationsBefore(ctx, pgtype.Timestamptz{Time: notificationBefore, Valid: true})
	if err != nil {
		return fmt.Errorf("delete read notifications: %w", err)
	}
	j.addItems("notifications", notificationDeleted)

	fileBefore := time.Now().Add(-j.config.FileDeletedRetention)
	filesTouched, err := j.queries.SoftDeleteDeletedFileObjectsBefore(ctx, pgtype.Timestamptz{Time: fileBefore, Valid: true})
	if err != nil {
		return fmt.Errorf("cleanup deleted file objects: %w", err)
	}
	j.addItems("file_objects", filesTouched)

	expiredExports, err := j.queries.SoftDeleteExpiredTenantDataExports(ctx)
	if err != nil {
		return fmt.Errorf("soft delete expired tenant data exports: %w", err)
	}
	j.addItems("tenant_data_exports", expiredExports)

	return nil
}

func (j *DataLifecycleJob) addItems(kind string, count int64) {
	if j.metrics != nil {
		j.metrics.IncDataLifecycleItems(kind, count)
	}
}
