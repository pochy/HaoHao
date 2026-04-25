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
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCustomerSignalInput  = errors.New("invalid customer signal input")
	ErrInvalidCustomerSignalUpdate = errors.New("invalid customer signal update")
	ErrCustomerSignalNotFound      = errors.New("customer signal not found")
)

const (
	maxCustomerSignalCustomerNameLength = 120
	maxCustomerSignalTitleLength        = 200
	maxCustomerSignalBodyLength         = 4000

	defaultCustomerSignalSource   = "other"
	defaultCustomerSignalPriority = "medium"
	defaultCustomerSignalStatus   = "new"
)

var (
	allowedCustomerSignalSources = map[string]struct{}{
		"support":          {},
		"sales":            {},
		"customer_success": {},
		"research":         {},
		"internal":         {},
		"other":            {},
	}
	allowedCustomerSignalPriorities = map[string]struct{}{
		"low":    {},
		"medium": {},
		"high":   {},
		"urgent": {},
	}
	allowedCustomerSignalStatuses = map[string]struct{}{
		"new":     {},
		"triaged": {},
		"planned": {},
		"closed":  {},
	}
)

type CustomerSignal struct {
	ID           int64
	PublicID     string
	CustomerName string
	Title        string
	Body         string
	Source       string
	Priority     string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CustomerSignalCreateInput struct {
	CustomerName string
	Title        string
	Body         string
	Source       string
	Priority     string
	Status       string
}

type CustomerSignalUpdateInput struct {
	CustomerName *string
	Title        *string
	Body         *string
	Source       *string
	Priority     *string
	Status       *string
}

type CustomerSignalListInput struct {
	Query    string
	Status   string
	Priority string
	Source   string
	Cursor   string
	Limit    int
}

type CustomerSignalListResult struct {
	Items      []CustomerSignal
	NextCursor string
}

type CustomerSignalService struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	audit    AuditRecorder
	webhooks *WebhookService
}

func NewCustomerSignalService(pool *pgxpool.Pool, queries *db.Queries, audit AuditRecorder, webhooks ...*WebhookService) *CustomerSignalService {
	var webhookService *WebhookService
	if len(webhooks) > 0 {
		webhookService = webhooks[0]
	}
	return &CustomerSignalService{
		pool:     pool,
		queries:  queries,
		audit:    audit,
		webhooks: webhookService,
	}
}

func (s *CustomerSignalService) List(ctx context.Context, tenantID int64) ([]CustomerSignal, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("customer signal service is not configured")
	}

	rows, err := s.queries.ListCustomerSignalsByTenantID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list customer signals: %w", err)
	}

	items := make([]CustomerSignal, 0, len(rows))
	for _, row := range rows {
		items = append(items, customerSignalFromDB(row))
	}
	return items, nil
}

func (s *CustomerSignalService) Search(ctx context.Context, tenantID int64, input CustomerSignalListInput) (CustomerSignalListResult, error) {
	if s == nil || s.queries == nil {
		return CustomerSignalListResult{}, fmt.Errorf("customer signal service is not configured")
	}
	normalized, cursor, err := normalizeCustomerSignalListInput(input)
	if err != nil {
		return CustomerSignalListResult{}, err
	}
	limitPlusOne := normalized.Limit + 1
	rows, err := s.queries.SearchCustomerSignals(ctx, db.SearchCustomerSignalsParams{
		TenantID:        tenantID,
		Q:               pgText(normalized.Query),
		Status:          pgText(normalized.Status),
		Priority:        pgText(normalized.Priority),
		Source:          pgText(normalized.Source),
		CursorCreatedAt: pgTimestamp(cursor.CreatedAt),
		CursorID:        pgInt8(optionalCursorID(cursor)),
		ResultLimit:     int32(limitPlusOne),
	})
	if err != nil {
		return CustomerSignalListResult{}, fmt.Errorf("search customer signals: %w", err)
	}
	items := make([]CustomerSignal, 0, len(rows))
	for _, row := range rows {
		items = append(items, customerSignalFromDB(row))
	}
	result := CustomerSignalListResult{Items: items}
	if len(result.Items) > normalized.Limit {
		next := result.Items[normalized.Limit-1]
		nextCursor, err := EncodeCreatedAtIDCursor(CreatedAtIDCursor{CreatedAt: next.CreatedAt, ID: next.ID})
		if err != nil {
			return CustomerSignalListResult{}, err
		}
		result.NextCursor = nextCursor
		result.Items = result.Items[:normalized.Limit]
	}
	return result, nil
}

