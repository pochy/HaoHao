package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type DriveOfficeSession struct {
	PublicID          string
	FilePublicID      string
	Provider          string
	ProviderSessionID string
	AccessLevel       string
	LaunchURL         string
	ExpiresAt         time.Time
	CreatedAt         time.Time
}

type DriveOfficeWebhookInput struct {
	Provider        string
	ProviderEventID string
	ProviderFileID  string
	Revision        string
	Checksum        string
}

type DriveOfficeWebhookResult struct {
	ProviderEventID string
	Result          string
	ProviderFileID  string
	Revision        string
}

type DriveEDiscoveryConnection struct {
	PublicID  string
	Provider  string
	Status    string
	CreatedAt time.Time
}

type DriveEDiscoveryExport struct {
	PublicID         string
	CasePublicID     string
	Status           string
	ManifestHash     string
	ProviderExportID string
	ItemCount        int
	CreatedAt        time.Time
}

type DriveHSMDeployment struct {
	PublicID        string
	Provider        string
	EndpointURL     string
	Status          string
	HealthStatus    string
	AttestationHash string
	KeyPublicID     string
	KeyStatus       string
	CreatedAt       time.Time
}

type DriveGateway struct {
	PublicID               string
	Name                   string
	Status                 string
	EndpointURL            string
	CertificateFingerprint string
	LastSeenAt             *time.Time
	CreatedAt              time.Time
}

type DriveGatewayObject struct {
	GatewayPublicID string
	FilePublicID    string
	ManifestHash    string
	Status          string
}

type DriveE2EEUserKey struct {
	PublicID     string
	UserPublicID string
	Algorithm    string
	Status       string
	CreatedAt    time.Time
}

type DriveE2EEFileKey struct {
	PublicID         string
	FilePublicID     string
	KeyVersion       int
	Algorithm        string
	CiphertextSHA256 string
	CreatedAt        time.Time
}

type DriveE2EEEnvelope struct {
	FileKeyPublicID string
	RecipientUserID string
	WrappedFileKey  string
	WrapAlgorithm   string
	CreatedAt       time.Time
}

type DriveAIJob struct {
	PublicID     string
	FilePublicID string
	JobType      string
	Provider     string
	Status       string
	CreatedAt    time.Time
}

type DriveAISummary struct {
	PublicID     string
	FilePublicID string
	SummaryText  string
	Provider     string
	CreatedAt    time.Time
}

type DriveAIClassification struct {
	Label      string
	Confidence float64
	Provider   string
	CreatedAt  time.Time
}

type DriveMarketplaceApp struct {
	PublicID      string
	Slug          string
	Name          string
	PublisherName string
	Version       string
	Scopes        []string
}

type DriveMarketplaceInstallation struct {
	PublicID  string
	AppSlug   string
	AppName   string
	Status    string
	Scopes    []string
	CreatedAt time.Time
}

type DriveOfficeProvider interface {
	CreateEditSession(context.Context, DriveFile, string) (providerSessionID, launchURL string, err error)
}

type FakeDriveOfficeProvider struct{}

func (FakeDriveOfficeProvider) CreateEditSession(_ context.Context, file DriveFile, accessLevel string) (string, string, error) {
	sessionID := "fake-office-session-" + file.PublicID
	return sessionID, "https://office.local.invalid/launch/" + file.PublicID + "?access=" + accessLevel, nil
}

type DriveAIProvider interface {
	Classify(context.Context, DriveFile) ([]DriveAIClassification, error)
	Summarize(context.Context, DriveFile) (string, error)
}

type FakeDriveAIProvider struct{}

func (FakeDriveAIProvider) Classify(_ context.Context, file DriveFile) ([]DriveAIClassification, error) {
	label := "general"
	name := strings.ToLower(file.OriginalFilename)
	switch {
	case strings.Contains(name, "legal"):
		label = "legal"
	case strings.Contains(name, "finance"):
		label = "finance"
	}
	return []DriveAIClassification{{Label: label, Confidence: 0.9900, Provider: "fake", CreatedAt: time.Now()}}, nil
}

func (FakeDriveAIProvider) Summarize(_ context.Context, file DriveFile) (string, error) {
	return fmt.Sprintf("Fake summary for %s (%d bytes).", file.OriginalFilename, file.ByteSize), nil
}

func (s *DriveService) CreateOfficeSession(ctx context.Context, tenantID, actorUserID int64, filePublicID, accessLevel string, auditCtx AuditContext) (DriveOfficeSession, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveOfficeSession{}, err
	}
	actor, file, err := s.driveFileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return DriveOfficeSession{}, err
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, tenantID)
	if err != nil {
		return DriveOfficeSession{}, err
	}
	if !policy.OfficeCoauthoringEnabled {
		return DriveOfficeSession{}, ErrDrivePolicyDenied
	}
	if s.driveFileUsesZeroKnowledgeEncryption(ctx, tenantID, file.ID) {
		return DriveOfficeSession{}, ErrDrivePolicyDenied
	}
	if !driveOfficeCoauthoringSupported(file) {
		return DriveOfficeSession{}, ErrDriveInvalidInput
	}
	accessLevel = strings.ToLower(strings.TrimSpace(accessLevel))
	if accessLevel == "" {
		accessLevel = "view"
	}
	switch accessLevel {
	case "view":
		if err := s.authz.CanViewFile(ctx, actor, file); err != nil {
			s.auditDenied(ctx, actor, "drive.office.session.create", "drive_file", file.PublicID, err, auditCtx)
			return DriveOfficeSession{}, err
		}
	case "edit":
		if err := s.authz.CanEditFile(ctx, actor, file); err != nil {
			s.auditDenied(ctx, actor, "drive.office.session.create", "drive_file", file.PublicID, err, auditCtx)
			return DriveOfficeSession{}, err
		}
	default:
		return DriveOfficeSession{}, ErrDriveInvalidInput
	}
	if err := s.ensureFileDownloadAllowed(ctx, actor, file, auditCtx, "drive.office.session.create"); err != nil {
		return DriveOfficeSession{}, err
	}
	provider := FakeDriveOfficeProvider{}
	providerSessionID, launchURL, err := provider.CreateEditSession(ctx, file, accessLevel)
	if err != nil {
		return DriveOfficeSession{}, err
	}
	providerFileID := "fake-office-file-" + file.PublicID
	_, err = s.pool.Exec(ctx, `
INSERT INTO drive_office_provider_files (tenant_id, file_object_id, provider, provider_file_id, compatibility_state, provider_revision, content_checksum, last_synced_at)
VALUES ($1, $2, 'fake', $3, 'compatible', COALESCE(NULLIF($4, ''), '1'), NULLIF($5, ''), now())
ON CONFLICT (tenant_id, file_object_id, provider) DO UPDATE
SET compatibility_state = 'compatible',
    updated_at = now()
`, tenantID, file.ID, providerFileID, driveFileContentRevision(file), file.SHA256Hex)
	if err != nil {
		return DriveOfficeSession{}, fmt.Errorf("upsert drive office provider file: %w", err)
	}
	var out DriveOfficeSession
	err = s.pool.QueryRow(ctx, `
INSERT INTO drive_office_edit_sessions (tenant_id, file_object_id, actor_user_id, provider, provider_session_id, access_level, launch_url, expires_at)
VALUES ($1, $2, $3, 'fake', $4, $5, $6, now() + interval '30 minutes')
RETURNING public_id::text, provider, provider_session_id, access_level, launch_url, expires_at, created_at
`, tenantID, file.ID, actor.UserID, providerSessionID+"-"+driveUniqueTokenSuffix(), accessLevel, launchURL).Scan(&out.PublicID, &out.Provider, &out.ProviderSessionID, &out.AccessLevel, &out.LaunchURL, &out.ExpiresAt, &out.CreatedAt)
	if err != nil {
		return DriveOfficeSession{}, fmt.Errorf("create drive office edit session: %w", err)
	}
	out.FilePublicID = file.PublicID
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.office.session.create", "drive_file", file.PublicID, map[string]any{
		"provider":    out.Provider,
		"accessLevel": accessLevel,
	})
	return out, nil
}

