package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"example.com/haohao/backend/internal/auth"
	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrWebhookNotFound          = errors.New("webhook not found")
	ErrWebhookDeliveryNotFound  = errors.New("webhook delivery not found")
	ErrInvalidWebhookInput      = errors.New("invalid webhook input")
	ErrWebhookEntitlementDenied = errors.New("webhook entitlement denied")
	ErrWebhookSecretUnavailable = errors.New("webhook secret encryption key is not configured")
	ErrWebhookDeliveryFailed    = errors.New("webhook delivery failed")
)

const (
	FeatureWebhooksEnabled = "webhooks.enabled"

	defaultWebhookMaxAttempts = 8
)

var allowedWebhookEventTypes = map[string]struct{}{
	"customer_signal.created": {},
	"customer_signal.updated": {},
	"customer_signal.deleted": {},
}

type WebhookEndpoint struct {
	ID              int64
	PublicID        string
	TenantID        int64
	Name            string
	URL             string
	EventTypes      []string
	Active          bool
	SecretPlaintext string
	LastDeliveryAt  *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type WebhookDelivery struct {
	ID              int64
	PublicID        string
	TenantID        int64
	EndpointID      int64
	EventType       string
	Status          string
	AttemptCount    int32
	MaxAttempts     int32
	LastHTTPStatus  *int32
	LastError       string
	ResponsePreview string
	DeliveredAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type WebhookEndpointInput struct {
	Name       string
	URL        string
	EventTypes []string
	Active     *bool
}

type WebhookService struct {
	queries      *db.Queries
	outbox       *OutboxService
	entitlements *EntitlementService
	audit        AuditRecorder
	secretBox    *auth.SecretBox
	httpClient   *http.Client
}

func NewWebhookService(queries *db.Queries, outbox *OutboxService, entitlements *EntitlementService, audit AuditRecorder, secretBox *auth.SecretBox, httpTimeout time.Duration) *WebhookService {
	if httpTimeout <= 0 {
		httpTimeout = 10 * time.Second
	}
	return &WebhookService{
		queries:      queries,
		outbox:       outbox,
		entitlements: entitlements,
		audit:        audit,
		secretBox:    secretBox,
		httpClient:   &http.Client{Timeout: httpTimeout},
	}
}

func (s *WebhookService) List(ctx context.Context, tenantID int64) ([]WebhookEndpoint, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("webhook service is not configured")
	}
	rows, err := s.queries.ListWebhookEndpoints(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	items := make([]WebhookEndpoint, 0, len(rows))
	for _, row := range rows {
		items = append(items, webhookEndpointFromDB(row, ""))
	}
	return items, nil
}

func (s *WebhookService) Get(ctx context.Context, tenantID int64, publicID string) (WebhookEndpoint, error) {
	row, err := s.getEndpointRow(ctx, tenantID, publicID)
	if err != nil {
		return WebhookEndpoint{}, err
	}
	return webhookEndpointFromDB(row, ""), nil
}

func (s *WebhookService) Create(ctx context.Context, tenantID, userID int64, input WebhookEndpointInput, auditCtx AuditContext) (WebhookEndpoint, error) {
	if s == nil || s.queries == nil {
		return WebhookEndpoint{}, fmt.Errorf("webhook service is not configured")
	}
	if err := s.requireEnabled(ctx, tenantID); err != nil {
		return WebhookEndpoint{}, err
	}
	if s.secretBox == nil {
		return WebhookEndpoint{}, ErrWebhookSecretUnavailable
	}
	normalized, err := normalizeWebhookInput(input, true)
	if err != nil {
		return WebhookEndpoint{}, err
	}
	secret, err := generateWebhookSecret()
	if err != nil {
		return WebhookEndpoint{}, err
	}
	ciphertext, err := s.secretBox.Seal(secret)
	if err != nil {
		return WebhookEndpoint{}, err
	}
	row, err := s.queries.CreateWebhookEndpoint(ctx, db.CreateWebhookEndpointParams{
		TenantID:         tenantID,
		CreatedByUserID:  pgtype.Int8{Int64: userID, Valid: userID > 0},
		Name:             normalized.Name,
		Url:              normalized.URL,
		EventTypes:       normalized.EventTypes,
		SecretCiphertext: ciphertext,
		SecretKeyVersion: int32(s.secretBox.KeyVersion()),
	})
	if err != nil {
		return WebhookEndpoint{}, fmt.Errorf("create webhook endpoint: %w", err)
	}
	if s.audit != nil {
		auditCtx.TenantID = &tenantID
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "webhook.create",
			TargetType:   "webhook_endpoint",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"eventTypes": row.EventTypes,
			},
		})
	}
	return webhookEndpointFromDB(row, secret), nil
}

