package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	AuditActorUser          = "user"
	AuditActorMachineClient = "machine_client"
	AuditActorSystem        = "system"
)

var ErrInvalidAuditEvent = errors.New("invalid audit event")

type AuditRequest struct {
	RequestID string
	ClientIP  string
	UserAgent string
}

type AuditContext struct {
	ActorType            string
	ActorUserID          *int64
	ActorMachineClientID *int64
	TenantID             *int64
	Request              AuditRequest
	SupportAccessID      *int64
	ImpersonatedUserID   *int64
}

type AuditEventInput struct {
	AuditContext
	Action     string
	TargetType string
	TargetID   string
	Metadata   map[string]any
}

type AuditRecorder interface {
	Record(ctx context.Context, event AuditEventInput) error
	RecordWithQueries(ctx context.Context, queries *db.Queries, event AuditEventInput) error
	RecordBestEffort(ctx context.Context, event AuditEventInput)
}

type AuditService struct {
	queries *db.Queries
}

func NewAuditService(queries *db.Queries) *AuditService {
	return &AuditService{queries: queries}
}

func (s *AuditService) Record(ctx context.Context, event AuditEventInput) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("audit service is not configured")
	}
	return s.RecordWithQueries(ctx, s.queries, event)
}

func (s *AuditService) RecordWithQueries(ctx context.Context, queries *db.Queries, event AuditEventInput) error {
	if queries == nil {
		return fmt.Errorf("audit queries are not configured")
	}

	normalized, err := normalizeAuditEvent(event)
	if err != nil {
		return err
	}

	metadataPayload, err := json.Marshal(normalized.Metadata)
	if err != nil {
		return fmt.Errorf("encode audit metadata: %w", err)
	}

	if _, err := queries.CreateAuditEvent(ctx, db.CreateAuditEventParams{
		ActorType:            normalized.ActorType,
		ActorUserID:          auditInt8(normalized.ActorUserID),
		ActorMachineClientID: auditInt8(normalized.ActorMachineClientID),
		TenantID:             auditInt8(normalized.TenantID),
		Action:               normalized.Action,
		TargetType:           normalized.TargetType,
		TargetID:             normalized.TargetID,
		RequestID:            normalized.Request.RequestID,
		ClientIp:             normalized.Request.ClientIP,
		UserAgent:            normalized.Request.UserAgent,
		Metadata:             metadataPayload,
	}); err != nil {
		return fmt.Errorf("create audit event: %w", err)
	}

	return nil
}

func (s *AuditService) RecordBestEffort(ctx context.Context, event AuditEventInput) {
	if err := s.Record(ctx, event); err != nil {
		slog.WarnContext(ctx, "audit event failed", "error", err, "action", event.Action, "target_type", event.TargetType, "target_id", event.TargetID)
	}
}

func normalizeAuditEvent(event AuditEventInput) (AuditEventInput, error) {
	event.ActorType = strings.TrimSpace(event.ActorType)
	if event.ActorType == "" {
		event.ActorType = AuditActorUser
	}
	switch event.ActorType {
	case AuditActorUser:
		if event.ActorUserID == nil || *event.ActorUserID <= 0 {
			return AuditEventInput{}, fmt.Errorf("%w: actor user id is required", ErrInvalidAuditEvent)
		}
	case AuditActorMachineClient:
		if event.ActorMachineClientID == nil || *event.ActorMachineClientID <= 0 {
			return AuditEventInput{}, fmt.Errorf("%w: actor machine client id is required", ErrInvalidAuditEvent)
		}
	case AuditActorSystem:
	default:
		return AuditEventInput{}, fmt.Errorf("%w: unsupported actor type", ErrInvalidAuditEvent)
	}

	event.Action = strings.ToLower(strings.TrimSpace(event.Action))
	event.TargetType = strings.ToLower(strings.TrimSpace(event.TargetType))
	event.TargetID = strings.TrimSpace(event.TargetID)
	if event.Action == "" || event.TargetType == "" || event.TargetID == "" {
		return AuditEventInput{}, fmt.Errorf("%w: action, target type, and target id are required", ErrInvalidAuditEvent)
	}

	event.Request.RequestID = strings.TrimSpace(event.Request.RequestID)
	event.Request.ClientIP = strings.TrimSpace(event.Request.ClientIP)
	event.Request.UserAgent = strings.TrimSpace(event.Request.UserAgent)
	if event.Metadata == nil {
		event.Metadata = map[string]any{}
	}
	if event.SupportAccessID != nil {
		event.Metadata["supportAccessId"] = *event.SupportAccessID
	}
	if event.ImpersonatedUserID != nil {
		event.Metadata["impersonatedUserId"] = *event.ImpersonatedUserID
	}

	return event, nil
}

func UserAuditContext(userID int64, tenantID *int64, request AuditRequest) AuditContext {
	return AuditContext{
		ActorType:   AuditActorUser,
		ActorUserID: &userID,
		TenantID:    tenantID,
		Request:     request,
	}
}

func auditInt8(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}
