package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type NotificationBody struct {
	PublicID  string     `json:"publicId" format:"uuid"`
	TenantID  *int64     `json:"tenantId,omitempty"`
	Channel   string     `json:"channel" example:"in_app"`
	Template  string     `json:"template" example:"tenant_invitation"`
	Subject   string     `json:"subject" example:"Invitation"`
	Body      string     `json:"body" example:"You have a new notification."`
	Status    string     `json:"status" example:"queued"`
	ReadAt    *time.Time `json:"readAt,omitempty" format:"date-time"`
	CreatedAt time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt time.Time  `json:"updatedAt" format:"date-time"`
}

type ListNotificationsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Limit         int         `query:"limit" minimum:"1" maximum:"100"`
}

type NotificationListOutput struct {
	Body struct {
		Items []NotificationBody `json:"items"`
	}
}

type MarkNotificationReadInput struct {
	SessionCookie        http.Cookie `cookie:"SESSION_ID"`
	CSRFToken            string      `header:"X-CSRF-Token" required:"true"`
	NotificationPublicID string      `path:"notificationPublicId" format:"uuid"`
}

type NotificationOutput struct {
	Body NotificationBody
}

func registerNotificationRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listNotifications",
		Method:      http.MethodGet,
		Path:        "/api/v1/notifications",
		Summary:     "現在の user 宛の notifications を返す",
		Tags:        []string{"notifications"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListNotificationsInput) (*NotificationListOutput, error) {
		if deps.NotificationService == nil {
			return nil, huma.Error503ServiceUnavailable("notification service is not configured")
		}
		current, authCtx, err := currentSessionAuthContext(ctx, deps, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}
		var tenantID *int64
		if authCtx.ActiveTenant != nil {
			tenantID = &authCtx.ActiveTenant.ID
		}
		items, err := deps.NotificationService.List(ctx, current.User.ID, tenantID, input.Limit)
		if err != nil {
			return nil, toNotificationHTTPError(err)
		}
		out := &NotificationListOutput{}
		out.Body.Items = make([]NotificationBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toNotificationBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "markNotificationRead",
		Method:      http.MethodPost,
		Path:        "/api/v1/notifications/{notificationPublicId}/read",
		Summary:     "notification を既読にする",
		Tags:        []string{"notifications"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *MarkNotificationReadInput) (*NotificationOutput, error) {
		if deps.NotificationService == nil {
			return nil, huma.Error503ServiceUnavailable("notification service is not configured")
		}
		current, authCtx, err := currentSessionAuthContextWithCSRF(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPError(err)
		}
		var tenantID *int64
		if authCtx.ActiveTenant != nil {
			tenantID = &authCtx.ActiveTenant.ID
		}
		item, err := deps.NotificationService.MarkRead(ctx, current.User.ID, input.NotificationPublicID, sessionAuditContext(ctx, current, tenantID))
		if err != nil {
			return nil, toNotificationHTTPError(err)
		}
		return &NotificationOutput{Body: toNotificationBody(item)}, nil
	})
}

func toNotificationBody(item service.Notification) NotificationBody {
	return NotificationBody{
		PublicID:  item.PublicID,
		TenantID:  item.TenantID,
		Channel:   item.Channel,
		Template:  item.Template,
		Subject:   item.Subject,
		Body:      item.Body,
		Status:    item.Status,
		ReadAt:    item.ReadAt,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func toNotificationHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrNotificationNotFound):
		return huma.Error404NotFound("notification not found")
	case errors.Is(err, service.ErrInvalidNotification):
		return huma.Error400BadRequest("invalid notification")
	default:
		return toHTTPError(err)
	}
}
