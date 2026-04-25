package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"example.com/haohao/backend/internal/auth"
	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrMachineClientNotFound    = errors.New("machine client not found")
	ErrInvalidMachineClient     = errors.New("invalid machine client")
	ErrMachineClientInactive    = errors.New("machine client inactive")
	ErrMachineClientScopeDenied = errors.New("machine client scope denied")
)

type MachineClient struct {
	ID               int64
	Provider         string
	ProviderClientID string
	DisplayName      string
	DefaultTenant    *TenantAccess
	AllowedScopes    []string
	Active           bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type MachineClientInput struct {
	Provider         string
	ProviderClientID string
	DisplayName      string
	DefaultTenantID  *int64
	AllowedScopes    []string
	Active           *bool
}

type MachineClientContext struct {
	Provider         string
	ProviderClientID string
	Scopes           []string
	Client           MachineClient
	Claims           auth.BearerTokenClaims
}

type machineClientContextKey struct{}

type MachineClientService struct {
	pool                *pgxpool.Pool
	queries             *db.Queries
	requiredScopePrefix string
	audit               AuditRecorder
}

func NewMachineClientService(pool *pgxpool.Pool, queries *db.Queries, requiredScopePrefix string, audit AuditRecorder) *MachineClientService {
	return &MachineClientService{
		pool:                pool,
		queries:             queries,
		requiredScopePrefix: strings.TrimSpace(requiredScopePrefix),
		audit:               audit,
	}
}

func ContextWithMachineClient(ctx context.Context, machineCtx MachineClientContext) context.Context {
	return context.WithValue(ctx, machineClientContextKey{}, machineCtx)
}

func MachineClientFromContext(ctx context.Context) (MachineClientContext, bool) {
	machineCtx, ok := ctx.Value(machineClientContextKey{}).(MachineClientContext)
	return machineCtx, ok
}

func (s *MachineClientService) List(ctx context.Context) ([]MachineClient, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("machine client service is not configured")
	}

	rows, err := s.queries.ListMachineClients(ctx)
	if err != nil {
		return nil, fmt.Errorf("list machine clients: %w", err)
	}

	items := make([]MachineClient, 0, len(rows))
	for _, row := range rows {
		item, err := s.machineClientFromDB(ctx, row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *MachineClientService) Get(ctx context.Context, id int64) (MachineClient, error) {
	if s == nil || s.queries == nil {
		return MachineClient{}, fmt.Errorf("machine client service is not configured")
	}

	row, err := s.queries.GetMachineClientByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return MachineClient{}, ErrMachineClientNotFound
	}
	if err != nil {
		return MachineClient{}, fmt.Errorf("get machine client: %w", err)
	}
	return s.machineClientFromDB(ctx, row)
}

func (s *MachineClientService) Create(ctx context.Context, input MachineClientInput, auditCtx AuditContext) (MachineClient, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return MachineClient{}, fmt.Errorf("machine client service is not configured")
	}
	if s.audit == nil {
		return MachineClient{}, fmt.Errorf("audit recorder is not configured")
	}

	normalized, err := s.normalizeInput(ctx, input, true, true)
	if err != nil {
		return MachineClient{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return MachineClient{}, fmt.Errorf("begin machine client create transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	row, err := qtx.CreateMachineClient(ctx, db.CreateMachineClientParams{
		Provider:         normalized.Provider,
		ProviderClientID: normalized.ProviderClientID,
		DisplayName:      normalized.DisplayName,
		DefaultTenantID:  int64ToPgInt8(normalized.DefaultTenantID),
		AllowedScopes:    normalized.AllowedScopes,
		Active:           *normalized.Active,
	})
	if err != nil {
		return MachineClient{}, fmt.Errorf("create machine client: %w", err)
	}
	item, err := s.machineClientFromDBWithQueries(ctx, qtx, row)
	if err != nil {
		return MachineClient{}, err
	}

	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "machine_client.create",
		TargetType:   "machine_client",
		TargetID:     strconv.FormatInt(item.ID, 10),
		Metadata: map[string]any{
			"provider":          item.Provider,
			"defaultTenantID":   machineClientDefaultTenantID(item),
			"allowedScopeCount": len(item.AllowedScopes),
			"active":            item.Active,
		},
	}); err != nil {
		return MachineClient{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return MachineClient{}, fmt.Errorf("commit machine client create transaction: %w", err)
	}
	return item, nil
}

