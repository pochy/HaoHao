package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTenantAdminInvalidInput      = errors.New("invalid tenant admin input")
	ErrTenantAdminTenantNotFound    = errors.New("tenant not found")
	ErrTenantAdminUserNotFound      = errors.New("user not found")
	ErrTenantAdminUserInactive      = errors.New("user inactive")
	ErrTenantAdminRoleNotFound      = errors.New("tenant role not found")
	ErrTenantAdminLocalRoleNotFound = errors.New("local tenant role not found")
	ErrTenantAdminDuplicateTenant   = errors.New("tenant already exists")
	ErrTenantAdminLastAdmin         = errors.New("cannot remove the last tenant admin")
)

const (
	tenantMembershipSourceLocal = "local_override"

	minTenantSlugLength        = 3
	maxTenantSlugLength        = 64
	maxTenantDisplayNameLength = 120
)

type TenantAdminTenant struct {
	ID                int64
	Slug              string
	DisplayName       string
	Active            bool
	ActiveMemberCount int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type TenantAdminTenantDetail struct {
	Tenant      TenantAdminTenant
	Memberships []TenantAdminMembership
}

type TenantAdminMembership struct {
	UserPublicID string
	Email        string
	DisplayName  string
	Deactivated  bool
	Roles        []TenantAdminRoleBinding
}

type TenantAdminRoleBinding struct {
	RoleCode string
	Source   string
	Active   bool
}

type TenantAdminTenantInput struct {
	Slug        string
	DisplayName string
	Active      *bool
}

type TenantRoleGrantInput struct {
	UserEmail string
	RoleCode  string
}

type TenantAdminService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	audit   AuditRecorder
}

func NewTenantAdminService(pool *pgxpool.Pool, queries *db.Queries, audit AuditRecorder) *TenantAdminService {
	return &TenantAdminService{
		pool:    pool,
		queries: queries,
		audit:   audit,
	}
}

func (s *TenantAdminService) ListTenants(ctx context.Context) ([]TenantAdminTenant, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("tenant admin service is not configured")
	}

	rows, err := s.queries.ListTenantAdminTenants(ctx)
	if err != nil {
		return nil, fmt.Errorf("list admin tenants: %w", err)
	}

	items := make([]TenantAdminTenant, 0, len(rows))
	for _, row := range rows {
		items = append(items, tenantAdminTenantFromListRow(row))
	}
	return items, nil
}

func (s *TenantAdminService) GetTenant(ctx context.Context, tenantSlug string) (TenantAdminTenantDetail, error) {
	if s == nil || s.queries == nil {
		return TenantAdminTenantDetail{}, fmt.Errorf("tenant admin service is not configured")
	}

	normalizedSlug, err := normalizeTenantSlug(tenantSlug)
	if err != nil {
		return TenantAdminTenantDetail{}, err
	}

	row, err := s.queries.GetTenantAdminTenant(ctx, normalizedSlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return TenantAdminTenantDetail{}, ErrTenantAdminTenantNotFound
	}
	if err != nil {
		return TenantAdminTenantDetail{}, fmt.Errorf("get admin tenant: %w", err)
	}

	membershipRows, err := s.queries.ListTenantAdminMembershipRows(ctx, normalizedSlug)
	if err != nil {
		return TenantAdminTenantDetail{}, fmt.Errorf("list admin tenant memberships: %w", err)
	}

	return TenantAdminTenantDetail{
		Tenant:      tenantAdminTenantFromGetRow(row),
		Memberships: tenantAdminMembershipsFromRows(membershipRows),
	}, nil
}

