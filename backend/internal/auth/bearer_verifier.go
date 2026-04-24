package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4/jwt"
)

var (
	ErrMissingBearerToken    = errors.New("missing bearer token")
	ErrInvalidBearerToken    = errors.New("invalid bearer token")
	ErrInvalidBearerIssuer   = errors.New("invalid bearer issuer")
	ErrInvalidBearerAudience = errors.New("invalid bearer audience")
	ErrInvalidBearerScope    = errors.New("invalid bearer scope")
	ErrInvalidBearerRole     = errors.New("invalid bearer role")
)

type BearerVerifier struct {
	issuer string
	keySet *oidc.RemoteKeySet
}

type BearerTokenClaims struct {
	jwt.Claims
	AuthorizedParty   string             `json:"azp,omitempty"`
	ClientID          string             `json:"client_id,omitempty"`
	Scope             spaceSeparatedList `json:"scope,omitempty"`
	Groups            claimStringList    `json:"groups,omitempty"`
	Roles             []string           `json:"-"`
	Email             string             `json:"email,omitempty"`
	Name              string             `json:"name,omitempty"`
	PreferredUsername string             `json:"preferred_username,omitempty"`
}

type bearerTokenClaimsJSON struct {
	jwt.Claims
	AuthorizedParty   string             `json:"azp,omitempty"`
	ClientID          string             `json:"client_id,omitempty"`
	Scope             spaceSeparatedList `json:"scope,omitempty"`
	Groups            claimStringList    `json:"groups,omitempty"`
	Email             string             `json:"email,omitempty"`
	Name              string             `json:"name,omitempty"`
	PreferredUsername string             `json:"preferred_username,omitempty"`
}

func (c *BearerTokenClaims) UnmarshalJSON(data []byte) error {
	var decoded bearerTokenClaimsJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	var rawClaims map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawClaims); err != nil {
		return err
	}

	c.Claims = decoded.Claims
	c.AuthorizedParty = strings.TrimSpace(decoded.AuthorizedParty)
	c.ClientID = strings.TrimSpace(decoded.ClientID)
	if c.AuthorizedParty == "" {
		c.AuthorizedParty = c.ClientID
	}
	c.Scope = decoded.Scope
	c.Groups = decoded.Groups
	c.Roles = extractZitadelRoleClaims(rawClaims)
	c.Email = decoded.Email
	c.Name = decoded.Name
	c.PreferredUsername = decoded.PreferredUsername

	return nil
}

func NewBearerVerifier(ctx context.Context, issuer string) (*BearerVerifier, error) {
	trimmedIssuer := strings.TrimRight(strings.TrimSpace(issuer), "/")
	if trimmedIssuer == "" {
		return nil, fmt.Errorf("issuer is required")
	}

	provider, err := oidc.NewProvider(ctx, trimmedIssuer)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider: %w", err)
	}

	var discovery struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := provider.Claims(&discovery); err != nil {
		return nil, fmt.Errorf("decode oidc discovery document: %w", err)
	}
	if strings.TrimSpace(discovery.JWKSURI) == "" {
		return nil, fmt.Errorf("jwks_uri missing from oidc discovery document")
	}

	return &BearerVerifier{
		issuer: trimmedIssuer,
		keySet: oidc.NewRemoteKeySet(ctx, discovery.JWKSURI),
	}, nil
}

