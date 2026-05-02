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
	ErrNotificationNotFound = errors.New("notification not found")
	ErrInvalidNotification  = errors.New("invalid notification")
)

const (
	defaultNotificationListLimit = 25
	maxNotificationListLimit     = 100
	maxNotificationSearchLength  = 200

	notificationReadStateAll    = "all"
	notificationReadStateUnread = "unread"
	notificationReadStateRead   = "read"
)

var (
	allowedNotificationChannels = map[string]struct{}{
		"in_app": {},
		"email":  {},
	}
	allowedNotificationReadStates = map[string]struct{}{
		notificationReadStateAll:    {},
		notificationReadStateUnread: {},
		notificationReadStateRead:   {},
	}
)

type Notification struct {
	PublicID  string
	TenantID  *int64
	Channel   string
	Template  string
	Subject   string
	Body      string
	Status    string
	ReadAt    *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type NotificationInput struct {
	TenantID        *int64
	RecipientUserID int64
	Channel         string
	Template        string
	Subject         string
	Body            string
	Metadata        map[string]any
	OutboxEventID   *int64
}

type NotificationListInput struct {
	Query        string
	ReadState    string
	Channel      string
	CreatedAfter time.Time
	Cursor       string
	Limit        int
}

type NotificationListResult struct {
	Items         []Notification
	NextCursor    string
	TotalCount    int64
	FilteredCount int64
	UnreadCount   int64
	ReadCount     int64
}

type NotificationService struct {
	queries  *db.Queries
	audit    AuditRecorder
	realtime RealtimePublisher
}

func NewNotificationService(queries *db.Queries, audit AuditRecorder) *NotificationService {
	return &NotificationService{queries: queries, audit: audit}
}

func (s *NotificationService) SetRealtimeService(realtime RealtimePublisher) {
	if s != nil {
		s.realtime = realtime
	}
}

func (s *NotificationService) Create(ctx context.Context, input NotificationInput) (Notification, error) {
	if s == nil || s.queries == nil {
		return Notification{}, fmt.Errorf("notification service is not configured")
	}
	row, err := s.CreateWithQueries(ctx, s.queries, input)
	if err != nil {
		return Notification{}, err
	}
	item := notificationFromDB(row)
	s.publishNotificationEvent(ctx, "notification.created", item, input.RecipientUserID)
	return item, nil
}

func (s *NotificationService) CreateWithQueries(ctx context.Context, queries *db.Queries, input NotificationInput) (db.Notification, error) {
	if queries == nil {
		return db.Notification{}, fmt.Errorf("notification queries are not configured")
	}
	normalized, err := normalizeNotificationInput(input)
	if err != nil {
		return db.Notification{}, err
	}
	metadata, err := json.Marshal(normalized.Metadata)
	if err != nil {
		return db.Notification{}, fmt.Errorf("encode notification metadata: %w", err)
	}
	return queries.CreateNotification(ctx, db.CreateNotificationParams{
		TenantID:        pgInt8(normalized.TenantID),
		RecipientUserID: normalized.RecipientUserID,
		Channel:         normalized.Channel,
		Template:        normalized.Template,
		Subject:         normalized.Subject,
		Body:            normalized.Body,
		Metadata:        metadata,
		OutboxEventID:   pgInt8(normalized.OutboxEventID),
	})
}

func (s *NotificationService) List(ctx context.Context, userID int64, tenantID *int64, input NotificationListInput) (NotificationListResult, error) {
	if s == nil || s.queries == nil {
		return NotificationListResult{}, fmt.Errorf("notification service is not configured")
	}
	normalized, cursor, err := normalizeNotificationListInput(input)
	if err != nil {
		return NotificationListResult{}, err
	}
	tenantParam := tenantIDParam(tenantID)

	summary, err := s.queries.CountNotificationSummaryForUser(ctx, db.CountNotificationSummaryForUserParams{
		RecipientUserID: userID,
		TenantID:        tenantParam,
	})
	if err != nil {
		return NotificationListResult{}, fmt.Errorf("count notification summary: %w", err)
	}
	filteredCount, err := s.queries.CountFilteredNotificationsForUser(ctx, db.CountFilteredNotificationsForUserParams{
		RecipientUserID: userID,
		TenantID:        tenantParam,
		Q:               pgText(normalized.Query),
		ReadState:       normalized.ReadState,
		Channel:         pgText(normalized.Channel),
		CreatedAfter:    pgTimestamp(normalized.CreatedAfter),
	})
	if err != nil {
		return NotificationListResult{}, fmt.Errorf("count filtered notifications: %w", err)
	}
	limitPlusOne := normalized.Limit + 1
	rows, err := s.queries.ListNotificationsForUser(ctx, db.ListNotificationsForUserParams{
		RecipientUserID: userID,
		TenantID:        tenantParam,
		Q:               pgText(normalized.Query),
		ReadState:       normalized.ReadState,
		Channel:         pgText(normalized.Channel),
		CreatedAfter:    pgTimestamp(normalized.CreatedAfter),
		CursorCreatedAt: pgTimestamp(cursor.CreatedAt),
		CursorID:        pgInt8(optionalCursorID(cursor)),
		ResultLimit:     int32(limitPlusOne),
	})
	if err != nil {
		return NotificationListResult{}, fmt.Errorf("list notifications: %w", err)
	}
	items := make([]Notification, 0, len(rows))
	for _, row := range rows {
		items = append(items, notificationFromDB(row))
	}
	result := NotificationListResult{
		Items:         items,
		TotalCount:    summary.TotalCount,
		FilteredCount: filteredCount,
		UnreadCount:   summary.UnreadCount,
		ReadCount:     summary.ReadCount,
	}
	if len(result.Items) > normalized.Limit {
		next := result.Items[normalized.Limit-1]
		nextCursor, err := EncodeCreatedAtIDCursor(CreatedAtIDCursor{CreatedAt: next.CreatedAt, ID: rows[normalized.Limit-1].ID})
		if err != nil {
			return NotificationListResult{}, err
		}
		result.NextCursor = nextCursor
		result.Items = result.Items[:normalized.Limit]
	}
	return result, nil
}

func (s *NotificationService) MarkRead(ctx context.Context, userID int64, publicID string, auditCtx AuditContext) (Notification, error) {
	if s == nil || s.queries == nil {
		return Notification{}, fmt.Errorf("notification service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return Notification{}, ErrNotificationNotFound
	}
	row, err := s.queries.MarkNotificationRead(ctx, db.MarkNotificationReadParams{
		PublicID:        parsed,
		RecipientUserID: userID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Notification{}, ErrNotificationNotFound
	}
	if err != nil {
		return Notification{}, fmt.Errorf("mark notification read: %w", err)
	}
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "notification.read",
			TargetType:   "notification",
			TargetID:     row.PublicID.String(),
		})
	}
	item := notificationFromDB(row)
	s.publishNotificationEvent(ctx, "notification.read", item, userID)
	return item, nil
}

