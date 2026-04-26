package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/net/idna"
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
	LinkSharingEnabled                  bool
	PublicLinksEnabled                  bool
	ExternalUserSharingEnabled          bool
	PasswordProtectedLinksEnabled       bool
	RequireShareLinkPassword            bool
	RequireExternalShareApproval        bool
	AllowedExternalDomains              []string
	BlockedExternalDomains              []string
	MaxShareLinkTTLHours                int
	ViewerDownloadEnabled               bool
	ExternalDownloadEnabled             bool
	EditorCanReshare                    bool
	EditorCanDelete                     bool
	AdminContentAccessMode              string
	AnonymousEditorLinksEnabled         bool
	AnonymousEditorLinksRequirePassword bool
	AnonymousEditorLinkMaxTTLMinutes    int
	ContentScanEnabled                  bool
	BlockDownloadUntilScanComplete      bool
	BlockShareUntilScanComplete         bool
	DLPEnabled                          bool
	PlanCode                            string
	MaxFileSizeBytes                    int64
	MaxWorkspaceCount                   int
	MaxPublicLinkCount                  int
	PasswordLinksPlanEnabled            bool
	DLPPlanEnabled                      bool
	M2MDriveAPIEnabled                  bool
	SearchEnabled                       bool
	CollaborationEnabled                bool
	SyncEnabled                         bool
	MobileOfflineEnabled                bool
	OfflineCacheAllowed                 bool
	OfflineCacheMaxBytes                int64
	OfflineCacheMaxDays                 int
	MobileDownloadRequiresBiometric     bool
	MobileRemoteWipeRequired            bool
	CMKEnabled                          bool
	DataResidencyEnabled                bool
	LegalDiscoveryEnabled               bool
	CleanRoomEnabled                    bool
	CleanRoomRawExportEnabled           bool
	EncryptionMode                      string
	PrimaryRegion                       string
	AllowedRegions                      []string
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
	policy, err := normalizeDrivePolicyForSave(input)
	if err != nil {
		return DrivePolicy{}, err
	}
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
	if _, ok := input.Features["drive"]; ok {
		policy := drivePolicyFromFeatures(input.Features)
		normalized, err := normalizeDrivePolicyForSave(policy)
		if err != nil {
			return TenantSettingsInput{}, err
		}
		input.Features = cloneFeatureMap(input.Features)
		input.Features["drive"] = drivePolicyToFeatureMap(normalized)
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
		LinkSharingEnabled:                  true,
		PublicLinksEnabled:                  true,
		ExternalUserSharingEnabled:          false,
		PasswordProtectedLinksEnabled:       false,
		RequireShareLinkPassword:            false,
		RequireExternalShareApproval:        false,
		AllowedExternalDomains:              []string{},
		BlockedExternalDomains:              []string{},
		MaxShareLinkTTLHours:                168,
		ViewerDownloadEnabled:               true,
		ExternalDownloadEnabled:             false,
		EditorCanReshare:                    false,
		EditorCanDelete:                     false,
		AdminContentAccessMode:              "disabled",
		AnonymousEditorLinksEnabled:         false,
		AnonymousEditorLinksRequirePassword: true,
		AnonymousEditorLinkMaxTTLMinutes:    60,
		ContentScanEnabled:                  false,
		BlockDownloadUntilScanComplete:      true,
		BlockShareUntilScanComplete:         true,
		DLPEnabled:                          false,
		PlanCode:                            "standard",
		MaxFileSizeBytes:                    10 * 1024 * 1024,
		MaxWorkspaceCount:                   25,
		MaxPublicLinkCount:                  1000,
		PasswordLinksPlanEnabled:            true,
		DLPPlanEnabled:                      true,
		M2MDriveAPIEnabled:                  false,
		SearchEnabled:                       true,
		CollaborationEnabled:                false,
		SyncEnabled:                         false,
		MobileOfflineEnabled:                false,
		OfflineCacheAllowed:                 false,
		OfflineCacheMaxBytes:                100 * 1024 * 1024,
		OfflineCacheMaxDays:                 30,
		MobileDownloadRequiresBiometric:     false,
		MobileRemoteWipeRequired:            true,
		CMKEnabled:                          false,
		DataResidencyEnabled:                false,
		LegalDiscoveryEnabled:               false,
		CleanRoomEnabled:                    false,
		CleanRoomRawExportEnabled:           false,
		EncryptionMode:                      "service_managed",
		PrimaryRegion:                       "global",
		AllowedRegions:                      []string{"global"},
	}
}

