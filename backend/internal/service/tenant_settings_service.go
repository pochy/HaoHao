package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrInvalidTenantSettings = errors.New("invalid tenant settings")

type TenantSettings struct {
	TenantID                      int64
	FileQuotaBytes                int64
	RateLimitLoginPerMinute       *int32
	RateLimitBrowserAPIPerMinute  *int32
	RateLimitExternalAPIPerMinute *int32
	NotificationsEnabled          bool
	Features                      map[string]any
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
}

type TenantSettingsInput struct {
	FileQuotaBytes                int64
	RateLimitLoginPerMinute       *int32
	RateLimitBrowserAPIPerMinute  *int32
	RateLimitExternalAPIPerMinute *int32
	NotificationsEnabled          bool
	Features                      map[string]any
}

type RateLimitDefaults struct {
	LoginPerMinute       int
	BrowserAPIPerMinute  int
	ExternalAPIPerMinute int
}

type DrivePolicy struct {
	LinkSharingEnabled         bool
	AnonymousLinksEnabled      bool
	LinkExpiresRequired        bool
	MaxLinkTTLHours            int
	ViewerDownloadEnabled      bool
	ShareLinkDownloadEnabled   bool
	EditorCanReshare           bool
	EditorCanDelete            bool
	ExternalUserSharingEnabled bool
}

type TenantSettingsService struct {
	queries               *db.Queries
	audit                 AuditRecorder
	defaultFileQuotaBytes int64
}

func NewTenantSettingsService(queries *db.Queries, audit AuditRecorder, defaultFileQuotaBytes int64) *TenantSettingsService {
	if defaultFileQuotaBytes <= 0 {
		defaultFileQuotaBytes = 100 * 1024 * 1024
	}
	return &TenantSettingsService{
		queries:               queries,
		audit:                 audit,
		defaultFileQuotaBytes: defaultFileQuotaBytes,
	}
}

func (s *TenantSettingsService) Get(ctx context.Context, tenantID int64) (TenantSettings, error) {
	if s == nil || s.queries == nil {
		return TenantSettings{}, fmt.Errorf("tenant settings service is not configured")
	}
	row, err := s.queries.GetTenantSettings(ctx, tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.defaultSettings(tenantID), nil
	}
	if err != nil {
		return TenantSettings{}, fmt.Errorf("get tenant settings: %w", err)
	}
	return tenantSettingsFromDB(row), nil
}

func (s *TenantSettingsService) Update(ctx context.Context, tenantID int64, input TenantSettingsInput, auditCtx AuditContext) (TenantSettings, error) {
	if s == nil || s.queries == nil {
		return TenantSettings{}, fmt.Errorf("tenant settings service is not configured")
	}
	normalized, err := normalizeTenantSettingsInput(input, s.defaultFileQuotaBytes)
	if err != nil {
		return TenantSettings{}, err
	}
	features, err := json.Marshal(normalized.Features)
	if err != nil {
		return TenantSettings{}, fmt.Errorf("encode tenant settings features: %w", err)
	}
	row, err := s.queries.UpsertTenantSettings(ctx, db.UpsertTenantSettingsParams{
		TenantID:                      tenantID,
		FileQuotaBytes:                normalized.FileQuotaBytes,
		RateLimitLoginPerMinute:       pgOptionalInt4(normalized.RateLimitLoginPerMinute),
		RateLimitBrowserApiPerMinute:  pgOptionalInt4(normalized.RateLimitBrowserAPIPerMinute),
		RateLimitExternalApiPerMinute: pgOptionalInt4(normalized.RateLimitExternalAPIPerMinute),
		NotificationsEnabled:          normalized.NotificationsEnabled,
		Features:                      features,
	})
	if err != nil {
		return TenantSettings{}, fmt.Errorf("upsert tenant settings: %w", err)
	}
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "tenant_settings.update",
			TargetType:   "tenant",
			TargetID:     fmt.Sprintf("%d", tenantID),
			Metadata: map[string]any{
				"fileQuotaBytes": normalized.FileQuotaBytes,
			},
		})
	}
	return tenantSettingsFromDB(row), nil
}

func (s *TenantSettingsService) ResolveEffectiveRateLimit(ctx context.Context, tenantID int64, policy string, defaults RateLimitDefaults) (int, error) {
	settings, err := s.Get(ctx, tenantID)
	if err != nil {
		return defaultRateLimitForPolicy(policy, defaults), err
	}
	return effectiveRateLimitForSettings(settings, policy, defaults), nil
}