func (s *MachineClientService) Update(ctx context.Context, id int64, input MachineClientInput, auditCtx AuditContext) (MachineClient, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return MachineClient{}, fmt.Errorf("machine client service is not configured")
	}
	if s.audit == nil {
		return MachineClient{}, fmt.Errorf("audit recorder is not configured")
	}

	existing, err := s.Get(ctx, id)
	if err != nil {
		return MachineClient{}, err
	}
	if input.Provider == "" {
		input.Provider = existing.Provider
	}
	if input.Active == nil {
		active := existing.Active
		input.Active = &active
	}

	normalized, err := s.normalizeInput(ctx, input, false, false)
	if err != nil {
		return MachineClient{}, err
	}
	if normalized.Provider != existing.Provider {
		return MachineClient{}, fmt.Errorf("%w: provider cannot be changed", ErrInvalidMachineClient)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return MachineClient{}, fmt.Errorf("begin machine client update transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	row, err := qtx.UpdateMachineClient(ctx, db.UpdateMachineClientParams{
		ID:               id,
		ProviderClientID: normalized.ProviderClientID,
		DisplayName:      normalized.DisplayName,
		DefaultTenantID:  int64ToPgInt8(normalized.DefaultTenantID),
		AllowedScopes:    normalized.AllowedScopes,
		Active:           *normalized.Active,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return MachineClient{}, ErrMachineClientNotFound
	}
	if err != nil {
		return MachineClient{}, fmt.Errorf("update machine client: %w", err)
	}
	item, err := s.machineClientFromDBWithQueries(ctx, qtx, row)
	if err != nil {
		return MachineClient{}, err
	}

	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "machine_client.update",
		TargetType:   "machine_client",
		TargetID:     strconv.FormatInt(item.ID, 10),
		Metadata: map[string]any{
			"changedFields": machineClientChangedFields(existing, item),
		},
	}); err != nil {
		return MachineClient{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return MachineClient{}, fmt.Errorf("commit machine client update transaction: %w", err)
	}
	return item, nil
}

func (s *MachineClientService) Disable(ctx context.Context, id int64, auditCtx AuditContext) (MachineClient, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return MachineClient{}, fmt.Errorf("machine client service is not configured")
	}
	if s.audit == nil {
		return MachineClient{}, fmt.Errorf("audit recorder is not configured")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return MachineClient{}, fmt.Errorf("begin machine client disable transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	row, err := qtx.DisableMachineClient(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return MachineClient{}, ErrMachineClientNotFound
	}
	if err != nil {
		return MachineClient{}, fmt.Errorf("disable machine client: %w", err)
	}
	item, err := s.machineClientFromDBWithQueries(ctx, qtx, row)
	if err != nil {
		return MachineClient{}, err
	}

	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "machine_client.disable",
		TargetType:   "machine_client",
		TargetID:     strconv.FormatInt(item.ID, 10),
	}); err != nil {
		return MachineClient{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return MachineClient{}, fmt.Errorf("commit machine client disable transaction: %w", err)
	}
	return item, nil
}

