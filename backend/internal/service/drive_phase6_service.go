package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

const DriveShareLinkPasswordCookieName = "DRIVE_SHARE_LINK_VERIFICATION"

type DriveShareLinkPasswordVerification struct {
	CookieName  string
	CookieValue string
	ExpiresAt   time.Time
}

type shareLinkPasswordState struct {
	ID                int64
	PublicID          string
	TenantID          int64
	TokenHash         string
	PasswordRequired  bool
	PasswordHash      string
	PasswordUpdatedAt time.Time
	Status            string
	ExpiresAt         time.Time
}

func (s *DriveService) CreateShareInvitation(ctx context.Context, input DriveCreateShareInvitationInput, auditCtx AuditContext) (DriveShareInvitation, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveShareInvitation{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveShareInvitation{}, err
	}
	resource, err := s.resolveShareableResource(ctx, actor, input.Resource)
	if err != nil {
		return DriveShareInvitation{}, err
	}
	if err := s.ensureResourceShareAllowed(ctx, actor, resource, auditCtx); err != nil {
		return DriveShareInvitation{}, err
	}
	role := normalizeDriveRole(input.Role)
	if role == "" {
		return DriveShareInvitation{}, fmt.Errorf("%w: invitation role is required", ErrDriveInvalidInput)
	}
	email := normalizeEmail(input.InviteeEmail)
	if email == "" {
		return DriveShareInvitation{}, fmt.Errorf("%w: invitee email is required", ErrDriveInvalidInput)
	}
	domain := emailDomain(email)
	if domain == "" {
		return DriveShareInvitation{}, fmt.Errorf("%w: invitee email domain is required", ErrDriveInvalidInput)
	}
	policy, err := s.drivePolicy(ctx, input.TenantID)
	if err != nil {
		return DriveShareInvitation{}, err
	}
	if !policy.ExternalUserSharingEnabled {
		s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.share.external_denied", "drive_"+string(resource.Type), resource.PublicID, map[string]any{
			"reason": "external_sharing_disabled",
		})
		return DriveShareInvitation{}, ErrDrivePolicyDenied
	}
	if ok, reason := drivePolicyAllowsExternalDomain(policy, domain); !ok {
		s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.share.external_denied", "drive_"+string(resource.Type), resource.PublicID, map[string]any{
			"reason": reason,
			"domain": domain,
		})
		return DriveShareInvitation{}, ErrDrivePolicyDenied
	}

	var inviteeUserID *int64
	if strings.TrimSpace(input.InviteeUserPublicID) != "" {
		userID, err := s.lookupUserIDByPublicID(ctx, input.InviteeUserPublicID)
		if err != nil {
			return DriveShareInvitation{}, err
		}
		inviteeUserID = &userID
	} else if user, err := s.queries.GetUserByEmail(ctx, email); err == nil {
		id := user.ID
		inviteeUserID = &id
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return DriveShareInvitation{}, fmt.Errorf("lookup invitee user: %w", err)
	}

	expiresAt := input.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = s.now().Add(7 * 24 * time.Hour)
	}
	if !expiresAt.After(s.now()) {
		return DriveShareInvitation{}, fmt.Errorf("%w: invitation expiry must be in the future", ErrDriveInvalidInput)
	}
	rawToken, tokenHash, err := newDriveShareLinkToken()
	if err != nil {
		return DriveShareInvitation{}, err
	}
	status := "pending"
	if policy.RequireExternalShareApproval {
		status = "pending_approval"
	}
	var invitation DriveShareInvitation
	var inviteeUser pgtype.Int8
	var approvedBy pgtype.Int8
	var approvedAt pgtype.Timestamptz
	var acceptedAt pgtype.Timestamptz
	err = s.pool.QueryRow(ctx, `
INSERT INTO drive_share_invitations (
    tenant_id,
    resource_type,
    resource_id,
    invitee_email_hash,
    invitee_email_domain,
    invitee_user_id,
    role,
    status,
    expires_at,
    created_by_user_id,
    accept_token_hash,
    accept_token_expires_at,
    masked_invitee_email
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
RETURNING id, public_id::text, tenant_id, resource_type, resource_id, invitee_email_domain, invitee_user_id,
          role, status, expires_at, approved_by_user_id, approved_at, accepted_at,
          created_by_user_id, masked_invitee_email, created_at, updated_at`,
		input.TenantID,
		string(resource.Type),
		resource.ID,
		emailHash(email),
		domain,
		pgInt8(inviteeUserID),
		string(role),
		status,
		expiresAt,
		actor.UserID,
		tokenHash,
		expiresAt,
		maskEmail(email),
	).Scan(
		&invitation.ID,
		&invitation.PublicID,
		&invitation.TenantID,
		&invitation.Resource.Type,
		&invitation.Resource.ID,
		&invitation.InviteeEmailDomain,
		&inviteeUser,
		&invitation.Role,
		&invitation.Status,
		&invitation.ExpiresAt,
		&approvedBy,
		&approvedAt,
		&acceptedAt,
		&invitation.CreatedByUserID,
		&invitation.MaskedInviteeEmail,
		&invitation.CreatedAt,
		&invitation.UpdatedAt,
	)
	if err != nil {
		return DriveShareInvitation{}, fmt.Errorf("create drive share invitation: %w", err)
	}
	invitation.Resource = resource
	invitation.InviteeUserID = optionalPgInt8(inviteeUser)
	invitation.ApprovedByUserID = optionalPgInt8(approvedBy)
	invitation.ApprovedAt = optionalPgTime(approvedAt)
	invitation.AcceptedAt = optionalPgTime(acceptedAt)
	invitation.RawAcceptToken = rawToken
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.share_invitation.create", "drive_share_invitation", invitation.PublicID, map[string]any{
		"resourceType": resource.Type,
		"role":         role,
		"status":       status,
		"domain":       domain,
	})
	return invitation, nil
}

