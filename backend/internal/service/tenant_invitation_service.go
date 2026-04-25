package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidTenantInvitation       = errors.New("invalid tenant invitation")
	ErrTenantInvitationNotFound      = errors.New("tenant invitation not found")
	ErrTenantInvitationEmailMismatch = errors.New("tenant invitation email mismatch")
)

type TenantInvitation struct {
	ID                     int64
	PublicID               string
	TenantID               int64
	InviteeEmailNormalized string
	RoleCodes              []string
	Status                 string
	Token                  string
	AcceptURL              string
	ExpiresAt              time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type TenantInvitationInput struct {
	TenantID  int64
	ActorID   int64
	Email     string
	RoleCodes []string
}

type TenantInvitationService struct {
	pool            *pgxpool.Pool
	queries         *db.Queries
	outbox          *OutboxService
	audit           AuditRecorder
	ttl             time.Duration
	frontendBaseURL string
}

func NewTenantInvitationService(pool *pgxpool.Pool, queries *db.Queries, outbox *OutboxService, audit AuditRecorder, ttl time.Duration, frontendBaseURL string) *TenantInvitationService {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &TenantInvitationService{
		pool:            pool,
		queries:         queries,
		outbox:          outbox,
		audit:           audit,
		ttl:             ttl,
		frontendBaseURL: strings.TrimRight(frontendBaseURL, "/"),
	}
}

func (s *TenantInvitationService) Create(ctx context.Context, input TenantInvitationInput, auditCtx AuditContext) (TenantInvitation, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return TenantInvitation{}, fmt.Errorf("tenant invitation service is not configured")
	}
	normalized, err := normalizeTenantInvitationInput(input)
	if err != nil {
		return TenantInvitation{}, err
	}
	token, err := newInvitationToken()
	if err != nil {
		return TenantInvitation{}, err
	}
	rolePayload, err := json.Marshal(normalized.RoleCodes)
	if err != nil {
		return TenantInvitation{}, fmt.Errorf("encode invitation roles: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TenantInvitation{}, fmt.Errorf("begin tenant invitation transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)

	row, err := qtx.CreateTenantInvitation(ctx, db.CreateTenantInvitationParams{
		TenantID:               normalized.TenantID,
		InvitedByUserID:        pgtype.Int8{Int64: normalized.ActorID, Valid: normalized.ActorID > 0},
		InviteeEmailNormalized: normalized.Email,
		RoleCodes:              rolePayload,
		TokenHash:              invitationTokenHash(token),
		ExpiresAt:              pgTimestamp(time.Now().Add(s.ttl)),
	})
	if err != nil {
		return TenantInvitation{}, fmt.Errorf("create tenant invitation: %w", err)
	}
	if s.outbox != nil {
		if _, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
			TenantID:      &normalized.TenantID,
			AggregateType: "tenant_invitation",
			AggregateID:   row.PublicID.String(),
			EventType:     "tenant_invitation.created",
			Payload: map[string]any{
				"invitationId": row.ID,
				"tenantId":     row.TenantID,
			},
		}); err != nil {
			return TenantInvitation{}, err
		}
	}
	if invitee, err := qtx.GetUserByEmail(ctx, normalized.Email); err == nil && !invitee.DeactivatedAt.Valid {
		metadata, _ := json.Marshal(map[string]any{
			"tenantId":     row.TenantID,
			"invitationId": row.PublicID.String(),
		})
		if _, err := qtx.CreateNotification(ctx, db.CreateNotificationParams{
			TenantID:        pgtype.Int8{Int64: row.TenantID, Valid: true},
			RecipientUserID: invitee.ID,
			Channel:         "in_app",
			Template:        "tenant_invitation",
			Subject:         "Tenant invitation",
			Body:            "You have a new tenant invitation.",
			Metadata:        metadata,
		}); err != nil {
			return TenantInvitation{}, fmt.Errorf("create invitation notification: %w", err)
		}
		if s.outbox != nil {
			if _, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
				TenantID:      &normalized.TenantID,
				AggregateType: "tenant_invitation",
				AggregateID:   row.PublicID.String(),
				EventType:     "notification.email_requested",
				Payload: map[string]any{
					"recipientUserId": invitee.ID,
					"subject":         "Tenant invitation",
					"body":            "You have a new tenant invitation.",
				},
			}); err != nil {
				return TenantInvitation{}, err
			}
		}
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return TenantInvitation{}, fmt.Errorf("lookup invitee user: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "tenant_invitation.create",
			TargetType:   "tenant_invitation",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"roleCodes": rowRoleCodes(row),
			},
		}); err != nil {
			return TenantInvitation{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return TenantInvitation{}, fmt.Errorf("commit tenant invitation transaction: %w", err)
	}
	item := tenantInvitationFromDB(row)
	item.Token = token
	item.AcceptURL = s.acceptURL(token)
	return item, nil
}