func (s *TenantSettingsService) GetDrivePolicy(ctx context.Context, tenantID int64) (DrivePolicy, error) {
	settings, err := s.Get(ctx, tenantID)
	if err != nil {
		return defaultDrivePolicy(), err
	}
	return drivePolicyFromFeatures(settings.Features), nil
}

func (s *TenantSettingsService) UpdateDrivePolicy(ctx context.Context, tenantID int64, input DrivePolicy, auditCtx AuditContext) (DrivePolicy, error) {
	settings, err := s.Get(ctx, tenantID)
	if err != nil {
		return DrivePolicy{}, err
	}
	policy := normalizeDrivePolicy(input)
	features := cloneFeatureMap(settings.Features)
	features["drive"] = drivePolicyToFeatureMap(policy)

	_, err = s.Update(ctx, tenantID, TenantSettingsInput{
		FileQuotaBytes:                settings.FileQuotaBytes,
		RateLimitLoginPerMinute:       settings.RateLimitLoginPerMinute,
		RateLimitBrowserAPIPerMinute:  settings.RateLimitBrowserAPIPerMinute,
		RateLimitExternalAPIPerMinute: settings.RateLimitExternalAPIPerMinute,
		NotificationsEnabled:          settings.NotificationsEnabled,
		Features:                      features,
	}, auditCtx)
	if err != nil {
		return DrivePolicy{}, err
	}
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "tenant_settings.drive_policy.update",
			TargetType:   "tenant",
			TargetID:     fmt.Sprintf("%d", tenantID),
		})
	}
	return policy, nil
}

func effectiveRateLimitForSettings(settings TenantSettings, policy string, defaults RateLimitDefaults) int {
	switch policy {
	case "login":
		if settings.RateLimitLoginPerMinute != nil {
			return int(*settings.RateLimitLoginPerMinute)
		}
	case "browser_api":
		if settings.RateLimitBrowserAPIPerMinute != nil {
			return int(*settings.RateLimitBrowserAPIPerMinute)
		}
	case "external_api":
		if settings.RateLimitExternalAPIPerMinute != nil {
			return int(*settings.RateLimitExternalAPIPerMinute)
		}
	}

	return defaultRateLimitForPolicy(policy, defaults)
}

func (s *TenantSettingsService) CheckFileQuota(ctx context.Context, tenantID int64, incomingBytes int64) (bool, int64, int64, error) {
	settings, err := s.Get(ctx, tenantID)
	if err != nil {
		return false, 0, 0, err
	}
	used, err := s.queries.SumActiveFileBytesForTenant(ctx, tenantID)
	if err != nil {
		return false, 0, 0, fmt.Errorf("sum tenant file bytes: %w", err)
	}
	return used+incomingBytes <= settings.FileQuotaBytes, used, settings.FileQuotaBytes, nil
}