func (s *DriveService) ListShareInvitationsForUser(ctx context.Context, userID int64) ([]DriveShareInvitation, error) {
	if err := s.ensureConfigured(false); err != nil {
		return nil, err
	}
	user, err := s.queries.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) || user.DeactivatedAt.Valid {
		return nil, ErrDrivePermissionDenied
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
SELECT i.id, i.public_id::text, i.tenant_id, i.resource_type, i.resource_id, i.invitee_email_domain,
       i.invitee_user_id, i.role, i.status, i.expires_at, i.approved_by_user_id, i.approved_at,
       i.accepted_at, i.created_by_user_id, i.masked_invitee_email, i.created_at, i.updated_at,
       COALESCE(f.public_id::text, d.public_id::text, '') AS resource_public_id
FROM drive_share_invitations i
LEFT JOIN file_objects f ON i.resource_type = 'file' AND f.id = i.resource_id AND f.tenant_id = i.tenant_id AND f.purpose = 'drive'
LEFT JOIN drive_folders d ON i.resource_type = 'folder' AND d.id = i.resource_id AND d.tenant_id = i.tenant_id
WHERE i.status IN ('pending', 'pending_approval')
  AND i.expires_at > now()
  AND (
      i.invitee_email_hash = $1
      OR i.invitee_user_id = $2
  )
ORDER BY i.created_at DESC, i.id DESC
LIMIT 200`,
		emailHash(user.Email),
		user.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("list drive share invitations: %w", err)
	}
	defer rows.Close()
	items := []DriveShareInvitation{}
	for rows.Next() {
		item, err := scanDriveShareInvitation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *DriveService) AcceptShareInvitation(ctx context.Context, input DriveAcceptShareInvitationInput, auditCtx AuditContext) (DriveShare, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveShare{}, err
	}
	user, err := s.queries.GetUserByID(ctx, input.ActorUserID)
	if errors.Is(err, pgx.ErrNoRows) || user.DeactivatedAt.Valid {
		return DriveShare{}, ErrDrivePermissionDenied
	}
	if err != nil {
		return DriveShare{}, fmt.Errorf("get user: %w", err)
	}
	invitationID, err := uuid.Parse(strings.TrimSpace(input.InvitationPublicID))
	if err != nil {
		return DriveShare{}, ErrDriveNotFound
	}
	invitation, tokenHash, tokenExpiresAt, err := s.getShareInvitationForUpdate(ctx, invitationID)
	if err != nil {
		return DriveShare{}, err
	}
	if invitation.Status == "pending_approval" {
		return DriveShare{}, ErrDrivePolicyDenied
	}
	if invitation.Status != "pending" || !invitation.ExpiresAt.After(s.now()) {
		return DriveShare{}, ErrDriveNotFound
	}
	if tokenHash == "" || !tokenExpiresAt.After(s.now()) || !hmac.Equal([]byte(tokenHash), []byte(driveShareLinkTokenHash(input.AcceptToken))) {
		return DriveShare{}, ErrDrivePermissionDenied
	}
	if invitation.InviteeUserID != nil && *invitation.InviteeUserID != user.ID {
		return DriveShare{}, ErrDrivePermissionDenied
	}
	if invitation.InviteeUserID == nil && emailHash(user.Email) == "" {
		return DriveShare{}, ErrDrivePermissionDenied
	}
	if invitation.InviteeUserID == nil && !s.shareInvitationEmailMatches(ctx, invitation.ID, user.Email) {
		return DriveShare{}, ErrDrivePermissionDenied
	}

	resource, err := s.resolveInvitationResource(ctx, invitation.TenantID, invitation.Resource)
	if err != nil {
		return DriveShare{}, err
	}
	row, err := s.queries.CreateDriveResourceShare(ctx, db.CreateDriveResourceShareParams{
		TenantID:        invitation.TenantID,
		ResourceType:    string(resource.Type),
		ResourceID:      resource.ID,
		SubjectType:     string(DriveShareSubjectUser),
		SubjectID:       user.ID,
		Role:            string(invitation.Role),
		Status:          "active",
		CreatedByUserID: invitation.CreatedByUserID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return DriveShare{}, fmt.Errorf("%w: share already exists", ErrDriveInvalidInput)
		}
		return DriveShare{}, fmt.Errorf("create accepted drive share: %w", err)
	}
	share := driveShareFromDB(row, resource, user.PublicID.String())
	if err := s.authz.WriteShareTuple(ctx, share); err != nil {
		_, _ = s.queries.MarkDriveResourceSharePendingSync(context.Background(), db.MarkDriveResourceSharePendingSyncParams{
			ID:       share.ID,
			TenantID: share.TenantID,
		})
		return DriveShare{}, err
	}
	if _, err := s.pool.Exec(ctx, `
UPDATE drive_share_invitations
SET status = 'accepted',
    invitee_user_id = COALESCE(invitee_user_id, $2),
    accepted_at = now(),
    updated_at = now(),
    accept_token_hash = NULL,
    accept_token_expires_at = NULL
WHERE id = $1`, invitation.ID, user.ID); err != nil {
		return DriveShare{}, fmt.Errorf("mark drive share invitation accepted: %w", err)
	}
	tenantID := invitation.TenantID
	s.recordAuditWithActor(ctx, AuditContext{ActorType: AuditActorUser, ActorUserID: &user.ID, TenantID: &tenantID, Request: auditCtx.Request}, "drive.share_invitation.accept", "drive_share_invitation", invitation.PublicID, map[string]any{
		"resourceType": resource.Type,
		"role":         share.Role,
	})
	return share, nil
}

func (s *DriveService) RevokeShareInvitation(ctx context.Context, input DriveRevokeShareInvitationInput, auditCtx AuditContext) error {
	if err := s.ensureConfigured(false); err != nil {
		return err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return err
	}
	publicID, err := uuid.Parse(strings.TrimSpace(input.InvitationPublicID))
	if err != nil {
		return ErrDriveNotFound
	}
	var invitation DriveShareInvitation
	err = s.pool.QueryRow(ctx, `
UPDATE drive_share_invitations
SET status = 'revoked',
    revoked_by_user_id = $3,
    revoked_at = now(),
    updated_at = now(),
    accept_token_hash = NULL,
    accept_token_expires_at = NULL
WHERE public_id = $1
  AND tenant_id = $2
  AND status IN ('pending', 'pending_approval')
RETURNING id, public_id::text, tenant_id, resource_type, resource_id, role, status, created_by_user_id, created_at, updated_at`,
		publicID,
		input.TenantID,
		actor.UserID,
	).Scan(&invitation.ID, &invitation.PublicID, &invitation.TenantID, &invitation.Resource.Type, &invitation.Resource.ID, &invitation.Role, &invitation.Status, &invitation.CreatedByUserID, &invitation.CreatedAt, &invitation.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	}
	if err != nil {
		return fmt.Errorf("revoke drive share invitation: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.share_invitation.revoke", "drive_share_invitation", invitation.PublicID, map[string]any{
		"resourceType": invitation.Resource.Type,
	})
	return nil
}

func (s *DriveService) ListShareApprovals(ctx context.Context, tenantID int64) ([]DriveShareInvitation, error) {
	return s.listTenantShareInvitations(ctx, tenantID, "pending_approval")
}

func (s *DriveService) ApproveShareInvitation(ctx context.Context, tenantID, actorUserID int64, invitationPublicID string, auditCtx AuditContext) error {
	return s.decideShareApproval(ctx, tenantID, actorUserID, invitationPublicID, true, auditCtx)
}

func (s *DriveService) RejectShareInvitation(ctx context.Context, tenantID, actorUserID int64, invitationPublicID string, auditCtx AuditContext) error {
	return s.decideShareApproval(ctx, tenantID, actorUserID, invitationPublicID, false, auditCtx)
}

type driveInvitationScanner interface {
	Scan(dest ...any) error
}

func scanDriveShareInvitation(row driveInvitationScanner) (DriveShareInvitation, error) {
	var item DriveShareInvitation
	var inviteeUser pgtype.Int8
	var approvedBy pgtype.Int8
	var approvedAt pgtype.Timestamptz
	var acceptedAt pgtype.Timestamptz
	var resourcePublicID string
	err := row.Scan(
		&item.ID,
		&item.PublicID,
		&item.TenantID,
		&item.Resource.Type,
		&item.Resource.ID,
		&item.InviteeEmailDomain,
		&inviteeUser,
		&item.Role,
		&item.Status,
		&item.ExpiresAt,
		&approvedBy,
		&approvedAt,
		&acceptedAt,
		&item.CreatedByUserID,
		&item.MaskedInviteeEmail,
		&item.CreatedAt,
		&item.UpdatedAt,
		&resourcePublicID,
	)
	if err != nil {
		return DriveShareInvitation{}, err
	}
	item.Resource.PublicID = resourcePublicID
	item.Resource.TenantID = item.TenantID
	item.InviteeUserID = optionalPgInt8(inviteeUser)
	item.ApprovedByUserID = optionalPgInt8(approvedBy)
	item.ApprovedAt = optionalPgTime(approvedAt)
	item.AcceptedAt = optionalPgTime(acceptedAt)
	return item, nil
}

func (s *DriveService) listTenantShareInvitations(ctx context.Context, tenantID int64, status string) ([]DriveShareInvitation, error) {
	rows, err := s.pool.Query(ctx, `
SELECT i.id, i.public_id::text, i.tenant_id, i.resource_type, i.resource_id, i.invitee_email_domain,
       i.invitee_user_id, i.role, i.status, i.expires_at, i.approved_by_user_id, i.approved_at,
       i.accepted_at, i.created_by_user_id, i.masked_invitee_email, i.created_at, i.updated_at,
       COALESCE(f.public_id::text, d.public_id::text, '') AS resource_public_id
FROM drive_share_invitations i
LEFT JOIN file_objects f ON i.resource_type = 'file' AND f.id = i.resource_id AND f.tenant_id = i.tenant_id AND f.purpose = 'drive'
LEFT JOIN drive_folders d ON i.resource_type = 'folder' AND d.id = i.resource_id AND d.tenant_id = i.tenant_id
WHERE i.tenant_id = $1
  AND ($2 = '' OR i.status = $2)
ORDER BY i.created_at DESC, i.id DESC
LIMIT 200`, tenantID, status)
	if err != nil {
		return nil, fmt.Errorf("list tenant drive share invitations: %w", err)
	}
	defer rows.Close()
	items := []DriveShareInvitation{}
	for rows.Next() {
		item, err := scanDriveShareInvitation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *DriveService) getShareInvitationForUpdate(ctx context.Context, publicID uuid.UUID) (DriveShareInvitation, string, time.Time, error) {
	var item DriveShareInvitation
	var inviteeUser pgtype.Int8
	var approvedBy pgtype.Int8
	var approvedAt pgtype.Timestamptz
	var acceptedAt pgtype.Timestamptz
	var tokenHash string
	var tokenExpiresAt pgtype.Timestamptz
	var resourcePublicID string
	err := s.pool.QueryRow(ctx, `
SELECT i.id, i.public_id::text, i.tenant_id, i.resource_type, i.resource_id, i.invitee_email_domain,
       i.invitee_user_id, i.role, i.status, i.expires_at, i.approved_by_user_id, i.approved_at,
       i.accepted_at, i.created_by_user_id, i.masked_invitee_email, i.created_at, i.updated_at,
       COALESCE(i.accept_token_hash, ''), COALESCE(i.accept_token_expires_at, '-infinity'::timestamptz),
       COALESCE(f.public_id::text, d.public_id::text, '') AS resource_public_id
FROM drive_share_invitations i
LEFT JOIN file_objects f ON i.resource_type = 'file' AND f.id = i.resource_id AND f.tenant_id = i.tenant_id AND f.purpose = 'drive'
LEFT JOIN drive_folders d ON i.resource_type = 'folder' AND d.id = i.resource_id AND d.tenant_id = i.tenant_id
WHERE i.public_id = $1`, publicID).Scan(
		&item.ID,
		&item.PublicID,
		&item.TenantID,
		&item.Resource.Type,
		&item.Resource.ID,
		&item.InviteeEmailDomain,
		&inviteeUser,
		&item.Role,
		&item.Status,
		&item.ExpiresAt,
		&approvedBy,
		&approvedAt,
		&acceptedAt,
		&item.CreatedByUserID,
		&item.MaskedInviteeEmail,
		&item.CreatedAt,
		&item.UpdatedAt,
		&tokenHash,
		&tokenExpiresAt,
		&resourcePublicID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveShareInvitation{}, "", time.Time{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveShareInvitation{}, "", time.Time{}, fmt.Errorf("get drive share invitation: %w", err)
	}
	item.Resource.PublicID = resourcePublicID
	item.Resource.TenantID = item.TenantID
	item.InviteeUserID = optionalPgInt8(inviteeUser)
	item.ApprovedByUserID = optionalPgInt8(approvedBy)
	item.ApprovedAt = optionalPgTime(approvedAt)
	item.AcceptedAt = optionalPgTime(acceptedAt)
	expires := time.Time{}
	if tokenExpiresAt.Valid {
		expires = tokenExpiresAt.Time
	}
	return item, tokenHash, expires, nil
}

func (s *DriveService) shareInvitationEmailMatches(ctx context.Context, invitationID int64, email string) bool {
	var ok bool
	err := s.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM drive_share_invitations
    WHERE id = $1
      AND invitee_email_hash = $2
)`, invitationID, emailHash(email)).Scan(&ok)
	return err == nil && ok
}