func (s *WebhookService) Update(ctx context.Context, tenantID int64, publicID string, input WebhookEndpointInput, auditCtx AuditContext) (WebhookEndpoint, error) {
	if err := s.requireEnabled(ctx, tenantID); err != nil {
		return WebhookEndpoint{}, err
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return WebhookEndpoint{}, ErrWebhookNotFound
	}
	normalized, err := normalizeWebhookInput(input, false)
	if err != nil {
		return WebhookEndpoint{}, err
	}
	row, err := s.queries.UpdateWebhookEndpoint(ctx, db.UpdateWebhookEndpointParams{
		PublicID:   parsed,
		TenantID:   tenantID,
		Name:       normalized.Name,
		Url:        normalized.URL,
		EventTypes: normalized.EventTypes,
		Active:     *normalized.Active,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return WebhookEndpoint{}, ErrWebhookNotFound
	}
	if err != nil {
		return WebhookEndpoint{}, fmt.Errorf("update webhook endpoint: %w", err)
	}
	if s.audit != nil {
		auditCtx.TenantID = &tenantID
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "webhook.update",
			TargetType:   "webhook_endpoint",
			TargetID:     row.PublicID.String(),
		})
	}
	return webhookEndpointFromDB(row, ""), nil
}

func (s *WebhookService) RotateSecret(ctx context.Context, tenantID int64, publicID string, auditCtx AuditContext) (WebhookEndpoint, error) {
	if err := s.requireEnabled(ctx, tenantID); err != nil {
		return WebhookEndpoint{}, err
	}
	if s.secretBox == nil {
		return WebhookEndpoint{}, ErrWebhookSecretUnavailable
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return WebhookEndpoint{}, ErrWebhookNotFound
	}
	secret, err := generateWebhookSecret()
	if err != nil {
		return WebhookEndpoint{}, err
	}
	ciphertext, err := s.secretBox.Seal(secret)
	if err != nil {
		return WebhookEndpoint{}, err
	}
	row, err := s.queries.RotateWebhookSecret(ctx, db.RotateWebhookSecretParams{
		PublicID:         parsed,
		TenantID:         tenantID,
		SecretCiphertext: ciphertext,
		SecretKeyVersion: int32(s.secretBox.KeyVersion()),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return WebhookEndpoint{}, ErrWebhookNotFound
	}
	if err != nil {
		return WebhookEndpoint{}, fmt.Errorf("rotate webhook secret: %w", err)
	}
	if s.audit != nil {
		auditCtx.TenantID = &tenantID
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "webhook.secret_rotate",
			TargetType:   "webhook_endpoint",
			TargetID:     row.PublicID.String(),
		})
	}
	return webhookEndpointFromDB(row, secret), nil
}

func (s *WebhookService) Delete(ctx context.Context, tenantID int64, publicID string, auditCtx AuditContext) error {
	if err := s.requireEnabled(ctx, tenantID); err != nil {
		return err
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return ErrWebhookNotFound
	}
	row, err := s.queries.SoftDeleteWebhookEndpoint(ctx, db.SoftDeleteWebhookEndpointParams{
		PublicID: parsed,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrWebhookNotFound
	}
	if err != nil {
		return fmt.Errorf("delete webhook endpoint: %w", err)
	}
	if s.audit != nil {
		auditCtx.TenantID = &tenantID
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "webhook.delete",
			TargetType:   "webhook_endpoint",
			TargetID:     row.PublicID.String(),
		})
	}
	return nil
}