func (s *MachineClientService) AuthenticateM2M(ctx context.Context, provider string, principal auth.M2MPrincipal) (MachineClientContext, error) {
	if s == nil || s.queries == nil {
		return MachineClientContext{}, fmt.Errorf("machine client service is not configured")
	}
	if err := principal.Validate(); err != nil {
		return MachineClientContext{}, err
	}

	normalizedProvider := normalizeMachineProvider(provider)
	row, err := s.queries.GetMachineClientByProviderClientID(ctx, db.GetMachineClientByProviderClientIDParams{
		Provider:         normalizedProvider,
		ProviderClientID: strings.TrimSpace(principal.ProviderClientID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return MachineClientContext{}, ErrMachineClientNotFound
	}
	if err != nil {
		return MachineClientContext{}, fmt.Errorf("lookup machine client: %w", err)
	}
	if !row.Active {
		return MachineClientContext{}, ErrMachineClientInactive
	}

	allowedScopes := scopeSet(row.AllowedScopes)
	m2mScopes := principal.ScopeValuesWithPrefix(s.requiredScopePrefix)
	if len(m2mScopes) == 0 {
		return MachineClientContext{}, ErrMachineClientScopeDenied
	}
	for _, scope := range m2mScopes {
		if _, ok := allowedScopes[scope]; !ok {
			return MachineClientContext{}, ErrMachineClientScopeDenied
		}
	}

	client, err := s.machineClientFromDB(ctx, row)
	if err != nil {
		return MachineClientContext{}, err
	}
	return MachineClientContext{
		Provider:         normalizedProvider,
		ProviderClientID: principal.ProviderClientID,
		Scopes:           m2mScopes,
		Client:           client,
		Claims:           principal.Claims,
	}, nil
}

func (s *MachineClientService) normalizeInput(ctx context.Context, input MachineClientInput, defaultProvider bool, defaultActive bool) (MachineClientInput, error) {
	input.Provider = normalizeMachineProvider(input.Provider)
	if !defaultProvider && strings.TrimSpace(input.Provider) == "" {
		return MachineClientInput{}, fmt.Errorf("%w: provider is required", ErrInvalidMachineClient)
	}

	input.ProviderClientID = strings.TrimSpace(input.ProviderClientID)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.ProviderClientID == "" {
		return MachineClientInput{}, fmt.Errorf("%w: provider client id is required", ErrInvalidMachineClient)
	}
	if input.DisplayName == "" {
		return MachineClientInput{}, fmt.Errorf("%w: display name is required", ErrInvalidMachineClient)
	}

	allowedScopes, err := s.normalizeAllowedScopes(input.AllowedScopes)
	if err != nil {
		return MachineClientInput{}, err
	}
	input.AllowedScopes = allowedScopes

	if input.DefaultTenantID != nil {
		if *input.DefaultTenantID <= 0 {
			return MachineClientInput{}, fmt.Errorf("%w: default tenant id must be positive", ErrInvalidMachineClient)
		}
		tenant, err := s.queries.GetTenantByID(ctx, *input.DefaultTenantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return MachineClientInput{}, fmt.Errorf("%w: default tenant not found", ErrInvalidMachineClient)
		}
		if err != nil {
			return MachineClientInput{}, fmt.Errorf("load default tenant: %w", err)
		}
		if !tenant.Active {
			return MachineClientInput{}, fmt.Errorf("%w: default tenant is inactive", ErrInvalidMachineClient)
		}
	}

	if input.Active == nil {
		active := defaultActive
		input.Active = &active
	}
	return input, nil
}

func (s *MachineClientService) normalizeAllowedScopes(scopes []string) ([]string, error) {
	set := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed == "" {
			continue
		}
		if s.requiredScopePrefix != "" && !strings.HasPrefix(trimmed, s.requiredScopePrefix) {
			return nil, fmt.Errorf("%w: allowed scope must use %s prefix", ErrInvalidMachineClient, s.requiredScopePrefix)
		}
		set[trimmed] = struct{}{}
	}

	normalized := make([]string, 0, len(set))
	for scope := range set {
		normalized = append(normalized, scope)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func (s *MachineClientService) machineClientFromDB(ctx context.Context, row db.MachineClient) (MachineClient, error) {
	return s.machineClientFromDBWithQueries(ctx, s.queries, row)
}

func (s *MachineClientService) machineClientFromDBWithQueries(ctx context.Context, queries *db.Queries, row db.MachineClient) (MachineClient, error) {
	var defaultTenant *TenantAccess
	if row.DefaultTenantID.Valid {
		tenant, err := queries.GetTenantByID(ctx, row.DefaultTenantID.Int64)
		if err != nil {
			return MachineClient{}, fmt.Errorf("load machine client default tenant: %w", err)
		}
		defaultTenant = &TenantAccess{
			ID:          tenant.ID,
			Slug:        tenant.Slug,
			DisplayName: tenant.DisplayName,
			Default:     true,
		}
	}

	return MachineClient{
		ID:               row.ID,
		Provider:         row.Provider,
		ProviderClientID: row.ProviderClientID,
		DisplayName:      row.DisplayName,
		DefaultTenant:    defaultTenant,
		AllowedScopes:    append([]string(nil), row.AllowedScopes...),
		Active:           row.Active,
		CreatedAt:        timestamptzTime(row.CreatedAt),
		UpdatedAt:        timestamptzTime(row.UpdatedAt),
	}, nil
}

func machineClientDefaultTenantID(item MachineClient) *int64 {
	if item.DefaultTenant == nil {
		return nil
	}
	id := item.DefaultTenant.ID
	return &id
}

func machineClientChangedFields(before, after MachineClient) []string {
	fields := make([]string, 0, 5)
	if before.ProviderClientID != after.ProviderClientID {
		fields = append(fields, "providerClientId")
	}
	if before.DisplayName != after.DisplayName {
		fields = append(fields, "displayName")
	}
	if !sameOptionalInt64(machineClientDefaultTenantID(before), machineClientDefaultTenantID(after)) {
		fields = append(fields, "defaultTenantId")
	}
	if !sameStringSet(before.AllowedScopes, after.AllowedScopes) {
		fields = append(fields, "allowedScopes")
	}
	if before.Active != after.Active {
		fields = append(fields, "active")
	}
	return fields
}

func sameOptionalInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func normalizeMachineProvider(provider string) string {
	trimmed := strings.ToLower(strings.TrimSpace(provider))
	if trimmed == "" {
		return "zitadel"
	}
	return trimmed
}

func int64ToPgInt8(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}

func timestamptzTime(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

func scopeSet(scopes []string) map[string]struct{} {
	set := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}
	return set
}