func (s *DriveService) resolveInvitationResource(ctx context.Context, tenantID int64, ref DriveResourceRef) (DriveResourceRef, error) {
	switch ref.Type {
	case DriveResourceTypeFile:
		row, err := s.getDriveFileRow(ctx, tenantID, ref)
		if err != nil {
			return DriveResourceRef{}, err
		}
		return driveFileFromDB(row).ResourceRef(), nil
	case DriveResourceTypeFolder:
		folder, err := s.getFolderByID(ctx, tenantID, ref.ID)
		if err != nil {
			return DriveResourceRef{}, err
		}
		return folder.ResourceRef(), nil
	default:
		return DriveResourceRef{}, ErrDriveNotFound
	}
}

func (s *DriveService) decideShareApproval(ctx context.Context, tenantID, actorUserID int64, invitationPublicID string, approve bool, auditCtx AuditContext) error {
	if err := s.ensureConfigured(false); err != nil {
		return err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	publicID, err := uuid.Parse(strings.TrimSpace(invitationPublicID))
	if err != nil {
		return ErrDriveNotFound
	}
	status := "pending"
	action := "drive.share_approval.approve"
	if !approve {
		status = "rejected"
		action = "drive.share_approval.reject"
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE drive_share_invitations
SET status = $4,
    approved_by_user_id = CASE WHEN $5 THEN $3 ELSE approved_by_user_id END,
    approved_at = CASE WHEN $5 THEN now() ELSE approved_at END,
    revoked_by_user_id = CASE WHEN $5 THEN revoked_by_user_id ELSE $3 END,
    revoked_at = CASE WHEN $5 THEN revoked_at ELSE now() END,
    accept_token_hash = CASE WHEN $5 THEN accept_token_hash ELSE NULL END,
    accept_token_expires_at = CASE WHEN $5 THEN accept_token_expires_at ELSE NULL END,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND status = 'pending_approval'`, publicID, tenantID, actor.UserID, status, approve)
	if err != nil {
		return fmt.Errorf("decide drive share approval: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDriveNotFound
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, action, "drive_share_invitation", publicID.String(), nil)
	return nil
}

func (s *DriveService) lookupUserIDByPublicID(ctx context.Context, publicID string) (int64, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return 0, ErrDriveNotFound
	}
	user, err := s.queries.GetUserByPublicID(ctx, parsed)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrDriveNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get user by public id: %w", err)
	}
	if user.DeactivatedAt.Valid {
		return 0, ErrDrivePermissionDenied
	}
	return user.ID, nil
}

func (s *DriveService) recordAuditWithActor(ctx context.Context, auditCtx AuditContext, action, targetType, targetID string, metadata map[string]any) {
	if s == nil || s.audit == nil {
		return
	}
	s.audit.RecordBestEffort(ctx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       action,
		TargetType:   targetType,
		TargetID:     targetID,
		Metadata:     metadata,
	})
}

func (s *DriveService) setShareLinkPassword(ctx context.Context, linkID, tenantID int64, password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return fmt.Errorf("%w: share link password is required", ErrDriveInvalidInput)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash share link password: %w", err)
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE drive_share_links
SET password_hash = $3,
    password_required = true,
    password_updated_at = now(),
    updated_at = now()
WHERE id = $1
  AND tenant_id = $2`, linkID, tenantID, string(hash))
	if err != nil {
		return fmt.Errorf("set share link password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDriveNotFound
	}
	return nil
}

func (s *DriveService) hydrateShareLinkPasswordState(ctx context.Context, link *DriveShareLink) {
	if s == nil || link == nil || s.pool == nil || link.ID <= 0 {
		return
	}
	var required bool
	if err := s.pool.QueryRow(ctx, `
SELECT password_required
FROM drive_share_links
WHERE id = $1
  AND tenant_id = $2`, link.ID, link.TenantID).Scan(&required); err == nil {
		link.PasswordRequired = required
	}
}

func (s *DriveService) VerifyPublicShareLinkPassword(ctx context.Context, token, password, requesterKey string) (DriveShareLinkPasswordVerification, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveShareLinkPasswordVerification{}, err
	}
	token = strings.TrimSpace(token)
	password = strings.TrimSpace(password)
	if token == "" || password == "" {
		return DriveShareLinkPasswordVerification{}, ErrDrivePermissionDenied
	}
	tokenHash := driveShareLinkTokenHash(token)
	state, err := s.getShareLinkPasswordState(ctx, tokenHash)
	if err != nil {
		return DriveShareLinkPasswordVerification{}, err
	}
	if !state.PasswordRequired {
		return DriveShareLinkPasswordVerification{
			CookieName:  DriveShareLinkPasswordCookieName,
			CookieValue: "",
			ExpiresAt:   s.now().Add(5 * time.Minute),
		}, nil
	}
	if blocked, err := s.shareLinkPasswordBlocked(ctx, tokenHash, requesterKey); err != nil {
		return DriveShareLinkPasswordVerification{}, err
	} else if blocked {
		return DriveShareLinkPasswordVerification{}, ErrDrivePermissionDenied
	}
	if bcrypt.CompareHashAndPassword([]byte(state.PasswordHash), []byte(password)) != nil {
		_ = s.recordShareLinkPasswordFailure(ctx, tokenHash, requesterKey)
		s.recordPublicPasswordAudit(ctx, state, "drive.share_link.password_failed")
		return DriveShareLinkPasswordVerification{}, ErrDrivePermissionDenied
	}
	_ = s.clearShareLinkPasswordFailures(ctx, tokenHash, requesterKey)
	expiresAt := s.now().Add(10 * time.Minute)
	if state.ExpiresAt.Before(expiresAt) {
		expiresAt = state.ExpiresAt
	}
	value := signShareLinkVerificationCookie(state, expiresAt)
	return DriveShareLinkPasswordVerification{
		CookieName:  DriveShareLinkPasswordCookieName,
		CookieValue: value,
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *DriveService) PublicShareLinkContentWithVerification(ctx context.Context, token, verificationCookie string) (DriveFileDownload, error) {
	link, file, _, err := s.resolvePublicShareLink(ctx, token, true)
	if err != nil {
		return DriveFileDownload{}, err
	}
	s.hydrateShareLinkPasswordState(ctx, &link)
	if link.PasswordRequired {
		state, err := s.getShareLinkPasswordState(ctx, driveShareLinkTokenHash(token))
		if err != nil {
			return DriveFileDownload{}, err
		}
		if !verifyShareLinkVerificationCookie(state, verificationCookie, s.now) {
			return DriveFileDownload{}, ErrDrivePermissionDenied
		}
	}
	if file == nil {
		return DriveFileDownload{}, ErrDriveInvalidInput
	}
	policy, err := s.drivePolicy(ctx, file.TenantID)
	if err != nil {
		return DriveFileDownload{}, err
	}
	if file.DLPBlocked || file.ScanStatus == "infected" || file.ScanStatus == "blocked" || (policy.ContentScanEnabled && policy.BlockDownloadUntilScanComplete && file.ScanStatus == "pending") {
		s.recordPublicLinkAudit(ctx, link, "drive.file.download_denied_scan", map[string]any{
			"scanStatus": file.ScanStatus,
			"dlpBlocked": file.DLPBlocked,
		})
		return DriveFileDownload{}, ErrDrivePolicyDenied
	}
	body, err := s.storage.Open(ctx, file.StorageKey)
	if err != nil {
		return DriveFileDownload{}, err
	}
	s.recordPublicLinkAudit(ctx, link, "drive.share_link.access", map[string]any{
		"resourceType": "file",
		"content":      true,
	})
	return DriveFileDownload{File: *file, Body: body}, nil
}

func (s *DriveService) getShareLinkPasswordState(ctx context.Context, tokenHash string) (shareLinkPasswordState, error) {
	var state shareLinkPasswordState
	var passwordUpdatedAt pgtype.Timestamptz
	err := s.pool.QueryRow(ctx, `
SELECT id, public_id::text, tenant_id, token_hash, password_required, COALESCE(password_hash, ''),
       password_updated_at, status, expires_at
FROM drive_share_links
WHERE token_hash = $1
  AND status = 'active'
  AND expires_at > now()`, tokenHash).Scan(
		&state.ID,
		&state.PublicID,
		&state.TenantID,
		&state.TokenHash,
		&state.PasswordRequired,
		&state.PasswordHash,
		&passwordUpdatedAt,
		&state.Status,
		&state.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return shareLinkPasswordState{}, ErrDriveNotFound
	}
	if err != nil {
		return shareLinkPasswordState{}, fmt.Errorf("get share link password state: %w", err)
	}
	if passwordUpdatedAt.Valid {
		state.PasswordUpdatedAt = passwordUpdatedAt.Time
	}
	return state, nil
}

func (s *DriveService) shareLinkPasswordBlocked(ctx context.Context, tokenHash, requesterKey string) (bool, error) {
	var blocked bool
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(blocked_until > now(), false)
FROM drive_share_link_password_attempts
WHERE token_hash = $1
  AND requester_key = $2`, tokenHash, normalizeRequesterKey(requesterKey)).Scan(&blocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check share link password block: %w", err)
	}
	return blocked, nil
}

func (s *DriveService) recordShareLinkPasswordFailure(ctx context.Context, tokenHash, requesterKey string) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO drive_share_link_password_attempts (
    token_hash,
    requester_key,
    failed_count,
    blocked_until,
    last_failed_at
) VALUES ($1, $2, 1, NULL, now())
ON CONFLICT (token_hash, requester_key) DO UPDATE
SET failed_count = drive_share_link_password_attempts.failed_count + 1,
    blocked_until = CASE
        WHEN drive_share_link_password_attempts.failed_count + 1 >= 5 THEN now() + interval '15 minutes'
        ELSE drive_share_link_password_attempts.blocked_until
    END,
    last_failed_at = now(),
    updated_at = now()`, tokenHash, normalizeRequesterKey(requesterKey))
	return err
}

func (s *DriveService) clearShareLinkPasswordFailures(ctx context.Context, tokenHash, requesterKey string) error {
	_, err := s.pool.Exec(ctx, `
DELETE FROM drive_share_link_password_attempts
WHERE token_hash = $1
  AND requester_key = $2`, tokenHash, normalizeRequesterKey(requesterKey))
	return err
}

func signShareLinkVerificationCookie(state shareLinkPasswordState, expiresAt time.Time) string {
	updatedUnix := state.PasswordUpdatedAt.Unix()
	payload := strings.Join([]string{
		"v1",
		state.PublicID,
		strconv.FormatInt(updatedUnix, 10),
		strconv.FormatInt(expiresAt.Unix(), 10),
	}, ":")
	mac := hmac.New(sha256.New, []byte(state.PasswordHash+"."+state.TokenHash))
	_, _ = mac.Write([]byte(payload))
	signature := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func verifyShareLinkVerificationCookie(state shareLinkPasswordState, value string, now func() time.Time) bool {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) != 2 {
		return false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	payload := string(payloadBytes)
	fields := strings.Split(payload, ":")
	if len(fields) != 4 || fields[0] != "v1" || fields[1] != state.PublicID {
		return false
	}
	updatedUnix, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || updatedUnix != state.PasswordUpdatedAt.Unix() {
		return false
	}
	expiresUnix, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil || !time.Unix(expiresUnix, 0).After(now()) {
		return false
	}
	mac := hmac.New(sha256.New, []byte(state.PasswordHash+"."+state.TokenHash))
	_, _ = mac.Write([]byte(payload))
	return hmac.Equal(signature, mac.Sum(nil))
}

func (s *DriveService) recordPublicPasswordAudit(ctx context.Context, state shareLinkPasswordState, action string) {
	tenantID := state.TenantID
	s.recordAuditWithActor(ctx, AuditContext{ActorType: AuditActorSystem, TenantID: &tenantID}, action, "drive_share_link", state.PublicID, nil)
}

func normalizeRequesterKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *DriveService) ListAdminDriveShares(ctx context.Context, tenantID int64) ([]DriveAdminShareState, error) {
	rows, err := s.pool.Query(ctx, `
SELECT s.public_id::text,
       s.resource_type,
       COALESCE(f.public_id::text, d.public_id::text, '') AS resource_public_id,
       COALESCE(f.original_filename, d.name, '') AS resource_name,
       s.subject_type,
       COALESCE(u.public_id::text, g.public_id::text, '') AS subject_public_id,
       s.role,
       s.status,
       s.created_at,
       s.updated_at
FROM drive_resource_shares s
LEFT JOIN file_objects f ON s.resource_type = 'file' AND f.id = s.resource_id AND f.tenant_id = s.tenant_id AND f.purpose = 'drive'
LEFT JOIN drive_folders d ON s.resource_type = 'folder' AND d.id = s.resource_id AND d.tenant_id = s.tenant_id
LEFT JOIN users u ON s.subject_type = 'user' AND u.id = s.subject_id
LEFT JOIN drive_groups g ON s.subject_type = 'group' AND g.id = s.subject_id AND g.tenant_id = s.tenant_id
WHERE s.tenant_id = $1
ORDER BY s.created_at DESC, s.id DESC
LIMIT 500`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list admin drive shares: %w", err)
	}
	defer rows.Close()
	items := []DriveAdminShareState{}
	for rows.Next() {
		var item DriveAdminShareState
		if err := rows.Scan(&item.PublicID, &item.ResourceType, &item.ResourcePublicID, &item.ResourceName, &item.SubjectType, &item.SubjectPublicID, &item.Role, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *DriveService) ListAdminDriveShareLinks(ctx context.Context, tenantID int64) ([]DriveAdminShareLinkState, error) {
	rows, err := s.pool.Query(ctx, `
SELECT l.public_id::text,
       l.resource_type,
       COALESCE(f.public_id::text, d.public_id::text, '') AS resource_public_id,
       COALESCE(f.original_filename, d.name, '') AS resource_name,
       l.can_download,
       l.password_required,
       l.status,
       l.expires_at,
       l.created_at,
       l.updated_at
FROM drive_share_links l
LEFT JOIN file_objects f ON l.resource_type = 'file' AND f.id = l.resource_id AND f.tenant_id = l.tenant_id AND f.purpose = 'drive'
LEFT JOIN drive_folders d ON l.resource_type = 'folder' AND d.id = l.resource_id AND d.tenant_id = l.tenant_id
WHERE l.tenant_id = $1
ORDER BY l.created_at DESC, l.id DESC
LIMIT 500`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list admin drive share links: %w", err)
	}
	defer rows.Close()
	items := []DriveAdminShareLinkState{}
	for rows.Next() {
		var item DriveAdminShareLinkState
		if err := rows.Scan(&item.PublicID, &item.ResourceType, &item.ResourcePublicID, &item.ResourceName, &item.CanDownload, &item.PasswordRequired, &item.Status, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *DriveService) ListAdminDriveInvitations(ctx context.Context, tenantID int64) ([]DriveShareInvitation, error) {
	return s.listTenantShareInvitations(ctx, tenantID, "")
}

func (s *DriveService) ListAdminDriveAuditEvents(ctx context.Context, tenantID int64, limit int32) ([]DriveAdminAuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
SELECT public_id::text, actor_type, action, target_type, target_id, metadata, occurred_at
FROM audit_events
WHERE tenant_id = $1
  AND action LIKE 'drive.%'
ORDER BY occurred_at DESC, id DESC
LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list admin drive audit events: %w", err)
	}
	defer rows.Close()
	items := []DriveAdminAuditEvent{}
	for rows.Next() {
		var item DriveAdminAuditEvent
		var metadata []byte
		if err := rows.Scan(&item.PublicID, &item.ActorType, &item.Action, &item.TargetType, &item.TargetID, &metadata, &item.OccurredAt); err != nil {
			return nil, err
		}
		item.Metadata = map[string]any{}
		_ = json.Unmarshal(metadata, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *DriveService) OpenFGADrift(ctx context.Context, tenantID int64) (DriveOpenFGASyncResult, error) {
	return s.openFGASync(ctx, tenantID, true, AuditContext{ActorType: AuditActorSystem, TenantID: &tenantID})
}

func (s *DriveService) RepairOpenFGASync(ctx context.Context, tenantID, actorUserID int64, auditCtx AuditContext) (DriveOpenFGASyncResult, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveOpenFGASyncResult{}, err
	}
	result, err := s.openFGASync(ctx, tenantID, false, auditCtx)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.openfga_sync.repair", "tenant", strconv.FormatInt(tenantID, 10), map[string]any{
		"items": len(result.Items),
	})
	return result, err
}

func (s *DriveService) openFGASync(ctx context.Context, tenantID int64, dryRun bool, auditCtx AuditContext) (DriveOpenFGASyncResult, error) {
	result := DriveOpenFGASyncResult{DryRun: dryRun}
	shares, err := s.expectedDriveShares(ctx, tenantID)
	if err != nil {
		return result, err
	}
	for _, share := range shares {
		item := DriveOpenFGASyncItem{Kind: "share", PublicID: share.PublicID, Status: share.Status}
		if err := s.authz.CheckShareTuple(ctx, share); err == nil {
			item.Action = "ok"
			result.Items = append(result.Items, item)
			continue
		}
		item.Action = "write"
		if dryRun {
			result.Items = append(result.Items, item)
			continue
		}
		if err := s.authz.WriteShareTuple(ctx, share); err != nil {
			item.Error = err.Error()
			result.Items = append(result.Items, item)
			continue
		}
		_, _ = s.pool.Exec(ctx, `UPDATE drive_resource_shares SET status = 'active', updated_at = now() WHERE id = $1 AND tenant_id = $2 AND status = 'pending_sync'`, share.ID, share.TenantID)
		result.Items = append(result.Items, item)
	}
	links, err := s.expectedDriveShareLinks(ctx, tenantID)
	if err != nil {
		return result, err
	}
	for _, link := range links {
		item := DriveOpenFGASyncItem{Kind: "share_link", PublicID: link.PublicID, Status: link.Status}
		if err := s.authz.CheckShareLinkTuple(ctx, link); err == nil {
			item.Action = "ok"
			result.Items = append(result.Items, item)
			continue
		}
		item.Action = "write"
		if dryRun {
			result.Items = append(result.Items, item)
			continue
		}
		if err := s.authz.WriteShareLinkTuple(ctx, link); err != nil {
			item.Error = err.Error()
			result.Items = append(result.Items, item)
			continue
		}
		_, _ = s.pool.Exec(ctx, `UPDATE drive_share_links SET status = 'active', updated_at = now() WHERE id = $1 AND tenant_id = $2 AND status = 'pending_sync'`, link.ID, link.TenantID)
		result.Items = append(result.Items, item)
	}
	if dryRun && s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "drive.openfga_sync.drift_check",
			TargetType:   "tenant",
			TargetID:     strconv.FormatInt(tenantID, 10),
			Metadata:     map[string]any{"items": len(result.Items)},
		})
	}
	return result, nil
}

