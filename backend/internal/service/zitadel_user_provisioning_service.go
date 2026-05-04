package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrIdentityProvisioningUnavailable = errors.New("identity provisioning is unavailable")
	ErrIdentityProvisioningFailed      = errors.New("identity provisioning failed")
)

type TenantInvitationIdentityProvisioner interface {
	EnsureHumanUser(ctx context.Context, email string) (IdentityProvisioningResult, error)
}

type IdentityProvisioningResult struct {
	DeliveryMode          string
	UserID                string
	InviteCode            string
	LoginURL              string
	EmailVerificationCode string
}

type UnavailableIdentityProvisioner struct {
	reason string
}

func NewUnavailableIdentityProvisioner(reason string) UnavailableIdentityProvisioner {
	return UnavailableIdentityProvisioner{reason: strings.TrimSpace(reason)}
}

func (p UnavailableIdentityProvisioner) EnsureHumanUser(ctx context.Context, email string) (IdentityProvisioningResult, error) {
	if p.reason == "" {
		return IdentityProvisioningResult{}, ErrIdentityProvisioningUnavailable
	}
	return IdentityProvisioningResult{}, fmt.Errorf("%w: %s", ErrIdentityProvisioningUnavailable, p.reason)
}

type ZitadelUserProvisioningConfig struct {
	Issuer          string
	ManagementToken string
	OrganizationID  string
	DefaultLanguage string
	DeliveryMode    string
	HTTPClient      *http.Client
}

type ZitadelUserProvisioningService struct {
	baseURL         string
	token           string
	organizationID  string
	defaultLanguage string
	deliveryMode    string
	client          *http.Client
}

type zitadelCreateUserResponse struct {
	ID        string `json:"id"`
	EmailCode string `json:"emailCode"`
}

type zitadelOrgMeResponse struct {
	Org struct {
		ID string `json:"id"`
	} `json:"org"`
}

type zitadelListUsersResponse struct {
	Result []struct {
		UserID     string   `json:"userId"`
		Username   string   `json:"username"`
		LoginNames []string `json:"loginNames"`
		Human      *struct {
			Email struct {
				Email string `json:"email"`
			} `json:"email"`
			PasswordChanged string `json:"passwordChanged"`
		} `json:"human"`
	} `json:"result"`
}

func NewZitadelUserProvisioningService(cfg ZitadelUserProvisioningConfig) *ZitadelUserProvisioningService {
	return &ZitadelUserProvisioningService{
		baseURL:         strings.TrimRight(strings.TrimSpace(cfg.Issuer), "/"),
		token:           strings.TrimSpace(cfg.ManagementToken),
		organizationID:  strings.TrimSpace(cfg.OrganizationID),
		defaultLanguage: defaultLanguage(cfg.DefaultLanguage),
		deliveryMode:    zitadelInviteDeliveryMode(cfg.DeliveryMode),
		client:          httpClientOrDefault(cfg.HTTPClient),
	}
}

