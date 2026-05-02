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

func (s *NotificationService) List(ctx context.Context, userID int64, tenantID *int64, limit int) ([]Notification, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("notification service is not configured")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	filterTenantID := int64(0)
	if tenantID != nil {
		filterTenantID = *tenantID
	}
	rows, err := s.queries.ListNotificationsForUser(ctx, db.ListNotificationsForUserParams{
		RecipientUserID: userID,
		Column2:         filterTenantID,
		Limit:           int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	items := make([]Notification, 0, len(rows))
	for _, row := range rows {
		items = append(items, notificationFromDB(row))
	}
	return items, nil
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
