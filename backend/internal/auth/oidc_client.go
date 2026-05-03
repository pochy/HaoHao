package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type IdentityClaims struct {
	Subject       string   `json:"sub"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Name          string   `json:"name"`
	Groups        []string `json:"groups,omitempty"`
}

type OIDCIdentity struct {
	Claims     IdentityClaims
	RawIDToken string
}

type OIDCClient struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	config   *oauth2.Config
}

func NewOIDCClient(ctx context.Context, issuer, clientID, clientSecret, redirectURI, scopes string) (*OIDCClient, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider: %w", err)
	}

	oauthScopes := strings.Fields(scopes)
	if len(oauthScopes) == 0 {
		oauthScopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	return &OIDCClient{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  redirectURI,
			Scopes:       oauthScopes,
		},
	}, nil
}

func (c *OIDCClient) AuthorizeURL(state, nonce, codeVerifier string) string {
	return c.config.AuthCodeURL(
		state,
		oidc.Nonce(nonce),
		oauth2.SetAuthURLParam("code_challenge", pkceS256(codeVerifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func (c *OIDCClient) ExchangeCode(ctx context.Context, code, codeVerifier, expectedNonce string) (OIDCIdentity, error) {
	token, err := c.config.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("exchange authorization code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return OIDCIdentity{}, fmt.Errorf("id_token missing from token response")
	}

	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("verify id token: %w", err)
	}

	var verified struct {
		Subject string `json:"sub"`
		Nonce   string `json:"nonce"`
	}
	if err := idToken.Claims(&verified); err != nil {
		return OIDCIdentity{}, fmt.Errorf("decode id token claims: %w", err)
	}
	if verified.Nonce != expectedNonce {
		return OIDCIdentity{}, fmt.Errorf("oidc nonce mismatch")
	}

	userInfo, err := c.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("fetch userinfo: %w", err)
	}

	var claims IdentityClaims
	if err := userInfo.Claims(&claims); err != nil {
		return OIDCIdentity{}, fmt.Errorf("decode userinfo claims: %w", err)
	}

	claims.Subject = verified.Subject
	return OIDCIdentity{
		Claims:     claims,
		RawIDToken: rawIDToken,
	}, nil
}

func pkceS256(codeVerifier string) string {
	hash := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}