func (s *ZitadelUserProvisioningService) EnsureHumanUser(ctx context.Context, email string) (IdentityProvisioningResult, error) {
	if s == nil || s.baseURL == "" || s.token == "" {
		return IdentityProvisioningResult{}, ErrIdentityProvisioningUnavailable
	}
	email = normalizeEmail(email)
	if email == "" {
		return IdentityProvisioningResult{}, fmt.Errorf("%w: email is required", ErrInvalidTenantInvitation)
	}

	organizationID, err := s.resolveOrganizationID(ctx)
	if err != nil {
		return IdentityProvisioningResult{}, err
	}

	body, err := json.Marshal(zitadelCreateHumanUserRequest(email, organizationID, s.defaultLanguage, s.deliveryMode))
	if err != nil {
		return IdentityProvisioningResult{}, fmt.Errorf("%w: encode request: %v", ErrIdentityProvisioningFailed, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/v2/users/new", bytes.NewReader(body))
	if err != nil {
		return IdentityProvisioningResult{}, fmt.Errorf("%w: build request: %v", ErrIdentityProvisioningFailed, err)
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if organizationID != "" {
		req.Header.Set("x-zitadel-orgid", organizationID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return IdentityProvisioningResult{}, fmt.Errorf("%w: call zitadel: %v", ErrIdentityProvisioningFailed, err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var created zitadelCreateUserResponse
		_ = json.Unmarshal(responseBody, &created)
		if created.ID != "" {
			result, err := s.createInviteCode(ctx, created.ID, email, organizationID)
			result.UserID = created.ID
			result.EmailVerificationCode = created.EmailCode
			return result, err
		}
		return IdentityProvisioningResult{DeliveryMode: s.deliveryMode, EmailVerificationCode: created.EmailCode}, nil
	}
	if zitadelConflictMeansAlreadyExists(resp.StatusCode, responseBody) {
		existing, hasPassword, err := s.findHumanUser(ctx, email, organizationID)
		if err != nil {
			return IdentityProvisioningResult{}, err
		}
		if existing == "" || hasPassword {
			return IdentityProvisioningResult{DeliveryMode: s.deliveryMode, UserID: existing}, nil
		}
		return s.createInviteCode(ctx, existing, email, organizationID)
	}
	return IdentityProvisioningResult{}, fmt.Errorf("%w: zitadel returned %d: %s", ErrIdentityProvisioningFailed, resp.StatusCode, strings.TrimSpace(string(responseBody)))
}

func zitadelCreateHumanUserRequest(email, organizationID, defaultLanguage, deliveryMode string) map[string]any {
	localPart := email
	if at := strings.IndexByte(email, '@'); at > 0 {
		localPart = email[:at]
	}
	request := map[string]any{
		"username":       email,
		"organizationId": organizationID,
		"human": map[string]any{
			"profile": map[string]any{
				"givenName":         localPart,
				"familyName":        "User",
				"displayName":       localPart,
				"preferredLanguage": defaultLanguage,
			},
			"email": zitadelCodeDeliveryRequest(email, deliveryMode),
		},
	}
	return request
}

func zitadelCodeDeliveryRequest(email, deliveryMode string) map[string]any {
	request := map[string]any{"email": email}
	if deliveryMode == "return_code" {
		request["returnCode"] = map[string]any{}
	} else {
		request["sendCode"] = map[string]any{}
	}
	return request
}

func (s *ZitadelUserProvisioningService) resolveOrganizationID(ctx context.Context) (string, error) {
	if s.organizationID != "" {
		return s.organizationID, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/management/v1/orgs/me", nil)
	if err != nil {
		return "", fmt.Errorf("%w: build organization request: %v", ErrIdentityProvisioningFailed, err)
	}
	s.setHeaders(req, "")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: call zitadel organization: %v", ErrIdentityProvisioningFailed, err)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%w: zitadel organization returned %d: %s", ErrIdentityProvisioningFailed, resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var out zitadelOrgMeResponse
	if err := json.Unmarshal(responseBody, &out); err != nil {
		return "", fmt.Errorf("%w: decode organization response: %v", ErrIdentityProvisioningFailed, err)
	}
	if out.Org.ID == "" {
		return "", fmt.Errorf("%w: organization id is empty", ErrIdentityProvisioningFailed)
	}
	return out.Org.ID, nil
}

func (s *ZitadelUserProvisioningService) createInviteCode(ctx context.Context, userID, loginName, organizationID string) (IdentityProvisioningResult, error) {
	body := `{"sendCode":{}}`
	if s.deliveryMode == "return_code" {
		body = `{"returnCode":{}}`
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/v2/users/"+userID+"/invite_code", strings.NewReader(body))
	if err != nil {
		return IdentityProvisioningResult{}, fmt.Errorf("%w: build invite code request: %v", ErrIdentityProvisioningFailed, err)
	}
	s.setHeaders(req, organizationID)
	resp, err := s.client.Do(req)
	if err != nil {
		return IdentityProvisioningResult{}, fmt.Errorf("%w: call zitadel invite code: %v", ErrIdentityProvisioningFailed, err)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result := IdentityProvisioningResult{DeliveryMode: s.deliveryMode, UserID: userID}
		if s.deliveryMode == "return_code" {
			var out struct {
				InviteCode string `json:"inviteCode"`
			}
			_ = json.Unmarshal(responseBody, &out)
			result.InviteCode = out.InviteCode
			result.LoginURL = s.inviteLoginURL(userID, loginName, out.InviteCode, organizationID)
		}
		return result, nil
	}
	return IdentityProvisioningResult{}, fmt.Errorf("%w: zitadel invite code returned %d: %s", ErrIdentityProvisioningFailed, resp.StatusCode, strings.TrimSpace(string(responseBody)))
}

func (s *ZitadelUserProvisioningService) inviteLoginURL(userID, loginName, code, organizationID string) string {
	if s == nil || s.deliveryMode != "return_code" || s.baseURL == "" || userID == "" || loginName == "" || code == "" {
		return ""
	}
	values := url.Values{}
	values.Set("userID", userID)
	values.Set("loginname", loginName)
	values.Set("code", code)
	if organizationID != "" {
		values.Set("orgID", organizationID)
	}
	return s.baseURL + "/ui/login/user/invite?" + values.Encode()
}

func (s *ZitadelUserProvisioningService) findHumanUser(ctx context.Context, email, organizationID string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/v2/users", strings.NewReader(`{"query":{"offset":0,"limit":100}}`))
	if err != nil {
		return "", false, fmt.Errorf("%w: build user search request: %v", ErrIdentityProvisioningFailed, err)
	}
	s.setHeaders(req, organizationID)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("%w: call zitadel user search: %v", ErrIdentityProvisioningFailed, err)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, fmt.Errorf("%w: zitadel user search returned %d: %s", ErrIdentityProvisioningFailed, resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var out zitadelListUsersResponse
	if err := json.Unmarshal(responseBody, &out); err != nil {
		return "", false, fmt.Errorf("%w: decode user search response: %v", ErrIdentityProvisioningFailed, err)
	}
	for _, user := range out.Result {
		if user.Human == nil {
			continue
		}
		if normalizeEmail(user.Username) == email || normalizeEmail(user.Human.Email.Email) == email || containsNormalized(user.LoginNames, email) {
			return user.UserID, user.Human.PasswordChanged != "", nil
		}
	}
	return "", false, nil
}

func (s *ZitadelUserProvisioningService) setHeaders(req *http.Request, organizationID string) {
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if organizationID != "" {
		req.Header.Set("x-zitadel-orgid", organizationID)
	}
}

func containsNormalized(values []string, target string) bool {
	for _, value := range values {
		if normalizeEmail(value) == target {
			return true
		}
	}
	return false
}

func zitadelConflictMeansAlreadyExists(statusCode int, body []byte) bool {
	if statusCode != http.StatusConflict {
		return false
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "already") ||
		strings.Contains(lower, "exists") ||
		strings.Contains(lower, "already_exists") ||
		strings.Contains(lower, "user_already_exists") ||
		strings.Contains(lower, "resource_already_exists")
}

func defaultLanguage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "ja"
	}
	return value
}

func zitadelInviteDeliveryMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "return_code", "code", "dev":
		return "return_code"
	default:
		return "email"
	}
}

func httpClientOrDefault(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: 10 * time.Second}
}