func (s *TenantAdminService) CreateTenant(ctx context.Context, input TenantAdminTenantInput, auditCtx AuditContext) (TenantAdminTenant, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return TenantAdminTenant{}, fmt.Errorf("tenant admin service is not configured")
	}
	if s.audit == nil {
		return TenantAdminTenant{}, fmt.Errorf("audit recorder is not configured")
	}

	normalized, err := normalizeTenantAdminTenantInput(input, true)
	if err != nil {
		return TenantAdminTenant{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TenantAdminTenant{}, fmt.Errorf("begin tenant create transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	row, err := qtx.CreateTenantAdminTenant(ctx, db.CreateTenantAdminTenantParams{
		Slug:        normalized.Slug,
		DisplayName: normalized.DisplayName,
	})
	if isUniqueViolation(err) {
		return TenantAdminTenant{}, ErrTenantAdminDuplicateTenant
	}
	if err != nil {
		return TenantAdminTenant{}, fmt.Errorf("create tenant: %w", err)
	}

	tenant := tenantAdminTenantFromDB(row, 0)
	auditCtx.TenantID = &tenant.ID
	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "tenant.create",
		TargetType:   "tenant",
		TargetID:     tenant.Slug,
		Metadata: map[string]any{
			"displayNameLength": len([]rune(tenant.DisplayName)),
		},
	}); err != nil {
		return TenantAdminTenant{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return TenantAdminTenant{}, fmt.Errorf("commit tenant create transaction: %w", err)
	}
	return tenant, nil
}