func (s *TenantSettingsService) defaultSettings(tenantID int64) TenantSettings {
	now := time.Now()
	return TenantSettings{
		TenantID:             tenantID,
		FileQuotaBytes:       s.defaultFileQuotaBytes,
		NotificationsEnabled: true,
		Features:             map[string]any{},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func normalizeTenantSettingsInput(input TenantSettingsInput, defaultFileQuotaBytes int64) (TenantSettingsInput, error) {
	if input.FileQuotaBytes <= 0 {
		input.FileQuotaBytes = defaultFileQuotaBytes
	}
	if input.FileQuotaBytes < 0 {
		return TenantSettingsInput{}, fmt.Errorf("%w: file quota must be non-negative", ErrInvalidTenantSettings)
	}
	for _, value := range []*int32{input.RateLimitLoginPerMinute, input.RateLimitBrowserAPIPerMinute, input.RateLimitExternalAPIPerMinute} {
		if value != nil && *value <= 0 {
			return TenantSettingsInput{}, fmt.Errorf("%w: rate limit override must be positive", ErrInvalidTenantSettings)
		}
	}
	if input.Features == nil {
		input.Features = map[string]any{}
	}
	return input, nil
}

func defaultRateLimitForPolicy(policy string, defaults RateLimitDefaults) int {
	switch policy {
	case "login":
		return defaults.LoginPerMinute
	case "external_api":
		return defaults.ExternalAPIPerMinute
	default:
		return defaults.BrowserAPIPerMinute
	}
}

func defaultDrivePolicy() DrivePolicy {
	return DrivePolicy{
		LinkSharingEnabled:         true,
		AnonymousLinksEnabled:      true,
		LinkExpiresRequired:        true,
		MaxLinkTTLHours:            720,
		ViewerDownloadEnabled:      true,
		ShareLinkDownloadEnabled:   true,
		EditorCanReshare:           false,
		EditorCanDelete:            false,
		ExternalUserSharingEnabled: false,
	}
}

func drivePolicyFromFeatures(features map[string]any) DrivePolicy {
	policy := defaultDrivePolicy()
	raw, ok := features["drive"].(map[string]any)
	if !ok {
		return policy
	}

	policy.LinkSharingEnabled = featureBool(raw, "linkSharingEnabled", policy.LinkSharingEnabled)
	policy.AnonymousLinksEnabled = featureBool(raw, "anonymousLinksEnabled", policy.AnonymousLinksEnabled)
	policy.LinkExpiresRequired = featureBool(raw, "linkExpiresRequired", policy.LinkExpiresRequired)
	policy.MaxLinkTTLHours = featureInt(raw, "maxLinkTtlHours", policy.MaxLinkTTLHours)
	policy.ViewerDownloadEnabled = featureBool(raw, "viewerDownloadEnabled", policy.ViewerDownloadEnabled)
	policy.ShareLinkDownloadEnabled = featureBool(raw, "shareLinkDownloadEnabled", policy.ShareLinkDownloadEnabled)
	policy.EditorCanReshare = featureBool(raw, "editorCanReshare", policy.EditorCanReshare)
	policy.EditorCanDelete = featureBool(raw, "editorCanDelete", policy.EditorCanDelete)
	policy.ExternalUserSharingEnabled = featureBool(raw, "externalUserSharingEnabled", policy.ExternalUserSharingEnabled)

	return normalizeDrivePolicy(policy)
}

func normalizeDrivePolicy(policy DrivePolicy) DrivePolicy {
	if policy.MaxLinkTTLHours <= 0 {
		policy.MaxLinkTTLHours = defaultDrivePolicy().MaxLinkTTLHours
	}
	return policy
}

func drivePolicyToFeatureMap(policy DrivePolicy) map[string]any {
	policy = normalizeDrivePolicy(policy)
	return map[string]any{
		"linkSharingEnabled":         policy.LinkSharingEnabled,
		"anonymousLinksEnabled":      policy.AnonymousLinksEnabled,
		"linkExpiresRequired":        policy.LinkExpiresRequired,
		"maxLinkTtlHours":            policy.MaxLinkTTLHours,
		"viewerDownloadEnabled":      policy.ViewerDownloadEnabled,
		"shareLinkDownloadEnabled":   policy.ShareLinkDownloadEnabled,
		"editorCanReshare":           policy.EditorCanReshare,
		"editorCanDelete":            policy.EditorCanDelete,
		"externalUserSharingEnabled": policy.ExternalUserSharingEnabled,
	}
}

func cloneFeatureMap(features map[string]any) map[string]any {
	cloned := make(map[string]any, len(features)+1)
	for key, value := range features {
		cloned[key] = value
	}
	return cloned
}

func featureBool(values map[string]any, key string, fallback bool) bool {
	value, ok := values[key].(bool)
	if !ok {
		return fallback
	}
	return value
}

func featureInt(values map[string]any, key string, fallback int) int {
	switch value := values[key].(type) {
	case int:
		if value > 0 {
			return value
		}
	case int32:
		if value > 0 {
			return int(value)
		}
	case int64:
		if value > 0 {
			return int(value)
		}
	case float64:
		if value > 0 {
			return int(value)
		}
	}
	return fallback
}

func tenantSettingsFromDB(row db.TenantSetting) TenantSettings {
	features := map[string]any{}
	if len(row.Features) > 0 {
		_ = json.Unmarshal(row.Features, &features)
	}
	return TenantSettings{
		TenantID:                      row.TenantID,
		FileQuotaBytes:                row.FileQuotaBytes,
		RateLimitLoginPerMinute:       optionalPgInt4(row.RateLimitLoginPerMinute),
		RateLimitBrowserAPIPerMinute:  optionalPgInt4(row.RateLimitBrowserApiPerMinute),
		RateLimitExternalAPIPerMinute: optionalPgInt4(row.RateLimitExternalApiPerMinute),
		NotificationsEnabled:          row.NotificationsEnabled,
		Features:                      features,
		CreatedAt:                     row.CreatedAt.Time,
		UpdatedAt:                     row.UpdatedAt.Time,
	}
}

func pgOptionalInt4(value *int32) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *value, Valid: true}
}

func optionalPgInt4(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}
	v := value.Int32
	return &v
}