func (s *DriveService) expectedDriveShares(ctx context.Context, tenantID int64) ([]DriveShare, error) {
	rows, err := s.pool.Query(ctx, `
SELECT s.id, s.public_id::text, s.tenant_id, s.resource_type, s.resource_id,
       COALESCE(f.public_id::text, d.public_id::text, '') AS resource_public_id,
       s.subject_type, s.subject_id, COALESCE(u.public_id::text, g.public_id::text, '') AS subject_public_id,
       s.role, s.status, s.created_by_user_id, s.created_at, s.updated_at
FROM drive_resource_shares s
LEFT JOIN file_objects f ON s.resource_type = 'file' AND f.id = s.resource_id AND f.tenant_id = s.tenant_id AND f.purpose = 'drive' AND f.deleted_at IS NULL
LEFT JOIN drive_folders d ON s.resource_type = 'folder' AND d.id = s.resource_id AND d.tenant_id = s.tenant_id AND d.deleted_at IS NULL
LEFT JOIN users u ON s.subject_type = 'user' AND u.id = s.subject_id AND u.deactivated_at IS NULL
LEFT JOIN drive_groups g ON s.subject_type = 'group' AND g.id = s.subject_id AND g.tenant_id = s.tenant_id AND g.deleted_at IS NULL
WHERE s.tenant_id = $1
  AND s.status IN ('active', 'pending_sync')
ORDER BY s.id`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list expected drive shares: %w", err)
	}
	defer rows.Close()
	items := []DriveShare{}
	for rows.Next() {
		var item DriveShare
		if err := rows.Scan(&item.ID, &item.PublicID, &item.TenantID, &item.Resource.Type, &item.Resource.ID, &item.Resource.PublicID, &item.SubjectType, &item.SubjectID, &item.SubjectPublicID, &item.Role, &item.Status, &item.CreatedByUserID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Resource.TenantID = item.TenantID
		if item.Resource.PublicID == "" || item.SubjectPublicID == "" {
			continue
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *DriveService) expectedDriveShareLinks(ctx context.Context, tenantID int64) ([]DriveShareLink, error) {
	rows, err := s.pool.Query(ctx, `
SELECT l.id, l.public_id::text, l.tenant_id, l.resource_type, l.resource_id,
       COALESCE(f.public_id::text, d.public_id::text, '') AS resource_public_id,
       l.role, l.can_download, l.password_required, l.expires_at, l.status, l.created_by_user_id, l.created_at, l.updated_at
FROM drive_share_links l
LEFT JOIN file_objects f ON l.resource_type = 'file' AND f.id = l.resource_id AND f.tenant_id = l.tenant_id AND f.purpose = 'drive' AND f.deleted_at IS NULL
LEFT JOIN drive_folders d ON l.resource_type = 'folder' AND d.id = l.resource_id AND d.tenant_id = l.tenant_id AND d.deleted_at IS NULL
WHERE l.tenant_id = $1
  AND l.status IN ('active', 'pending_sync')
  AND l.expires_at > now()
ORDER BY l.id`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list expected drive share links: %w", err)
	}
	defer rows.Close()
	items := []DriveShareLink{}
	for rows.Next() {
		var item DriveShareLink
		if err := rows.Scan(&item.ID, &item.PublicID, &item.TenantID, &item.Resource.Type, &item.Resource.ID, &item.Resource.PublicID, &item.Role, &item.CanDownload, &item.PasswordRequired, &item.ExpiresAt, &item.Status, &item.CreatedByUserID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Resource.TenantID = item.TenantID
		if item.Resource.PublicID == "" {
			continue
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *DriveService) CleanupExpiredDriveAccess(ctx context.Context, tenantID int64) (DriveOpenFGASyncResult, error) {
	result := DriveOpenFGASyncResult{}
	links, err := s.expiredDriveShareLinks(ctx, tenantID)
	if err != nil {
		return result, err
	}
	for _, link := range links {
		item := DriveOpenFGASyncItem{Kind: "share_link", PublicID: link.PublicID, Status: link.Status, Action: "expire"}
		if err := s.authz.DeleteShareLinkTuple(ctx, link); err != nil {
			item.Error = err.Error()
			result.Items = append(result.Items, item)
			continue
		}
		_, _ = s.pool.Exec(ctx, `UPDATE drive_share_links SET status = 'expired', updated_at = now() WHERE id = $1 AND tenant_id = $2 AND status = 'active'`, link.ID, link.TenantID)
		result.Items = append(result.Items, item)
	}
	_, err = s.pool.Exec(ctx, `
UPDATE drive_share_invitations
SET status = 'expired',
    accept_token_hash = NULL,
    accept_token_expires_at = NULL,
    updated_at = now()
WHERE tenant_id = $1
  AND status IN ('pending', 'pending_approval')
  AND expires_at <= now()`, tenantID)
	return result, err
}

func (s *DriveService) expiredDriveShareLinks(ctx context.Context, tenantID int64) ([]DriveShareLink, error) {
	rows, err := s.pool.Query(ctx, `
SELECT l.id, l.public_id::text, l.tenant_id, l.resource_type, l.resource_id,
       COALESCE(f.public_id::text, d.public_id::text, '') AS resource_public_id,
       l.role, l.can_download, l.password_required, l.expires_at, l.status, l.created_by_user_id, l.created_at, l.updated_at
FROM drive_share_links l
LEFT JOIN file_objects f ON l.resource_type = 'file' AND f.id = l.resource_id AND f.tenant_id = l.tenant_id AND f.purpose = 'drive'
LEFT JOIN drive_folders d ON l.resource_type = 'folder' AND d.id = l.resource_id AND d.tenant_id = l.tenant_id
WHERE l.tenant_id = $1
  AND l.status = 'active'
  AND l.expires_at <= now()`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DriveShareLink{}
	for rows.Next() {
		var item DriveShareLink
		if err := rows.Scan(&item.ID, &item.PublicID, &item.TenantID, &item.Resource.Type, &item.Resource.ID, &item.Resource.PublicID, &item.Role, &item.CanDownload, &item.PasswordRequired, &item.ExpiresAt, &item.Status, &item.CreatedByUserID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Resource.TenantID = item.TenantID
		if item.Resource.PublicID != "" {
			items = append(items, item)
		}
	}
	return items, rows.Err()
}

func (s *DriveService) ensureFileMutationAllowed(ctx context.Context, tenantID, fileID int64) error {
	var legalHoldAt pgtype.Timestamptz
	var purgeBlockReason pgtype.Text
	err := s.pool.QueryRow(ctx, `
SELECT legal_hold_at, purge_block_reason
FROM file_objects
WHERE id = $1
  AND tenant_id = $2
  AND purpose = 'drive'`, fileID, tenantID).Scan(&legalHoldAt, &purgeBlockReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	}
	if err != nil {
		return fmt.Errorf("check drive file lifecycle guard: %w", err)
	}
	if legalHoldAt.Valid || strings.TrimSpace(optionalText(purgeBlockReason)) != "" {
		return ErrDriveLocked
	}
	return nil
}

func (s *DriveService) ensureFolderMutationAllowed(ctx context.Context, tenantID, folderID int64) error {
	var legalHoldAt pgtype.Timestamptz
	var purgeBlockReason pgtype.Text
	err := s.pool.QueryRow(ctx, `
SELECT legal_hold_at, purge_block_reason
FROM drive_folders
WHERE id = $1
  AND tenant_id = $2`, folderID, tenantID).Scan(&legalHoldAt, &purgeBlockReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	}
	if err != nil {
		return fmt.Errorf("check drive folder lifecycle guard: %w", err)
	}
	if legalHoldAt.Valid || strings.TrimSpace(optionalText(purgeBlockReason)) != "" {
		return ErrDriveLocked
	}
	return nil
}

func emailDomain(email string) string {
	parts := strings.Split(normalizeEmail(email), "@")
	if len(parts) != 2 || parts[0] == "" {
		return ""
	}
	return normalizeExternalDomain(parts[1])
}

func emailHash(email string) string {
	email = normalizeEmail(email)
	if email == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(email))
	return hex.EncodeToString(sum[:])
}

func maskEmail(email string) string {
	email = normalizeEmail(email)
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	local := parts[0]
	if local == "" {
		return "***@" + parts[1]
	}
	return local[:1] + "***@" + parts[1]
}

func drivePolicyAllowsExternalDomain(policy DrivePolicy, domain string) (bool, string) {
	domain = normalizeExternalDomain(domain)
	if domain == "" {
		return false, "invalid_domain"
	}
	for _, blocked := range policy.BlockedExternalDomains {
		if domainMatchesDrivePolicy(domain, blocked) {
			return false, "blocked_domain"
		}
	}
	if len(policy.AllowedExternalDomains) == 0 {
		return true, ""
	}
	for _, allowed := range policy.AllowedExternalDomains {
		if domainMatchesDrivePolicy(domain, allowed) {
			return true, ""
		}
	}
	return false, "domain_not_allowed"
}

func domainMatchesDrivePolicy(domain, rule string) bool {
	domain = normalizeExternalDomain(domain)
	rule = normalizeExternalDomain(rule)
	if domain == "" || rule == "" {
		return false
	}
	return domain == rule || strings.HasSuffix(domain, "."+rule)
}
