package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestZitadelUserProvisioningServiceEnsureHumanUserCreatesHumanUser(t *testing.T) {
	var gotAuthorization string
	var gotOrganization string
	var gotPayload map[string]any
	var inviteCodeCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v2/users/created-user/invite_code" {
			inviteCodeCalled = true
			_, _ = w.Write([]byte(`{}`))
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v2/users/new" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		gotAuthorization = r.Header.Get("Authorization")
		gotOrganization = r.Header.Get("x-zitadel-orgid")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"created-user"}`))
	}))
	defer server.Close()

	svc := NewZitadelUserProvisioningService(ZitadelUserProvisioningConfig{
		Issuer:          server.URL,
		ManagementToken: "token",
		OrganizationID:  "org-1",
		DefaultLanguage: "ja",
	})

	if _, err := svc.EnsureHumanUser(context.Background(), " Test@Nobori.Example "); err != nil {
		t.Fatalf("EnsureHumanUser returned error: %v", err)
	}
	if gotAuthorization != "Bearer token" {
		t.Fatalf("authorization header = %q", gotAuthorization)
	}
	if gotOrganization != "org-1" {
		t.Fatalf("organization header = %q", gotOrganization)
	}
	if gotPayload["username"] != "test@nobori.example" {
		t.Fatalf("username = %#v", gotPayload["username"])
	}
	if gotPayload["organizationId"] != "org-1" {
		t.Fatalf("organizationId = %#v", gotPayload["organizationId"])
	}
	human, ok := gotPayload["human"].(map[string]any)
	if !ok {
		t.Fatalf("human payload = %#v", gotPayload["human"])
	}
	email, ok := human["email"].(map[string]any)
	if !ok || email["email"] != "test@nobori.example" || email["sendCode"] == nil {
		t.Fatalf("email payload = %#v", human["email"])
	}
	if !inviteCodeCalled {
		t.Fatal("expected invite code request")
	}
}

func TestZitadelUserProvisioningServiceEnsureHumanUserConflictIsSuccess(t *testing.T) {
	var inviteCodeCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/management/v1/orgs/me":
			_, _ = w.Write([]byte(`{"org":{"id":"org-1"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v2/users/new":
			http.Error(w, `{"message":"user already exists"}`, http.StatusConflict)
		case r.Method == http.MethodPost && r.URL.Path == "/v2/users":
			_, _ = w.Write([]byte(`{"result":[{"userId":"existing-user","username":"test@nobori.example","loginNames":["test@nobori.example"],"human":{"email":{"email":"test@nobori.example"}}}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v2/users/existing-user/invite_code":
			inviteCodeCalled = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	svc := NewZitadelUserProvisioningService(ZitadelUserProvisioningConfig{
		Issuer:          server.URL,
		ManagementToken: "token",
	})

	if _, err := svc.EnsureHumanUser(context.Background(), "test@nobori.example"); err != nil {
		t.Fatalf("EnsureHumanUser returned error: %v", err)
	}
	if !inviteCodeCalled {
		t.Fatal("expected invite code request")
	}
}

func TestZitadelUserProvisioningServiceEnsureHumanUserUnavailable(t *testing.T) {
	svc := NewZitadelUserProvisioningService(ZitadelUserProvisioningConfig{
		Issuer: "http://example.test",
	})

	if _, err := svc.EnsureHumanUser(context.Background(), "test@nobori.example"); err == nil {
		t.Fatal("expected error")
	} else if !errors.Is(err, ErrIdentityProvisioningUnavailable) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}

func TestZitadelUserProvisioningServiceEnsureHumanUserFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/management/v1/orgs/me":
			_, _ = w.Write([]byte(`{"org":{"id":"org-1"}}`))
		default:
			http.Error(w, `{"message":"down"}`, http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()

	svc := NewZitadelUserProvisioningService(ZitadelUserProvisioningConfig{
		Issuer:          server.URL,
		ManagementToken: "token",
	})

	if _, err := svc.EnsureHumanUser(context.Background(), "test@nobori.example"); err == nil {
		t.Fatal("expected error")
	} else if !errors.Is(err, ErrIdentityProvisioningFailed) {
		t.Fatalf("expected provisioning failed error, got %v", err)
	}
}

func TestZitadelUserProvisioningServiceEnsureHumanUserReturnCode(t *testing.T) {
	var gotCreatePayload map[string]any
	var gotInvitePayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v2/users/new":
			if err := json.NewDecoder(r.Body).Decode(&gotCreatePayload); err != nil {
				t.Fatalf("decode create request: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":"created-user","emailCode":"EMAIL1"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v2/users/created-user/invite_code":
			if err := json.NewDecoder(r.Body).Decode(&gotInvitePayload); err != nil {
				t.Fatalf("decode invite request: %v", err)
			}
			_, _ = w.Write([]byte(`{"inviteCode":"INVITE1"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	svc := NewZitadelUserProvisioningService(ZitadelUserProvisioningConfig{
		Issuer:          server.URL,
		ManagementToken: "token",
		OrganizationID:  "org-1",
		DeliveryMode:    "return_code",
	})

	result, err := svc.EnsureHumanUser(context.Background(), "test@nobori.example")
	if err != nil {
		t.Fatalf("EnsureHumanUser returned error: %v", err)
	}
	human := gotCreatePayload["human"].(map[string]any)
	email := human["email"].(map[string]any)
	if email["returnCode"] == nil || email["sendCode"] != nil {
		t.Fatalf("email payload = %#v", email)
	}
	if gotInvitePayload["returnCode"] == nil || gotInvitePayload["sendCode"] != nil {
		t.Fatalf("invite payload = %#v", gotInvitePayload)
	}
	if result.DeliveryMode != "return_code" || result.InviteCode != "INVITE1" || result.EmailVerificationCode != "EMAIL1" {
		t.Fatalf("unexpected result: %#v", result)
	}
	expectedLoginURL := server.URL + "/ui/login/user/invite?code=INVITE1&loginname=test%40nobori.example&orgID=org-1&userID=created-user"
	if result.LoginURL != expectedLoginURL {
		t.Fatalf("login url = %q, want %q", result.LoginURL, expectedLoginURL)
	}
}
