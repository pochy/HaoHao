package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/redis/go-redis/v9"
)

var ErrInvalidRealtimeEvent = errors.New("invalid realtime event")

const realtimeRedisChannelPrefix = "haohao:realtime:user:"

type RealtimeMetrics interface {
	IncRealtimeConnection()
	DecRealtimeConnection()
	IncRealtimeEventPublished(eventType string)
	IncRealtimeEventDelivered(eventType, transport string)
	IncRealtimePublishError(reason string)
	IncRealtimePollTimeout()
}

type RealtimeConfig struct {
	Enabled           bool
	HeartbeatInterval time.Duration
	LongPollTimeout   time.Duration
	EventRetention    time.Duration
	BackfillLimit     int
}

type RealtimeEvent struct {
	ID               int64
	Cursor           string
	PublicID         string
	TenantID         *int64
	RecipientUserID  int64
	EventType        string
	ResourceType     string
	ResourcePublicID string
	Payload          map[string]any
	CreatedAt        time.Time
}

type RealtimeEventInput struct {
	TenantID         *int64
	RecipientUserID  int64
	EventType        string
	ResourceType     string
	ResourcePublicID string
	Payload          map[string]any
}

type RealtimePublisher interface {
	Publish(ctx context.Context, input RealtimeEventInput) (RealtimeEvent, error)
}

type RealtimeService struct {
	queries *db.Queries
	redis   *redis.Client
	config  RealtimeConfig
	metrics RealtimeMetrics
}

func NewRealtimeService(queries *db.Queries, redisClient *redis.Client, config RealtimeConfig, metrics RealtimeMetrics) *RealtimeService {
	return &RealtimeService{
		queries: queries,
		redis:   redisClient,
		config:  normalizeRealtimeConfig(config),
		metrics: metrics,
	}
}

func (s *RealtimeService) Enabled() bool {
	return s != nil && s.config.Enabled
}

func (s *RealtimeService) HeartbeatInterval() time.Duration {
	if s == nil {
		return 25 * time.Second
	}
	return s.config.HeartbeatInterval
}

func (s *RealtimeService) LongPollTimeout() time.Duration {
	if s == nil {
		return 25 * time.Second
	}
	return s.config.LongPollTimeout
}

func (s *RealtimeService) BackfillLimit() int32 {
	if s == nil || s.config.BackfillLimit <= 0 {
		return 100
	}
	return int32(s.config.BackfillLimit)
}

func (s *RealtimeService) Publish(ctx context.Context, input RealtimeEventInput) (RealtimeEvent, error) {
	if s == nil || !s.config.Enabled {
		return RealtimeEvent{}, nil
	}
	if s.queries == nil {
		return RealtimeEvent{}, fmt.Errorf("realtime queries are not configured")
	}
	normalized, err := normalizeRealtimeEventInput(input)
	if err != nil {
		return RealtimeEvent{}, err
	}
	payload, err := json.Marshal(normalized.Payload)
	if err != nil {
		return RealtimeEvent{}, fmt.Errorf("encode realtime payload: %w", err)
	}
	row, err := s.queries.CreateRealtimeEvent(ctx, db.CreateRealtimeEventParams{
		TenantID:         pgInt8(normalized.TenantID),
		RecipientUserID:  normalized.RecipientUserID,
		EventType:        normalized.EventType,
		ResourceType:     normalized.ResourceType,
		ResourcePublicID: normalized.ResourcePublicID,
		Payload:          payload,
		ExpiresAt:        pgTimestamp(time.Now().Add(s.config.EventRetention)),
	})
	if err != nil {
		return RealtimeEvent{}, fmt.Errorf("create realtime event: %w", err)
	}
	item := realtimeEventFromDB(row)
	if s.metrics != nil {
		s.metrics.IncRealtimeEventPublished(item.EventType)
	}
	if s.redis != nil {
		if err := s.redis.Publish(ctx, realtimeChannelForUser(normalized.RecipientUserID), strconv.FormatInt(row.ID, 10)).Err(); err != nil && s.metrics != nil {
			s.metrics.IncRealtimePublishError("redis_publish")
		}
	}
	return item, nil
}