func drivePolicyFromFeatures(features map[string]any) DrivePolicy {
	policy := defaultDrivePolicy()
	raw, ok := features["drive"].(map[string]any)
	if !ok {
		return policy
	}

	policy.LinkSharingEnabled = featureBool(raw, "linkSharingEnabled", policy.LinkSharingEnabled)
	policy.PublicLinksEnabled = featureBool(raw, "publicLinksEnabled", featureBool(raw, "anonymousLinksEnabled", policy.PublicLinksEnabled))
	policy.ExternalUserSharingEnabled = featureBool(raw, "externalUserSharingEnabled", policy.ExternalUserSharingEnabled)
	policy.PasswordProtectedLinksEnabled = featureBool(raw, "passwordProtectedLinksEnabled", policy.PasswordProtectedLinksEnabled)
	policy.RequireShareLinkPassword = featureBool(raw, "requireShareLinkPassword", policy.RequireShareLinkPassword)
	policy.RequireExternalShareApproval = featureBool(raw, "requireExternalShareApproval", policy.RequireExternalShareApproval)
	policy.AllowedExternalDomains = featureStringSlice(raw, "allowedExternalDomains", policy.AllowedExternalDomains)
	policy.BlockedExternalDomains = featureStringSlice(raw, "blockedExternalDomains", policy.BlockedExternalDomains)
	policy.MaxShareLinkTTLHours = featureInt(raw, "maxShareLinkTTLHours", featureInt(raw, "maxLinkTtlHours", policy.MaxShareLinkTTLHours))
	policy.ViewerDownloadEnabled = featureBool(raw, "viewerDownloadEnabled", policy.ViewerDownloadEnabled)
	policy.ExternalDownloadEnabled = featureBool(raw, "externalDownloadEnabled", featureBool(raw, "shareLinkDownloadEnabled", policy.ExternalDownloadEnabled))
	policy.EditorCanReshare = featureBool(raw, "editorCanReshare", policy.EditorCanReshare)
	policy.EditorCanDelete = featureBool(raw, "editorCanDelete", policy.EditorCanDelete)
	policy.AdminContentAccessMode = featureString(raw, "adminContentAccessMode", policy.AdminContentAccessMode)
	policy.AnonymousEditorLinksEnabled = featureBool(raw, "anonymousEditorLinksEnabled", policy.AnonymousEditorLinksEnabled)
	policy.AnonymousEditorLinksRequirePassword = featureBool(raw, "anonymousEditorLinksRequirePassword", policy.AnonymousEditorLinksRequirePassword)
	policy.AnonymousEditorLinkMaxTTLMinutes = featureInt(raw, "anonymousEditorLinkMaxTTLMinutes", policy.AnonymousEditorLinkMaxTTLMinutes)
	policy.ContentScanEnabled = featureBool(raw, "contentScanEnabled", policy.ContentScanEnabled)
	policy.BlockDownloadUntilScanComplete = featureBool(raw, "blockDownloadUntilScanComplete", policy.BlockDownloadUntilScanComplete)
	policy.BlockShareUntilScanComplete = featureBool(raw, "blockShareUntilScanComplete", policy.BlockShareUntilScanComplete)
	policy.DLPEnabled = featureBool(raw, "dlpEnabled", policy.DLPEnabled)
	policy.PlanCode = featureString(raw, "planCode", policy.PlanCode)
	policy.MaxFileSizeBytes = int64(featureInt(raw, "maxFileSizeBytes", int(policy.MaxFileSizeBytes)))
	policy.MaxWorkspaceCount = featureInt(raw, "maxWorkspaceCount", policy.MaxWorkspaceCount)
	policy.MaxPublicLinkCount = featureInt(raw, "maxPublicLinkCount", policy.MaxPublicLinkCount)
	policy.PasswordLinksPlanEnabled = featureBool(raw, "passwordLinksPlanEnabled", policy.PasswordLinksPlanEnabled)
	policy.DLPPlanEnabled = featureBool(raw, "dlpPlanEnabled", policy.DLPPlanEnabled)
	policy.M2MDriveAPIEnabled = featureBool(raw, "m2mDriveAPIEnabled", policy.M2MDriveAPIEnabled)
	policy.SearchEnabled = featureBool(raw, "searchEnabled", policy.SearchEnabled)
	policy.CollaborationEnabled = featureBool(raw, "collaborationEnabled", policy.CollaborationEnabled)
	policy.SyncEnabled = featureBool(raw, "syncEnabled", policy.SyncEnabled)
	policy.MobileOfflineEnabled = featureBool(raw, "mobileOfflineEnabled", policy.MobileOfflineEnabled)
	policy.OfflineCacheAllowed = featureBool(raw, "offlineCacheAllowed", policy.OfflineCacheAllowed)
	policy.OfflineCacheMaxBytes = int64(featureInt(raw, "offlineCacheMaxBytes", int(policy.OfflineCacheMaxBytes)))
	policy.OfflineCacheMaxDays = featureInt(raw, "offlineCacheMaxDays", policy.OfflineCacheMaxDays)
	policy.MobileDownloadRequiresBiometric = featureBool(raw, "mobileDownloadRequiresBiometric", policy.MobileDownloadRequiresBiometric)
	policy.MobileRemoteWipeRequired = featureBool(raw, "mobileRemoteWipeRequired", policy.MobileRemoteWipeRequired)
	policy.CMKEnabled = featureBool(raw, "cmkEnabled", policy.CMKEnabled)
	policy.DataResidencyEnabled = featureBool(raw, "dataResidencyEnabled", policy.DataResidencyEnabled)
	policy.LegalDiscoveryEnabled = featureBool(raw, "legalDiscoveryEnabled", policy.LegalDiscoveryEnabled)
	policy.CleanRoomEnabled = featureBool(raw, "cleanRoomEnabled", policy.CleanRoomEnabled)
	policy.CleanRoomRawExportEnabled = featureBool(raw, "cleanRoomRawExportEnabled", policy.CleanRoomRawExportEnabled)
	policy.EncryptionMode = featureString(raw, "encryptionMode", policy.EncryptionMode)
	policy.PrimaryRegion = featureString(raw, "primaryRegion", policy.PrimaryRegion)
	policy.AllowedRegions = featureStringSlice(raw, "allowedRegions", policy.AllowedRegions)

	return normalizeDrivePolicy(policy)
}

