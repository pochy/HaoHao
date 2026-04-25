package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrIdempotencyConflict   = errors.New("idempotency key was used with a different request")
	ErrIdempotencyInProgress = errors.New("idempotency key is already processing")
	ErrIdempotencyFailed     = errors.New("idempotency key is associated with a failed request")
)

type IdempotencyInput struct {
	Key         string
	Method      string
	Path        string
	Scope       string
	TenantID    *int64
	ActorUserID *int64
	RequestBody any
}

type IdempotencyAttempt struct {
	Key         db.IdempotencyKey
	RequestHash string
	Replay      bool
	StatusCode  int
	Body        []byte
}

type IdempotencyService struct {
	queries *db.Queries
	ttl     time.Duration
}

func NewIdempotencyService(queries *db.Queries, ttl time.Duration) *IdempotencyService {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &IdempotencyService{queries: queries, ttl: ttl}
}

func (s *IdempotencyService) Begin(ctx context.Context, input IdempotencyInput) (IdempotencyAttempt, error) {
	if s == nil || s.queries == nil || strings.TrimSpace(input.Key) == "" {
		return IdempotencyAttempt{}, nil
	}

	normalized, err := normalizeIdempotencyInput(input)
	if err != nil {
		return IdempotencyAttempt{}, err
	}

	requestHash, err := hashJSON(normalized.RequestBody)
	if err != nil {
		return IdempotencyAttempt{}, err
	}
	keyHash := hashString(normalized.Key)
	row, err := s.queries.CreateIdempotencyKey(ctx, db.CreateIdempotencyKeyParams{
		TenantID:           pgInt8(normalized.TenantID),
		ActorUserID:        pgInt8(normalized.ActorUserID),
		Scope:              normalized.Scope,
		IdempotencyKeyHash: keyHash,
		Method:             normalized.Method,
		Path:               normalized.Path,
		RequestHash:        requestHash,
		ExpiresAt:          pgtype.Timestamptz{Time: time.Now().Add(s.ttl), Valid: true},
	})
	if err == nil {
		return IdempotencyAttempt{Key: row, RequestHash: requestHash}, nil
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return IdempotencyAttempt{}, fmt.Errorf("create idempotency key: %w", err)
	}

	row, err = s.queries.GetIdempotencyKeyForScope(ctx, db.GetIdempotencyKeyForScopeParams{
		Scope:              normalized.Scope,
		IdempotencyKeyHash: keyHash,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return IdempotencyAttempt{}, ErrIdempotencyInProgress
	}
	if err != nil {
		return IdempotencyAttempt{}, fmt.Errorf("get idempotency key: %w", err)
	}
	if row.RequestHash != requestHash {
		return IdempotencyAttempt{}, ErrIdempotencyConflict
	}

	switch row.Status {
	case "completed":
		status := http.StatusOK
		if row.ResponseStatus.Valid {
			status = int(row.ResponseStatus.Int32)
		}
		return IdempotencyAttempt{
			Key:         row,
			RequestHash: requestHash,
			Replay:      true,
			StatusCode:  status,
			Body:        row.ResponseSummary,
		}, nil
	case "failed":
		return IdempotencyAttempt{}, ErrIdempotencyFailed
	default:
		return IdempotencyAttempt{}, ErrIdempotencyInProgress
	}
}

func (s *IdempotencyService) Complete(ctx context.Context, attempt IdempotencyAttempt, status int, body any) error {
	if s == nil || s.queries == nil || attempt.Key.ID == 0 || attempt.Replay {
		return nil
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode idempotency response: %w", err)
	}
	_, err = s.queries.CompleteIdempotencyKey(ctx, db.CompleteIdempotencyKeyParams{
		ID:              attempt.Key.ID,
		ResponseStatus:  pgtype.Int4{Int32: int32(status), Valid: true},
		ResponseSummary: payload,
	})
	return err
}

func (s *IdempotencyService) Fail(ctx context.Context, attempt IdempotencyAttempt, status int, message string) {
	if s == nil || s.queries == nil || attempt.Key.ID == 0 || attempt.Replay {
		return
	}
	payload, _ := json.Marshal(map[string]string{"error": message})
	_, _ = s.queries.FailIdempotencyKey(ctx, db.FailIdempotencyKeyParams{
		ID:              attempt.Key.ID,
		ResponseStatus:  pgtype.Int4{Int32: int32(status), Valid: true},
		ResponseSummary: payload,
	})
}

func normalizeIdempotencyInput(input IdempotencyInput) (IdempotencyInput, error) {
	input.Key = strings.TrimSpace(input.Key)
	input.Method = strings.ToUpper(strings.TrimSpace(input.Method))
	input.Path = strings.TrimSpace(input.Path)
	input.Scope = strings.TrimSpace(input.Scope)
	if input.Key == "" {
		return IdempotencyInput{}, nil
	}
	if input.Method == "" || input.Path == "" || input.Scope == "" {
		return IdempotencyInput{}, fmt.Errorf("%w: method, path, and scope are required", ErrIdempotencyConflict)
	}
	return input, nil
}

func hashJSON(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode request hash input: %w", err)
	}
	return hashBytes(payload), nil
}

func hashString(value string) string {
	return hashBytes([]byte(value))
}

func hashBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