func (v *BearerVerifier) Verify(ctx context.Context, rawToken, expectedAudience, requiredScopePrefix string) (BearerTokenClaims, error) {
	if strings.TrimSpace(rawToken) == "" {
		return BearerTokenClaims{}, ErrMissingBearerToken
	}
	if v == nil || v.keySet == nil {
		return BearerTokenClaims{}, fmt.Errorf("%w: verifier is not configured", ErrInvalidBearerToken)
	}

	payload, err := v.keySet.VerifySignature(ctx, rawToken)
	if err != nil {
		return BearerTokenClaims{}, fmt.Errorf("%w: verify signature: %v", ErrInvalidBearerToken, err)
	}

	var claims BearerTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return BearerTokenClaims{}, fmt.Errorf("%w: decode claims: %v", ErrInvalidBearerToken, err)
	}

	expected := jwt.Expected{
		Issuer: v.issuer,
		Time:   time.Now(),
	}
	if audience := strings.TrimSpace(expectedAudience); audience != "" {
		expected.AnyAudience = jwt.Audience{audience}
	}

	if err := claims.Claims.ValidateWithLeeway(expected, time.Minute); err != nil {
		switch {
		case errors.Is(err, jwt.ErrInvalidIssuer):
			return BearerTokenClaims{}, ErrInvalidBearerIssuer
		case errors.Is(err, jwt.ErrInvalidAudience):
			return BearerTokenClaims{}, ErrInvalidBearerAudience
		default:
			return BearerTokenClaims{}, fmt.Errorf("%w: %v", ErrInvalidBearerToken, err)
		}
	}

	if strings.TrimSpace(claims.Subject) == "" {
		return BearerTokenClaims{}, fmt.Errorf("%w: subject is required", ErrInvalidBearerToken)
	}
	if prefix := strings.TrimSpace(requiredScopePrefix); prefix != "" && !claims.HasScopePrefix(prefix) {
		return BearerTokenClaims{}, ErrInvalidBearerScope
	}

	return claims, nil
}

func (c BearerTokenClaims) ScopeValues() []string {
	return append([]string(nil), c.Scope...)
}

func (c BearerTokenClaims) GroupValues() []string {
	return append([]string(nil), c.Groups...)
}

func (c BearerTokenClaims) RoleValues() []string {
	return append([]string(nil), c.Roles...)
}

func (c BearerTokenClaims) HasScopePrefix(prefix string) bool {
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" {
		return true
	}

	for _, scope := range c.Scope {
		if scope == trimmedPrefix || strings.HasPrefix(scope, trimmedPrefix) {
			return true
		}
	}

	return false
}

func extractZitadelRoleClaims(rawClaims map[string]json.RawMessage) []string {
	roleSet := make(map[string]struct{})
	for name, raw := range rawClaims {
		if !isZitadelRoleClaim(name) {
			continue
		}

		for _, role := range roleNamesFromClaim(raw) {
			roleSet[role] = struct{}{}
		}
	}

	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

func isZitadelRoleClaim(name string) bool {
	return name == "urn:zitadel:iam:org:project:roles" ||
		(strings.HasPrefix(name, "urn:zitadel:iam:org:project:") && strings.HasSuffix(name, ":roles"))
}

func roleNamesFromClaim(raw json.RawMessage) []string {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err == nil {
		roles := make([]string, 0, len(object))
		for role := range object {
			if trimmed := strings.TrimSpace(role); trimmed != "" {
				roles = append(roles, trimmed)
			}
		}
		return roles
	}

	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		roles := make([]string, 0, len(many))
		for _, role := range many {
			if trimmed := strings.TrimSpace(role); trimmed != "" {
				roles = append(roles, trimmed)
			}
		}
		return roles
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if trimmed := strings.TrimSpace(single); trimmed != "" {
			return []string{trimmed}
		}
	}

	return nil
}

type spaceSeparatedList []string

func (s *spaceSeparatedList) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = append((*s)[:0], strings.Fields(single)...)
		return nil
	}

	var many []string
	if err := json.Unmarshal(data, &many); err == nil {
		items := make([]string, 0, len(many))
		for _, item := range many {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
		*s = items
		return nil
	}

	return fmt.Errorf("unsupported scope claim format")
}

type claimStringList []string

func (s *claimStringList) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			*s = nil
			return nil
		}
		*s = []string{single}
		return nil
	}

	var many []string
	if err := json.Unmarshal(data, &many); err == nil {
		items := make([]string, 0, len(many))
		for _, item := range many {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
		*s = items
		return nil
	}

	return fmt.Errorf("unsupported string list claim format")
}