func (s *DriveService) RevokeOfficeSession(ctx context.Context, tenantID, actorUserID int64, sessionPublicID string, auditCtx AuditContext) error {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE drive_office_edit_sessions
SET revoked_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND actor_user_id = $3
  AND revoked_at IS NULL
`, sessionPublicID, tenantID, actor.UserID)
	if err != nil {
		return fmt.Errorf("revoke drive office session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDriveNotFound
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.office.session.revoke", "drive_office_edit_session", sessionPublicID, nil)
	return nil
}

func (s *DriveService) AcceptOfficeWebhook(ctx context.Context, input DriveOfficeWebhookInput) (DriveOfficeWebhookResult, error) {
	provider := strings.TrimSpace(input.Provider)
	eventID := strings.TrimSpace(input.ProviderEventID)
	providerFileID := strings.TrimSpace(input.ProviderFileID)
	revision := strings.TrimSpace(input.Revision)
	if provider == "" || eventID == "" || providerFileID == "" || revision == "" {
		return DriveOfficeWebhookResult{}, ErrDriveInvalidInput
	}
	var tenantID, fileID int64
	var currentRevision string
	err := s.pool.QueryRow(ctx, `
SELECT tenant_id, file_object_id, provider_revision
FROM drive_office_provider_files
WHERE provider = $1 AND provider_file_id = $2
`, provider, providerFileID).Scan(&tenantID, &fileID, &currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveOfficeWebhookResult{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveOfficeWebhookResult{}, fmt.Errorf("get drive office provider file: %w", err)
	}
	payloadHash := driveStableHash(provider + ":" + eventID + ":" + providerFileID + ":" + revision + ":" + input.Checksum)
	var webhookID int64
	err = s.pool.QueryRow(ctx, `
INSERT INTO drive_office_webhook_events (provider, provider_event_id, tenant_id, file_object_id, payload_hash, provider_revision)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (provider, provider_event_id) DO NOTHING
RETURNING id
`, provider, eventID, tenantID, fileID, payloadHash, revision).Scan(&webhookID)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveOfficeWebhookResult{ProviderEventID: eventID, Result: "duplicate", ProviderFileID: providerFileID, Revision: revision}, nil
	}
	if err != nil {
		return DriveOfficeWebhookResult{}, fmt.Errorf("record drive office webhook: %w", err)
	}
	result := "accepted"
	if !driveProviderRevisionNewer(revision, currentRevision) {
		result = "stale"
	}
	if result == "accepted" {
		_, err = s.pool.Exec(ctx, `
UPDATE drive_office_provider_files
SET provider_revision = $1,
    content_checksum = NULLIF($2, ''),
    last_synced_at = now(),
    updated_at = now()
WHERE provider = $3 AND provider_file_id = $4
`, revision, strings.TrimSpace(input.Checksum), provider, providerFileID)
		if err != nil {
			return DriveOfficeWebhookResult{}, fmt.Errorf("sync drive office provider revision: %w", err)
		}
		_, err = s.pool.Exec(ctx, `
UPDATE file_objects
SET office_last_revision = $1,
    updated_at = now()
WHERE id = $2 AND tenant_id = $3
`, revision, fileID, tenantID)
		if err != nil {
			return DriveOfficeWebhookResult{}, fmt.Errorf("sync drive office file revision: %w", err)
		}
	}
	_, _ = s.pool.Exec(ctx, `UPDATE drive_office_webhook_events SET processed_at = now(), result = $1 WHERE id = $2`, result, webhookID)
	return DriveOfficeWebhookResult{ProviderEventID: eventID, Result: result, ProviderFileID: providerFileID, Revision: revision}, nil
}

func (s *DriveService) CreateEDiscoveryConnection(ctx context.Context, tenantID, actorUserID int64, provider string, auditCtx AuditContext) (DriveEDiscoveryConnection, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveEDiscoveryConnection{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "tenant_admin", "drive_legal_discovery_admin", "legal_admin"); err != nil {
		return DriveEDiscoveryConnection{}, err
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, tenantID)
	if err != nil {
		return DriveEDiscoveryConnection{}, err
	}
	if !policy.EDiscoveryProviderExportEnabled {
		return DriveEDiscoveryConnection{}, ErrDrivePolicyDenied
	}
	provider = driveStringDefault(provider, "fake")
	var out DriveEDiscoveryConnection
	err = s.pool.QueryRow(ctx, `
INSERT INTO drive_ediscovery_provider_connections (tenant_id, provider, status, created_by_user_id)
VALUES ($1, $2, 'active', $3)
ON CONFLICT (tenant_id, provider) DO UPDATE
SET status = 'active',
    updated_at = now()
RETURNING public_id::text, provider, status, created_at
`, tenantID, provider, actor.UserID).Scan(&out.PublicID, &out.Provider, &out.Status, &out.CreatedAt)
	if err != nil {
		return DriveEDiscoveryConnection{}, fmt.Errorf("create drive ediscovery connection: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.ediscovery.provider.connect", "drive_ediscovery_provider_connection", out.PublicID, map[string]any{"provider": provider})
	return out, nil
}

func (s *DriveService) RequestEDiscoveryExport(ctx context.Context, tenantID, actorUserID int64, connectionPublicID, casePublicID string, auditCtx AuditContext) (DriveEDiscoveryExport, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveEDiscoveryExport{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "drive_legal_discovery_admin", "legal_exporter"); err != nil {
		return DriveEDiscoveryExport{}, err
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, tenantID)
	if err != nil {
		return DriveEDiscoveryExport{}, err
	}
	if !policy.EDiscoveryProviderExportEnabled {
		return DriveEDiscoveryExport{}, ErrDrivePolicyDenied
	}
	var connectionID int64
	err = s.pool.QueryRow(ctx, `SELECT id FROM drive_ediscovery_provider_connections WHERE public_id = $1 AND tenant_id = $2 AND status = 'active'`, connectionPublicID, tenantID).Scan(&connectionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveEDiscoveryExport{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveEDiscoveryExport{}, fmt.Errorf("get drive ediscovery connection: %w", err)
	}
	var caseID int64
	err = s.pool.QueryRow(ctx, `SELECT id FROM drive_legal_cases WHERE public_id = $1 AND tenant_id = $2 AND status = 'active'`, casePublicID, tenantID).Scan(&caseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveEDiscoveryExport{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveEDiscoveryExport{}, fmt.Errorf("get drive legal case: %w", err)
	}
	manifestHash := driveStableHash(fmt.Sprintf("%d:%s:%d", tenantID, casePublicID, s.now().UnixNano()))
	var out DriveEDiscoveryExport
	err = s.pool.QueryRow(ctx, `
INSERT INTO drive_ediscovery_exports (tenant_id, case_id, case_public_id, provider_connection_id, requested_by_user_id, status, manifest_hash)
VALUES ($1, $2, $3, $4, $5, 'pending_approval', $6)
RETURNING public_id::text, case_public_id::text, status, manifest_hash, created_at
`, tenantID, caseID, casePublicID, connectionID, actor.UserID, manifestHash).Scan(&out.PublicID, &out.CasePublicID, &out.Status, &out.ManifestHash, &out.CreatedAt)
	if err != nil {
		return DriveEDiscoveryExport{}, fmt.Errorf("create drive ediscovery export: %w", err)
	}
	err = s.pool.QueryRow(ctx, `
WITH inserted AS (
    INSERT INTO drive_ediscovery_export_items (export_id, file_object_id, file_revision, content_sha256, status)
    SELECT e.id,
           f.id,
           COALESCE(NULLIF(f.office_last_revision, ''), '1'),
           COALESCE(NULLIF(f.content_sha256, ''), f.sha256_hex),
           CASE WHEN f.encryption_mode = 'zero_knowledge' THEN 'skipped' ELSE 'pending' END
    FROM drive_ediscovery_exports e
    JOIN drive_legal_case_resources r ON r.case_id = e.case_id AND r.tenant_id = e.tenant_id AND r.resource_type = 'file' AND r.hold_enabled
    JOIN file_objects f ON f.id = r.resource_id AND f.tenant_id = e.tenant_id AND f.deleted_at IS NULL
    WHERE e.public_id = $1 AND e.tenant_id = $2
    ON CONFLICT DO NOTHING
    RETURNING id
)
SELECT count(*) FROM inserted
`, out.PublicID, tenantID).Scan(&out.ItemCount)
	if err != nil {
		return DriveEDiscoveryExport{}, fmt.Errorf("create drive ediscovery export items: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.ediscovery.export.request", "drive_ediscovery_export", out.PublicID, map[string]any{"itemCount": out.ItemCount})
	return out, nil
}

func (s *DriveService) ApproveEDiscoveryExport(ctx context.Context, tenantID, actorUserID int64, exportPublicID string, auditCtx AuditContext) (DriveEDiscoveryExport, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveEDiscoveryExport{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "drive_legal_discovery_admin", "legal_reviewer"); err != nil {
		return DriveEDiscoveryExport{}, err
	}
	var requestedBy int64
	err = s.pool.QueryRow(ctx, `SELECT requested_by_user_id FROM drive_ediscovery_exports WHERE public_id = $1 AND tenant_id = $2 AND status = 'pending_approval'`, exportPublicID, tenantID).Scan(&requestedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveEDiscoveryExport{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveEDiscoveryExport{}, fmt.Errorf("get drive ediscovery export: %w", err)
	}
	if requestedBy == actor.UserID {
		return DriveEDiscoveryExport{}, ErrDrivePermissionDenied
	}
	providerExportID := "fake-ediscovery-export-" + exportPublicID
	_, err = s.pool.Exec(ctx, `
UPDATE drive_ediscovery_exports
SET status = 'exported',
    approved_by_user_id = $1,
    provider_export_id = $2,
    updated_at = now()
WHERE public_id = $3 AND tenant_id = $4
`, actor.UserID, providerExportID, exportPublicID, tenantID)
	if err != nil {
		return DriveEDiscoveryExport{}, fmt.Errorf("approve drive ediscovery export: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
UPDATE drive_ediscovery_export_items
SET status = 'uploaded',
    provider_item_id = 'fake-item-' || id::text
WHERE export_id = (SELECT id FROM drive_ediscovery_exports WHERE public_id = $1 AND tenant_id = $2)
  AND status = 'pending'
`, exportPublicID, tenantID)
	if err != nil {
		return DriveEDiscoveryExport{}, fmt.Errorf("approve drive ediscovery export items: %w", err)
	}
	out, err := s.getEDiscoveryExport(ctx, tenantID, exportPublicID)
	if err != nil {
		return DriveEDiscoveryExport{}, err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.ediscovery.export.approve", "drive_ediscovery_export", exportPublicID, nil)
	return out, nil
}