func (s *WebhookService) ListDeliveries(ctx context.Context, tenantID int64, webhookPublicID string, limit int) ([]WebhookDelivery, error) {
	endpoint, err := s.getEndpointRow(ctx, tenantID, webhookPublicID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.queries.ListWebhookDeliveriesForEndpoint(ctx, db.ListWebhookDeliveriesForEndpointParams{
		TenantID:          tenantID,
		WebhookEndpointID: endpoint.ID,
		Limit:             int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list webhook deliveries: %w", err)
	}
	items := make([]WebhookDelivery, 0, len(rows))
	for _, row := range rows {
		items = append(items, webhookDeliveryFromDB(row))
	}
	return items, nil
}

func (s *WebhookService) EnqueueTenantEventWithQueries(ctx context.Context, queries *db.Queries, tenantID int64, eventType string, payload any) error {
	if s == nil || queries == nil || s.outbox == nil {
		return nil
	}
	if s.entitlements == nil {
		return nil
	}
	enabled, err := s.entitlements.IsEnabledWithQueries(ctx, queries, tenantID, FeatureWebhooksEnabled)
	if err != nil || !enabled {
		return err
	}
	normalizedEventType := normalizeWebhookEventType(eventType)
	if _, ok := allowedWebhookEventTypes[normalizedEventType]; !ok {
		return ErrInvalidWebhookInput
	}
	endpoints, err := queries.ListActiveWebhookEndpointsForEvent(ctx, db.ListActiveWebhookEndpointsForEventParams{
		TenantID:  tenantID,
		EventType: normalizedEventType,
	})
	if err != nil {
		return fmt.Errorf("list active webhook endpoints: %w", err)
	}
	if len(endpoints) == 0 {
		return nil
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode webhook payload: %w", err)
	}
	for _, endpoint := range endpoints {
		delivery, err := queries.CreateWebhookDelivery(ctx, db.CreateWebhookDeliveryParams{
			TenantID:          tenantID,
			WebhookEndpointID: endpoint.ID,
			EventType:         normalizedEventType,
			Payload:           payloadBytes,
			MaxAttempts:       defaultWebhookMaxAttempts,
		})
		if err != nil {
			return fmt.Errorf("create webhook delivery: %w", err)
		}
		event, err := s.outbox.EnqueueWithQueries(ctx, queries, OutboxEventInput{
			TenantID:      &tenantID,
			AggregateType: "webhook_delivery",
			AggregateID:   delivery.PublicID.String(),
			EventType:     "webhook.delivery_requested",
			Payload: map[string]any{
				"deliveryId": delivery.ID,
				"tenantId":   tenantID,
			},
		})
		if err != nil {
			return err
		}
		if _, err := queries.SetWebhookDeliveryOutboxEvent(ctx, db.SetWebhookDeliveryOutboxEventParams{
			ID:            delivery.ID,
			OutboxEventID: pgtype.Int8{Int64: event.ID, Valid: true},
		}); err != nil {
			return fmt.Errorf("set webhook delivery outbox event: %w", err)
		}
	}
	return nil
}

func (s *WebhookService) Deliver(ctx context.Context, deliveryID int64) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("webhook service is not configured")
	}
	if s.secretBox == nil {
		return ErrWebhookSecretUnavailable
	}
	row, err := s.queries.GetWebhookDeliveryByID(ctx, deliveryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrWebhookDeliveryNotFound
	}
	if err != nil {
		return fmt.Errorf("get webhook delivery: %w", err)
	}
	if row.Status == "delivered" || row.Status == "dead" {
		return nil
	}
	if !row.EndpointActive {
		return s.markDeliveryFailed(ctx, row.ID, 0, "webhook endpoint is inactive", "")
	}
	secret, err := s.secretBox.Open(row.EndpointSecretCiphertext)
	if err != nil {
		return err
	}
	body, err := buildWebhookRequestBody(row)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, row.EndpointUrl, bytes.NewReader(body))
	if err != nil {
		return s.markDeliveryFailed(ctx, row.ID, 0, err.Error(), "")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "HaoHao-Webhooks/1.0")
	req.Header.Set("X-HaoHao-Event-ID", row.PublicID.String())
	req.Header.Set("X-HaoHao-Signature", BuildWebhookSignature(secret, now, body))
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return s.markDeliveryFailed(ctx, row.ID, 0, err.Error(), "")
	}
	defer resp.Body.Close()
	previewBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1000))
	preview := string(previewBytes)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if _, err := s.queries.MarkWebhookDeliveryDelivered(ctx, db.MarkWebhookDeliveryDeliveredParams{
			ID:             row.ID,
			LastHttpStatus: pgtype.Int4{Int32: int32(resp.StatusCode), Valid: true},
			Left:           preview,
		}); err != nil {
			return fmt.Errorf("mark webhook delivered: %w", err)
		}
		_, _ = s.queries.TouchWebhookEndpointDelivery(ctx, row.WebhookEndpointID)
		return nil
	}
	return s.markDeliveryFailed(ctx, row.ID, resp.StatusCode, fmt.Sprintf("webhook returned %d", resp.StatusCode), preview)
}

