package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidOutboxEvent = errors.New("invalid outbox event")

type OutboxEventInput struct {
	TenantID      *int64
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       any
}

type OutboxHandler interface {
	HandleOutboxEvent(context.Context, db.OutboxEvent) error
}

type OutboxService struct {
	pool        *pgxpool.Pool
	queries     *db.Queries
	maxAttempts int
}

func NewOutboxService(pool *pgxpool.Pool, queries *db.Queries, maxAttempts int) *OutboxService {
	if maxAttempts <= 0 {
		maxAttempts = 8
	}
	return &OutboxService{
		pool:        pool,
		queries:     queries,
		maxAttempts: maxAttempts,
	}
}

func (s *OutboxService) Enqueue(ctx context.Context, input OutboxEventInput) (db.OutboxEvent, error) {
	if s == nil || s.queries == nil {
		return db.OutboxEvent{}, fmt.Errorf("outbox service is not configured")
	}
	return s.EnqueueWithQueries(ctx, s.queries, input)
}

func (s *OutboxService) EnqueueWithQueries(ctx context.Context, queries *db.Queries, input OutboxEventInput) (db.OutboxEvent, error) {
	if queries == nil {
		return db.OutboxEvent{}, fmt.Errorf("outbox queries are not configured")
	}

	normalized, err := normalizeOutboxEventInput(input)
	if err != nil {
		return db.OutboxEvent{}, err
	}
	payload, err := json.Marshal(normalized.Payload)
	if err != nil {
		return db.OutboxEvent{}, fmt.Errorf("encode outbox payload: %w", err)
	}

	return queries.CreateOutboxEvent(ctx, db.CreateOutboxEventParams{
		TenantID:      pgInt8(normalized.TenantID),
		AggregateType: normalized.AggregateType,
		AggregateID:   normalized.AggregateID,
		EventType:     normalized.EventType,
		Payload:       payload,
		MaxAttempts:   int32(s.maxAttempts),
	})
}

func (s *OutboxService) Claim(ctx context.Context, workerID string, batchSize int) ([]db.OutboxEvent, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("outbox service is not configured")
	}
	if batchSize <= 0 {
		batchSize = 20
	}
	return s.queries.ClaimOutboxEvents(ctx, db.ClaimOutboxEventsParams{
		WorkerID:  pgtype.Text{String: strings.TrimSpace(workerID), Valid: strings.TrimSpace(workerID) != ""},
		BatchSize: int32(batchSize),
	})
}

func (s *OutboxService) MarkSent(ctx context.Context, event db.OutboxEvent) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("outbox service is not configured")
	}
	_, err := s.queries.MarkOutboxEventSent(ctx, event.ID)
	return err
}

func (s *OutboxService) MarkFailed(ctx context.Context, event db.OutboxEvent, cause error) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("outbox service is not configured")
	}
	message := "handler failed"
	if cause != nil {
		message = cause.Error()
	}
	if event.Attempts >= event.MaxAttempts {
		_, err := s.queries.MarkOutboxEventDead(ctx, db.MarkOutboxEventDeadParams{
			ID:        event.ID,
			LastError: message,
		})
		return err
	}

	backoff := time.Duration(event.Attempts)
	if backoff < 1 {
		backoff = 1
	}
	if backoff > 60 {
		backoff = 60
	}
	_, err := s.queries.MarkOutboxEventRetry(ctx, db.MarkOutboxEventRetryParams{
		ID:        event.ID,
		LastError: message,
		Backoff:   time.Duration(backoff) * time.Second,
	})
	return err
}

func normalizeOutboxEventInput(input OutboxEventInput) (OutboxEventInput, error) {
	input.AggregateType = strings.ToLower(strings.TrimSpace(input.AggregateType))
	input.AggregateID = strings.TrimSpace(input.AggregateID)
	input.EventType = strings.ToLower(strings.TrimSpace(input.EventType))
	if input.AggregateType == "" || input.AggregateID == "" || input.EventType == "" {
		return OutboxEventInput{}, fmt.Errorf("%w: aggregate type, aggregate id, and event type are required", ErrInvalidOutboxEvent)
	}
	if input.Payload == nil {
		input.Payload = map[string]any{}
	}
	return input, nil
}