func (s *CustomerSignalService) Get(ctx context.Context, tenantID int64, publicID string) (CustomerSignal, error) {
	if s == nil || s.queries == nil {
		return CustomerSignal{}, fmt.Errorf("customer signal service is not configured")
	}

	parsedPublicID, err := parseCustomerSignalPublicID(publicID)
	if err != nil {
		return CustomerSignal{}, err
	}

	row, err := s.queries.GetCustomerSignalByPublicIDForTenant(ctx, db.GetCustomerSignalByPublicIDForTenantParams{
		PublicID: parsedPublicID,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerSignal{}, ErrCustomerSignalNotFound
	}
	if err != nil {
		return CustomerSignal{}, fmt.Errorf("get customer signal: %w", err)
	}
	return customerSignalFromDB(row), nil
}

func (s *CustomerSignalService) Create(ctx context.Context, tenantID, userID int64, input CustomerSignalCreateInput, auditCtx AuditContext) (CustomerSignal, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return CustomerSignal{}, fmt.Errorf("customer signal service is not configured")
	}
	if s.audit == nil {
		return CustomerSignal{}, fmt.Errorf("audit recorder is not configured")
	}

	normalized, err := normalizeCustomerSignalCreateInput(input)
	if err != nil {
		return CustomerSignal{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CustomerSignal{}, fmt.Errorf("begin customer signal create transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	row, err := qtx.CreateCustomerSignal(ctx, db.CreateCustomerSignalParams{
		TenantID:        tenantID,
		CreatedByUserID: pgtype.Int8{Int64: userID, Valid: true},
		CustomerName:    normalized.CustomerName,
		Title:           normalized.Title,
		Body:            normalized.Body,
		Source:          normalized.Source,
		Priority:        normalized.Priority,
		Status:          normalized.Status,
	})
	if err != nil {
		return CustomerSignal{}, fmt.Errorf("create customer signal: %w", err)
	}
	item := customerSignalFromDB(row)

	auditCtx.TenantID = &tenantID
	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "customer_signal.create",
		TargetType:   "customer_signal",
		TargetID:     item.PublicID,
		Metadata: map[string]any{
			"titleLength": len([]rune(normalized.Title)),
			"bodyLength":  len([]rune(normalized.Body)),
			"source":      item.Source,
			"priority":    item.Priority,
			"status":      item.Status,
		},
	}); err != nil {
		return CustomerSignal{}, err
	}
	if s.webhooks != nil {
		if err := s.webhooks.EnqueueTenantEventWithQueries(ctx, qtx, tenantID, "customer_signal.created", map[string]any{
			"publicId":     item.PublicID,
			"customerName": item.CustomerName,
			"title":        item.Title,
			"source":       item.Source,
			"priority":     item.Priority,
			"status":       item.Status,
		}); err != nil {
			return CustomerSignal{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return CustomerSignal{}, fmt.Errorf("commit customer signal create transaction: %w", err)
	}
	return item, nil
}

func (s *CustomerSignalService) Update(ctx context.Context, tenantID int64, publicID string, input CustomerSignalUpdateInput, auditCtx AuditContext) (CustomerSignal, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return CustomerSignal{}, fmt.Errorf("customer signal service is not configured")
	}
	if s.audit == nil {
		return CustomerSignal{}, fmt.Errorf("audit recorder is not configured")
	}
	if input.CustomerName == nil && input.Title == nil && input.Body == nil && input.Source == nil && input.Priority == nil && input.Status == nil {
		return CustomerSignal{}, ErrInvalidCustomerSignalUpdate
	}

	parsedPublicID, err := parseCustomerSignalPublicID(publicID)
	if err != nil {
		return CustomerSignal{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CustomerSignal{}, fmt.Errorf("begin customer signal update transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	existing, err := qtx.GetCustomerSignalByPublicIDForTenant(ctx, db.GetCustomerSignalByPublicIDForTenantParams{
		PublicID: parsedPublicID,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerSignal{}, ErrCustomerSignalNotFound
	}
	if err != nil {
		return CustomerSignal{}, fmt.Errorf("get customer signal before update: %w", err)
	}

	params, changedFields, err := customerSignalUpdateParams(parsedPublicID, tenantID, existing, input)
	if err != nil {
		return CustomerSignal{}, err
	}

	row, err := qtx.UpdateCustomerSignalByPublicIDForTenant(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerSignal{}, ErrCustomerSignalNotFound
	}
	if err != nil {
		return CustomerSignal{}, fmt.Errorf("update customer signal: %w", err)
	}
	item := customerSignalFromDB(row)

	auditCtx.TenantID = &tenantID
	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "customer_signal.update",
		TargetType:   "customer_signal",
		TargetID:     item.PublicID,
		Metadata: map[string]any{
			"changedFields": changedFields,
		},
	}); err != nil {
		return CustomerSignal{}, err
	}
	if s.webhooks != nil {
		if err := s.webhooks.EnqueueTenantEventWithQueries(ctx, qtx, tenantID, "customer_signal.updated", map[string]any{
			"publicId":      item.PublicID,
			"changedFields": changedFields,
			"status":        item.Status,
		}); err != nil {
			return CustomerSignal{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return CustomerSignal{}, fmt.Errorf("commit customer signal update transaction: %w", err)
	}
	return item, nil
}

func (s *CustomerSignalService) Delete(ctx context.Context, tenantID int64, publicID string, auditCtx AuditContext) error {
	if s == nil || s.pool == nil || s.queries == nil {
		return fmt.Errorf("customer signal service is not configured")
	}
	if s.audit == nil {
		return fmt.Errorf("audit recorder is not configured")
	}

	parsedPublicID, err := parseCustomerSignalPublicID(publicID)
	if err != nil {
		return err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin customer signal delete transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	existing, err := qtx.GetCustomerSignalByPublicIDForTenant(ctx, db.GetCustomerSignalByPublicIDForTenantParams{
		PublicID: parsedPublicID,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCustomerSignalNotFound
	}
	if err != nil {
		return fmt.Errorf("get customer signal before delete: %w", err)
	}

	affectedRows, err := qtx.SoftDeleteCustomerSignalByPublicIDForTenant(ctx, db.SoftDeleteCustomerSignalByPublicIDForTenantParams{
		PublicID: parsedPublicID,
		TenantID: tenantID,
	})
	if err != nil {
		return fmt.Errorf("delete customer signal: %w", err)
	}
	if affectedRows == 0 {
		return ErrCustomerSignalNotFound
	}

	auditCtx.TenantID = &tenantID
	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "customer_signal.delete",
		TargetType:   "customer_signal",
		TargetID:     parsedPublicID.String(),
		Metadata: map[string]any{
			"status": existing.Status,
		},
	}); err != nil {
		return err
	}
	if s.webhooks != nil {
		if err := s.webhooks.EnqueueTenantEventWithQueries(ctx, qtx, tenantID, "customer_signal.deleted", map[string]any{
			"publicId": parsedPublicID.String(),
			"status":   existing.Status,
		}); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit customer signal delete transaction: %w", err)
	}
	return nil
}

func customerSignalUpdateParams(parsedPublicID uuid.UUID, tenantID int64, existing db.CustomerSignal, input CustomerSignalUpdateInput) (db.UpdateCustomerSignalByPublicIDForTenantParams, []string, error) {
	customerName := existing.CustomerName
	title := existing.Title
	body := existing.Body
	source := existing.Source
	priority := existing.Priority
	status := existing.Status
	changedFields := make([]string, 0, 6)

	if input.CustomerName != nil {
		normalized, err := normalizeCustomerSignalText(*input.CustomerName, maxCustomerSignalCustomerNameLength, true)
		if err != nil {
			return db.UpdateCustomerSignalByPublicIDForTenantParams{}, nil, err
		}
		customerName = normalized
		changedFields = append(changedFields, "customerName")
	}
	if input.Title != nil {
		normalized, err := normalizeCustomerSignalText(*input.Title, maxCustomerSignalTitleLength, true)
		if err != nil {
			return db.UpdateCustomerSignalByPublicIDForTenantParams{}, nil, err
		}
		title = normalized
		changedFields = append(changedFields, "title")
	}
	if input.Body != nil {
		normalized, err := normalizeCustomerSignalText(*input.Body, maxCustomerSignalBodyLength, false)
		if err != nil {
			return db.UpdateCustomerSignalByPublicIDForTenantParams{}, nil, err
		}
		body = normalized
		changedFields = append(changedFields, "body")
	}
	if input.Source != nil {
		normalized, err := normalizeCustomerSignalEnum(*input.Source, "", allowedCustomerSignalSources)
		if err != nil {
			return db.UpdateCustomerSignalByPublicIDForTenantParams{}, nil, err
		}
		source = normalized
		changedFields = append(changedFields, "source")
	}
	if input.Priority != nil {
		normalized, err := normalizeCustomerSignalEnum(*input.Priority, "", allowedCustomerSignalPriorities)
		if err != nil {
			return db.UpdateCustomerSignalByPublicIDForTenantParams{}, nil, err
		}
		priority = normalized
		changedFields = append(changedFields, "priority")
	}
	if input.Status != nil {
		normalized, err := normalizeCustomerSignalEnum(*input.Status, "", allowedCustomerSignalStatuses)
		if err != nil {
			return db.UpdateCustomerSignalByPublicIDForTenantParams{}, nil, err
		}
		status = normalized
		changedFields = append(changedFields, "status")
	}

	return db.UpdateCustomerSignalByPublicIDForTenantParams{
		PublicID:     parsedPublicID,
		TenantID:     tenantID,
		CustomerName: customerName,
		Title:        title,
		Body:         body,
		Source:       source,
		Priority:     priority,
		Status:       status,
	}, changedFields, nil
}

func normalizeCustomerSignalCreateInput(input CustomerSignalCreateInput) (CustomerSignalCreateInput, error) {
	var err error
	input.CustomerName, err = normalizeCustomerSignalText(input.CustomerName, maxCustomerSignalCustomerNameLength, true)
	if err != nil {
		return CustomerSignalCreateInput{}, err
	}
	input.Title, err = normalizeCustomerSignalText(input.Title, maxCustomerSignalTitleLength, true)
	if err != nil {
		return CustomerSignalCreateInput{}, err
	}
	input.Body, err = normalizeCustomerSignalText(input.Body, maxCustomerSignalBodyLength, false)
	if err != nil {
		return CustomerSignalCreateInput{}, err
	}
	input.Source, err = normalizeCustomerSignalEnum(input.Source, defaultCustomerSignalSource, allowedCustomerSignalSources)
	if err != nil {
		return CustomerSignalCreateInput{}, err
	}
	input.Priority, err = normalizeCustomerSignalEnum(input.Priority, defaultCustomerSignalPriority, allowedCustomerSignalPriorities)
	if err != nil {
		return CustomerSignalCreateInput{}, err
	}
	input.Status, err = normalizeCustomerSignalEnum(input.Status, defaultCustomerSignalStatus, allowedCustomerSignalStatuses)
	if err != nil {
		return CustomerSignalCreateInput{}, err
	}
	return input, nil
}

func normalizeCustomerSignalListInput(input CustomerSignalListInput) (CustomerSignalListInput, CreatedAtIDCursor, error) {
	input.Query = strings.TrimSpace(input.Query)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.Priority = strings.ToLower(strings.TrimSpace(input.Priority))
	input.Source = strings.ToLower(strings.TrimSpace(input.Source))
	if input.Status != "" {
		if _, ok := allowedCustomerSignalStatuses[input.Status]; !ok {
			return CustomerSignalListInput{}, CreatedAtIDCursor{}, ErrInvalidCustomerSignalInput
		}
	}
	if input.Priority != "" {
		if _, ok := allowedCustomerSignalPriorities[input.Priority]; !ok {
			return CustomerSignalListInput{}, CreatedAtIDCursor{}, ErrInvalidCustomerSignalInput
		}
	}
	if input.Source != "" {
		if _, ok := allowedCustomerSignalSources[input.Source]; !ok {
			return CustomerSignalListInput{}, CreatedAtIDCursor{}, ErrInvalidCustomerSignalInput
		}
	}
	if input.Limit <= 0 {
		input.Limit = 25
	}
	if input.Limit > 100 {
		input.Limit = 100
	}
	cursor, err := DecodeCreatedAtIDCursor(input.Cursor)
	if err != nil {
		return CustomerSignalListInput{}, CreatedAtIDCursor{}, err
	}
	return input, cursor, nil
}

func optionalCursorID(cursor CreatedAtIDCursor) *int64 {
	if cursor.ID <= 0 {
		return nil
	}
	return &cursor.ID
}

func normalizeCustomerSignalText(value string, maxLength int, required bool) (string, error) {
	normalized := strings.TrimSpace(value)
	length := len([]rune(normalized))
	if (required && normalized == "") || length > maxLength {
		return "", ErrInvalidCustomerSignalInput
	}
	return normalized, nil
}

func normalizeCustomerSignalEnum(value, defaultValue string, allowed map[string]struct{}) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" && defaultValue != "" {
		normalized = defaultValue
	}
	if _, ok := allowed[normalized]; !ok {
		return "", ErrInvalidCustomerSignalInput
	}
	return normalized, nil
}

func parseCustomerSignalPublicID(publicID string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return uuid.Nil, ErrCustomerSignalNotFound
	}
	return parsed, nil
}

func customerSignalFromDB(row db.CustomerSignal) CustomerSignal {
	return CustomerSignal{
		ID:           row.ID,
		PublicID:     row.PublicID.String(),
		CustomerName: row.CustomerName,
		Title:        row.Title,
		Body:         row.Body,
		Source:       row.Source,
		Priority:     row.Priority,
		Status:       row.Status,
		CreatedAt:    timestamptzTime(row.CreatedAt),
		UpdatedAt:    timestamptzTime(row.UpdatedAt),
	}
}
