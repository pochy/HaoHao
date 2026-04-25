package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const scimIdentityProvider = "scim"

var ErrInvalidSCIMUser = errors.New("invalid scim user")

type ProvisionedUserInput struct {
	ExternalID  string
	UserName    string
	DisplayName string
	Active      bool
	Groups      []string
}

type ProvisionedUser struct {
	ID          int64
	PublicID    string
	ExternalID  string
	UserName    string
	DisplayName string
	Active      bool
}

type ProvisioningService struct {
	pool              *pgxpool.Pool
	queries           *db.Queries
	sessionService    *SessionService
	delegationService *DelegationService
	authzService      *AuthzService
}

func NewProvisioningService(pool *pgxpool.Pool, queries *db.Queries, sessionService *SessionService, delegationService *DelegationService, authzService *AuthzService) *ProvisioningService {
	return &ProvisioningService{
		pool:              pool,
		queries:           queries,
		sessionService:    sessionService,
		delegationService: delegationService,
		authzService:      authzService,
	}
}

func (s *ProvisioningService) UpsertUser(ctx context.Context, input ProvisionedUserInput) (ProvisionedUser, error) {
	normalized, err := normalizeProvisionedUser(input)
	if err != nil {
		return ProvisionedUser{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ProvisionedUser{}, fmt.Errorf("begin provisioning transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	user, err := qtx.GetProvisionedUserByExternalID(ctx, db.GetProvisionedUserByExternalIDParams{
		Provider:   scimIdentityProvider,
		ExternalID: pgtype.Text{String: normalized.ExternalID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		created, err := s.createProvisionedUser(ctx, qtx, normalized)
		if err != nil {
			return ProvisionedUser{}, err
		}
		user = created
	} else if err != nil {
		return ProvisionedUser{}, fmt.Errorf("lookup provisioned user by external id: %w", err)
	} else {
		if err := s.updateProvisionedUser(ctx, qtx, user.ID, normalized); err != nil {
			return ProvisionedUser{}, err
		}
		user.Email = normalized.UserName
		user.DisplayName = normalized.DisplayName
		user.DeactivatedAt = deactivatedAtForActive(normalized.Active)
	}

	if err := tx.Commit(ctx); err != nil {
		return ProvisionedUser{}, fmt.Errorf("commit provisioning transaction: %w", err)
	}

	if s.authzService != nil && normalized.Groups != nil {
		if _, err := s.authzService.SyncTenantMemberships(ctx, user.ID, "scim", normalized.Groups); err != nil {
			return ProvisionedUser{}, err
		}
	}
	if !normalized.Active {
		if err := s.deactivateSideEffects(ctx, user.ID); err != nil {
			return ProvisionedUser{}, err
		}
	}

	return provisionedUserFromRow(user), nil
}

func (s *ProvisioningService) GetUser(ctx context.Context, publicID string) (ProvisionedUser, error) {
	id, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return ProvisionedUser{}, ErrInvalidSCIMUser
	}

	user, err := s.queries.GetProvisionedUserByPublicID(ctx, db.GetProvisionedUserByPublicIDParams{
		Provider: scimIdentityProvider,
		PublicID: id,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ProvisionedUser{}, ErrUnauthorized
	}
	if err != nil {
		return ProvisionedUser{}, fmt.Errorf("load provisioned user: %w", err)
	}
	return provisionedUserFromPublicIDRow(user), nil
}

func (s *ProvisioningService) GetUserByExternalID(ctx context.Context, externalID string) (ProvisionedUser, error) {
	user, err := s.queries.GetProvisionedUserByExternalID(ctx, db.GetProvisionedUserByExternalIDParams{
		Provider:   scimIdentityProvider,
		ExternalID: pgtype.Text{String: strings.TrimSpace(externalID), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ProvisionedUser{}, ErrUnauthorized
	}
	if err != nil {
		return ProvisionedUser{}, fmt.Errorf("load provisioned user by external id: %w", err)
	}
	return provisionedUserFromRow(user), nil
}

func (s *ProvisioningService) ListUsers(ctx context.Context, startIndex, count int32) ([]ProvisionedUser, error) {
	if startIndex < 1 {
		startIndex = 1
	}
	if count <= 0 || count > 100 {
		count = 100
	}

	rows, err := s.queries.ListProvisionedUsers(ctx, db.ListProvisionedUsersParams{
		Provider: scimIdentityProvider,
		Limit:    count,
		Offset:   startIndex - 1,
	})
	if err != nil {
		return nil, fmt.Errorf("list provisioned users: %w", err)
	}

	users := make([]ProvisionedUser, 0, len(rows))
	for _, row := range rows {
		users = append(users, provisionedUserFromListRow(row))
	}
	return users, nil
}

func (s *ProvisioningService) DeactivateUser(ctx context.Context, publicID string) (ProvisionedUser, error) {
	user, err := s.GetUser(ctx, publicID)
	if err != nil {
		return ProvisionedUser{}, err
	}

	updated, err := s.queries.SetUserDeactivatedAt(ctx, db.SetUserDeactivatedAtParams{
		ID:            user.ID,
		DeactivatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		return ProvisionedUser{}, fmt.Errorf("deactivate provisioned user: %w", err)
	}
	if err := s.deactivateSideEffects(ctx, user.ID); err != nil {
		return ProvisionedUser{}, err
	}

	return ProvisionedUser{
		ID:          updated.ID,
		PublicID:    updated.PublicID.String(),
		ExternalID:  user.ExternalID,
		UserName:    updated.Email,
		DisplayName: updated.DisplayName,
		Active:      false,
	}, nil
}

func (s *ProvisioningService) createProvisionedUser(ctx context.Context, qtx *db.Queries, input ProvisionedUserInput) (db.GetProvisionedUserByExternalIDRow, error) {
	existing, err := qtx.GetUserByEmail(ctx, input.UserName)
	if err == nil {
		if err := qtx.CreateProvisionedUserIdentity(ctx, db.CreateProvisionedUserIdentityParams{
			UserID:             existing.ID,
			Provider:           scimIdentityProvider,
			Subject:            input.ExternalID,
			Email:              input.UserName,
			EmailVerified:      true,
			ExternalID:         pgtype.Text{String: input.ExternalID, Valid: true},
			ProvisioningSource: pgtype.Text{String: "scim", Valid: true},
		}); err != nil {
			return db.GetProvisionedUserByExternalIDRow{}, fmt.Errorf("create provisioned identity: %w", err)
		}
		if err := s.updateProvisionedUser(ctx, qtx, existing.ID, input); err != nil {
			return db.GetProvisionedUserByExternalIDRow{}, err
		}
		return rowFromUser(existing.ID, existing.PublicID.String(), input.UserName, input.DisplayName, deactivatedAtForActive(input.Active), existing.DefaultTenantID, input.ExternalID), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.GetProvisionedUserByExternalIDRow{}, fmt.Errorf("lookup user by email: %w", err)
	}

	created, err := qtx.CreateOIDCUser(ctx, db.CreateOIDCUserParams{
		Email:       input.UserName,
		DisplayName: input.DisplayName,
	})
	if err != nil {
		return db.GetProvisionedUserByExternalIDRow{}, fmt.Errorf("create provisioned user: %w", err)
	}
	if err := qtx.CreateProvisionedUserIdentity(ctx, db.CreateProvisionedUserIdentityParams{
		UserID:             created.ID,
		Provider:           scimIdentityProvider,
		Subject:            input.ExternalID,
		Email:              input.UserName,
		EmailVerified:      true,
		ExternalID:         pgtype.Text{String: input.ExternalID, Valid: true},
		ProvisioningSource: pgtype.Text{String: "scim", Valid: true},
	}); err != nil {
		return db.GetProvisionedUserByExternalIDRow{}, fmt.Errorf("create provisioned identity: %w", err)
	}
	if !input.Active {
		if _, err := qtx.SetUserDeactivatedAt(ctx, db.SetUserDeactivatedAtParams{
			ID:            created.ID,
			DeactivatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		}); err != nil {
			return db.GetProvisionedUserByExternalIDRow{}, fmt.Errorf("deactivate created provisioned user: %w", err)
		}
	}

	return rowFromUser(created.ID, created.PublicID.String(), input.UserName, input.DisplayName, deactivatedAtForActive(input.Active), created.DefaultTenantID, input.ExternalID), nil
}

func (s *ProvisioningService) updateProvisionedUser(ctx context.Context, qtx *db.Queries, userID int64, input ProvisionedUserInput) error {
	if _, err := qtx.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		ID:          userID,
		Email:       input.UserName,
		DisplayName: input.DisplayName,
	}); err != nil {
		return fmt.Errorf("update provisioned user profile: %w", err)
	}
	if _, err := qtx.SetUserDeactivatedAt(ctx, db.SetUserDeactivatedAtParams{
		ID:            userID,
		DeactivatedAt: deactivatedAtForActive(input.Active),
	}); err != nil {
		return fmt.Errorf("update provisioned user active state: %w", err)
	}
	if err := qtx.UpdateUserIdentityProvisioningProfile(ctx, db.UpdateUserIdentityProvisioningProfileParams{
		Provider:           scimIdentityProvider,
		Subject:            input.ExternalID,
		Email:              input.UserName,
		EmailVerified:      true,
		ExternalID:         pgtype.Text{String: input.ExternalID, Valid: true},
		ProvisioningSource: pgtype.Text{String: "scim", Valid: true},
	}); err != nil {
		return fmt.Errorf("update provisioned identity profile: %w", err)
	}
	return nil
}

func (s *ProvisioningService) deactivateSideEffects(ctx context.Context, userID int64) error {
	if s.sessionService != nil {
		if err := s.sessionService.DeleteUserSessions(ctx, userID); err != nil {
			return err
		}
	}
	if s.delegationService != nil {
		if err := s.delegationService.DeleteAllGrantsForUser(ctx, userID); err != nil {
			return err
		}
	}
	if err := s.queries.DeleteOAuthUserGrantsByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete deactivated user grants: %w", err)
	}
	return nil
}

func normalizeProvisionedUser(input ProvisionedUserInput) (ProvisionedUserInput, error) {
	externalID := strings.TrimSpace(input.ExternalID)
	userName := strings.ToLower(strings.TrimSpace(input.UserName))
	displayName := strings.TrimSpace(input.DisplayName)
	if externalID == "" || userName == "" {
		return ProvisionedUserInput{}, ErrInvalidSCIMUser
	}
	if displayName == "" {
		displayName = fallbackDisplayName(userName, externalID)
	}
	return ProvisionedUserInput{
		ExternalID:  externalID,
		UserName:    userName,
		DisplayName: displayName,
		Active:      input.Active,
		Groups:      input.Groups,
	}, nil
}

func deactivatedAtForActive(active bool) pgtype.Timestamptz {
	if active {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
}

func rowFromUser(id int64, publicID, email, displayName string, deactivatedAt pgtype.Timestamptz, defaultTenantID pgtype.Int8, externalID string) db.GetProvisionedUserByExternalIDRow {
	parsed, _ := uuid.Parse(publicID)
	return db.GetProvisionedUserByExternalIDRow{
		ID:                 id,
		PublicID:           parsed,
		Email:              email,
		DisplayName:        displayName,
		DeactivatedAt:      deactivatedAt,
		DefaultTenantID:    defaultTenantID,
		Provider:           scimIdentityProvider,
		Subject:            externalID,
		ExternalID:         pgtype.Text{String: externalID, Valid: true},
		ProvisioningSource: pgtype.Text{String: "scim", Valid: true},
	}
}

func provisionedUserFromRow(row db.GetProvisionedUserByExternalIDRow) ProvisionedUser {
	externalID := ""
	if row.ExternalID.Valid {
		externalID = row.ExternalID.String
	}
	return ProvisionedUser{
		ID:          row.ID,
		PublicID:    row.PublicID.String(),
		ExternalID:  externalID,
		UserName:    row.Email,
		DisplayName: row.DisplayName,
		Active:      !row.DeactivatedAt.Valid,
	}
}

func provisionedUserFromPublicIDRow(row db.GetProvisionedUserByPublicIDRow) ProvisionedUser {
	return provisionedUserFromRow(db.GetProvisionedUserByExternalIDRow(row))
}

func provisionedUserFromListRow(row db.ListProvisionedUsersRow) ProvisionedUser {
	return provisionedUserFromRow(db.GetProvisionedUserByExternalIDRow(row))
}