func (s *NotificationService) MarkReadMany(ctx context.Context, userID int64, publicIDs []string, auditCtx AuditContext) ([]Notification, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("notification service is not configured")
	}
	parsed, err := parseNotificationPublicIDs(publicIDs)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.MarkNotificationsReadByPublicIDs(ctx, db.MarkNotificationsReadByPublicIDsParams{
		RecipientUserID: userID,
		PublicIds:       parsed,
	})
	if err != nil {
		return nil, fmt.Errorf("mark notifications read: %w", err)
	}
	items := notificationsFromDB(rows)
	s.recordBulkRead(ctx, auditCtx, "notification.read_many", len(items))
	for _, item := range items {
		s.publishNotificationEvent(ctx, "notification.read", item, userID)
	}
	return items, nil
}

func (s *NotificationService) MarkReadAll(ctx context.Context, userID int64, tenantID *int64, input NotificationListInput, auditCtx AuditContext) ([]Notification, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("notification service is not configured")
	}
	normalized, _, err := normalizeNotificationListInput(NotificationListInput{
		Query:        input.Query,
		ReadState:    input.ReadState,
		Channel:      input.Channel,
		CreatedAfter: input.CreatedAfter,
		Limit:        defaultNotificationListLimit,
	})
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.MarkFilteredNotificationsRead(ctx, db.MarkFilteredNotificationsReadParams{
		RecipientUserID: userID,
		TenantID:        tenantIDParam(tenantID),
		Q:               pgText(normalized.Query),
		ReadState:       normalized.ReadState,
		Channel:         pgText(normalized.Channel),
		CreatedAfter:    pgTimestamp(normalized.CreatedAfter),
	})
	if err != nil {
		return nil, fmt.Errorf("mark filtered notifications read: %w", err)
	}
	items := notificationsFromDB(rows)
	s.recordBulkRead(ctx, auditCtx, "notification.read_all", len(items))
	for _, item := range items {
		s.publishNotificationEvent(ctx, "notification.read", item, userID)
	}
	return items, nil
}

