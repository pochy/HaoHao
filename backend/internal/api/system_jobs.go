package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type SystemJobBody struct {
	Type                   string         `json:"type"`
	PublicID               string         `json:"publicId" format:"uuid"`
	Title                  string         `json:"title"`
	SubjectType            string         `json:"subjectType,omitempty"`
	SubjectPublicID        string         `json:"subjectPublicId,omitempty"`
	RequestedByDisplayName string         `json:"requestedByDisplayName,omitempty"`
	RequestedByEmail       string         `json:"requestedByEmail,omitempty"`
	Status                 string         `json:"status"`
	StatusGroup            string         `json:"statusGroup"`
	Action                 string         `json:"action,omitempty"`
	ErrorMessage           string         `json:"errorMessage,omitempty"`
	OutboxEventPublicID    string         `json:"outboxEventPublicId,omitempty"`
	CreatedAt              time.Time      `json:"createdAt" format:"date-time"`
	UpdatedAt              time.Time      `json:"updatedAt" format:"date-time"`
	StartedAt              *time.Time     `json:"startedAt,omitempty" format:"date-time"`
	CompletedAt            *time.Time     `json:"completedAt,omitempty" format:"date-time"`
	Metadata               map[string]any `json:"metadata"`
	CanStop                bool           `json:"canStop"`
}

type SystemJobListInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Query         string      `query:"query"`
	Type          string      `query:"type"`
	Status        string      `query:"status"`
	StatusGroup   string      `query:"statusGroup" enum:"active,terminal"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"100" default:"25"`
	Offset        int32       `query:"offset" minimum:"0" default:"0"`
}

type SystemJobPathInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token"`
	JobType       string      `path:"jobType"`
	JobPublicID   string      `path:"jobPublicId" format:"uuid"`
}

type SystemJobListOutput struct {
	Body struct {
		Items  []SystemJobBody `json:"items"`
		Total  int64           `json:"total"`
		Limit  int32           `json:"limit"`
		Offset int32           `json:"offset"`
	}
}

type SystemJobOutput struct {
	Body SystemJobBody
}

func registerSystemJobRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listSystemJobs",
		Method:      http.MethodGet,
		Path:        "/api/v1/jobs",
		Tags:        []string{DocTagTenantAdministration},
		Summary:     "system jobs を検索する",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *SystemJobListInput) (*SystemJobListOutput, error) {
		current, authCtx, err := requireSystemJobAdmin(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		_ = current
		result, err := deps.SystemJobService.List(ctx, authCtx.ActiveTenant.ID, service.SystemJobListFilter{
			Query:       input.Query,
			Type:        input.Type,
			Status:      input.Status,
			StatusGroup: input.StatusGroup,
			Limit:       input.Limit,
			Offset:      input.Offset,
		})
		if err != nil {
			return nil, toSystemJobHTTPError(err)
		}
		out := &SystemJobListOutput{}
		out.Body.Total = result.Total
		out.Body.Limit = result.Limit
		out.Body.Offset = result.Offset
		out.Body.Items = make([]SystemJobBody, 0, len(result.Items))
		for _, item := range result.Items {
			out.Body.Items = append(out.Body.Items, toSystemJobBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getSystemJob",
		Method:      http.MethodGet,
		Path:        "/api/v1/jobs/{jobType}/{jobPublicId}",
		Tags:        []string{DocTagTenantAdministration},
		Summary:     "system job detail を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *SystemJobPathInput) (*SystemJobOutput, error) {
		_, authCtx, err := requireSystemJobAdmin(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.SystemJobService.Get(ctx, authCtx.ActiveTenant.ID, input.JobType, input.JobPublicID)
		if err != nil {
			return nil, toSystemJobHTTPError(err)
		}
		return &SystemJobOutput{Body: toSystemJobBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "stopSystemJob",
		Method:      http.MethodPost,
		Path:        "/api/v1/jobs/{jobType}/{jobPublicId}/stop",
		Tags:        []string{DocTagTenantAdministration},
		Summary:     "system job を手動停止する",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *SystemJobPathInput) (*SystemJobOutput, error) {
		current, authCtx, err := requireSystemJobAdmin(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.SystemJobService.Stop(ctx, authCtx.ActiveTenant.ID, input.JobType, input.JobPublicID, current.User.ID)
		if err != nil {
			return nil, toSystemJobHTTPError(err)
		}
		return &SystemJobOutput{Body: toSystemJobBody(item)}, nil
	})
}

func requireSystemJobAdmin(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, service.AuthContext, error) {
	if deps.SystemJobService == nil {
		return service.CurrentSession{}, service.AuthContext{}, huma.Error503ServiceUnavailable("system job service is not configured")
	}
	var current service.CurrentSession
	var authCtx service.AuthContext
	var err error
	if csrfToken == "" {
		current, authCtx, err = currentSessionAuthContext(ctx, deps, sessionID)
	} else {
		current, authCtx, err = currentSessionAuthContextWithCSRF(ctx, deps, sessionID, csrfToken)
	}
	if err != nil {
		var statusErr huma.StatusError
		if errors.As(err, &statusErr) {
			return service.CurrentSession{}, service.AuthContext{}, err
		}
		return service.CurrentSession{}, service.AuthContext{}, toHTTPErrorWithLog(ctx, deps, "", err)
	}
	if authCtx.ActiveTenant == nil {
		return service.CurrentSession{}, service.AuthContext{}, huma.Error409Conflict("active tenant is required")
	}
	if !authCtx.HasRole("tenant_admin") {
		return service.CurrentSession{}, service.AuthContext{}, huma.Error403Forbidden("tenant_admin role is required")
	}
	return current, authCtx, nil
}

func toSystemJobHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrSystemJobNotFound):
		return huma.Error404NotFound("system job not found")
	case errors.Is(err, service.ErrSystemJobNotStoppable):
		return huma.Error409Conflict("system job is not stoppable")
	default:
		return huma.Error500InternalServerError("system job request failed")
	}
}

func toSystemJobBody(item service.SystemJob) SystemJobBody {
	return SystemJobBody{
		Type:                   item.Type,
		PublicID:               item.PublicID,
		Title:                  item.Title,
		SubjectType:            item.SubjectType,
		SubjectPublicID:        item.SubjectPublicID,
		RequestedByDisplayName: item.RequestedByDisplayName,
		RequestedByEmail:       item.RequestedByEmail,
		Status:                 item.Status,
		StatusGroup:            item.StatusGroup,
		Action:                 item.Action,
		ErrorMessage:           item.ErrorMessage,
		OutboxEventPublicID:    item.OutboxEventPublicID,
		CreatedAt:              item.CreatedAt,
		UpdatedAt:              item.UpdatedAt,
		StartedAt:              item.StartedAt,
		CompletedAt:            item.CompletedAt,
		Metadata:               item.Metadata,
		CanStop:                item.CanStop,
	}
}