func (s *WebhookService) RetryDelivery(ctx context.Context, tenantID int64, webhookPublicID, deliveryPublicID string, auditCtx AuditContext) (WebhookDelivery, error) {
	if err := s.requireEnabled(ctx, tenantID); err != nil {
		return WebhookDelivery{}, err
	}
	endpoint, err := s.getEndpointRow(ctx, tenantID, webhookPublicID)
	if err != nil {
		return WebhookDelivery{}, err
	}
	parsed, err := uuid.Parse(strings.TrimSpace(deliveryPublicID))
	if err != nil {
		return WebhookDelivery{}, ErrWebhookDeliveryNotFound
	}
	delivery, err := s.queries.ResetWebhookDeliveryForRetry(ctx, db.ResetWebhookDeliveryForRetryParams{
		PublicID: parsed,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) || delivery.WebhookEndpointID != endpoint.ID {
		return WebhookDelivery{}, ErrWebhookDeliveryNotFound
	}
	if err != nil {
		return WebhookDelivery{}, fmt.Errorf("reset webhook delivery: %w", err)
	}
	event, err := s.outbox.Enqueue(ctx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "webhook_delivery",
		AggregateID:   delivery.PublicID.String(),
		EventType:     "webhook.delivery_requested",
		Payload: map[string]any{
			"deliveryId": delivery.ID,
			"tenantId":   tenantID,
		},
	})
	if err != nil {
		return WebhookDelivery{}, err
	}
	delivery, err = s.queries.SetWebhookDeliveryOutboxEvent(ctx, db.SetWebhookDeliveryOutboxEventParams{
		ID:            delivery.ID,
		OutboxEventID: pgtype.Int8{Int64: event.ID, Valid: true},
	})
	if err != nil {
		return WebhookDelivery{}, err
	}
	if s.audit != nil {
		auditCtx.TenantID = &tenantID
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "webhook.delivery_retry",
			TargetType:   "webhook_delivery",
			TargetID:     delivery.PublicID.String(),
		})
	}
	return webhookDeliveryFromDB(delivery), nil
}

func (s *WebhookService) requireEnabled(ctx context.Context, tenantID int64) error {
	if s == nil || s.entitlements == nil {
		return ErrWebhookEntitlementDenied
	}
	enabled, err := s.entitlements.IsEnabled(ctx, tenantID, FeatureWebhooksEnabled)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrWebhookEntitlementDenied
	}
	return nil
}