func (s *RealtimeService) CurrentCursor(ctx context.Context, userID int64, tenantID *int64) (int64, error) {
	if s == nil || s.queries == nil {
		return 0, fmt.Errorf("realtime service is not configured")
	}
	return s.queries.GetRealtimeCurrentCursor(ctx, db.GetRealtimeCurrentCursorParams{
		RecipientUserID: userID,
		TenantID:        tenantIDParam(tenantID),
	})
}

func (s *RealtimeService) ListAfter(ctx context.Context, userID int64, tenantID *int64, afterID int64, limit int32) ([]RealtimeEvent, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("realtime service is not configured")
	}
	if limit <= 0 {
		limit = s.BackfillLimit()
	}
	rows, err := s.queries.ListRealtimeEventsAfterCursor(ctx, db.ListRealtimeEventsAfterCursorParams{
		RecipientUserID: userID,
		AfterID:         afterID,
		TenantID:        tenantIDParam(tenantID),
		LimitCount:      limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list realtime events: %w", err)
	}
	items := make([]RealtimeEvent, 0, len(rows))
	for _, row := range rows {
		items = append(items, realtimeEventFromDB(row))
	}
	return items, nil
}

func (s *RealtimeService) Subscribe(ctx context.Context, userID int64) *redis.PubSub {
	if s == nil || s.redis == nil || userID <= 0 {
		return nil
	}
	return s.redis.Subscribe(ctx, realtimeChannelForUser(userID))
}

func (s *RealtimeService) IncConnection() {
	if s != nil && s.metrics != nil {
		s.metrics.IncRealtimeConnection()
	}
}

func (s *RealtimeService) DecConnection() {
	if s != nil && s.metrics != nil {
		s.metrics.DecRealtimeConnection()
	}
}

func (s *RealtimeService) IncDelivered(eventType, transport string) {
	if s != nil && s.metrics != nil {
		s.metrics.IncRealtimeEventDelivered(eventType, transport)
	}
}

func (s *RealtimeService) IncPollTimeout() {
	if s != nil && s.metrics != nil {
		s.metrics.IncRealtimePollTimeout()
	}
}

func RealtimeCursor(id int64) string {
	if id <= 0 {
		return "0"
	}
	return strconv.FormatInt(id, 10)
}

func ParseRealtimeCursor(value string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || id < 0 {
		return 0, ErrInvalidRealtimeEvent
	}
	return id, nil
}

func normalizeRealtimeConfig(config RealtimeConfig) RealtimeConfig {
	if config.HeartbeatInterval <= 0 {
		config.HeartbeatInterval = 25 * time.Second
	}
	if config.LongPollTimeout <= 0 {
		config.LongPollTimeout = 25 * time.Second
	}
	if config.EventRetention <= 0 {
		config.EventRetention = 168 * time.Hour
	}
	if config.BackfillLimit <= 0 {
		config.BackfillLimit = 100
	}
	if config.BackfillLimit > 500 {
		config.BackfillLimit = 500
	}
	return config
}

func normalizeRealtimeEventInput(input RealtimeEventInput) (RealtimeEventInput, error) {
	input.EventType = strings.TrimSpace(input.EventType)
	input.ResourceType = strings.TrimSpace(input.ResourceType)
	input.ResourcePublicID = strings.TrimSpace(input.ResourcePublicID)
	if input.RecipientUserID <= 0 || input.EventType == "" {
		return RealtimeEventInput{}, ErrInvalidRealtimeEvent
	}
	if input.Payload == nil {
		input.Payload = map[string]any{}
	}
	return input, nil
}

func realtimeEventFromDB(row db.RealtimeEvent) RealtimeEvent {
	payload := map[string]any{}
	if len(row.Payload) > 0 {
		_ = json.Unmarshal(row.Payload, &payload)
	}
	return RealtimeEvent{
		ID:               row.ID,
		Cursor:           RealtimeCursor(row.ID),
		PublicID:         row.PublicID.String(),
		TenantID:         optionalPgInt8(row.TenantID),
		RecipientUserID:  row.RecipientUserID,
		EventType:        row.EventType,
		ResourceType:     row.ResourceType,
		ResourcePublicID: row.ResourcePublicID,
		Payload:          payload,
		CreatedAt:        row.CreatedAt.Time,
	}
}

func tenantIDParam(tenantID *int64) int64 {
	if tenantID == nil {
		return 0
	}
	return *tenantID
}

func realtimeChannelForUser(userID int64) string {
	return realtimeRedisChannelPrefix + strconv.FormatInt(userID, 10)
}