func normalizeDrivePolicy(policy DrivePolicy) DrivePolicy {
	defaults := defaultDrivePolicy()
	if policy.MaxShareLinkTTLHours <= 0 {
		policy.MaxShareLinkTTLHours = defaults.MaxShareLinkTTLHours
	}
	if policy.AnonymousEditorLinkMaxTTLMinutes <= 0 {
		policy.AnonymousEditorLinkMaxTTLMinutes = defaults.AnonymousEditorLinkMaxTTLMinutes
	}
	policy.AllowedExternalDomains = normalizeDomainList(policy.AllowedExternalDomains)
	policy.BlockedExternalDomains = normalizeDomainList(policy.BlockedExternalDomains)
	if strings.TrimSpace(policy.AdminContentAccessMode) == "" {
		policy.AdminContentAccessMode = defaults.AdminContentAccessMode
	}
	policy.PlanCode = strings.ToLower(strings.TrimSpace(policy.PlanCode))
	if policy.PlanCode == "" {
		policy.PlanCode = defaults.PlanCode
	}
	if policy.MaxFileSizeBytes <= 0 {
		policy.MaxFileSizeBytes = defaults.MaxFileSizeBytes
	}
	if policy.MaxWorkspaceCount <= 0 {
		policy.MaxWorkspaceCount = defaults.MaxWorkspaceCount
	}
	if policy.MaxPublicLinkCount <= 0 {
		policy.MaxPublicLinkCount = defaults.MaxPublicLinkCount
	}
	if policy.OfflineCacheMaxBytes <= 0 {
		policy.OfflineCacheMaxBytes = defaults.OfflineCacheMaxBytes
	}
	if policy.OfflineCacheMaxDays <= 0 {
		policy.OfflineCacheMaxDays = defaults.OfflineCacheMaxDays
	}
	policy.EncryptionMode = strings.ToLower(strings.TrimSpace(policy.EncryptionMode))
	if policy.EncryptionMode == "" {
		policy.EncryptionMode = defaults.EncryptionMode
	}
	policy.PrimaryRegion = strings.ToLower(strings.TrimSpace(policy.PrimaryRegion))
	if policy.PrimaryRegion == "" {
		policy.PrimaryRegion = defaults.PrimaryRegion
	}
	policy.AllowedRegions = normalizeRegionList(policy.AllowedRegions)
	if len(policy.AllowedRegions) == 0 {
		policy.AllowedRegions = defaults.AllowedRegions
	}
	policy = applyDrivePlanCaps(policy)
	return policy
}

