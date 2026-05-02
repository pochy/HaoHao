package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrCustomerSignalSavedFilterNotFound = errors.New("customer signal saved filter not found")
	ErrInvalidCustomerSignalSavedFilter  = errors.New("invalid customer signal saved filter")
	ErrSavedFilterEntitlementDenied      = errors.New("saved filter entitlement denied")
)

const FeatureCustomerSignalSavedFilters = "customer_signals.saved_filters"

const (
	defaultCustomerSignalSavedFilterListLimit = 25
	maxCustomerSignalSavedFilterListLimit     = 100
	maxCustomerSignalSavedFilterSearchLength  = 200
)

type CustomerSignalSavedFilter struct {
	ID          int64
	PublicID    string
	TenantID    int64
	OwnerUserID int64
	Name        string
	Query       string
	Filters     map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CustomerSignalSavedFilterInput struct {
	Name    string
	Query   string
	Filters map[string]any
}

type CustomerSignalSavedFilterListInput struct {
	Query    string
	Status   string
	Priority string
	Source   string
	Cursor   string
	Limit    int
}

type CustomerSignalSavedFilterListResult struct {
	Items      []CustomerSignalSavedFilter
	NextCursor string
}

type CustomerSignalSavedFilterService struct {
	queries      *db.Queries
	entitlements *EntitlementService
	audit        AuditRecorder
}

func NewCustomerSignalSavedFilterService(queries *db.Queries, entitlements *EntitlementService, audit AuditRecorder) *CustomerSignalSavedFilterService {
	return &CustomerSignalSavedFilterService{queries: queries, entitlements: entitlements, audit: audit}
}

func (s *CustomerSignalSavedFilterService) List(ctx context.Context, tenantID, ownerUserID int64) ([]CustomerSignalSavedFilter, error) {
	if err := s.requireEnabled(ctx, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListCustomerSignalSavedFilters(ctx, db.ListCustomerSignalSavedFiltersParams{
		TenantID:    tenantID,
		OwnerUserID: ownerUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("list saved filters: %w", err)
	}
	items := make([]CustomerSignalSavedFilter, 0, len(rows))
	for _, row := range rows {
		items = append(items, customerSignalSavedFilterFromDB(row))
	}
	return items, nil
}

func (s *CustomerSignalSavedFilterService) Search(ctx context.Context, tenantID, ownerUserID int64, input CustomerSignalSavedFilterListInput) (CustomerSignalSavedFilterListResult, error) {
	if err := s.requireEnabled(ctx, tenantID); err != nil {
		return CustomerSignalSavedFilterListResult{}, err
	}
	normalized, cursor, err := normalizeSavedFilterListInput(input)
	if err != nil {
		return CustomerSignalSavedFilterListResult{}, err
	}
	limitPlusOne := normalized.Limit + 1
	rows, err := s.queries.SearchCustomerSignalSavedFilters(ctx, db.SearchCustomerSignalSavedFiltersParams{
		TenantID:        tenantID,
		OwnerUserID:     ownerUserID,
		Q:               pgText(normalized.Query),
		Status:          pgText(normalized.Status),
		Priority:        pgText(normalized.Priority),
		Source:          pgText(normalized.Source),
		CursorCreatedAt: pgTimestamp(cursor.CreatedAt),
		CursorID:        pgInt8(optionalCursorID(cursor)),
		ResultLimit:     int32(limitPlusOne),
	})
	if err != nil {
		return CustomerSignalSavedFilterListResult{}, fmt.Errorf("search saved filters: %w", err)
	}
	items := make([]CustomerSignalSavedFilter, 0, len(rows))
	for _, row := range rows {
		items = append(items, customerSignalSavedFilterFromDB(row))
	}
	result := CustomerSignalSavedFilterListResult{Items: items}
	if len(result.Items) > normalized.Limit {
		next := result.Items[normalized.Limit-1]
		nextCursor, err := EncodeCreatedAtIDCursor(CreatedAtIDCursor{CreatedAt: next.CreatedAt, ID: next.ID})
		if err != nil {
			return CustomerSignalSavedFilterListResult{}, err
		}
		result.NextCursor = nextCursor
		result.Items = result.Items[:normalized.Limit]
	}
	return result, nil
}

func (s *CustomerSignalSavedFilterService) Create(ctx context.Context, tenantID, ownerUserID int64, input CustomerSignalSavedFilterInput, auditCtx AuditContext) (CustomerSignalSavedFilter, error) {
	if err := s.requireEnabled(ctx, tenantID); err != nil {
		return CustomerSignalSavedFilter{}, err
	}
	normalized, payload, err := normalizeSavedFilterInput(input)
	if err != nil {
		return CustomerSignalSavedFilter{}, err
	}
	row, err := s.queries.CreateCustomerSignalSavedFilter(ctx, db.CreateCustomerSignalSavedFilterParams{
		TenantID:    tenantID,
		OwnerUserID: ownerUserID,
		Name:        normalized.Name,
		Query:       normalized.Query,
		Filters:     payload,
	})
	if err != nil {
		return CustomerSignalSavedFilter{}, fmt.Errorf("create saved filter: %w", err)
	}
	if s.audit != nil {
		auditCtx.TenantID = &tenantID
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "customer_signal_filter.create",
			TargetType:   "customer_signal_filter",
			TargetID:     row.PublicID.String(),
		})
	}
	return customerSignalSavedFilterFromDB(row), nil
}

func (s *CustomerSignalSavedFilterService) Update(ctx context.Context, tenantID, ownerUserID int64, publicID string, input CustomerSignalSavedFilterInput, auditCtx AuditContext) (CustomerSignalSavedFilter, error) {
	if err := s.requireEnabled(ctx, tenantID); err != nil {
		return CustomerSignalSavedFilter{}, err
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return CustomerSignalSavedFilter{}, ErrCustomerSignalSavedFilterNotFound
	}
	normalized, payload, err := normalizeSavedFilterInput(input)
	if err != nil {
		return CustomerSignalSavedFilter{}, err
	}
	row, err := s.queries.UpdateCustomerSignalSavedFilter(ctx, db.UpdateCustomerSignalSavedFilterParams{
		PublicID:    parsed,
		TenantID:    tenantID,
		OwnerUserID: ownerUserID,
		Name:        normalized.Name,
		Query:       normalized.Query,
		Filters:     payload,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerSignalSavedFilter{}, ErrCustomerSignalSavedFilterNotFound
	}
	if err != nil {
		return CustomerSignalSavedFilter{}, fmt.Errorf("update saved filter: %w", err)
	}
	if s.audit != nil {
		auditCtx.TenantID = &tenantID
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "customer_signal_filter.update",
			TargetType:   "customer_signal_filter",
			TargetID:     row.PublicID.String(),
		})
	}
	return customerSignalSavedFilterFromDB(row), nil
}

