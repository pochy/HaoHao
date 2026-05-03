package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidExternalIdentity = errors.New("invalid external identity")

type ExternalIdentity struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
}

type IdentityService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewIdentityService(pool *pgxpool.Pool, queries *db.Queries) *IdentityService {
	return &IdentityService{
		pool:    pool,
		queries: queries,
	}
}

func (s *IdentityService) ResolveOrCreateUser(ctx context.Context, identity ExternalIdentity) (User, error) {
	normalized, err := normalizeExternalIdentity(identity)
	if err != nil {
		return User{}, err
	}

	existing, err := s.queries.GetUserByProviderSubject(ctx, db.GetUserByProviderSubjectParams{
		Provider: normalized.Provider,
		Subject:  normalized.Subject,
	})
	if err == nil {
		_ = s.queries.UpdateUserIdentityProfile(ctx, db.UpdateUserIdentityProfileParams{
			Provider:      normalized.Provider,
			Subject:       normalized.Subject,
			Email:         normalized.Email,
			EmailVerified: normalized.EmailVerified,
		})

		return s.syncUserProfile(ctx, s.queries, dbUser(existing.ID, existing.PublicID.String(), existing.Email, existing.DisplayName), normalized)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return User{}, fmt.Errorf("lookup identity by provider subject: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, fmt.Errorf("begin identity transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	user, err := s.resolveUserForIdentity(ctx, qtx, normalized)
	if err != nil {
		return User{}, err
	}

	if err := qtx.CreateUserIdentity(ctx, db.CreateUserIdentityParams{
		UserID:        user.ID,
		Provider:      normalized.Provider,
		Subject:       normalized.Subject,
		Email:         normalized.Email,
		EmailVerified: normalized.EmailVerified,
	}); err != nil {
		return User{}, fmt.Errorf("create user identity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit identity transaction: %w", err)
	}

	return user, nil
}

func (s *IdentityService) resolveUserForIdentity(ctx context.Context, queries *db.Queries, identity ExternalIdentity) (User, error) {
	if identity.EmailVerified {
		existing, err := queries.GetUserByEmail(ctx, identity.Email)
		if err == nil {
			return s.syncUserProfile(ctx, queries, dbUser(existing.ID, existing.PublicID.String(), existing.Email, existing.DisplayName), identity)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return User{}, fmt.Errorf("lookup user by email: %w", err)
		}
	}

	created, err := queries.CreateOIDCUser(ctx, db.CreateOIDCUserParams{
		Email:       identity.Email,
		DisplayName: identity.DisplayName,
	})
	if err != nil {
		return User{}, fmt.Errorf("create oidc user: %w", err)
	}

	return dbUser(created.ID, created.PublicID.String(), created.Email, created.DisplayName), nil
}

func (s *IdentityService) syncUserProfile(ctx context.Context, queries *db.Queries, user User, identity ExternalIdentity) (User, error) {
	nextEmail := user.Email
	if identity.EmailVerified && identity.Email != "" {
		nextEmail = identity.Email
	}

	nextDisplayName := user.DisplayName
	if identity.DisplayName != "" {
		nextDisplayName = identity.DisplayName
	}

	if nextEmail == user.Email && nextDisplayName == user.DisplayName {
		return user, nil
	}

	updated, err := queries.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		ID:          user.ID,
		Email:       nextEmail,
		DisplayName: nextDisplayName,
	})
	if err != nil {
		return User{}, fmt.Errorf("update user profile: %w", err)
	}

	return dbUser(updated.ID, updated.PublicID.String(), updated.Email, updated.DisplayName), nil
}

func normalizeExternalIdentity(identity ExternalIdentity) (ExternalIdentity, error) {
	provider := strings.ToLower(strings.TrimSpace(identity.Provider))
	subject := strings.TrimSpace(identity.Subject)
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	displayName := strings.TrimSpace(identity.DisplayName)

	if provider == "" || subject == "" || email == "" {
		return ExternalIdentity{}, ErrInvalidExternalIdentity
	}
	if displayName == "" {
		displayName = fallbackDisplayName(email, subject)
	}

	return ExternalIdentity{
		Provider:      provider,
		Subject:       subject,
		Email:         email,
		EmailVerified: identity.EmailVerified,
		DisplayName:   displayName,
	}, nil
}

func fallbackDisplayName(email, subject string) string {
	if email != "" {
		if head, _, ok := strings.Cut(email, "@"); ok && head != "" {
			return head
		}
		return email
	}
	return subject
}

func dbUser(id int64, publicID, email, displayName string) User {
	return User{
		ID:          id,
		PublicID:    publicID,
		Email:       email,
		DisplayName: displayName,
	}
}