func normalizeDrivePolicyForSave(policy DrivePolicy) (DrivePolicy, error) {
	policy = normalizeDrivePolicy(policy)
	if policy.MaxShareLinkTTLHours < 1 || policy.MaxShareLinkTTLHours > 2160 {
		return DrivePolicy{}, fmt.Errorf("%w: maxShareLinkTTLHours must be between 1 and 2160", ErrInvalidTenantSettings)
	}
	if policy.RequireShareLinkPassword && !policy.PasswordProtectedLinksEnabled {
		return DrivePolicy{}, fmt.Errorf("%w: requireShareLinkPassword requires passwordProtectedLinksEnabled", ErrInvalidTenantSettings)
	}
	switch policy.AdminContentAccessMode {
	case "disabled", "break_glass":
	default:
		return DrivePolicy{}, fmt.Errorf("%w: unsupported adminContentAccessMode", ErrInvalidTenantSettings)
	}
	if policy.AnonymousEditorLinkMaxTTLMinutes < 1 || policy.AnonymousEditorLinkMaxTTLMinutes > 1440 {
		return DrivePolicy{}, fmt.Errorf("%w: anonymousEditorLinkMaxTTLMinutes must be between 1 and 1440", ErrInvalidTenantSettings)
	}
	if policy.MaxFileSizeBytes < 1 {
		return DrivePolicy{}, fmt.Errorf("%w: maxFileSizeBytes must be positive", ErrInvalidTenantSettings)
	}
	if policy.MaxWorkspaceCount < 1 || policy.MaxWorkspaceCount > 1000 {
		return DrivePolicy{}, fmt.Errorf("%w: maxWorkspaceCount must be between 1 and 1000", ErrInvalidTenantSettings)
	}
	if policy.MaxPublicLinkCount < 1 || policy.MaxPublicLinkCount > 100000 {
		return DrivePolicy{}, fmt.Errorf("%w: maxPublicLinkCount must be between 1 and 100000", ErrInvalidTenantSettings)
	}
	if policy.OfflineCacheMaxBytes < 1 {
		return DrivePolicy{}, fmt.Errorf("%w: offlineCacheMaxBytes must be positive", ErrInvalidTenantSettings)
	}
	if policy.OfflineCacheMaxDays < 1 || policy.OfflineCacheMaxDays > 365 {
		return DrivePolicy{}, fmt.Errorf("%w: offlineCacheMaxDays must be between 1 and 365", ErrInvalidTenantSettings)
	}
	switch policy.EncryptionMode {
	case "service_managed", "tenant_managed", "workspace_managed", "file_managed":
	default:
		return DrivePolicy{}, fmt.Errorf("%w: unsupported encryptionMode", ErrInvalidTenantSettings)
	}
	return policy, nil
}

func drivePolicyToFeatureMap(policy DrivePolicy) map[string]any {
	policy = normalizeDrivePolicy(policy)
	return map[string]any{
		"linkSharingEnabled":                  policy.LinkSharingEnabled,
		"publicLinksEnabled":                  policy.PublicLinksEnabled,
		"externalUserSharingEnabled":          policy.ExternalUserSharingEnabled,
		"passwordProtectedLinksEnabled":       policy.PasswordProtectedLinksEnabled,
		"requireShareLinkPassword":            policy.RequireShareLinkPassword,
		"requireExternalShareApproval":        policy.RequireExternalShareApproval,
		"allowedExternalDomains":              policy.AllowedExternalDomains,
		"blockedExternalDomains":              policy.BlockedExternalDomains,
		"maxShareLinkTTLHours":                policy.MaxShareLinkTTLHours,
		"viewerDownloadEnabled":               policy.ViewerDownloadEnabled,
		"externalDownloadEnabled":             policy.ExternalDownloadEnabled,
		"editorCanReshare":                    policy.EditorCanReshare,
		"editorCanDelete":                     policy.EditorCanDelete,
		"adminContentAccessMode":              policy.AdminContentAccessMode,
		"anonymousEditorLinksEnabled":         policy.AnonymousEditorLinksEnabled,
		"anonymousEditorLinksRequirePassword": policy.AnonymousEditorLinksRequirePassword,
		"anonymousEditorLinkMaxTTLMinutes":    policy.AnonymousEditorLinkMaxTTLMinutes,
		"contentScanEnabled":                  policy.ContentScanEnabled,
		"blockDownloadUntilScanComplete":      policy.BlockDownloadUntilScanComplete,
		"blockShareUntilScanComplete":         policy.BlockShareUntilScanComplete,
		"dlpEnabled":                          policy.DLPEnabled,
		"planCode":                            policy.PlanCode,
		"maxFileSizeBytes":                    policy.MaxFileSizeBytes,
		"maxWorkspaceCount":                   policy.MaxWorkspaceCount,
		"maxPublicLinkCount":                  policy.MaxPublicLinkCount,
		"passwordLinksPlanEnabled":            policy.PasswordLinksPlanEnabled,
		"dlpPlanEnabled":                      policy.DLPPlanEnabled,
		"m2mDriveAPIEnabled":                  policy.M2MDriveAPIEnabled,
		"searchEnabled":                       policy.SearchEnabled,
		"collaborationEnabled":                policy.CollaborationEnabled,
		"syncEnabled":                         policy.SyncEnabled,
		"mobileOfflineEnabled":                policy.MobileOfflineEnabled,
		"offlineCacheAllowed":                 policy.OfflineCacheAllowed,
		"offlineCacheMaxBytes":                policy.OfflineCacheMaxBytes,
		"offlineCacheMaxDays":                 policy.OfflineCacheMaxDays,
		"mobileDownloadRequiresBiometric":     policy.MobileDownloadRequiresBiometric,
		"mobileRemoteWipeRequired":            policy.MobileRemoteWipeRequired,
		"cmkEnabled":                          policy.CMKEnabled,
		"dataResidencyEnabled":                policy.DataResidencyEnabled,
		"legalDiscoveryEnabled":               policy.LegalDiscoveryEnabled,
		"cleanRoomEnabled":                    policy.CleanRoomEnabled,
		"cleanRoomRawExportEnabled":           policy.CleanRoomRawExportEnabled,
		"encryptionMode":                      policy.EncryptionMode,
		"primaryRegion":                       policy.PrimaryRegion,
		"allowedRegions":                      policy.AllowedRegions,
	}
}