func (s *NotificationService) publishNotificationEvent(ctx context.Context, eventType string, item Notification, recipientUserID int64) {
	if s == nil || s.realtime == nil || recipientUserID <= 0 {
		return
	}
	_, _ = s.realtime.Publish(ctx, RealtimeEventInput{
		TenantID:         item.TenantID,
		RecipientUserID:  recipientUserID,
		EventType:        eventType,
		ResourceType:     "notification",
		ResourcePublicID: item.PublicID,
		Payload: map[string]any{
			"notification": notificationPayload(item),
		},
	})
}

func (s *NotificationService) recordBulkRead(ctx context.Context, auditCtx AuditContext, action string, count int) {
	if s == nil || s.audit == nil || count == 0 {
		return
	}
	s.audit.RecordBestEffort(ctx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       action,
		TargetType:   "notification",
		TargetID:     "bulk",
		Metadata: map[string]any{
			"count": count,
		},
	})
}

func normalizeNotificationInput(input NotificationInput) (NotificationInput, error) {
	input.Channel = strings.ToLower(strings.TrimSpace(input.Channel))
	if input.Channel == "" {
		input.Channel = "in_app"
	}
	input.Template = strings.ToLower(strings.TrimSpace(input.Template))
	input.Subject = strings.TrimSpace(input.Subject)
	input.Body = strings.TrimSpace(input.Body)
	if input.RecipientUserID <= 0 || input.Template == "" {
		return NotificationInput{}, fmt.Errorf("%w: recipient and template are required", ErrInvalidNotification)
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	return input, nil
}

func normalizeNotificationListInput(input NotificationListInput) (NotificationListInput, CreatedAtIDCursor, error) {
	input.Query = strings.TrimSpace(input.Query)
	if len([]rune(input.Query)) > maxNotificationSearchLength {
		return NotificationListInput{}, CreatedAtIDCursor{}, ErrInvalidNotification
	}
	input.ReadState = strings.ToLower(strings.TrimSpace(input.ReadState))
	if input.ReadState == "" {
		input.ReadState = notificationReadStateAll
	}
	if _, ok := allowedNotificationReadStates[input.ReadState]; !ok {
		return NotificationListInput{}, CreatedAtIDCursor{}, ErrInvalidNotification
	}
	input.Channel = strings.ToLower(strings.TrimSpace(input.Channel))
	if input.Channel != "" {
		if _, ok := allowedNotificationChannels[input.Channel]; !ok {
			return NotificationListInput{}, CreatedAtIDCursor{}, ErrInvalidNotification
		}
	}
	if input.Limit <= 0 {
		input.Limit = defaultNotificationListLimit
	}
	if input.Limit > maxNotificationListLimit {
		input.Limit = maxNotificationListLimit
	}
	cursor, err := DecodeCreatedAtIDCursor(input.Cursor)
	if err != nil {
		return NotificationListInput{}, CreatedAtIDCursor{}, err
	}
	return input, cursor, nil
}

func parseNotificationPublicIDs(values []string) ([]uuid.UUID, error) {
	if len(values) == 0 || len(values) > maxNotificationListLimit {
		return nil, ErrInvalidNotification
	}
	seen := make(map[uuid.UUID]struct{}, len(values))
	parsed := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		id, err := uuid.Parse(strings.TrimSpace(value))
		if err != nil {
			return nil, ErrInvalidNotification
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		parsed = append(parsed, id)
	}
	if len(parsed) == 0 {
		return nil, ErrInvalidNotification
	}
	return parsed, nil
}

func notificationFromDB(row db.Notification) Notification {
	return Notification{
		PublicID:  row.PublicID.String(),
		TenantID:  optionalPgInt8(row.TenantID),
		Channel:   row.Channel,
		Template:  row.Template,
		Subject:   row.Subject,
		Body:      row.Body,
		Status:    row.Status,
		ReadAt:    timeFromPg(row.ReadAt),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

func notificationsFromDB(rows []db.Notification) []Notification {
	items := make([]Notification, 0, len(rows))
	for _, row := range rows {
		items = append(items, notificationFromDB(row))
	}
	return items
}

func notificationPayload(item Notification) map[string]any {
	payload := map[string]any{
		"publicId":  item.PublicID,
		"channel":   item.Channel,
		"template":  item.Template,
		"subject":   item.Subject,
		"body":      item.Body,
		"status":    item.Status,
		"createdAt": item.CreatedAt,
		"updatedAt": item.UpdatedAt,
	}
	if item.TenantID != nil {
		payload["tenantId"] = *item.TenantID
	}
	if item.ReadAt != nil {
		payload["readAt"] = item.ReadAt
	}
	return payload
}
