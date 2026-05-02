package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type WebhookEndpointBody struct {
	PublicID       string     `json:"publicId" format:"uuid"`
	Name           string     `json:"name"`
	URL            string     `json:"url" format:"uri"`
	EventTypes     []string   `json:"eventTypes"`
	Active         bool       `json:"active"`
	Secret         string     `json:"secret,omitempty"`
	LastDeliveryAt *time.Time `json:"lastDeliveryAt,omitempty" format:"date-time"`
	CreatedAt      time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt      time.Time  `json:"updatedAt" format:"date-time"`
}

type WebhookDeliveryBody struct {
	PublicID        string     `json:"publicId" format:"uuid"`
	EventType       string     `json:"eventType"`
	Status          string     `json:"status"`
	AttemptCount    int32      `json:"attemptCount"`
	MaxAttempts     int32      `json:"maxAttempts"`
	LastHTTPStatus  *int32     `json:"lastHttpStatus,omitempty"`
	LastError       string     `json:"lastError,omitempty"`
	ResponsePreview string     `json:"responsePreview,omitempty"`
	DeliveredAt     *time.Time `json:"deliveredAt,omitempty" format:"date-time"`
	CreatedAt       time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt       time.Time  `json:"updatedAt" format:"date-time"`
}

type WebhookEndpointRequestBody struct {
	Name       string   `json:"name" minLength:"1" maxLength:"120"`
	URL        string   `json:"url" format:"uri"`
	EventTypes []string `json:"eventTypes"`
	Active     *bool    `json:"active,omitempty"`
}

type WebhookListInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
}

type WebhookEndpointInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	TenantSlug      string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	WebhookPublicID string      `path:"webhookPublicId" format:"uuid"`
}

type WebhookMutateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          WebhookEndpointRequestBody
}

type WebhookUpdateInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	CSRFToken       string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug      string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	WebhookPublicID string      `path:"webhookPublicId" format:"uuid"`
	Body            WebhookEndpointRequestBody
}

type WebhookActionInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	CSRFToken       string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug      string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	WebhookPublicID string      `path:"webhookPublicId" format:"uuid"`
}

type WebhookDeliveryListInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	TenantSlug      string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	WebhookPublicID string      `path:"webhookPublicId" format:"uuid"`
}

type WebhookDeliveryRetryInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	CSRFToken        string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug       string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	WebhookPublicID  string      `path:"webhookPublicId" format:"uuid"`
	DeliveryPublicID string      `path:"deliveryPublicId" format:"uuid"`
}

type WebhookListOutput struct {
	Body struct {
		Items []WebhookEndpointBody `json:"items"`
	}
}

type WebhookOutput struct {
	Body WebhookEndpointBody
}

type WebhookDeliveryListOutput struct {
	Body struct {
		Items []WebhookDeliveryBody `json:"items"`
	}
}

type WebhookDeliveryOutput struct {
	Body WebhookDeliveryBody
}

type WebhookNoContentOutput struct{}

func registerWebhookRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{OperationID: "listWebhooks", Method: http.MethodGet, Path: "/api/v1/admin/tenants/{tenantSlug}/webhooks", Summary: "webhook endpoint 一覧を返す", Tags: []string{DocTagPlatformIntegrations}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *WebhookListInput) (*WebhookListOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		items, err := deps.WebhookService.List(ctx, tenant.ID)
		if err != nil {
			return nil, toWebhookHTTPError(err)
		}
		out := &WebhookListOutput{}
		out.Body.Items = make([]WebhookEndpointBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toWebhookEndpointBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createWebhook", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/webhooks", Summary: "webhook endpoint を作成する", Tags: []string{DocTagPlatformIntegrations}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *WebhookMutateInput) (*WebhookOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.WebhookService.Create(ctx, tenant.ID, current.User.ID, webhookInputFromBody(input.Body), sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toWebhookHTTPError(err)
		}
		return &WebhookOutput{Body: toWebhookEndpointBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "getWebhook", Method: http.MethodGet, Path: "/api/v1/admin/tenants/{tenantSlug}/webhooks/{webhookPublicId}", Summary: "webhook endpoint detail を返す", Tags: []string{DocTagPlatformIntegrations}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *WebhookEndpointInput) (*WebhookOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.WebhookService.Get(ctx, tenant.ID, input.WebhookPublicID)
		if err != nil {
			return nil, toWebhookHTTPError(err)
		}
		return &WebhookOutput{Body: toWebhookEndpointBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "updateWebhook", Method: http.MethodPut, Path: "/api/v1/admin/tenants/{tenantSlug}/webhooks/{webhookPublicId}", Summary: "webhook endpoint を更新する", Tags: []string{DocTagPlatformIntegrations}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *WebhookUpdateInput) (*WebhookOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.WebhookService.Update(ctx, tenant.ID, input.WebhookPublicID, webhookInputFromBody(input.Body), sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toWebhookHTTPError(err)
		}
		return &WebhookOutput{Body: toWebhookEndpointBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "rotateWebhookSecret", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/webhooks/{webhookPublicId}/rotate-secret", Summary: "webhook secret を rotate する", Tags: []string{DocTagPlatformIntegrations}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *WebhookActionInput) (*WebhookOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if current.SupportAccess != nil {
			return nil, huma.Error403Forbidden("webhook secret rotate is disabled during support access")
		}
		item, err := deps.WebhookService.RotateSecret(ctx, tenant.ID, input.WebhookPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toWebhookHTTPError(err)
		}
		return &WebhookOutput{Body: toWebhookEndpointBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "deleteWebhook", Method: http.MethodDelete, Path: "/api/v1/admin/tenants/{tenantSlug}/webhooks/{webhookPublicId}", Summary: "webhook endpoint を削除する", Tags: []string{DocTagPlatformIntegrations}, DefaultStatus: http.StatusNoContent, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *WebhookActionInput) (*WebhookNoContentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if current.SupportAccess != nil {
			return nil, huma.Error403Forbidden("webhook delete is disabled during support access")
		}
		if err := deps.WebhookService.Delete(ctx, tenant.ID, input.WebhookPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toWebhookHTTPError(err)
		}
		return &WebhookNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "listWebhookDeliveries", Method: http.MethodGet, Path: "/api/v1/admin/tenants/{tenantSlug}/webhooks/{webhookPublicId}/deliveries", Summary: "webhook delivery log を返す", Tags: []string{DocTagPlatformIntegrations}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *WebhookDeliveryListInput) (*WebhookDeliveryListOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		items, err := deps.WebhookService.ListDeliveries(ctx, tenant.ID, input.WebhookPublicID, 50)
		if err != nil {
			return nil, toWebhookHTTPError(err)
		}
		out := &WebhookDeliveryListOutput{}
		out.Body.Items = make([]WebhookDeliveryBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toWebhookDeliveryBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{OperationID: "retryWebhookDelivery", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/webhooks/{webhookPublicId}/deliveries/{deliveryPublicId}/retry", Summary: "webhook delivery を retry する", Tags: []string{DocTagPlatformIntegrations}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *WebhookDeliveryRetryInput) (*WebhookDeliveryOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.WebhookService.RetryDelivery(ctx, tenant.ID, input.WebhookPublicID, input.DeliveryPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toWebhookHTTPError(err)
		}
		return &WebhookDeliveryOutput{Body: toWebhookDeliveryBody(item)}, nil
	})
}

func webhookInputFromBody(body WebhookEndpointRequestBody) service.WebhookEndpointInput {
	return service.WebhookEndpointInput{Name: body.Name, URL: body.URL, EventTypes: body.EventTypes, Active: body.Active}
}

func toWebhookEndpointBody(item service.WebhookEndpoint) WebhookEndpointBody {
	return WebhookEndpointBody{PublicID: item.PublicID, Name: item.Name, URL: item.URL, EventTypes: item.EventTypes, Active: item.Active, Secret: item.SecretPlaintext, LastDeliveryAt: item.LastDeliveryAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func toWebhookDeliveryBody(item service.WebhookDelivery) WebhookDeliveryBody {
	return WebhookDeliveryBody{PublicID: item.PublicID, EventType: item.EventType, Status: item.Status, AttemptCount: item.AttemptCount, MaxAttempts: item.MaxAttempts, LastHTTPStatus: item.LastHTTPStatus, LastError: item.LastError, ResponsePreview: item.ResponsePreview, DeliveredAt: item.DeliveredAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func toWebhookHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidWebhookInput):
		return huma.Error400BadRequest("invalid webhook input")
	case errors.Is(err, service.ErrWebhookSecretUnavailable):
		return huma.Error503ServiceUnavailable("webhook secret encryption key is not configured")
	case errors.Is(err, service.ErrWebhookEntitlementDenied):
		return huma.Error403Forbidden("webhooks entitlement is disabled")
	case errors.Is(err, service.ErrWebhookNotFound), errors.Is(err, service.ErrWebhookDeliveryNotFound):
		return huma.Error404NotFound("webhook not found")
	default:
		return toHTTPError(err)
	}
}