func applyDrivePlanCaps(policy DrivePolicy) DrivePolicy {
	switch policy.PlanCode {
	case "free":
		if policy.MaxFileSizeBytes > 5*1024*1024 {
			policy.MaxFileSizeBytes = 5 * 1024 * 1024
		}
		if policy.MaxWorkspaceCount > 1 {
			policy.MaxWorkspaceCount = 1
		}
		if policy.MaxPublicLinkCount > 5 {
			policy.MaxPublicLinkCount = 5
		}
		policy.PasswordLinksPlanEnabled = false
		policy.DLPPlanEnabled = false
		policy.DLPEnabled = false
		policy.M2MDriveAPIEnabled = false
	case "enterprise":
		if policy.MaxFileSizeBytes < 50*1024*1024 {
			policy.MaxFileSizeBytes = 50 * 1024 * 1024
		}
		policy.PasswordLinksPlanEnabled = true
		policy.DLPPlanEnabled = true
	default:
		policy.PlanCode = "standard"
		policy.PasswordLinksPlanEnabled = true
		policy.DLPPlanEnabled = true
	}
	if !policy.PasswordLinksPlanEnabled {
		policy.PasswordProtectedLinksEnabled = false
		policy.RequireShareLinkPassword = false
	}
	if !policy.DLPPlanEnabled {
		policy.DLPEnabled = false
	}
	return policy
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

func featureString(values map[string]any, key string, fallback string) string {
	value, ok := values[key].(string)
	if !ok {
		return fallback
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func featureStringSlice(values map[string]any, key string, fallback []string) []string {
	value, ok := values[key]
	if !ok {
		return fallback
	}
	switch items := value.(type) {
	case []string:
		return append([]string{}, items...)
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return fallback
	}
}

func normalizeDomainList(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		domain := normalizeExternalDomain(value)
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		out = append(out, domain)
	}
	return out
}

func normalizeRegionList(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		region := strings.ToLower(strings.TrimSpace(value))
		if region == "" || strings.ContainsAny(region, " /\\") {
			continue
		}
		if _, ok := seen[region]; ok {
			continue
		}
		seen[region] = struct{}{}
		out = append(out, region)
	}
	return out
}

func normalizeExternalDomain(value string) string {
	domain := strings.ToLower(strings.TrimSpace(value))
	domain = strings.TrimPrefix(domain, "@")
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" || strings.ContainsAny(domain, " /\\") {
		return ""
	}
	ascii, err := idna.Lookup.ToASCII(domain)
	if err != nil {
		return ""
	}
	ascii = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(ascii), "."))
	if ascii == "" || strings.ContainsAny(ascii, " /\\") {
		return ""
	}
	return ascii
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