func (s *WebhookService) getEndpointRow(ctx context.Context, tenantID int64, publicID string) (db.WebhookEndpoint, error) {
	if s == nil || s.queries == nil {
		return db.WebhookEndpoint{}, fmt.Errorf("webhook service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return db.WebhookEndpoint{}, ErrWebhookNotFound
	}
	row, err := s.queries.GetWebhookEndpointForTenant(ctx, db.GetWebhookEndpointForTenantParams{
		PublicID: parsed,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.WebhookEndpoint{}, ErrWebhookNotFound
	}
	if err != nil {
		return db.WebhookEndpoint{}, fmt.Errorf("get webhook endpoint: %w", err)
	}
	return row, nil
}

func (s *WebhookService) markDeliveryFailed(ctx context.Context, deliveryID int64, status int, message, preview string) error {
	backoff := 5 * time.Second
	row, err := s.queries.MarkWebhookDeliveryFailed(ctx, db.MarkWebhookDeliveryFailedParams{
		ID: deliveryID,
		LastHttpStatus: pgtype.Int4{
			Int32: int32(status),
			Valid: status > 0,
		},
		Left:    message,
		Left_2:  preview,
		Column5: pgtype.Interval{Microseconds: backoff.Microseconds(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("mark webhook failed: %w", err)
	}
	if row.Status == "dead" {
		return nil
	}
	return ErrWebhookDeliveryFailed
}

func buildWebhookRequestBody(row db.GetWebhookDeliveryByIDRow) ([]byte, error) {
	var data any = map[string]any{}
	if len(row.Payload) > 0 {
		if err := json.Unmarshal(row.Payload, &data); err != nil {
			return nil, err
		}
	}
	body := map[string]any{
		"eventId":   row.PublicID.String(),
		"type":      row.EventType,
		"tenantId":  row.TenantID,
		"createdAt": timestamptzTime(row.CreatedAt).UTC().Format(time.RFC3339Nano),
		"data":      data,
	}
	return json.Marshal(body)
}

func normalizeWebhookInput(input WebhookEndpointInput, create bool) (WebhookEndpointInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimSpace(input.URL)
	if input.Active == nil {
		active := true
		input.Active = &active
	}
	if input.Name == "" || len([]rune(input.Name)) > 120 || input.URL == "" || len([]rune(input.URL)) > 1000 {
		return WebhookEndpointInput{}, ErrInvalidWebhookInput
	}
	parsed, err := url.Parse(input.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return WebhookEndpointInput{}, ErrInvalidWebhookInput
	}
	events := make(map[string]struct{})
	for _, item := range input.EventTypes {
		eventType := normalizeWebhookEventType(item)
		if _, ok := allowedWebhookEventTypes[eventType]; !ok {
			return WebhookEndpointInput{}, ErrInvalidWebhookInput
		}
		events[eventType] = struct{}{}
	}
	if len(events) == 0 && create {
		events["customer_signal.created"] = struct{}{}
	}
	input.EventTypes = make([]string, 0, len(events))
	for eventType := range events {
		input.EventTypes = append(input.EventTypes, eventType)
	}
	sort.Strings(input.EventTypes)
	if len(input.EventTypes) == 0 {
		return WebhookEndpointInput{}, ErrInvalidWebhookInput
	}
	return input, nil
}

func normalizeWebhookEventType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func generateWebhookSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "whsec_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func webhookEndpointFromDB(row db.WebhookEndpoint, secret string) WebhookEndpoint {
	return WebhookEndpoint{
		ID:              row.ID,
		PublicID:        row.PublicID.String(),
		TenantID:        row.TenantID,
		Name:            row.Name,
		URL:             row.Url,
		EventTypes:      append([]string{}, row.EventTypes...),
		Active:          row.Active,
		SecretPlaintext: secret,
		LastDeliveryAt:  timeFromPg(row.LastDeliveryAt),
		CreatedAt:       timestamptzTime(row.CreatedAt),
		UpdatedAt:       timestamptzTime(row.UpdatedAt),
	}
}

func webhookDeliveryFromDB(row db.WebhookDelivery) WebhookDelivery {
	return WebhookDelivery{
		ID:              row.ID,
		PublicID:        row.PublicID.String(),
		TenantID:        row.TenantID,
		EndpointID:      row.WebhookEndpointID,
		EventType:       row.EventType,
		Status:          row.Status,
		AttemptCount:    row.AttemptCount,
		MaxAttempts:     row.MaxAttempts,
		LastHTTPStatus:  optionalPgInt4(row.LastHttpStatus),
		LastError:       optionalText(row.LastError),
		ResponsePreview: optionalText(row.ResponsePreview),
		DeliveredAt:     timeFromPg(row.DeliveredAt),
		CreatedAt:       timestamptzTime(row.CreatedAt),
		UpdatedAt:       timestamptzTime(row.UpdatedAt),
	}
}
