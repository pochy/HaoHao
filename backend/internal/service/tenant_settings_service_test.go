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
	if !got.AnonymousLinksEnabled {
		t.Fatal("AnonymousLinksEnabled = false")
	}
	if !got.LinkExpiresRequired {
		t.Fatal("LinkExpiresRequired = false")
	}
	if got.MaxLinkTTLHours != 720 {
		t.Fatalf("MaxLinkTTLHours = %d, want 720", got.MaxLinkTTLHours)
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
			"linkSharingEnabled":         false,
			"anonymousLinksEnabled":      false,
			"linkExpiresRequired":        false,
			"maxLinkTtlHours":            float64(24),
			"viewerDownloadEnabled":      false,
			"shareLinkDownloadEnabled":   false,
			"editorCanReshare":           true,
			"editorCanDelete":            true,
			"externalUserSharingEnabled": true,
		},
	})

	if got.LinkSharingEnabled {
		t.Fatal("LinkSharingEnabled = true")
	}
	if got.AnonymousLinksEnabled {
		t.Fatal("AnonymousLinksEnabled = true")
	}
	if got.LinkExpiresRequired {
		t.Fatal("LinkExpiresRequired = true")
	}
	if got.MaxLinkTTLHours != 24 {
		t.Fatalf("MaxLinkTTLHours = %d, want 24", got.MaxLinkTTLHours)
	}
	if got.ViewerDownloadEnabled {
		t.Fatal("ViewerDownloadEnabled = true")
	}
	if got.ShareLinkDownloadEnabled {
		t.Fatal("ShareLinkDownloadEnabled = true")
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
}