func (s *DriveService) CreateHSMDeployment(ctx context.Context, tenantID, actorUserID int64, provider, endpointURL string, auditCtx AuditContext) (DriveHSMDeployment, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveHSMDeployment{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "tenant_admin", "drive_hsm_admin", "drive_security_admin"); err != nil {
		return DriveHSMDeployment{}, err
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, tenantID)
	if err != nil {
		return DriveHSMDeployment{}, err
	}
	if !policy.HSMEnabled {
		return DriveHSMDeployment{}, ErrDrivePolicyDenied
	}
	provider = driveStringDefault(provider, "fake")
	endpointURL = driveStringDefault(endpointURL, "https://hsm.local.invalid")
	attestation := driveStableHash(provider + ":" + endpointURL)
	var out DriveHSMDeployment
	err = s.pool.QueryRow(ctx, `
WITH deployment AS (
    INSERT INTO drive_hsm_deployments (tenant_id, provider, endpoint_url, status, attestation_hash, health_status, last_health_checked_at, created_by_user_id)
    VALUES ($1, $2, $3, 'active', $4, 'healthy', now(), $5)
    ON CONFLICT (tenant_id, provider) DO UPDATE
    SET endpoint_url = EXCLUDED.endpoint_url,
        status = 'active',
        attestation_hash = EXCLUDED.attestation_hash,
        health_status = 'healthy',
        last_health_checked_at = now(),
        updated_at = now()
    RETURNING id, public_id, provider, endpoint_url, status, health_status, attestation_hash, created_at
), key AS (
    INSERT INTO drive_hsm_keys (tenant_id, deployment_id, key_ref, key_version, purpose, status)
    SELECT $1, id, 'fake-hsm-key', '1', 'drive_file', 'active' FROM deployment
    ON CONFLICT (tenant_id, key_ref, key_version) DO UPDATE
    SET status = 'active',
        updated_at = now()
    RETURNING public_id, status
)
SELECT d.public_id::text, d.provider, d.endpoint_url, d.status, d.health_status, d.attestation_hash, k.public_id::text, k.status, d.created_at
FROM deployment d CROSS JOIN key k
`, tenantID, provider, endpointURL, attestation, actor.UserID).Scan(&out.PublicID, &out.Provider, &out.EndpointURL, &out.Status, &out.HealthStatus, &out.AttestationHash, &out.KeyPublicID, &out.KeyStatus, &out.CreatedAt)
	if err != nil {
		return DriveHSMDeployment{}, fmt.Errorf("create drive hsm deployment: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.hsm.deployment.create", "drive_hsm_deployment", out.PublicID, map[string]any{"provider": provider})
	return out, nil
}

func (s *DriveService) BindHSMKeyToFile(ctx context.Context, tenantID, actorUserID int64, filePublicID, keyPublicID string, auditCtx AuditContext) error {
	actor, file, err := s.driveFileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "drive_hsm_admin", "drive_security_admin"); err != nil {
		return err
	}
	if err := s.authz.CanEditFile(ctx, actor, file); err != nil {
		return err
	}
	var keyID int64
	err = s.pool.QueryRow(ctx, `SELECT id FROM drive_hsm_keys WHERE public_id = $1 AND tenant_id = $2`, keyPublicID, tenantID).Scan(&keyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	}
	if err != nil {
		return fmt.Errorf("get drive hsm key: %w", err)
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM drive_hsm_key_bindings WHERE tenant_id = $1 AND binding_scope = 'file' AND file_object_id = $2`, tenantID, file.ID)
	if err != nil {
		return fmt.Errorf("delete existing drive hsm key binding: %w", err)
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO drive_hsm_key_bindings (tenant_id, file_object_id, hsm_key_id, binding_scope) VALUES ($1, $2, $3, 'file')`, tenantID, file.ID, keyID)
	if err != nil {
		return fmt.Errorf("insert drive hsm key binding: %w", err)
	}
	_, err = s.pool.Exec(ctx, `UPDATE file_objects SET encryption_mode = 'hsm_managed', updated_at = now() WHERE id = $1 AND tenant_id = $2`, file.ID, tenantID)
	if err != nil {
		return fmt.Errorf("mark drive hsm file: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.hsm.key.bind", "drive_file", file.PublicID, map[string]any{"keyPublicId": keyPublicID})
	return nil
}

func (s *DriveService) SetHSMKeyStatus(ctx context.Context, tenantID, actorUserID int64, keyPublicID, status string, auditCtx AuditContext) (DriveHSMDeployment, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveHSMDeployment{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "drive_hsm_admin", "drive_security_admin"); err != nil {
		return DriveHSMDeployment{}, err
	}
	switch status {
	case "active", "disabled", "destroyed", "unavailable":
	default:
		return DriveHSMDeployment{}, ErrDriveInvalidInput
	}
	tag, err := s.pool.Exec(ctx, `UPDATE drive_hsm_keys SET status = $1, updated_at = now() WHERE public_id = $2 AND tenant_id = $3`, status, keyPublicID, tenantID)
	if err != nil {
		return DriveHSMDeployment{}, fmt.Errorf("update drive hsm key status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return DriveHSMDeployment{}, ErrDriveNotFound
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.hsm.key.status_updated", "drive_hsm_key", keyPublicID, map[string]any{"status": status})
	return s.GetHSMDeploymentForKey(ctx, tenantID, keyPublicID)
}

func (s *DriveService) GetHSMDeploymentForKey(ctx context.Context, tenantID int64, keyPublicID string) (DriveHSMDeployment, error) {
	var out DriveHSMDeployment
	err := s.pool.QueryRow(ctx, `
SELECT d.public_id::text, d.provider, d.endpoint_url, d.status, d.health_status, COALESCE(d.attestation_hash, ''), k.public_id::text, k.status, d.created_at
FROM drive_hsm_keys k
JOIN drive_hsm_deployments d ON d.id = k.deployment_id
WHERE k.public_id = $1 AND k.tenant_id = $2
`, keyPublicID, tenantID).Scan(&out.PublicID, &out.Provider, &out.EndpointURL, &out.Status, &out.HealthStatus, &out.AttestationHash, &out.KeyPublicID, &out.KeyStatus, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveHSMDeployment{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveHSMDeployment{}, fmt.Errorf("get drive hsm deployment: %w", err)
	}
	return out, nil
}

func (s *DriveService) RegisterGateway(ctx context.Context, tenantID, actorUserID int64, name, endpointURL, fingerprint string, auditCtx AuditContext) (DriveGateway, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveGateway{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "tenant_admin", "drive_gateway_admin", "drive_security_admin"); err != nil {
		return DriveGateway{}, err
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, tenantID)
	if err != nil {
		return DriveGateway{}, err
	}
	if !policy.OnPremGatewayEnabled {
		return DriveGateway{}, ErrDrivePolicyDenied
	}
	name = driveStringDefault(name, "Local Fake Gateway")
	endpointURL = driveStringDefault(endpointURL, "https://gateway.local.invalid")
	fingerprint = driveStringDefault(fingerprint, driveStableHash(name)[:16])
	var out DriveGateway
	var lastSeen *time.Time
	err = s.pool.QueryRow(ctx, `
INSERT INTO drive_storage_gateways (tenant_id, name, status, endpoint_url, certificate_fingerprint, last_seen_at, created_by_user_id)
VALUES ($1, $2, 'active', $3, $4, now(), $5)
ON CONFLICT (tenant_id, name) DO UPDATE
SET status = 'active',
    endpoint_url = EXCLUDED.endpoint_url,
    certificate_fingerprint = EXCLUDED.certificate_fingerprint,
    last_seen_at = now(),
    updated_at = now()
RETURNING public_id::text, name, status, endpoint_url, certificate_fingerprint, last_seen_at, created_at
`, tenantID, name, endpointURL, fingerprint, actor.UserID).Scan(&out.PublicID, &out.Name, &out.Status, &out.EndpointURL, &out.CertificateFingerprint, &lastSeen, &out.CreatedAt)
	if err != nil {
		return DriveGateway{}, fmt.Errorf("register drive gateway: %w", err)
	}
	out.LastSeenAt = lastSeen
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.gateway.register", "drive_storage_gateway", out.PublicID, nil)
	return out, nil
}

func (s *DriveService) BindGatewayFile(ctx context.Context, tenantID, actorUserID int64, gatewayPublicID, filePublicID string, auditCtx AuditContext) (DriveGatewayObject, error) {
	actor, file, err := s.driveFileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return DriveGatewayObject{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "drive_gateway_admin", "drive_security_admin"); err != nil {
		return DriveGatewayObject{}, err
	}
	if err := s.authz.CanEditFile(ctx, actor, file); err != nil {
		return DriveGatewayObject{}, err
	}
	var gatewayID int64
	err = s.pool.QueryRow(ctx, `SELECT id FROM drive_storage_gateways WHERE public_id = $1 AND tenant_id = $2 AND status = 'active'`, gatewayPublicID, tenantID).Scan(&gatewayID)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveGatewayObject{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveGatewayObject{}, fmt.Errorf("get drive gateway: %w", err)
	}
	manifestHash := driveStableHash(file.PublicID + ":" + file.SHA256Hex)
	var out DriveGatewayObject
	err = s.pool.QueryRow(ctx, `
INSERT INTO drive_gateway_objects (tenant_id, gateway_id, file_object_id, gateway_object_key, manifest_hash, replication_status, last_verified_at)
VALUES ($1, $2, $3, $4, $5, 'active', now())
ON CONFLICT (file_object_id) DO UPDATE
SET gateway_id = EXCLUDED.gateway_id,
    gateway_object_key = EXCLUDED.gateway_object_key,
    manifest_hash = EXCLUDED.manifest_hash,
    replication_status = 'active',
    last_verified_at = now()
RETURNING $6::text, $7::text, manifest_hash, replication_status
`, tenantID, gatewayID, file.ID, "gateway/"+file.PublicID, manifestHash, gatewayPublicID, file.PublicID).Scan(&out.GatewayPublicID, &out.FilePublicID, &out.ManifestHash, &out.Status)
	if err != nil {
		return DriveGatewayObject{}, fmt.Errorf("bind drive gateway object: %w", err)
	}
	_, err = s.pool.Exec(ctx, `UPDATE file_objects SET storage_driver = 'onprem_gateway', storage_gateway_id = $1, updated_at = now() WHERE id = $2 AND tenant_id = $3`, gatewayID, file.ID, tenantID)
	if err != nil {
		return DriveGatewayObject{}, fmt.Errorf("mark drive gateway file: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.gateway.upload.complete", "drive_file", file.PublicID, map[string]any{"gatewayPublicId": gatewayPublicID})
	return out, nil
}

func (s *DriveService) SetGatewayStatus(ctx context.Context, tenantID, actorUserID int64, gatewayPublicID, status string, auditCtx AuditContext) (DriveGateway, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveGateway{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "drive_gateway_admin", "drive_security_admin"); err != nil {
		return DriveGateway{}, err
	}
	switch status {
	case "active", "disabled", "disconnected":
	default:
		return DriveGateway{}, ErrDriveInvalidInput
	}
	var out DriveGateway
	var lastSeen *time.Time
	err = s.pool.QueryRow(ctx, `
UPDATE drive_storage_gateways
SET status = $1,
    updated_at = now()
WHERE public_id = $2 AND tenant_id = $3
RETURNING public_id::text, name, status, endpoint_url, certificate_fingerprint, last_seen_at, created_at
`, status, gatewayPublicID, tenantID).Scan(&out.PublicID, &out.Name, &out.Status, &out.EndpointURL, &out.CertificateFingerprint, &lastSeen, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveGateway{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveGateway{}, fmt.Errorf("update drive gateway status: %w", err)
	}
	out.LastSeenAt = lastSeen
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.gateway.disable", "drive_storage_gateway", gatewayPublicID, map[string]any{"status": status})
	return out, nil
}

func (s *DriveService) CreateE2EEUserKey(ctx context.Context, tenantID, actorUserID int64, algorithm string, publicKey map[string]any, auditCtx AuditContext) (DriveE2EEUserKey, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveE2EEUserKey{}, err
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, tenantID)
	if err != nil {
		return DriveE2EEUserKey{}, err
	}
	if !policy.E2EEEnabled {
		return DriveE2EEUserKey{}, ErrDrivePolicyDenied
	}
	if algorithm == "" {
		algorithm = "X25519+A256GCM"
	}
	keyJSON, _ := json.Marshal(publicKey)
	if len(keyJSON) == 0 || string(keyJSON) == "null" {
		keyJSON = []byte(`{"kty":"OKP","crv":"X25519","x":"local-fake"}`)
	}
	var out DriveE2EEUserKey
	_, err = s.pool.Exec(ctx, `
UPDATE drive_e2ee_user_keys SET status = 'retired', rotated_at = now()
WHERE tenant_id = $1 AND user_id = $2 AND status = 'active'
`, tenantID, actor.UserID)
	if err != nil {
		return DriveE2EEUserKey{}, fmt.Errorf("retire active e2ee user key: %w", err)
	}
	err = s.pool.QueryRow(ctx, `
INSERT INTO drive_e2ee_user_keys (tenant_id, user_id, key_algorithm, public_key_jwk, status)
VALUES ($1, $2, $3, $4::jsonb, 'active')
RETURNING public_id::text, key_algorithm, status, created_at
`, tenantID, actor.UserID, algorithm, string(keyJSON)).Scan(&out.PublicID, &out.Algorithm, &out.Status, &out.CreatedAt)
	if err != nil {
		return DriveE2EEUserKey{}, fmt.Errorf("create drive e2ee user key: %w", err)
	}
	out.UserPublicID = actor.PublicID
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.e2ee.user_key.create", "drive_e2ee_user_key", out.PublicID, nil)
	return out, nil
}

func (s *DriveService) CreateE2EEFileKey(ctx context.Context, tenantID, actorUserID int64, filePublicID, algorithm, ciphertextSHA256, wrappedFileKey, wrapAlgorithm string, metadata map[string]any, auditCtx AuditContext) (DriveE2EEFileKey, error) {
	actor, file, err := s.driveFileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return DriveE2EEFileKey{}, err
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, tenantID)
	if err != nil {
		return DriveE2EEFileKey{}, err
	}
	if !policy.E2EEEnabled {
		return DriveE2EEFileKey{}, ErrDrivePolicyDenied
	}
	if err := s.authz.CanEditFile(ctx, actor, file); err != nil {
		return DriveE2EEFileKey{}, err
	}
	userKeyID, err := s.activeE2EEUserKeyID(ctx, tenantID, actor.UserID)
	if err != nil {
		return DriveE2EEFileKey{}, err
	}
	algorithm = driveStringDefault(algorithm, "AES-GCM-256")
	wrapAlgorithm = driveStringDefault(wrapAlgorithm, "X25519+A256KW")
	ciphertextSHA256 = driveStringDefault(ciphertextSHA256, file.SHA256Hex)
	keyBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(wrappedFileKey))
	if err != nil || len(keyBytes) == 0 {
		return DriveE2EEFileKey{}, ErrDriveInvalidInput
	}
	metadataJSON, _ := json.Marshal(metadata)
	if len(metadataJSON) == 0 || string(metadataJSON) == "null" {
		metadataJSON = []byte(`{}`)
	}
	var out DriveE2EEFileKey
	err = s.pool.QueryRow(ctx, `
WITH next_version AS (
    SELECT COALESCE(MAX(key_version), 0) + 1 AS value
    FROM drive_e2ee_file_keys
    WHERE file_object_id = $2
), file_key AS (
    INSERT INTO drive_e2ee_file_keys (tenant_id, file_object_id, key_version, encryption_algorithm, ciphertext_sha256, encrypted_metadata, created_by_user_id)
    SELECT $1, $2, value, $3, $4, $5::jsonb, $6 FROM next_version
    RETURNING id, public_id, key_version, encryption_algorithm, ciphertext_sha256, created_at
), envelope AS (
    INSERT INTO drive_e2ee_key_envelopes (tenant_id, file_key_id, recipient_user_id, recipient_key_id, wrapped_file_key, wrap_algorithm, created_by_user_id)
    SELECT $1, id, $6, $7, $8, $9, $6 FROM file_key
    RETURNING id
)
SELECT public_id::text, key_version, encryption_algorithm, ciphertext_sha256, created_at FROM file_key
`, tenantID, file.ID, algorithm, ciphertextSHA256, string(metadataJSON), actor.UserID, userKeyID, keyBytes, wrapAlgorithm).Scan(&out.PublicID, &out.KeyVersion, &out.Algorithm, &out.CiphertextSHA256, &out.CreatedAt)
	if err != nil {
		return DriveE2EEFileKey{}, fmt.Errorf("create drive e2ee file key: %w", err)
	}
	_, err = s.pool.Exec(ctx, `UPDATE file_objects SET encryption_mode = 'zero_knowledge', e2ee_file_key_public_id = $1, updated_at = now() WHERE id = $2 AND tenant_id = $3`, out.PublicID, file.ID, tenantID)
	if err != nil {
		return DriveE2EEFileKey{}, fmt.Errorf("mark drive e2ee file: %w", err)
	}
	out.FilePublicID = file.PublicID
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.e2ee.file.create", "drive_file", file.PublicID, map[string]any{"keyVersion": out.KeyVersion})
	return out, nil
}

func (s *DriveService) CreateE2EERecipientEnvelope(ctx context.Context, tenantID, actorUserID int64, filePublicID, recipientUserPublicID, wrappedFileKey, wrapAlgorithm string, auditCtx AuditContext) (DriveE2EEEnvelope, error) {
	actor, file, err := s.driveFileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return DriveE2EEEnvelope{}, err
	}
	if err := s.authz.CanShareFile(ctx, actor, file); err != nil {
		return DriveE2EEEnvelope{}, err
	}
	var fileKeyID int64
	var fileKeyPublicID string
	err = s.pool.QueryRow(ctx, `SELECT id, public_id::text FROM drive_e2ee_file_keys WHERE file_object_id = $1 ORDER BY key_version DESC LIMIT 1`, file.ID).Scan(&fileKeyID, &fileKeyPublicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveE2EEEnvelope{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveE2EEEnvelope{}, fmt.Errorf("get drive e2ee file key: %w", err)
	}
	var recipientUserID int64
	var recipientKeyID int64
	err = s.pool.QueryRow(ctx, `
SELECT u.id, k.id
FROM users u
JOIN drive_e2ee_user_keys k ON k.user_id = u.id AND k.tenant_id = $1 AND k.status = 'active'
WHERE u.public_id = $2
`, tenantID, recipientUserPublicID).Scan(&recipientUserID, &recipientKeyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveE2EEEnvelope{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveE2EEEnvelope{}, fmt.Errorf("get recipient e2ee key: %w", err)
	}
	keyBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(wrappedFileKey))
	if err != nil || len(keyBytes) == 0 {
		return DriveE2EEEnvelope{}, ErrDriveInvalidInput
	}
	wrapAlgorithm = driveStringDefault(wrapAlgorithm, "X25519+A256KW")
	var out DriveE2EEEnvelope
	err = s.pool.QueryRow(ctx, `
INSERT INTO drive_e2ee_key_envelopes (tenant_id, file_key_id, recipient_user_id, recipient_key_id, wrapped_file_key, wrap_algorithm, created_by_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (file_key_id, recipient_user_id, recipient_key_id) DO UPDATE
SET wrapped_file_key = EXCLUDED.wrapped_file_key,
    wrap_algorithm = EXCLUDED.wrap_algorithm,
    revoked_at = NULL
RETURNING encode(wrapped_file_key, 'base64'), wrap_algorithm, created_at
`, tenantID, fileKeyID, recipientUserID, recipientKeyID, keyBytes, wrapAlgorithm, actor.UserID).Scan(&out.WrappedFileKey, &out.WrapAlgorithm, &out.CreatedAt)
	if err != nil {
		return DriveE2EEEnvelope{}, fmt.Errorf("create drive e2ee envelope: %w", err)
	}
	out.FileKeyPublicID = fileKeyPublicID
	out.RecipientUserID = recipientUserPublicID
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.e2ee.envelope.create", "drive_file", file.PublicID, map[string]any{"recipientUserPublicId": recipientUserPublicID})
	return out, nil
}

func (s *DriveService) GetE2EEEnvelope(ctx context.Context, tenantID, actorUserID int64, filePublicID string, auditCtx AuditContext) (DriveE2EEEnvelope, error) {
	actor, file, err := s.driveFileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return DriveE2EEEnvelope{}, err
	}
	if err := s.authz.CanDownloadFile(ctx, actor, file); err != nil {
		return DriveE2EEEnvelope{}, err
	}
	var out DriveE2EEEnvelope
	err = s.pool.QueryRow(ctx, `
SELECT fk.public_id::text, u.public_id::text, encode(e.wrapped_file_key, 'base64'), e.wrap_algorithm, e.created_at
FROM drive_e2ee_file_keys fk
JOIN drive_e2ee_key_envelopes e ON e.file_key_id = fk.id AND e.revoked_at IS NULL
JOIN users u ON u.id = e.recipient_user_id
WHERE fk.file_object_id = $1 AND e.recipient_user_id = $2
ORDER BY fk.key_version DESC
LIMIT 1
`, file.ID, actor.UserID).Scan(&out.FileKeyPublicID, &out.RecipientUserID, &out.WrappedFileKey, &out.WrapAlgorithm, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveE2EEEnvelope{}, ErrDrivePermissionDenied
	}
	if err != nil {
		return DriveE2EEEnvelope{}, fmt.Errorf("get drive e2ee envelope: %w", err)
	}
	return out, nil
}

func (s *DriveService) RevokeE2EEEnvelope(ctx context.Context, tenantID, actorUserID int64, filePublicID, recipientUserPublicID string, auditCtx AuditContext) error {
	actor, file, err := s.driveFileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return err
	}
	if err := s.authz.CanShareFile(ctx, actor, file); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE drive_e2ee_key_envelopes e
SET revoked_at = now()
FROM drive_e2ee_file_keys fk, users u
WHERE e.file_key_id = fk.id
  AND fk.file_object_id = $1
  AND e.recipient_user_id = u.id
  AND u.public_id = $2
  AND e.revoked_at IS NULL
`, file.ID, recipientUserPublicID)
	if err != nil {
		return fmt.Errorf("revoke drive e2ee envelope: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDriveNotFound
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.e2ee.envelope.revoke", "drive_file", file.PublicID, map[string]any{"recipientUserPublicId": recipientUserPublicID})
	return nil
}

func (s *DriveService) CreateAIJob(ctx context.Context, tenantID, actorUserID int64, filePublicID, jobType string, auditCtx AuditContext) (DriveAIJob, error) {
	actor, file, err := s.driveFileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return DriveAIJob{}, err
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, tenantID)
	if err != nil {
		return DriveAIJob{}, err
	}
	if !policy.AIEnabled {
		return DriveAIJob{}, ErrDrivePolicyDenied
	}
	if s.driveFileUsesZeroKnowledgeEncryption(ctx, tenantID, file.ID) {
		return DriveAIJob{}, ErrDrivePolicyDenied
	}
	if err := s.authz.CanEditFile(ctx, actor, file); err != nil {
		return DriveAIJob{}, err
	}
	if err := s.ensureFileDownloadAllowed(ctx, actor, file, auditCtx, "drive.ai.job.create"); err != nil {
		return DriveAIJob{}, err
	}
	jobType = driveStringDefault(jobType, "summary")
	if jobType != "summary" && jobType != "classification" {
		return DriveAIJob{}, ErrDriveInvalidInput
	}
	provider := FakeDriveAIProvider{}
	revision := driveFileContentRevision(file)
	var out DriveAIJob
	err = s.pool.QueryRow(ctx, `
INSERT INTO drive_ai_jobs (tenant_id, file_object_id, file_revision, job_type, provider, status, requested_by_user_id)
VALUES ($1, $2, $3, $4, 'fake', 'completed', $5)
ON CONFLICT (file_object_id, file_revision, job_type) DO UPDATE
SET status = 'completed',
    requested_by_user_id = EXCLUDED.requested_by_user_id,
    updated_at = now()
RETURNING public_id::text, job_type, provider, status, created_at
`, tenantID, file.ID, revision, jobType, actor.UserID).Scan(&out.PublicID, &out.JobType, &out.Provider, &out.Status, &out.CreatedAt)
	if err != nil {
		return DriveAIJob{}, fmt.Errorf("create drive ai job: %w", err)
	}
	out.FilePublicID = file.PublicID
	if jobType == "summary" {
		summary, err := provider.Summarize(ctx, file)
		if err != nil {
			return DriveAIJob{}, err
		}
		_, err = s.pool.Exec(ctx, `
INSERT INTO drive_ai_summaries (tenant_id, file_object_id, file_revision, summary_text, provider, input_hash)
VALUES ($1, $2, $3, $4, 'fake', $5)
ON CONFLICT (file_object_id, file_revision) DO UPDATE
SET summary_text = EXCLUDED.summary_text,
    provider = EXCLUDED.provider,
    input_hash = EXCLUDED.input_hash
`, tenantID, file.ID, revision, summary, driveStableHash(file.SHA256Hex))
		if err != nil {
			return DriveAIJob{}, fmt.Errorf("save drive ai summary: %w", err)
		}
	} else {
		labels, err := provider.Classify(ctx, file)
		if err != nil {
			return DriveAIJob{}, err
		}
		for _, label := range labels {
			_, err = s.pool.Exec(ctx, `
INSERT INTO drive_ai_classifications (tenant_id, file_object_id, file_revision, label, confidence, provider)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (file_object_id, file_revision, label) DO UPDATE
SET confidence = EXCLUDED.confidence,
    provider = EXCLUDED.provider
`, tenantID, file.ID, revision, label.Label, label.Confidence, label.Provider)
			if err != nil {
				return DriveAIJob{}, fmt.Errorf("save drive ai classification: %w", err)
			}
		}
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.ai.job.create", "drive_file", file.PublicID, map[string]any{"jobType": jobType})
	return out, nil
}

func (s *DriveService) GetAISummary(ctx context.Context, tenantID, actorUserID int64, filePublicID string, auditCtx AuditContext) (DriveAISummary, error) {
	actor, file, err := s.driveFileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return DriveAISummary{}, err
	}
	if err := s.authz.CanViewFile(ctx, actor, file); err != nil {
		return DriveAISummary{}, err
	}
	var out DriveAISummary
	err = s.pool.QueryRow(ctx, `
SELECT public_id::text, summary_text, provider, created_at
FROM drive_ai_summaries
WHERE tenant_id = $1 AND file_object_id = $2
ORDER BY created_at DESC
LIMIT 1
`, tenantID, file.ID).Scan(&out.PublicID, &out.SummaryText, &out.Provider, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveAISummary{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveAISummary{}, fmt.Errorf("get drive ai summary: %w", err)
	}
	out.FilePublicID = file.PublicID
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.ai.result.view", "drive_file", file.PublicID, map[string]any{"kind": "summary"})
	return out, nil
}

func (s *DriveService) ListAIClassifications(ctx context.Context, tenantID, actorUserID int64, filePublicID string) ([]DriveAIClassification, error) {
	actor, file, err := s.driveFileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return nil, err
	}
	if err := s.authz.CanViewFile(ctx, actor, file); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT label, confidence::float8, provider, created_at
FROM drive_ai_classifications
WHERE tenant_id = $1 AND file_object_id = $2
ORDER BY confidence DESC, label ASC
`, tenantID, file.ID)
	if err != nil {
		return nil, fmt.Errorf("list drive ai classifications: %w", err)
	}
	defer rows.Close()
	items := []DriveAIClassification{}
	for rows.Next() {
		var item DriveAIClassification
		if err := rows.Scan(&item.Label, &item.Confidence, &item.Provider, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan drive ai classification: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *DriveService) ListMarketplaceApps(ctx context.Context) ([]DriveMarketplaceApp, error) {
	rows, err := s.pool.Query(ctx, `
SELECT a.public_id::text, a.slug, a.name, a.publisher_name, v.version, v.manifest_json
FROM drive_marketplace_apps a
JOIN drive_marketplace_app_versions v ON v.app_id = a.id AND v.review_status = 'approved'
WHERE a.status = 'reviewed'
ORDER BY a.name ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list drive marketplace apps: %w", err)
	}
	defer rows.Close()
	items := []DriveMarketplaceApp{}
	for rows.Next() {
		var item DriveMarketplaceApp
		var manifest []byte
		if err := rows.Scan(&item.PublicID, &item.Slug, &item.Name, &item.PublisherName, &item.Version, &manifest); err != nil {
			return nil, fmt.Errorf("scan drive marketplace app: %w", err)
		}
		item.Scopes = driveMarketplaceManifestScopes(manifest)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *DriveService) InstallMarketplaceApp(ctx context.Context, tenantID, actorUserID int64, appSlug string, auditCtx AuditContext) (DriveMarketplaceInstallation, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveMarketplaceInstallation{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "tenant_admin", "drive_marketplace_admin"); err != nil {
		return DriveMarketplaceInstallation{}, err
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, tenantID)
	if err != nil {
		return DriveMarketplaceInstallation{}, err
	}
	if !policy.MarketplaceEnabled {
		return DriveMarketplaceInstallation{}, ErrDrivePolicyDenied
	}
	var appID, versionID int64
	var appName string
	var manifest []byte
	err = s.pool.QueryRow(ctx, `
SELECT a.id, v.id, a.name, v.manifest_json
FROM drive_marketplace_apps a
JOIN drive_marketplace_app_versions v ON v.app_id = a.id AND v.review_status = 'approved'
WHERE a.slug = $1 AND a.status = 'reviewed'
ORDER BY v.created_at DESC
LIMIT 1
`, appSlug).Scan(&appID, &versionID, &appName, &manifest)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveMarketplaceInstallation{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveMarketplaceInstallation{}, fmt.Errorf("get drive marketplace app: %w", err)
	}
	scopes := driveMarketplaceManifestScopes(manifest)
	var out DriveMarketplaceInstallation
	err = s.pool.QueryRow(ctx, `
WITH installation AS (
    INSERT INTO drive_marketplace_installations (tenant_id, app_id, app_version_id, status, installed_by_user_id)
    VALUES ($1, $2, $3, 'pending_approval', $4)
    ON CONFLICT (tenant_id, app_id) DO UPDATE
    SET app_version_id = EXCLUDED.app_version_id,
        status = 'pending_approval',
        installed_by_user_id = EXCLUDED.installed_by_user_id,
        approved_by_user_id = NULL,
        updated_at = now()
    RETURNING id, public_id, status, created_at
)
SELECT public_id::text, status, created_at FROM installation
`, tenantID, appID, versionID, actor.UserID).Scan(&out.PublicID, &out.Status, &out.CreatedAt)
	if err != nil {
		return DriveMarketplaceInstallation{}, fmt.Errorf("install drive marketplace app: %w", err)
	}
	for _, scope := range scopes {
		_, err = s.pool.Exec(ctx, `
INSERT INTO drive_marketplace_installation_scopes (installation_id, scope)
SELECT id, $1 FROM drive_marketplace_installations WHERE public_id = $2 AND tenant_id = $3
ON CONFLICT DO NOTHING
`, scope, out.PublicID, tenantID)
		if err != nil {
			return DriveMarketplaceInstallation{}, fmt.Errorf("create drive marketplace app scope: %w", err)
		}
	}
	out.AppSlug = appSlug
	out.AppName = appName
	out.Scopes = scopes
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.marketplace.install.request", "drive_marketplace_installation", out.PublicID, map[string]any{"appSlug": appSlug})
	return out, nil
}

func (s *DriveService) ApproveMarketplaceInstallation(ctx context.Context, tenantID, actorUserID int64, installationPublicID string, auditCtx AuditContext) (DriveMarketplaceInstallation, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveMarketplaceInstallation{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "tenant_admin", "drive_marketplace_admin"); err != nil {
		return DriveMarketplaceInstallation{}, err
	}
	var installedBy int64
	err = s.pool.QueryRow(ctx, `SELECT installed_by_user_id FROM drive_marketplace_installations WHERE public_id = $1 AND tenant_id = $2 AND status = 'pending_approval'`, installationPublicID, tenantID).Scan(&installedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveMarketplaceInstallation{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveMarketplaceInstallation{}, fmt.Errorf("get drive marketplace installation: %w", err)
	}
	if installedBy == actor.UserID {
		return DriveMarketplaceInstallation{}, ErrDrivePermissionDenied
	}
	_, err = s.pool.Exec(ctx, `UPDATE drive_marketplace_installations SET status = 'active', approved_by_user_id = $1, updated_at = now() WHERE public_id = $2 AND tenant_id = $3`, actor.UserID, installationPublicID, tenantID)
	if err != nil {
		return DriveMarketplaceInstallation{}, fmt.Errorf("approve drive marketplace installation: %w", err)
	}
	out, err := s.getMarketplaceInstallation(ctx, tenantID, installationPublicID)
	if err != nil {
		return DriveMarketplaceInstallation{}, err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.marketplace.install.approve", "drive_marketplace_installation", installationPublicID, nil)
	return out, nil
}

func (s *DriveService) UninstallMarketplaceInstallation(ctx context.Context, tenantID, actorUserID int64, installationPublicID string, auditCtx AuditContext) error {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "tenant_admin", "drive_marketplace_admin"); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE drive_marketplace_installations SET status = 'uninstalled', updated_at = now() WHERE public_id = $1 AND tenant_id = $2`, installationPublicID, tenantID)
	if err != nil {
		return fmt.Errorf("uninstall drive marketplace installation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDriveNotFound
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.marketplace.uninstall", "drive_marketplace_installation", installationPublicID, nil)
	return nil
}

func (s *DriveService) CheckMarketplaceScope(ctx context.Context, tenantID, actorUserID int64, installationPublicID, scope, filePublicID string) error {
	actor, file, err := s.driveFileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return err
	}
	var exists bool
	err = s.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM drive_marketplace_installations i
    JOIN drive_marketplace_installation_scopes s ON s.installation_id = i.id
    WHERE i.public_id = $1 AND i.tenant_id = $2 AND i.status = 'active' AND s.scope = $3
)
`, installationPublicID, tenantID, scope).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check drive marketplace scope: %w", err)
	}
	if !exists {
		return ErrDrivePermissionDenied
	}
	switch scope {
	case "drive.file.read":
		return s.authz.CanViewFile(ctx, actor, file)
	case "drive.file.write":
		return s.authz.CanEditFile(ctx, actor, file)
	default:
		return ErrDrivePermissionDenied
	}
}

func (s *DriveService) ensureDriveHSMAvailable(ctx context.Context, tenantID, fileObjectID int64) error {
	var status, deploymentStatus, healthStatus string
	err := s.pool.QueryRow(ctx, `
SELECT k.status, d.status, d.health_status
FROM drive_hsm_key_bindings b
JOIN drive_hsm_keys k ON k.id = b.hsm_key_id
JOIN drive_hsm_deployments d ON d.id = k.deployment_id
WHERE b.tenant_id = $1
  AND (
      (b.binding_scope = 'file' AND b.file_object_id = $2)
      OR b.binding_scope = 'tenant'
  )
ORDER BY CASE WHEN b.binding_scope = 'file' THEN 0 ELSE 1 END
LIMIT 1
`, tenantID, fileObjectID).Scan(&status, &deploymentStatus, &healthStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check drive hsm key: %w", err)
	}
	if status != "active" || deploymentStatus != "active" || healthStatus != "healthy" {
		return ErrDrivePolicyDenied
	}
	return nil
}

func (s *DriveService) ensureDriveGatewayAvailable(ctx context.Context, tenantID, gatewayID int64) error {
	if gatewayID <= 0 {
		return nil
	}
	var status string
	err := s.pool.QueryRow(ctx, `SELECT status FROM drive_storage_gateways WHERE id = $1 AND tenant_id = $2`, gatewayID, tenantID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDrivePolicyDenied
	}
	if err != nil {
		return fmt.Errorf("check drive gateway: %w", err)
	}
	if status != "active" {
		return ErrDrivePolicyDenied
	}
	return nil
}

func (s *DriveService) driveFileForActor(ctx context.Context, tenantID, actorUserID int64, filePublicID string) (DriveActor, DriveFile, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveActor{}, DriveFile{}, err
	}
	row, err := s.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: filePublicID})
	if err != nil {
		return DriveActor{}, DriveFile{}, err
	}
	return actor, driveFileFromDB(row), nil
}

func (s *DriveService) driveFileUsesZeroKnowledgeEncryption(ctx context.Context, tenantID, fileID int64) bool {
	var mode string
	err := s.pool.QueryRow(ctx, `SELECT encryption_mode FROM file_objects WHERE id = $1 AND tenant_id = $2`, fileID, tenantID).Scan(&mode)
	return err == nil && mode == "zero_knowledge"
}

func (s *DriveService) activeE2EEUserKeyID(ctx context.Context, tenantID, userID int64) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT id FROM drive_e2ee_user_keys WHERE tenant_id = $1 AND user_id = $2 AND status = 'active'`, tenantID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrDriveNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get active e2ee user key: %w", err)
	}
	return id, nil
}

func (s *DriveService) getEDiscoveryExport(ctx context.Context, tenantID int64, publicID string) (DriveEDiscoveryExport, error) {
	var out DriveEDiscoveryExport
	err := s.pool.QueryRow(ctx, `
SELECT e.public_id::text,
       COALESCE(e.case_public_id::text, ''),
       e.status,
       COALESCE(e.manifest_hash, ''),
       COALESCE(e.provider_export_id, ''),
       count(i.id)::int,
       e.created_at
FROM drive_ediscovery_exports e
LEFT JOIN drive_ediscovery_export_items i ON i.export_id = e.id
WHERE e.public_id = $1 AND e.tenant_id = $2
GROUP BY e.id
`, publicID, tenantID).Scan(&out.PublicID, &out.CasePublicID, &out.Status, &out.ManifestHash, &out.ProviderExportID, &out.ItemCount, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveEDiscoveryExport{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveEDiscoveryExport{}, fmt.Errorf("get drive ediscovery export: %w", err)
	}
	return out, nil
}

func (s *DriveService) getMarketplaceInstallation(ctx context.Context, tenantID int64, publicID string) (DriveMarketplaceInstallation, error) {
	var out DriveMarketplaceInstallation
	rows, err := s.pool.Query(ctx, `
SELECT i.public_id::text, a.slug, a.name, i.status, COALESCE(sc.scope, ''), i.created_at
FROM drive_marketplace_installations i
JOIN drive_marketplace_apps a ON a.id = i.app_id
LEFT JOIN drive_marketplace_installation_scopes sc ON sc.installation_id = i.id
WHERE i.public_id = $1 AND i.tenant_id = $2
ORDER BY sc.scope ASC
`, publicID, tenantID)
	if err != nil {
		return DriveMarketplaceInstallation{}, fmt.Errorf("get drive marketplace installation: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var scope string
		if err := rows.Scan(&out.PublicID, &out.AppSlug, &out.AppName, &out.Status, &scope, &out.CreatedAt); err != nil {
			return DriveMarketplaceInstallation{}, fmt.Errorf("scan drive marketplace installation: %w", err)
		}
		if scope != "" {
			out.Scopes = append(out.Scopes, scope)
		}
	}
	if out.PublicID == "" {
		return DriveMarketplaceInstallation{}, ErrDriveNotFound
	}
	return out, rows.Err()
}

func driveOfficeCoauthoringSupported(file DriveFile) bool {
	ext := strings.ToLower(filepath.Ext(file.OriginalFilename))
	switch ext {
	case ".docx", ".xlsx", ".pptx":
		return true
	}
	contentType := strings.ToLower(file.ContentType)
	return strings.Contains(contentType, "wordprocessingml") ||
		strings.Contains(contentType, "spreadsheetml") ||
		strings.Contains(contentType, "presentationml")
}

func driveProviderRevisionNewer(next, current string) bool {
	nextInt, nextErr := strconv.ParseInt(strings.TrimSpace(next), 10, 64)
	currentInt, currentErr := strconv.ParseInt(strings.TrimSpace(current), 10, 64)
	if nextErr == nil && currentErr == nil {
		return nextInt > currentInt
	}
	return strings.TrimSpace(next) > strings.TrimSpace(current)
}

func driveStableHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func driveUniqueTokenSuffix() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func driveStringDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func driveMarketplaceManifestScopes(raw []byte) []string {
	var manifest struct {
		RequestedScopes []string `json:"requestedScopes"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil || len(manifest.RequestedScopes) == 0 {
		return []string{"drive.file.read"}
	}
	return manifest.RequestedScopes
}

func driveFileContentRevision(file DriveFile) string {
	if strings.TrimSpace(file.SHA256Hex) != "" {
		return "1"
	}
	return "1"
}