func (s *CustomerSignalSavedFilterService) Delete(ctx context.Context, tenantID, ownerUserID int64, publicID string, auditCtx AuditContext) error {
	if err := s.requireEnabled(ctx, tenantID); err != nil {
		return err
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return ErrCustomerSignalSavedFilterNotFound
	}
	affected, err := s.queries.DeleteCustomerSignalSavedFilter(ctx, db.DeleteCustomerSignalSavedFilterParams{
		PublicID:    parsed,
		TenantID:    tenantID,
		OwnerUserID: ownerUserID,
	})
	if err != nil {
		return fmt.Errorf("delete saved filter: %w", err)
	}
	if affected == 0 {
		return ErrCustomerSignalSavedFilterNotFound
	}
	if s.audit != nil {
		auditCtx.TenantID = &tenantID
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "customer_signal_filter.delete",
			TargetType:   "customer_signal_filter",
			TargetID:     parsed.String(),
		})
	}
	return nil
}

func (s *CustomerSignalSavedFilterService) requireEnabled(ctx context.Context, tenantID int64) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("customer signal saved filter service is not configured")
	}
	if s.entitlements == nil {
		return nil
	}
	enabled, err := s.entitlements.IsEnabled(ctx, tenantID, FeatureCustomerSignalSavedFilters)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrSavedFilterEntitlementDenied
	}
	return nil
}

func normalizeSavedFilterInput(input CustomerSignalSavedFilterInput) (CustomerSignalSavedFilterInput, []byte, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Query = strings.TrimSpace(input.Query)
	if input.Name == "" || len([]rune(input.Name)) > 120 || len([]rune(input.Query)) > 200 {
		return CustomerSignalSavedFilterInput{}, nil, ErrInvalidCustomerSignalSavedFilter
	}
	if input.Filters == nil {
		input.Filters = map[string]any{}
	}
	payload, err := json.Marshal(input.Filters)
	if err != nil {
		return CustomerSignalSavedFilterInput{}, nil, ErrInvalidCustomerSignalSavedFilter
	}
	return input, payload, nil
}

func normalizeSavedFilterListInput(input CustomerSignalSavedFilterListInput) (CustomerSignalSavedFilterListInput, CreatedAtIDCursor, error) {
	input.Query = strings.TrimSpace(input.Query)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.Priority = strings.ToLower(strings.TrimSpace(input.Priority))
	input.Source = strings.ToLower(strings.TrimSpace(input.Source))
	if len([]rune(input.Query)) > maxCustomerSignalSavedFilterSearchLength {
		return CustomerSignalSavedFilterListInput{}, CreatedAtIDCursor{}, ErrInvalidCustomerSignalSavedFilter
	}
	if input.Status != "" {
		if _, ok := allowedCustomerSignalStatuses[input.Status]; !ok {
			return CustomerSignalSavedFilterListInput{}, CreatedAtIDCursor{}, ErrInvalidCustomerSignalSavedFilter
		}
	}
	if input.Priority != "" {
		if _, ok := allowedCustomerSignalPriorities[input.Priority]; !ok {
			return CustomerSignalSavedFilterListInput{}, CreatedAtIDCursor{}, ErrInvalidCustomerSignalSavedFilter
		}
	}
	if input.Source != "" {
		if _, ok := allowedCustomerSignalSources[input.Source]; !ok {
			return CustomerSignalSavedFilterListInput{}, CreatedAtIDCursor{}, ErrInvalidCustomerSignalSavedFilter
		}
	}
	if input.Limit <= 0 {
		input.Limit = defaultCustomerSignalSavedFilterListLimit
	}
	if input.Limit > maxCustomerSignalSavedFilterListLimit {
		input.Limit = maxCustomerSignalSavedFilterListLimit
	}
	cursor, err := DecodeCreatedAtIDCursor(input.Cursor)
	if err != nil {
		return CustomerSignalSavedFilterListInput{}, CreatedAtIDCursor{}, err
	}
	return input, cursor, nil
}

func customerSignalSavedFilterFromDB(row db.CustomerSignalSavedFilter) CustomerSignalSavedFilter {
	return CustomerSignalSavedFilter{
		ID:          row.ID,
		PublicID:    row.PublicID.String(),
		TenantID:    row.TenantID,
		OwnerUserID: row.OwnerUserID,
		Name:        row.Name,
		Query:       row.Query,
		Filters:     jsonObject(row.Filters),
		CreatedAt:   timestamptzTime(row.CreatedAt),
		UpdatedAt:   timestamptzTime(row.UpdatedAt),
	}
}
