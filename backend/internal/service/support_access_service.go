package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrSupportAccessNotFound      = errors.New("support access not found")
	ErrInvalidSupportAccessInput  = errors.New("invalid support access input")
	ErrSupportAccessEntitlement   = errors.New("support access entitlement denied")
	ErrSupportAccessTenantMissing = errors.New("impersonated user is not active in tenant")
)

const FeatureSupportAccessEnabled = "support_access.enabled"

type SupportAccess struct {
	ID                          int64
	PublicID                    string
	SupportUserID               int64
	SupportUserPublicID         string
	SupportUserEmail            string
	SupportUserDisplayName      string
	ImpersonatedUserID          int64
	ImpersonatedUserPublicID    string
	ImpersonatedUserEmail       string
	ImpersonatedUserDisplayName string
	TenantID                    int64
	TenantSlug                  string
	TenantDisplayName           string
	Reason                      string
	Status                      string
	StartedAt                   time.Time
	ExpiresAt                   time.Time
	EndedAt                     *time.Time
}

type SupportAccessStartInput struct {
	TenantSlug               string
	ImpersonatedUserPublicID string
	Reason                   string
	DurationMinutes          int
}

type SupportAccessService struct {
	queries      *db.Queries
	session      *SessionService
	entitlements *EntitlementService
	audit        AuditRecorder
	maxDuration  time.Duration
}

func NewSupportAccessService(queries *db.Queries, session *SessionService, entitlements *EntitlementService, audit AuditRecorder, maxDuration time.Duration) *SupportAccessService {
	if maxDuration <= 0 {
		maxDuration = time.Hour
	}
	return &SupportAccessService{queries: queries, session: session, entitlements: entitlements, audit: audit, maxDuration: maxDuration}
}

func (s *SupportAccessService) Start(ctx context.Context, sessionID string, supportUserID int64, input SupportAccessStartInput, auditCtx AuditContext) (SupportAccess, error) {
	if s == nil || s.queries == nil || s.session == nil {
		return SupportAccess{}, fmt.Errorf("support access service is not configured")
	}
	normalized, err := s.normalizeStartInput(input)
	if err != nil {
		return SupportAccess{}, err
	}
	tenant, err := s.queries.GetTenantBySlug(ctx, normalized.TenantSlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return SupportAccess{}, ErrSupportAccessTenantMissing
	}
	if err != nil {
		return SupportAccess{}, fmt.Errorf("get support access tenant: %w", err)
	}
	if s.entitlements != nil {
		enabled, err := s.entitlements.IsEnabled(ctx, tenant.ID, FeatureSupportAccessEnabled)
		if err != nil {
			return SupportAccess{}, err
		}
		if !enabled {
			return SupportAccess{}, ErrSupportAccessEntitlement
		}
	}
	impersonatedID, err := uuid.Parse(normalized.ImpersonatedUserPublicID)
	if err != nil {
		return SupportAccess{}, ErrInvalidSupportAccessInput
	}
	impersonated, err := s.queries.GetUserByPublicID(ctx, impersonatedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return SupportAccess{}, ErrInvalidSupportAccessInput
	}
	if err != nil {
		return SupportAccess{}, fmt.Errorf("get impersonated user: %w", err)
	}
	if impersonated.ID == supportUserID {
		return SupportAccess{}, ErrInvalidSupportAccessInput
	}
	ok, err := s.queries.UserHasActiveTenant(ctx, db.UserHasActiveTenantParams{
		UserID: impersonated.ID,
		ID:     tenant.ID,
	})
	if err != nil {
		return SupportAccess{}, fmt.Errorf("check impersonated tenant: %w", err)
	}
	if !ok {
		return SupportAccess{}, ErrSupportAccessTenantMissing
	}
	expiresAt := time.Now().UTC().Add(time.Duration(normalized.DurationMinutes) * time.Minute)
	row, err := s.queries.CreateSupportAccessSession(ctx, db.CreateSupportAccessSessionParams{
		SupportUserID:      supportUserID,
		ImpersonatedUserID: impersonated.ID,
		TenantID:           tenant.ID,
		Reason:             normalized.Reason,
		ExpiresAt:          pgTimestamp(expiresAt),
	})
	if err != nil {
		return SupportAccess{}, fmt.Errorf("create support access session: %w", err)
	}
	if err := s.session.SetSupportAccessSession(ctx, sessionID, row.ID, tenant.ID); err != nil {
		_, _ = s.queries.EndSupportAccessSession(ctx, row.ID)
		return SupportAccess{}, err
	}
	if s.audit != nil {
		auditCtx.TenantID = &tenant.ID
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "support_access.start",
			TargetType:   "support_access",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"impersonatedUserId": impersonated.PublicID.String(),
				"tenantSlug":         tenant.Slug,
				"expiresAt":          expiresAt.Format(time.RFC3339),
			},
		})
	}
	loaded, err := s.GetByID(ctx, row.ID)
	if err != nil {
		return SupportAccess{}, err
	}
	return loaded, nil
}

