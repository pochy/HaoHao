package service

import (
	"context"
	"testing"
)

func TestEffectiveRateLimitForSettings(t *testing.T) {
	loginLimit := int32(7)
	browserLimit := int32(3)
	externalLimit := int32(11)
	defaults := RateLimitDefaults{
		LoginPerMinute:       20,
		BrowserAPIPerMinute:  120,
		ExternalAPIPerMinute: 60,
	}

	tests := []struct {
		name     string
		settings TenantSettings
		policy   string
		want     int
	}{
		{
			name:     "browser override",
			settings: TenantSettings{RateLimitBrowserAPIPerMinute: &browserLimit},
			policy:   "browser_api",
			want:     3,
		},
		{
			name:     "browser null uses default",
			settings: TenantSettings{},
			policy:   "browser_api",
			want:     120,
		},
		{
			name:     "login override",
			settings: TenantSettings{RateLimitLoginPerMinute: &loginLimit},
			policy:   "login",
			want:     7,
		},
		{
			name:     "external override",
			settings: TenantSettings{RateLimitExternalAPIPerMinute: &externalLimit},
			policy:   "external_api",
			want:     11,
		},
		{
			name:     "unknown policy uses browser default",
			settings: TenantSettings{},
			policy:   "unknown",
			want:     120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := effectiveRateLimitForSettings(tt.settings, tt.policy, defaults); got != tt.want {
				t.Fatalf("effectiveRateLimitForSettings() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestResolveEffectiveRateLimitFallsBackToDefaultOnLookupError(t *testing.T) {
	defaults := RateLimitDefaults{
		LoginPerMinute:       20,
		BrowserAPIPerMinute:  120,
		ExternalAPIPerMinute: 60,
	}
	svc := NewTenantSettingsService(nil, nil, 100)

	got, err := svc.ResolveEffectiveRateLimit(context.Background(), 1, "browser_api", defaults)
	if err == nil {
		t.Fatal("ResolveEffectiveRateLimit() error = nil, want lookup error")
	}
	if got != defaults.BrowserAPIPerMinute {
		t.Fatalf("ResolveEffectiveRateLimit() = %d, want %d", got, defaults.BrowserAPIPerMinute)
	}
}

func TestDrivePolicyFromFeaturesDefaults(t *testing.T) {
	got := drivePolicyFromFeatures(nil)
	if !got.LinkSharingEnabled {
		t.Fatal("LinkSharingEnabled = false")
	}
	if !got.PublicLinksEnabled {
		t.Fatal("PublicLinksEnabled = false")
	}
	if got.MaxShareLinkTTLHours != 168 {
		t.Fatalf("MaxShareLinkTTLHours = %d, want 168", got.MaxShareLinkTTLHours)
	}
	if got.EditorCanReshare {
		t.Fatal("EditorCanReshare = true")
	}
	if got.EditorCanDelete {
		t.Fatal("EditorCanDelete = true")
	}
	if got.ExternalUserSharingEnabled {
		t.Fatal("ExternalUserSharingEnabled = true")
	}
}

func TestDrivePolicyFromFeaturesOverrides(t *testing.T) {
	got := drivePolicyFromFeatures(map[string]any{
		"drive": map[string]any{
			"linkSharingEnabled":            false,
			"publicLinksEnabled":            false,
			"maxShareLinkTTLHours":          float64(24),
			"viewerDownloadEnabled":         false,
			"externalDownloadEnabled":       true,
			"editorCanReshare":              true,
			"editorCanDelete":               true,
			"externalUserSharingEnabled":    true,
			"passwordProtectedLinksEnabled": true,
			"requireShareLinkPassword":      true,
			"requireExternalShareApproval":  true,
			"allowedExternalDomains":        []any{"@Example.com", "sub.Example.com."},
			"blockedExternalDomains":        []any{"blocked.example.com"},
		},
	})

	if got.LinkSharingEnabled {
		t.Fatal("LinkSharingEnabled = true")
	}
	if got.PublicLinksEnabled {
		t.Fatal("PublicLinksEnabled = true")
	}
	if got.MaxShareLinkTTLHours != 24 {
		t.Fatalf("MaxShareLinkTTLHours = %d, want 24", got.MaxShareLinkTTLHours)
	}
	if got.ViewerDownloadEnabled {
		t.Fatal("ViewerDownloadEnabled = true")
	}
	if !got.ExternalDownloadEnabled {
		t.Fatal("ExternalDownloadEnabled = false")
	}
	if !got.EditorCanReshare {
		t.Fatal("EditorCanReshare = false")
	}
	if !got.EditorCanDelete {
		t.Fatal("EditorCanDelete = false")
	}
	if !got.ExternalUserSharingEnabled {
		t.Fatal("ExternalUserSharingEnabled = false")
	}
	if !got.PasswordProtectedLinksEnabled || !got.RequireShareLinkPassword || !got.RequireExternalShareApproval {
		t.Fatal("password or approval policy override was not applied")
	}
	if len(got.AllowedExternalDomains) != 2 || got.AllowedExternalDomains[0] != "example.com" || got.AllowedExternalDomains[1] != "sub.example.com" {
		t.Fatalf("AllowedExternalDomains = %#v", got.AllowedExternalDomains)
	}
	if len(got.BlockedExternalDomains) != 1 || got.BlockedExternalDomains[0] != "blocked.example.com" {
		t.Fatalf("BlockedExternalDomains = %#v", got.BlockedExternalDomains)
	}
}

func TestDriveOCRPolicyFromFeaturesSupportsLMStudio(t *testing.T) {
	got := drivePolicyFromFeatures(map[string]any{
		"drive": map[string]any{
			"ocr": map[string]any{
				"enabled":                     true,
				"structuredExtractionEnabled": true,
				"structuredExtractor":         "lmstudio",
				"lmStudioBaseURL":             "http://127.0.0.1:1234/v1",
				"lmStudioModel":               "local-model",
			},
		},
	})

	if got.OCR.StructuredExtractor != "lmstudio" {
		t.Fatalf("StructuredExtractor = %q, want lmstudio", got.OCR.StructuredExtractor)
	}
	if got.OCR.LMStudioBaseURL != "http://127.0.0.1:1234/v1" {
		t.Fatalf("LMStudioBaseURL = %q", got.OCR.LMStudioBaseURL)
	}
	if got.OCR.LMStudioModel != "local-model" {
		t.Fatalf("LMStudioModel = %q, want local-model", got.OCR.LMStudioModel)
	}
}

func TestValidateDriveOCRPolicyRequiresLMStudioModel(t *testing.T) {
	policy := defaultDriveOCRPolicy()
	policy.StructuredExtractionEnabled = true
	policy.StructuredExtractor = "lmstudio"

	if err := validateDriveOCRPolicy(policy); err == nil {
		t.Fatal("validateDriveOCRPolicy() error = nil, want lmStudioModel validation error")
	}

	policy.LMStudioModel = "local-model"
	if err := validateDriveOCRPolicy(policy); err != nil {
		t.Fatalf("validateDriveOCRPolicy() error = %v", err)
	}
}

func TestValidateDriveOCRPolicyAcceptsLocalCommandExtractors(t *testing.T) {
	for _, extractor := range []string{"gemini", "codex", "claude"} {
		t.Run(extractor, func(t *testing.T) {
			policy := defaultDriveOCRPolicy()
			policy.StructuredExtractionEnabled = true
			policy.StructuredExtractor = extractor
			if err := validateDriveOCRPolicy(policy); err != nil {
				t.Fatalf("validateDriveOCRPolicy() error = %v", err)
			}
		})
	}
}

func TestNormalizeDrivePolicyForSaveValidation(t *testing.T) {
	_, err := normalizeDrivePolicyForSave(DrivePolicy{
		LinkSharingEnabled:            true,
		PublicLinksEnabled:            true,
		RequireShareLinkPassword:      true,
		PasswordProtectedLinksEnabled: false,
		MaxShareLinkTTLHours:          168,
		AdminContentAccessMode:        "disabled",
	})
	if err == nil {
		t.Fatal("normalizeDrivePolicyForSave() error = nil, want password dependency error")
	}
}