func (s *TenantAdminService) UpdateTenant(ctx context.Context, tenantSlug string, input TenantAdminTenantInput, auditCtx AuditContext) (TenantAdminTenant, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return TenantAdminTenant{}, fmt.Errorf("tenant admin service is not configured")
	}
	if s.audit == nil {
		return TenantAdminTenant{}, fmt.Errorf("audit recorder is not configured")
	}

	normalizedSlug, err := normalizeTenantSlug(tenantSlug)
	if err != nil {
		return TenantAdminTenant{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TenantAdminTenant{}, fmt.Errorf("begin tenant update transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	existingRow, err := qtx.GetTenantAdminTenant(ctx, normalizedSlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return TenantAdminTenant{}, ErrTenantAdminTenantNotFound
	}
	if err != nil {
		return TenantAdminTenant{}, fmt.Errorf("get tenant before update: %w", err)
	}
	existing := tenantAdminTenantFromGetRow(existingRow)

	normalized, err := normalizeTenantAdminTenantInput(input, false)
	if err != nil {
		return TenantAdminTenant{}, err
	}
	if normalized.Active == nil {
		normalized.Active = &existing.Active
	}

	row, err := qtx.UpdateTenantAdminTenant(ctx, db.UpdateTenantAdminTenantParams{
		Slug:        normalizedSlug,
		DisplayName: normalized.DisplayName,
		Active:      *normalized.Active,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return TenantAdminTenant{}, ErrTenantAdminTenantNotFound
	}
	if err != nil {
		return TenantAdminTenant{}, fmt.Errorf("update tenant: %w", err)
	}

	tenant := tenantAdminTenantFromDB(row, existing.ActiveMemberCount)
	auditCtx.TenantID = &tenant.ID
	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "tenant.update",
		TargetType:   "tenant",
		TargetID:     tenant.Slug,
		Metadata: map[string]any{
			"changedFields": tenantAdminChangedFields(existing, tenant),
		},
	}); err != nil {
		return TenantAdminTenant{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return TenantAdminTenant{}, fmt.Errorf("commit tenant update transaction: %w", err)
	}
	return tenant, nil
}

func (s *TenantAdminService) DeactivateTenant(ctx context.Context, tenantSlug string, auditCtx AuditContext) (TenantAdminTenant, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return TenantAdminTenant{}, fmt.Errorf("tenant admin service is not configured")
	}
	if s.audit == nil {
		return TenantAdminTenant{}, fmt.Errorf("audit recorder is not configured")
	}

	normalizedSlug, err := normalizeTenantSlug(tenantSlug)
	if err != nil {
		return TenantAdminTenant{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TenantAdminTenant{}, fmt.Errorf("begin tenant deactivate transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	existingRow, err := qtx.GetTenantAdminTenant(ctx, normalizedSlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return TenantAdminTenant{}, ErrTenantAdminTenantNotFound
	}
	if err != nil {
		return TenantAdminTenant{}, fmt.Errorf("get tenant before deactivate: %w", err)
	}
	existing := tenantAdminTenantFromGetRow(existingRow)

	row, err := qtx.DeactivateTenantAdminTenant(ctx, normalizedSlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return TenantAdminTenant{}, ErrTenantAdminTenantNotFound
	}
	if err != nil {
		return TenantAdminTenant{}, fmt.Errorf("deactivate tenant: %w", err)
	}

	tenant := tenantAdminTenantFromDB(row, existing.ActiveMemberCount)
	auditCtx.TenantID = &tenant.ID
	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "tenant.deactivate",
		TargetType:   "tenant",
		TargetID:     tenant.Slug,
	}); err != nil {
		return TenantAdminTenant{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return TenantAdminTenant{}, fmt.Errorf("commit tenant deactivate transaction: %w", err)
	}
	return tenant, nil
}

func (s *TenantAdminService) GrantRole(ctx context.Context, tenantSlug string, input TenantRoleGrantInput, auditCtx AuditContext) (TenantAdminMembership, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return TenantAdminMembership{}, fmt.Errorf("tenant admin service is not configured")
	}
	if s.audit == nil {
		return TenantAdminMembership{}, fmt.Errorf("audit recorder is not configured")
	}

	normalizedSlug, err := normalizeTenantSlug(tenantSlug)
	if err != nil {
		return TenantAdminMembership{}, err
	}
	userEmail, err := normalizeTenantAdminUserEmail(input.UserEmail)
	if err != nil {
		return TenantAdminMembership{}, err
	}
	roleCode, err := normalizeTenantRoleCode(input.RoleCode)
	if err != nil {
		return TenantAdminMembership{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TenantAdminMembership{}, fmt.Errorf("begin tenant role grant transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	tenant, err := qtx.GetTenantBySlug(ctx, normalizedSlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return TenantAdminMembership{}, ErrTenantAdminTenantNotFound
	}
	if err != nil {
		return TenantAdminMembership{}, fmt.Errorf("load tenant: %w", err)
	}
	if !tenant.Active {
		return TenantAdminMembership{}, fmt.Errorf("%w: tenant is inactive", ErrTenantAdminInvalidInput)
	}

	user, err := qtx.GetUserByEmail(ctx, userEmail)
	if errors.Is(err, pgx.ErrNoRows) {
		return TenantAdminMembership{}, ErrTenantAdminUserNotFound
	}
	if err != nil {
		return TenantAdminMembership{}, fmt.Errorf("load user by email: %w", err)
	}
	if user.DeactivatedAt.Valid {
		return TenantAdminMembership{}, ErrTenantAdminUserInactive
	}

	role, err := loadTenantAdminRole(ctx, qtx, roleCode)
	if err != nil {
		return TenantAdminMembership{}, err
	}

	if _, err := qtx.UpsertTenantAdminLocalMembership(ctx, db.UpsertTenantAdminLocalMembershipParams{
		UserID:   user.ID,
		TenantID: tenant.ID,
		RoleID:   role.ID,
	}); err != nil {
		return TenantAdminMembership{}, fmt.Errorf("upsert local tenant membership: %w", err)
	}

	auditCtx.TenantID = &tenant.ID
	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "tenant_role.grant",
		TargetType:   "tenant_role",
		TargetID:     tenant.Slug + ":" + user.PublicID.String() + ":" + role.Code,
		Metadata: map[string]any{
			"roleCode": role.Code,
			"source":   tenantMembershipSourceLocal,
		},
	}); err != nil {
		return TenantAdminMembership{}, err
	}

	membershipRows, err := qtx.ListTenantAdminMembershipRows(ctx, normalizedSlug)
	if err != nil {
		return TenantAdminMembership{}, fmt.Errorf("list membership after grant: %w", err)
	}
	membership, ok := findTenantAdminMembership(tenantAdminMembershipsFromRows(membershipRows), user.PublicID.String())
	if !ok {
		return TenantAdminMembership{}, fmt.Errorf("membership missing after grant")
	}

	if err := tx.Commit(ctx); err != nil {
		return TenantAdminMembership{}, fmt.Errorf("commit tenant role grant transaction: %w", err)
	}
	return membership, nil
}

