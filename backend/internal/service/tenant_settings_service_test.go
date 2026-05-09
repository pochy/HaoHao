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
	if got.LocalSearch.VectorEnabled {
		t.Fatal("LocalSearch.VectorEnabled = true")
	}
	if got.LocalSearch.EmbeddingRuntime != "none" {
		t.Fatalf("LocalSearch.EmbeddingRuntime = %q, want none", got.LocalSearch.EmbeddingRuntime)
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

func TestDriveLocalSearchPolicyFromFeaturesOverrides(t *testing.T) {
	got := drivePolicyFromFeatures(map[string]any{
		"drive": map[string]any{
			"localSearch": map[string]any{
				"vectorEnabled":    true,
				"embeddingRuntime": "ollama",
				"runtimeURL":       "http://127.0.0.1:11434",
				"model":            "bge-m3",
				"dimension":        float64(1024),
			},
		},
	})

	if !got.LocalSearch.VectorEnabled {
		t.Fatal("LocalSearch.VectorEnabled = false")
	}
	if got.LocalSearch.EmbeddingRuntime != "ollama" {
		t.Fatalf("LocalSearch.EmbeddingRuntime = %q, want ollama", got.LocalSearch.EmbeddingRuntime)
	}
	if got.LocalSearch.RuntimeURL != "http://127.0.0.1:11434" {
		t.Fatalf("LocalSearch.RuntimeURL = %q", got.LocalSearch.RuntimeURL)
	}
	if got.LocalSearch.Model != "bge-m3" {
		t.Fatalf("LocalSearch.Model = %q", got.LocalSearch.Model)
	}
	if got.LocalSearch.Dimension != 1024 {
		t.Fatalf("LocalSearch.Dimension = %d, want 1024", got.LocalSearch.Dimension)
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

func TestDriveOCRPolicyFromFeaturesSupportsRulesSettings(t *testing.T) {
	got := drivePolicyFromFeatures(map[string]any{
		"drive": map[string]any{
			"ocr": map[string]any{
				"enabled":             true,
				"structuredExtractor": "rules",
				"rules": map[string]any{
					"candidateScoreThreshold": float64(0),
					"maxBlockRunes":           float64(5000),
					"contextWindowRunes":      float64(1200),
					"priceExtractionEnabled":  false,
				},
			},
		},
	})

	if got.OCR.StructuredExtractor != "rules" {
		t.Fatalf("StructuredExtractor = %q, want rules", got.OCR.StructuredExtractor)
	}
	if got.OCR.Rules.CandidateScoreThreshold != 0 {
		t.Fatalf("CandidateScoreThreshold = %d, want 0", got.OCR.Rules.CandidateScoreThreshold)
	}
	if got.OCR.Rules.MaxBlockRunes != 5000 {
		t.Fatalf("MaxBlockRunes = %d, want 5000", got.OCR.Rules.MaxBlockRunes)
	}
	if got.OCR.Rules.ContextWindowRunes != 1200 {
		t.Fatalf("ContextWindowRunes = %d, want 1200", got.OCR.Rules.ContextWindowRunes)
	}
	if got.OCR.Rules.PriceExtractionEnabled {
		t.Fatal("PriceExtractionEnabled = true, want false")
	}

	roundTripped := driveOCRPolicyToFeatureMap(got.OCR)
	rules, ok := roundTripped["rules"].(map[string]any)
	if !ok {
		t.Fatalf("rules feature map type = %T", roundTripped["rules"])
	}
	if rules["candidateScoreThreshold"] != 0 {
		t.Fatalf("round-trip candidateScoreThreshold = %#v, want 0", rules["candidateScoreThreshold"])
	}
	if rules["priceExtractionEnabled"] != false {
		t.Fatalf("round-trip priceExtractionEnabled = %#v, want false", rules["priceExtractionEnabled"])
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

func TestValidateDriveOCRPolicyAcceptsRulesWithoutModel(t *testing.T) {
	policy := defaultDriveOCRPolicy()
	policy.StructuredExtractionEnabled = true
	policy.StructuredExtractor = "rules"
	policy.OllamaModel = ""
	policy.LMStudioModel = ""

	if err := validateDriveOCRPolicy(policy); err != nil {
		t.Fatalf("validateDriveOCRPolicy() error = %v", err)
	}
}

func TestValidateDriveLocalSearchPolicyDefaultsDisabled(t *testing.T) {
	policy := defaultDriveLocalSearchPolicy()
	if err := validateDriveLocalSearchPolicy(policy); err != nil {
		t.Fatalf("validateDriveLocalSearchPolicy() error = %v", err)
	}
}

func TestValidateDriveLocalSearchPolicyRequiresVectorRuntimeConfig(t *testing.T) {
	policy := defaultDriveLocalSearchPolicy()
	policy.VectorEnabled = true
	if err := validateDriveLocalSearchPolicy(policy); err == nil {
		t.Fatal("validateDriveLocalSearchPolicy() error = nil, want runtime validation error")
	}

	policy.EmbeddingRuntime = "ollama"
	policy.RuntimeURL = "http://127.0.0.1:11434"
	policy.Model = "bge-m3"
	policy.Dimension = 1024
	if err := validateDriveLocalSearchPolicy(policy); err != nil {
		t.Fatalf("validateDriveLocalSearchPolicy() error = %v", err)
	}
}

func TestValidateDriveLocalSearchPolicyAcceptsRuriVectorDimension(t *testing.T) {
	policy := defaultDriveLocalSearchPolicy()
	policy.VectorEnabled = true
	policy.EmbeddingRuntime = "infinity"
	policy.RuntimeURL = "http://127.0.0.1:7997"
	policy.Model = "cl-nagoya/ruri-v3-310m"
	policy.Dimension = 768

	if err := validateDriveLocalSearchPolicy(policy); err != nil {
		t.Fatalf("validateDriveLocalSearchPolicy() error = %v", err)
	}
}

func TestValidateDriveLocalSearchPolicyRejectsTooLargeVectorDimension(t *testing.T) {
	policy := defaultDriveLocalSearchPolicy()
	policy.VectorEnabled = true
	policy.EmbeddingRuntime = "ollama"
	policy.RuntimeURL = "http://127.0.0.1:11434"
	policy.Model = "oversized"
	policy.Dimension = LocalSearchMaxEmbeddingDimension + 1

	if err := validateDriveLocalSearchPolicy(policy); err == nil {
		t.Fatal("validateDriveLocalSearchPolicy() error = nil, want dimension validation error")
	}
}

func TestValidateDriveLocalSearchPolicyRejectsRemoteRuntimeURL(t *testing.T) {
	policy := defaultDriveLocalSearchPolicy()
	policy.RuntimeURL = "https://example.com/embed"
	if err := validateDriveLocalSearchPolicy(policy); err == nil {
		t.Fatal("validateDriveLocalSearchPolicy() error = nil, want runtimeURL validation error")
	}
}

func TestValidateDriveRAGPolicyRequiresExplicitLocalRuntimeAndVectorSearch(t *testing.T) {
	localSearch := defaultDriveLocalSearchPolicy()
	policy := defaultDriveRAGPolicy()
	policy.Enabled = true
	policy.GenerationRuntime = "lmstudio"
	policy.GenerationRuntimeURL = "http://127.0.0.1:1234"
	policy.GenerationModel = "qwen/qwen3.5-9b"

	if err := validateDriveRAGPolicy(policy, localSearch); err == nil {
		t.Fatal("validateDriveRAGPolicy() error = nil, want local search dependency error")
	}

	localSearch.VectorEnabled = true
	localSearch.EmbeddingRuntime = "lmstudio"
	localSearch.RuntimeURL = "http://127.0.0.1:1234"
	localSearch.Model = "ruri-v3-310m"
	localSearch.Dimension = 1024
	if err := validateDriveRAGPolicy(policy, localSearch); err != nil {
		t.Fatalf("validateDriveRAGPolicy() error = %v", err)
	}
}

func TestValidateDriveRAGPolicyRejectsRemoteRuntimeURL(t *testing.T) {
	policy := defaultDriveRAGPolicy()
	policy.GenerationRuntimeURL = "https://example.com/v1"
	if err := validateDriveRAGPolicy(policy, defaultDriveLocalSearchPolicy()); err == nil {
		t.Fatal("validateDriveRAGPolicy() error = nil, want runtimeURL validation error")
	}
}

func TestDriveRAGPolicyQueryRewriteDefaultsAndValidation(t *testing.T) {
	policy := normalizeDriveRAGPolicy(defaultDriveRAGPolicy())
	if !policy.QueryRewriteEnabled {
		t.Fatal("QueryRewriteEnabled = false, want default enabled")
	}
	if policy.QueryRewriteMode != "deterministic" {
		t.Fatalf("QueryRewriteMode = %q, want deterministic", policy.QueryRewriteMode)
	}
	if policy.QueryRewriteMaxQueries != 4 {
		t.Fatalf("QueryRewriteMaxQueries = %d, want 4", policy.QueryRewriteMaxQueries)
	}

	policy.QueryRewriteMode = "unsupported"
	if err := validateDriveRAGPolicy(policy, defaultDriveLocalSearchPolicy()); err == nil {
		t.Fatal("validateDriveRAGPolicy() error = nil, want queryRewriteMode validation error")
	}
}

func TestValidateDriveOCRPolicyRejectsInvalidRulesSettings(t *testing.T) {
	cases := []struct {
		name   string
		update func(*DriveOCRPolicy)
	}{
		{
			name: "candidate score threshold",
			update: func(policy *DriveOCRPolicy) {
				policy.Rules.CandidateScoreThreshold = 21
			},
		},
		{
			name: "max block runes",
			update: func(policy *DriveOCRPolicy) {
				policy.Rules.MaxBlockRunes = 499
			},
		},
		{
			name: "context window runes",
			update: func(policy *DriveOCRPolicy) {
				policy.Rules.ContextWindowRunes = 99
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := defaultDriveOCRPolicy()
			tc.update(&policy)
			if err := validateDriveOCRPolicy(policy); err == nil {
				t.Fatal("validateDriveOCRPolicy() error = nil, want validation error")
			}
		})
	}
}

func TestValidateDriveOCRPolicyAcceptsLocalCommandExtractors(t *testing.T) {
	for _, extractor := range []string{"gemini", "codex", "claude", "python", "ginza", "sudachipy"} {
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