func (s *SupportAccessService) Current(ctx context.Context, sessionID string) (SupportAccess, bool, error) {
	if s == nil || s.session == nil {
		return SupportAccess{}, false, fmt.Errorf("support access service is not configured")
	}
	current, err := s.session.CurrentSession(ctx, sessionID)
	if err != nil {
		return SupportAccess{}, false, err
	}
	if current.SupportAccess == nil {
		return SupportAccess{}, false, nil
	}
	return *current.SupportAccess, true, nil
}

func (s *SupportAccessService) End(ctx context.Context, sessionID string, auditCtx AuditContext) error {
	if s == nil || s.queries == nil || s.session == nil {
		return fmt.Errorf("support access service is not configured")
	}
	current, err := s.session.CurrentSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if current.SupportAccess == nil {
		return nil
	}
	if _, err := s.queries.EndSupportAccessSession(ctx, current.SupportAccess.ID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("end support access session: %w", err)
	}
	if err := s.session.ClearSupportAccessSession(ctx, sessionID); err != nil {
		return err
	}
	if s.audit != nil {
		tenantID := current.SupportAccess.TenantID
		auditCtx.TenantID = &tenantID
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "support_access.end",
			TargetType:   "support_access",
			TargetID:     current.SupportAccess.PublicID,
		})
	}
	return nil
}

func (s *SupportAccessService) GetByID(ctx context.Context, id int64) (SupportAccess, error) {
	row, err := s.queries.GetSupportAccessSessionByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return SupportAccess{}, ErrSupportAccessNotFound
	}
	if err != nil {
		return SupportAccess{}, fmt.Errorf("get support access session: %w", err)
	}
	return supportAccessFromRow(row), nil
}

func (s *SupportAccessService) normalizeStartInput(input SupportAccessStartInput) (SupportAccessStartInput, error) {
	input.TenantSlug = strings.ToLower(strings.TrimSpace(input.TenantSlug))
	input.ImpersonatedUserPublicID = strings.TrimSpace(input.ImpersonatedUserPublicID)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.DurationMinutes <= 0 {
		input.DurationMinutes = 30
	}
	if input.TenantSlug == "" || input.ImpersonatedUserPublicID == "" || len([]rune(input.Reason)) < 8 || time.Duration(input.DurationMinutes)*time.Minute > s.maxDuration {
		return SupportAccessStartInput{}, ErrInvalidSupportAccessInput
	}
	return input, nil
}

func supportAccessFromRow(row db.GetSupportAccessSessionByIDRow) SupportAccess {
	return SupportAccess{
		ID:                          row.ID,
		PublicID:                    row.PublicID.String(),
		SupportUserID:               row.SupportUserID,
		SupportUserPublicID:         row.SupportUserPublicID.String(),
		SupportUserEmail:            row.SupportUserEmail,
		SupportUserDisplayName:      row.SupportUserDisplayName,
		ImpersonatedUserID:          row.ImpersonatedUserID,
		ImpersonatedUserPublicID:    row.ImpersonatedUserPublicID.String(),
		ImpersonatedUserEmail:       row.ImpersonatedUserEmail,
		ImpersonatedUserDisplayName: row.ImpersonatedUserDisplayName,
		TenantID:                    row.TenantID,
		TenantSlug:                  row.TenantSlug,
		TenantDisplayName:           row.TenantDisplayName,
		Reason:                      row.Reason,
		Status:                      row.Status,
		StartedAt:                   timestamptzTime(row.StartedAt),
		ExpiresAt:                   timestamptzTime(row.ExpiresAt),
		EndedAt:                     timeFromPg(row.EndedAt),
	}
}

func supportAccessFromSessionRow(row db.GetSupportAccessSessionByIDRow) *SupportAccess {
	item := supportAccessFromRow(row)
	return &item
}

func supportAccessAuditContext(ctx context.Context, supportUserID int64, tenantID *int64, request AuditRequest) AuditContext {
	return AuditContext{
		ActorType:   AuditActorUser,
		ActorUserID: &supportUserID,
		TenantID:    tenantID,
		Request:     request,
	}
}

func pgSupportTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: !value.IsZero()}
}
