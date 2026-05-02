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
	Query         string      `query:"q"`
	ReadState     string      `query:"readState" enum:"all,unread,read"`
	Channel       string      `query:"channel" enum:"in_app,email"`
	CreatedAfter  time.Time   `query:"createdAfter" format:"date-time"`
	Cursor        string      `query:"cursor"`
	Limit         int         `query:"limit" minimum:"1" maximum:"100"`
}

type NotificationListBody struct {
	Items         []NotificationBody `json:"items"`
	NextCursor    string             `json:"nextCursor,omitempty"`
	TotalCount    int64              `json:"totalCount"`
	FilteredCount int64              `json:"filteredCount"`
	UnreadCount   int64              `json:"unreadCount"`
	ReadCount     int64              `json:"readCount"`
}

type NotificationListOutput struct {
	Body NotificationListBody
}

type MarkNotificationReadInput struct {
	SessionCookie        http.Cookie `cookie:"SESSION_ID"`
	CSRFToken            string      `header:"X-CSRF-Token" required:"true"`
	NotificationPublicID string      `path:"notificationPublicId" format:"uuid"`
}

type NotificationOutput struct {
	Body NotificationBody
}

type MarkNotificationsReadBody struct {
	PublicIDs []string `json:"publicIds" minItems:"1" maxItems:"100"`
}

type MarkNotificationsReadInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          MarkNotificationsReadBody
}

type NotificationBulkReadBody struct {
	Items        []NotificationBody `json:"items"`
	UpdatedCount int                `json:"updatedCount"`
}

type NotificationBulkReadOutput struct {
	Body NotificationBulkReadBody
}

type MarkAllNotificationsReadBody struct {
	Query        string    `json:"q,omitempty"`
	ReadState    string    `json:"readState,omitempty" enum:"all,unread,read"`
	Channel      string    `json:"channel,omitempty" enum:"in_app,email"`
	CreatedAfter time.Time `json:"createdAfter,omitempty" format:"date-time"`
}

type MarkAllNotificationsReadInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          MarkAllNotificationsReadBody
}

type NotificationReadAllBody struct {
	UpdatedCount int `json:"updatedCount"`
}

type NotificationReadAllOutput struct {
	Body NotificationReadAllBody
}

func registerNotificationRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listNotifications",
		Method:      http.MethodGet,
		Path:        "/api/v1/notifications",
		Summary:     "現在の user 宛の notifications を返す",
		Tags:        []string{DocTagTenantWorkspace},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListNotificationsInput) (*NotificationListOutput, error) {
		if deps.NotificationService == nil {
			return nil, huma.Error503ServiceUnavailable("notification service is not configured")
		}
		current, authCtx, err := currentSessionAuthContext(ctx, deps, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPErrorWithLog(ctx, deps, "", err)
		}
		var tenantID *int64
		if authCtx.ActiveTenant != nil {
			tenantID = &authCtx.ActiveTenant.ID
		}
		result, err := deps.NotificationService.List(ctx, current.User.ID, tenantID, service.NotificationListInput{
			Query:        input.Query,
			ReadState:    input.ReadState,
			Channel:      input.Channel,
			CreatedAfter: input.CreatedAfter,
			Cursor:       input.Cursor,
			Limit:        input.Limit,
		})
		if err != nil {
			return nil, toNotificationHTTPError(err)
		}
		out := &NotificationListOutput{}
		out.Body.Items = make([]NotificationBody, 0, len(result.Items))
		out.Body.NextCursor = result.NextCursor
		out.Body.TotalCount = result.TotalCount
		out.Body.FilteredCount = result.FilteredCount
		out.Body.UnreadCount = result.UnreadCount
		out.Body.ReadCount = result.ReadCount
		for _, item := range result.Items {
			out.Body.Items = append(out.Body.Items, toNotificationBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "markNotificationsRead",
		Method:      http.MethodPost,
		Path:        "/api/v1/notifications/read",
		Summary:     "selected notifications を既読にする",
		Tags:        []string{DocTagTenantWorkspace},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *MarkNotificationsReadInput) (*NotificationBulkReadOutput, error) {
		if deps.NotificationService == nil {
			return nil, huma.Error503ServiceUnavailable("notification service is not configured")
		}
		current, authCtx, err := currentSessionAuthContextWithCSRF(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPErrorWithLog(ctx, deps, "", err)
		}
		var tenantID *int64
		if authCtx.ActiveTenant != nil {
			tenantID = &authCtx.ActiveTenant.ID
		}
		items, err := deps.NotificationService.MarkReadMany(ctx, current.User.ID, input.Body.PublicIDs, sessionAuditContext(ctx, current, tenantID))
		if err != nil {
			return nil, toNotificationHTTPError(err)
		}
		out := &NotificationBulkReadOutput{}
		out.Body.Items = make([]NotificationBody, 0, len(items))
		out.Body.UpdatedCount = len(items)
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toNotificationBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "markAllNotificationsRead",
		Method:      http.MethodPost,
		Path:        "/api/v1/notifications/read-all",
		Summary:     "current filters に一致する unread notifications を全て既読にする",
		Tags:        []string{DocTagTenantWorkspace},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *MarkAllNotificationsReadInput) (*NotificationReadAllOutput, error) {
		if deps.NotificationService == nil {
			return nil, huma.Error503ServiceUnavailable("notification service is not configured")
		}
		current, authCtx, err := currentSessionAuthContextWithCSRF(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPErrorWithLog(ctx, deps, "", err)
		}
		var tenantID *int64
		if authCtx.ActiveTenant != nil {
			tenantID = &authCtx.ActiveTenant.ID
		}
		items, err := deps.NotificationService.MarkReadAll(ctx, current.User.ID, tenantID, service.NotificationListInput{
			Query:        input.Body.Query,
			ReadState:    input.Body.ReadState,
			Channel:      input.Body.Channel,
			CreatedAfter: input.Body.CreatedAfter,
		}, sessionAuditContext(ctx, current, tenantID))
		if err != nil {
			return nil, toNotificationHTTPError(err)
		}
		return &NotificationReadAllOutput{Body: NotificationReadAllBody{UpdatedCount: len(items)}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "markNotificationRead",
		Method:      http.MethodPost,
		Path:        "/api/v1/notifications/{notificationPublicId}/read",
		Summary:     "notification を既読にする",
		Tags:        []string{DocTagTenantWorkspace},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *MarkNotificationReadInput) (*NotificationOutput, error) {
		if deps.NotificationService == nil {
			return nil, huma.Error503ServiceUnavailable("notification service is not configured")
		}
		current, authCtx, err := currentSessionAuthContextWithCSRF(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPErrorWithLog(ctx, deps, "", err)
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
	case errors.Is(err, service.ErrInvalidCursor):
		return huma.Error400BadRequest("invalid cursor")
	default:
		return toHTTPError(err)
	}
}