func (s *TenantAdminService) RevokeLocalRole(ctx context.Context, tenantSlug, userPublicID, roleCode string, auditCtx AuditContext) error {
	if s == nil || s.pool == nil || s.queries == nil {
		return fmt.Errorf("tenant admin service is not configured")
	}
	if s.audit == nil {
		return fmt.Errorf("audit recorder is not configured")
	}

	normalizedSlug, err := normalizeTenantSlug(tenantSlug)
	if err != nil {
		return err
	}
	parsedUserPublicID, err := uuid.Parse(strings.TrimSpace(userPublicID))
	if err != nil {
		return ErrTenantAdminUserNotFound
	}
	normalizedRoleCode, err := normalizeTenantRoleCode(roleCode)
	if err != nil {
		return err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tenant role revoke transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	tenant, err := qtx.GetTenantBySlug(ctx, normalizedSlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTenantAdminTenantNotFound
	}
	if err != nil {
		return fmt.Errorf("load tenant: %w", err)
	}

	user, err := qtx.GetUserByPublicID(ctx, parsedUserPublicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTenantAdminUserNotFound
	}
	if err != nil {
		return fmt.Errorf("load user by public id: %w", err)
	}

	role, err := loadTenantAdminRole(ctx, qtx, normalizedRoleCode)
	if err != nil {
		return err
	}

	affectedRows, err := qtx.DeactivateTenantAdminLocalMembershipRole(ctx, db.DeactivateTenantAdminLocalMembershipRoleParams{
		UserID:   user.ID,
		TenantID: tenant.ID,
		RoleID:   role.ID,
	})
	if err != nil {
		return fmt.Errorf("deactivate local tenant membership role: %w", err)
	}
	if affectedRows == 0 {
		return ErrTenantAdminLocalRoleNotFound
	}
	if role.Code == "tenant_admin" {
		adminCount, err := qtx.CountActiveTenantAdmins(ctx, tenant.ID)
		if err != nil {
			return fmt.Errorf("count active tenant admins: %w", err)
		}
		if adminCount == 0 {
			return ErrTenantAdminLastAdmin
		}
	}

	auditCtx.TenantID = &tenant.ID
	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "tenant_role.revoke",
		TargetType:   "tenant_role",
		TargetID:     tenant.Slug + ":" + user.PublicID.String() + ":" + role.Code,
		Metadata: map[string]any{
			"roleCode": role.Code,
			"source":   tenantMembershipSourceLocal,
		},
	}); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tenant role revoke transaction: %w", err)
	}
	return nil
}

func loadTenantAdminRole(ctx context.Context, queries *db.Queries, roleCode string) (db.GetRolesByCodeRow, error) {
	roles, err := queries.GetRolesByCode(ctx, []string{roleCode})
	if err != nil {
		return db.GetRolesByCodeRow{}, fmt.Errorf("load tenant role: %w", err)
	}
	if len(roles) == 0 {
		return db.GetRolesByCodeRow{}, ErrTenantAdminRoleNotFound
	}
	return roles[0], nil
}

func normalizeTenantAdminTenantInput(input TenantAdminTenantInput, requireSlug bool) (TenantAdminTenantInput, error) {
	if requireSlug {
		slug, err := normalizeTenantSlug(input.Slug)
		if err != nil {
			return TenantAdminTenantInput{}, err
		}
		input.Slug = slug
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" || len([]rune(displayName)) > maxTenantDisplayNameLength {
		return TenantAdminTenantInput{}, fmt.Errorf("%w: display name must be 1-%d characters", ErrTenantAdminInvalidInput, maxTenantDisplayNameLength)
	}
	input.DisplayName = displayName
	return input, nil
}

func normalizeTenantSlug(slug string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(slug))
	if len(normalized) < minTenantSlugLength || len(normalized) > maxTenantSlugLength {
		return "", fmt.Errorf("%w: slug must be %d-%d characters", ErrTenantAdminInvalidInput, minTenantSlugLength, maxTenantSlugLength)
	}
	if strings.HasPrefix(normalized, "-") || strings.HasSuffix(normalized, "-") {
		return "", fmt.Errorf("%w: slug cannot start or end with hyphen", ErrTenantAdminInvalidInput)
	}
	for _, char := range normalized {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			continue
		}
		return "", fmt.Errorf("%w: slug must use lowercase letters, numbers, and hyphens", ErrTenantAdminInvalidInput)
	}
	return normalized, nil
}

func normalizeTenantAdminUserEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" || !strings.Contains(normalized, "@") {
		return "", fmt.Errorf("%w: user email is required", ErrTenantAdminInvalidInput)
	}
	return normalized, nil
}

func normalizeTenantRoleCode(roleCode string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(roleCode))
	if !IsSupportedTenantRole(normalized) {
		return "", ErrTenantAdminRoleNotFound
	}
	return normalized, nil
}

func tenantAdminTenantFromListRow(row db.ListTenantAdminTenantsRow) TenantAdminTenant {
	return TenantAdminTenant{
		ID:                row.ID,
		Slug:              row.Slug,
		DisplayName:       row.DisplayName,
		Active:            row.Active,
		ActiveMemberCount: row.ActiveMemberCount,
		CreatedAt:         timestamptzTime(row.CreatedAt),
		UpdatedAt:         timestamptzTime(row.UpdatedAt),
	}
}

func tenantAdminTenantFromGetRow(row db.GetTenantAdminTenantRow) TenantAdminTenant {
	return TenantAdminTenant{
		ID:                row.ID,
		Slug:              row.Slug,
		DisplayName:       row.DisplayName,
		Active:            row.Active,
		ActiveMemberCount: row.ActiveMemberCount,
		CreatedAt:         timestamptzTime(row.CreatedAt),
		UpdatedAt:         timestamptzTime(row.UpdatedAt),
	}
}

func tenantAdminTenantFromDB(row db.Tenant, activeMemberCount int64) TenantAdminTenant {
	return TenantAdminTenant{
		ID:                row.ID,
		Slug:              row.Slug,
		DisplayName:       row.DisplayName,
		Active:            row.Active,
		ActiveMemberCount: activeMemberCount,
		CreatedAt:         timestamptzTime(row.CreatedAt),
		UpdatedAt:         timestamptzTime(row.UpdatedAt),
	}
}

func tenantAdminMembershipsFromRows(rows []db.ListTenantAdminMembershipRowsRow) []TenantAdminMembership {
	byUser := make(map[string]*TenantAdminMembership)
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		publicID := row.UserPublicID.String()
		membership, ok := byUser[publicID]
		if !ok {
			membership = &TenantAdminMembership{
				UserPublicID: publicID,
				Email:        row.Email,
				DisplayName:  row.UserDisplayName,
				Deactivated:  row.UserDeactivatedAt.Valid,
				Roles:        []TenantAdminRoleBinding{},
			}
			byUser[publicID] = membership
			order = append(order, publicID)
		}
		membership.Roles = append(membership.Roles, TenantAdminRoleBinding{
			RoleCode: row.RoleCode,
			Source:   row.Source,
			Active:   row.Active,
		})
	}

	items := make([]TenantAdminMembership, 0, len(order))
	for _, publicID := range order {
		membership := *byUser[publicID]
		sort.SliceStable(membership.Roles, func(i, j int) bool {
			if membership.Roles[i].RoleCode == membership.Roles[j].RoleCode {
				return membership.Roles[i].Source < membership.Roles[j].Source
			}
			return membership.Roles[i].RoleCode < membership.Roles[j].RoleCode
		})
		items = append(items, membership)
	}
	return items
}

func findTenantAdminMembership(items []TenantAdminMembership, userPublicID string) (TenantAdminMembership, bool) {
	for _, item := range items {
		if item.UserPublicID == userPublicID {
			return item, true
		}
	}
	return TenantAdminMembership{}, false
}

func tenantAdminChangedFields(before, after TenantAdminTenant) []string {
	fields := make([]string, 0, 2)
	if before.DisplayName != after.DisplayName {
		fields = append(fields, "displayName")
	}
	if before.Active != after.Active {
		fields = append(fields, "active")
	}
	return fields
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
