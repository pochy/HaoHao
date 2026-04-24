package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type DelegatedOAuthClient struct {
	clientID           string
	clientSecret       string
	endpoint           oauth2.Endpoint
	revocationEndpoint string
}

type DelegatedToken struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
	Scopes       []string
}

func NewDelegatedOAuthClient(ctx context.Context, issuer, clientID, clientSecret string) (*DelegatedOAuthClient, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover delegated oauth provider: %w", err)
	}

	var metadata struct {
		RevocationEndpoint string `json:"revocation_endpoint"`
	}
	if err := provider.Claims(&metadata); err != nil {
		return nil, fmt.Errorf("decode delegated oauth provider metadata: %w", err)
	}

	return &DelegatedOAuthClient{
		clientID:           clientID,
		clientSecret:       clientSecret,
		endpoint:           provider.Endpoint(),
		revocationEndpoint: metadata.RevocationEndpoint,
	}, nil
}

func (c *DelegatedOAuthClient) AuthorizeURL(state, codeVerifier, redirectURI string, scopes []string) string {
	return c.config(redirectURI, scopes).AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge", pkceS256(codeVerifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func (c *DelegatedOAuthClient) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string, scopes []string) (DelegatedToken, error) {
	token, err := c.config(redirectURI, scopes).Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return DelegatedToken{}, fmt.Errorf("exchange delegated authorization code: %w", err)
	}

	return delegatedTokenFromOAuth2(token, scopes), nil
}

func (c *DelegatedOAuthClient) Refresh(ctx context.Context, refreshToken, redirectURI string, scopes []string) (DelegatedToken, error) {
	token, err := c.config(redirectURI, scopes).TokenSource(ctx, &oauth2.Token{
		RefreshToken: refreshToken,
	}).Token()
	if err != nil {
		return DelegatedToken{}, fmt.Errorf("refresh delegated access token: %w", err)
	}

	return delegatedTokenFromOAuth2(token, scopes), nil
}

func (c *DelegatedOAuthClient) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	if c == nil || c.revocationEndpoint == "" || refreshToken == "" {
		return nil
	}

	form := url.Values{}
	form.Set("token", refreshToken)
	form.Set("token_type_hint", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.revocationEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build token revocation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.clientID, c.clientSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("revoke delegated refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("revoke delegated refresh token: provider returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func IsInvalidGrantError(err error) bool {
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "invalid_grant" {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "invalid_grant")
}

func (c *DelegatedOAuthClient) config(redirectURI string, scopes []string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.clientID,
		ClientSecret: c.clientSecret,
		Endpoint:     c.endpoint,
		RedirectURL:  redirectURI,
		Scopes:       scopes,
	}
}

func delegatedTokenFromOAuth2(token *oauth2.Token, fallbackScopes []string) DelegatedToken {
	scopes := fallbackScopes
	if rawScope, ok := token.Extra("scope").(string); ok && strings.TrimSpace(rawScope) != "" {
		scopes = strings.Fields(rawScope)
	}

	return DelegatedToken{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
		Scopes:       scopes,
	}
}