func (s *TenantInvitationService) List(ctx context.Context, tenantID int64, limit int) ([]TenantInvitation, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("tenant invitation service is not configured")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.queries.ListTenantInvitations(ctx, db.ListTenantInvitationsParams{
		TenantID: tenantID,
		Limit:    int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list tenant invitations: %w", err)
	}
	items := make([]TenantInvitation, 0, len(rows))
	for _, row := range rows {
		items = append(items, tenantInvitationFromDB(row))
	}
	return items, nil
}

func (s *TenantInvitationService) Revoke(ctx context.Context, tenantID int64, publicID string, auditCtx AuditContext) error {
	if s == nil || s.pool == nil || s.queries == nil {
		return fmt.Errorf("tenant invitation service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return ErrTenantInvitationNotFound
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin revoke invitation transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	row, err := qtx.RevokeTenantInvitation(ctx, db.RevokeTenantInvitationParams{
		PublicID: parsed,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTenantInvitationNotFound
	}
	if err != nil {
		return fmt.Errorf("revoke tenant invitation: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "tenant_invitation.revoke",
			TargetType:   "tenant_invitation",
			TargetID:     row.PublicID.String(),
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *TenantInvitationService) Accept(ctx context.Context, user User, token string, auditCtx AuditContext) (TenantInvitation, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return TenantInvitation{}, fmt.Errorf("tenant invitation service is not configured")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TenantInvitation{}, fmt.Errorf("begin accept invitation transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	row, err := qtx.GetPendingTenantInvitationByTokenHash(ctx, invitationTokenHash(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return TenantInvitation{}, ErrTenantInvitationNotFound
	}
	if err != nil {
		return TenantInvitation{}, fmt.Errorf("get pending invitation: %w", err)
	}
	if normalizeEmail(user.Email) != row.InviteeEmailNormalized {
		return TenantInvitation{}, ErrTenantInvitationEmailMismatch
	}

	roleCodes := rowRoleCodes(row)
	roles, err := qtx.GetRolesByCode(ctx, roleCodes)
	if err != nil {
		return TenantInvitation{}, fmt.Errorf("load invitation roles: %w", err)
	}
	roleIDByCode := make(map[string]int64, len(roles))
	for _, role := range roles {
		roleIDByCode[role.Code] = role.ID
	}
	for _, roleCode := range roleCodes {
		roleID, ok := roleIDByCode[roleCode]
		if !ok {
			return TenantInvitation{}, fmt.Errorf("%w: unsupported role", ErrInvalidTenantInvitation)
		}
		if err := qtx.UpsertTenantMembership(ctx, db.UpsertTenantMembershipParams{
			UserID:   user.ID,
			TenantID: row.TenantID,
			RoleID:   roleID,
			Source:   "local_override",
		}); err != nil {
			return TenantInvitation{}, fmt.Errorf("upsert invitation membership: %w", err)
		}
	}
	accepted, err := qtx.AcceptTenantInvitation(ctx, db.AcceptTenantInvitationParams{
		ID:               row.ID,
		AcceptedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	})
	if err != nil {
		return TenantInvitation{}, fmt.Errorf("accept invitation: %w", err)
	}
	if user.DefaultTenantID == nil {
		if _, err := qtx.SetUserDefaultTenant(ctx, db.SetUserDefaultTenantParams{
			ID:              user.ID,
			DefaultTenantID: pgtype.Int8{Int64: row.TenantID, Valid: true},
		}); err != nil {
			return TenantInvitation{}, fmt.Errorf("set default tenant after invitation: %w", err)
		}
	}
	if s.audit != nil {
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "tenant_invitation.accept",
			TargetType:   "tenant_invitation",
			TargetID:     accepted.PublicID.String(),
			Metadata: map[string]any{
				"roleCodes": roleCodes,
			},
		}); err != nil {
			return TenantInvitation{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return TenantInvitation{}, fmt.Errorf("commit accept invitation transaction: %w", err)
	}
	return tenantInvitationFromDB(accepted), nil
}

func (s *TenantInvitationService) HandleInvitationCreated(ctx context.Context, invitationID int64) error {
	if s == nil || s.queries == nil {
		return nil
	}
	// Log delivery is handled from the outbox worker. If the invitee already exists,
	// create a short in-app notification without copying the token.
	return nil
}

func normalizeTenantInvitationInput(input TenantInvitationInput) (TenantInvitationInput, error) {
	input.Email = normalizeEmail(input.Email)
	if input.TenantID <= 0 || input.ActorID <= 0 || input.Email == "" {
		return TenantInvitationInput{}, fmt.Errorf("%w: tenant, actor, and email are required", ErrInvalidTenantInvitation)
	}
	if _, err := mail.ParseAddress(input.Email); err != nil {
		return TenantInvitationInput{}, fmt.Errorf("%w: invalid email", ErrInvalidTenantInvitation)
	}
	if len(input.RoleCodes) == 0 {
		input.RoleCodes = []string{"todo_user"}
	}
	set := make(map[string]struct{}, len(input.RoleCodes))
	for _, roleCode := range input.RoleCodes {
		normalized := strings.ToLower(strings.TrimSpace(roleCode))
		if !IsSupportedTenantRole(normalized) {
			return TenantInvitationInput{}, fmt.Errorf("%w: unsupported role", ErrInvalidTenantInvitation)
		}
		set[normalized] = struct{}{}
	}
	input.RoleCodes = input.RoleCodes[:0]
	for roleCode := range set {
		input.RoleCodes = append(input.RoleCodes, roleCode)
	}
	sort.Strings(input.RoleCodes)
	return input, nil
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func newInvitationToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate invitation token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func invitationTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func rowRoleCodes(row db.TenantInvitation) []string {
	var roles []string
	if len(row.RoleCodes) > 0 {
		_ = json.Unmarshal(row.RoleCodes, &roles)
	}
	sort.Strings(roles)
	return roles
}

func tenantInvitationFromDB(row db.TenantInvitation) TenantInvitation {
	return TenantInvitation{
		ID:                     row.ID,
		PublicID:               row.PublicID.String(),
		TenantID:               row.TenantID,
		InviteeEmailNormalized: row.InviteeEmailNormalized,
		RoleCodes:              rowRoleCodes(row),
		Status:                 row.Status,
		ExpiresAt:              row.ExpiresAt.Time,
		CreatedAt:              row.CreatedAt.Time,
		UpdatedAt:              row.UpdatedAt.Time,
	}
}

func (s *TenantInvitationService) acceptURL(token string) string {
	if s.frontendBaseURL == "" {
		return ""
	}
	values := url.Values{}
	values.Set("token", token)
	return s.frontendBaseURL + "/invitations/accept?" + values.Encode()
}
