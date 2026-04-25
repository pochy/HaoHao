package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidM2MToken       = errors.New("invalid m2m bearer token")
	ErrMissingM2MClientID    = errors.New("missing m2m client id")
	ErrHumanBearerToken      = errors.New("human bearer token is not allowed for m2m")
	ErrM2MVerifierNotEnabled = errors.New("m2m verifier is not configured")
)

type M2MVerifier struct {
	bearerVerifier      *BearerVerifier
	expectedAudience    string
	requiredScopePrefix string
}

type M2MPrincipal struct {
	ProviderClientID string
	Claims           BearerTokenClaims
}

func NewM2MVerifier(bearerVerifier *BearerVerifier, expectedAudience, requiredScopePrefix string) *M2MVerifier {
	return &M2MVerifier{
		bearerVerifier:      bearerVerifier,
		expectedAudience:    strings.TrimSpace(expectedAudience),
		requiredScopePrefix: strings.TrimSpace(requiredScopePrefix),
	}
}

func (v *M2MVerifier) Verify(ctx context.Context, rawToken string) (M2MPrincipal, error) {
	if v == nil || v.bearerVerifier == nil {
		return M2MPrincipal{}, ErrM2MVerifierNotEnabled
	}

	claims, err := v.bearerVerifier.Verify(ctx, rawToken, v.expectedAudience, v.requiredScopePrefix)
	if err != nil {
		return M2MPrincipal{}, err
	}

	if claims.HasHumanUserClaims() {
		return M2MPrincipal{}, ErrHumanBearerToken
	}

	providerClientID := strings.TrimSpace(claims.ClientID)
	if providerClientID == "" {
		providerClientID = strings.TrimSpace(claims.AuthorizedParty)
	}
	if providerClientID == "" {
		return M2MPrincipal{}, ErrMissingM2MClientID
	}

	return M2MPrincipal{
		ProviderClientID: providerClientID,
		Claims:           claims,
	}, nil
}

func (c BearerTokenClaims) HasHumanUserClaims() bool {
	return strings.TrimSpace(c.Email) != "" ||
		strings.TrimSpace(c.PreferredUsername) != "" ||
		len(c.Groups) > 0 ||
		len(c.Roles) > 0
}

func (p M2MPrincipal) ScopeValuesWithPrefix(prefix string) []string {
	requiredPrefix := strings.TrimSpace(prefix)
	if requiredPrefix == "" {
		return p.Claims.ScopeValues()
	}

	scopes := make([]string, 0, len(p.Claims.Scope))
	for _, scope := range p.Claims.Scope {
		trimmed := strings.TrimSpace(scope)
		if trimmed == requiredPrefix || strings.HasPrefix(trimmed, requiredPrefix) {
			scopes = append(scopes, trimmed)
		}
	}
	return scopes
}

func (p M2MPrincipal) Validate() error {
	if strings.TrimSpace(p.ProviderClientID) == "" {
		return fmt.Errorf("%w: provider client id is required", ErrInvalidM2MToken)
	}
	return nil
}
