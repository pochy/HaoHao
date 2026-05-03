package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DataPipelineBody struct {
	PublicID           string     `json:"publicId" format:"uuid"`
	Name               string     `json:"name" example:"Customer cleansing"`
	Description        string     `json:"description"`
	Status             string     `json:"status" example:"draft"`
	PublishedVersionID *int64     `json:"publishedVersionId,omitempty"`
	CreatedAt          time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt          time.Time  `json:"updatedAt" format:"date-time"`
	ArchivedAt         *time.Time `json:"archivedAt,omitempty" format:"date-time"`
}

type DataPipelineVersionBody struct {
	PublicID          string                                `json:"publicId" format:"uuid"`
	PipelineID        int64                                 `json:"pipelineId"`
	VersionNumber     int32                                 `json:"versionNumber"`
	Status            string                                `json:"status" example:"draft"`
	Graph             service.DataPipelineGraph             `json:"graph"`
	ValidationSummary service.DataPipelineValidationSummary `json:"validationSummary"`
	CreatedAt         time.Time                             `json:"createdAt" format:"date-time"`
	PublishedAt       *time.Time                            `json:"publishedAt,omitempty" format:"date-time"`
}

type DataPipelineRunStepBody struct {
	NodeID       string           `json:"nodeId"`
	StepType     string           `json:"stepType"`
	Status       string           `json:"status"`
	RowCount     int64            `json:"rowCount"`
	ErrorSummary string           `json:"errorSummary,omitempty"`
	ErrorSample  []map[string]any `json:"errorSample"`
	Metadata     map[string]any   `json:"metadata"`
	StartedAt    *time.Time       `json:"startedAt,omitempty" format:"date-time"`
	CompletedAt  *time.Time       `json:"completedAt,omitempty" format:"date-time"`
	CreatedAt    time.Time        `json:"createdAt" format:"date-time"`
	UpdatedAt    time.Time        `json:"updatedAt" format:"date-time"`
}

type DataPipelineRunBody struct {
	PublicID          string                    `json:"publicId" format:"uuid"`
	VersionID         int64                     `json:"versionId"`
	ScheduleID        *int64                    `json:"scheduleId,omitempty"`
	TriggerKind       string                    `json:"triggerKind" example:"manual"`
	Status            string                    `json:"status" example:"pending"`
	OutputWorkTableID *int64                    `json:"outputWorkTableId,omitempty"`
	RowCount          int64                     `json:"rowCount"`
	ErrorSummary      string                    `json:"errorSummary,omitempty"`
	StartedAt         *time.Time                `json:"startedAt,omitempty" format:"date-time"`
	CompletedAt       *time.Time                `json:"completedAt,omitempty" format:"date-time"`
	CreatedAt         time.Time                 `json:"createdAt" format:"date-time"`
	UpdatedAt         time.Time                 `json:"updatedAt" format:"date-time"`
	Steps             []DataPipelineRunStepBody `json:"steps"`
}

type DataPipelineScheduleBody struct {
	PublicID         string     `json:"publicId" format:"uuid"`
	VersionID        int64      `json:"versionId"`
	Frequency        string     `json:"frequency" enum:"daily,weekly,monthly"`
	Timezone         string     `json:"timezone" example:"Asia/Tokyo"`
	RunTime          string     `json:"runTime" example:"03:00"`
	Weekday          *int32     `json:"weekday,omitempty" minimum:"1" maximum:"7"`
	MonthDay         *int32     `json:"monthDay,omitempty" minimum:"1" maximum:"28"`
	Enabled          bool       `json:"enabled"`
	NextRunAt        time.Time  `json:"nextRunAt" format:"date-time"`
	LastRunAt        *time.Time `json:"lastRunAt,omitempty" format:"date-time"`
	LastStatus       string     `json:"lastStatus,omitempty"`
	LastErrorSummary string     `json:"lastErrorSummary,omitempty"`
	LastRunID        *int64     `json:"lastRunId,omitempty"`
	CreatedAt        time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt        time.Time  `json:"updatedAt" format:"date-time"`
}

type DataPipelineDetailBody struct {
	Pipeline         DataPipelineBody           `json:"pipeline"`
	PublishedVersion *DataPipelineVersionBody   `json:"publishedVersion,omitempty"`
	Versions         []DataPipelineVersionBody  `json:"versions"`
	Runs             []DataPipelineRunBody      `json:"runs"`
	Schedules        []DataPipelineScheduleBody `json:"schedules"`
}

type DataPipelineListBody struct {
	Items []DataPipelineBody `json:"items"`
}

type DataPipelineRunListBody struct {
	Items []DataPipelineRunBody `json:"items"`
}

type DataPipelineScheduleListBody struct {
	Items []DataPipelineScheduleBody `json:"items"`
}

type DataPipelinePreviewBody struct {
	NodeID      string           `json:"nodeId"`
	StepType    string           `json:"stepType"`
	Columns     []string         `json:"columns"`
	PreviewRows []map[string]any `json:"previewRows"`
}

type DataPipelineCreateBody struct {
	Name        string `json:"name" maxLength:"160"`
	Description string `json:"description,omitempty" maxLength:"2000"`
}

type DataPipelineUpdateBody struct {
	Name        string `json:"name" maxLength:"160"`
	Description string `json:"description,omitempty" maxLength:"2000"`
}

type DataPipelineVersionSaveBody struct {
	Graph service.DataPipelineGraph `json:"graph"`
}

type DataPipelinePreviewRequestBody struct {
	NodeID string `json:"nodeId,omitempty"`
	Limit  int32  `json:"limit,omitempty" minimum:"1" maximum:"1000"`
}

type DataPipelineScheduleWriteBody struct {
	Frequency string `json:"frequency,omitempty" enum:"daily,weekly,monthly"`
	Timezone  string `json:"timezone,omitempty"`
	RunTime   string `json:"runTime,omitempty"`
	Weekday   *int32 `json:"weekday,omitempty" minimum:"1" maximum:"7"`
	MonthDay  *int32 `json:"monthDay,omitempty" minimum:"1" maximum:"28"`
	Enabled   *bool  `json:"enabled,omitempty"`
}

type DataPipelineListOutput struct {
	Body DataPipelineListBody
}

type DataPipelineOutput struct {
	Body DataPipelineBody
}

type DataPipelineDetailOutput struct {
	Body DataPipelineDetailBody
}

type DataPipelineVersionOutput struct {
	Body DataPipelineVersionBody
}

type DataPipelinePreviewOutput struct {
	Body DataPipelinePreviewBody
}

type DataPipelineRunOutput struct {
	Body DataPipelineRunBody
}

type DataPipelineRunListOutput struct {
	Body DataPipelineRunListBody
}

type DataPipelineScheduleOutput struct {
	Body DataPipelineScheduleBody
}

type DataPipelineScheduleListOutput struct {
	Body DataPipelineScheduleListBody
}

type DataPipelineCreateInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	IdempotencyKey string      `header:"Idempotency-Key"`
	Body           DataPipelineCreateBody
}

type DataPipelineListInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"200" default:"100"`
}

type DataPipelineByPublicIDInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	PipelinePublicID string      `path:"pipelinePublicId" format:"uuid"`
}

type DataPipelineUpdateInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	CSRFToken        string      `header:"X-CSRF-Token" required:"true"`
	PipelinePublicID string      `path:"pipelinePublicId" format:"uuid"`
	Body             DataPipelineUpdateBody
}

type DataPipelineVersionSaveInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	CSRFToken        string      `header:"X-CSRF-Token" required:"true"`
	PipelinePublicID string      `path:"pipelinePublicId" format:"uuid"`
	Body             DataPipelineVersionSaveBody
}

type DataPipelineVersionInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	CSRFToken       string      `header:"X-CSRF-Token" required:"true"`
	VersionPublicID string      `path:"versionPublicId" format:"uuid"`
}

type DataPipelinePreviewInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	CSRFToken       string      `header:"X-CSRF-Token" required:"true"`
	VersionPublicID string      `path:"versionPublicId" format:"uuid"`
	Body            DataPipelinePreviewRequestBody
}

type DataPipelineRunCreateInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	CSRFToken       string      `header:"X-CSRF-Token" required:"true"`
	IdempotencyKey  string      `header:"Idempotency-Key"`
	VersionPublicID string      `path:"versionPublicId" format:"uuid"`
}

type DataPipelineRunsInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	PipelinePublicID string      `path:"pipelinePublicId" format:"uuid"`
	Limit            int32       `query:"limit" minimum:"1" maximum:"100" default:"25"`
}

type DataPipelineScheduleListInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	PipelinePublicID string      `path:"pipelinePublicId" format:"uuid"`
}

type DataPipelineScheduleCreateInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	CSRFToken        string      `header:"X-CSRF-Token" required:"true"`
	PipelinePublicID string      `path:"pipelinePublicId" format:"uuid"`
	Body             DataPipelineScheduleWriteBody
}

type DataPipelineScheduleUpdateInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	CSRFToken        string      `header:"X-CSRF-Token" required:"true"`
	SchedulePublicID string      `path:"schedulePublicId" format:"uuid"`
	Body             DataPipelineScheduleWriteBody
}

type DataPipelineScheduleDeleteInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	CSRFToken        string      `header:"X-CSRF-Token" required:"true"`
	SchedulePublicID string      `path:"schedulePublicId" format:"uuid"`
}

func registerDataPipelineRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listDataPipelines",
		Method:      http.MethodGet,
		Path:        "/api/v1/data-pipelines",
		Summary:     "active tenant の data pipeline 一覧を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DataPipelineListInput) (*DataPipelineListOutput, error) {
		_, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DataPipelineService.List(ctx, tenant.ID, input.Limit)
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "listDataPipelines", err)
		}
		out := &DataPipelineListOutput{}
		out.Body.Items = make([]DataPipelineBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDataPipelineBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDataPipeline",
		Method:      http.MethodPost,
		Path:        "/api/v1/data-pipelines",
		Summary:     "active tenant に data pipeline を作成する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DataPipelineCreateInput) (*DataPipelineOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		attempt, err := beginIdempotency(ctx, deps, input.IdempotencyKey, http.MethodPost, "/api/v1/data-pipelines", current.User.ID, &tenant.ID, input.Body)
		if err != nil {
			return nil, toIdempotencyHTTPError(err)
		}
		if attempt.Replay {
			body, err := replayIdempotencyBody[DataPipelineBody](attempt)
			if err != nil {
				return nil, err
			}
			return &DataPipelineOutput{Body: body}, nil
		}
		item, err := deps.DataPipelineService.Create(ctx, tenant.ID, current.User.ID, service.DataPipelineInput{Name: input.Body.Name, Description: input.Body.Description}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			if deps.IdempotencyService != nil {
				deps.IdempotencyService.Fail(ctx, attempt, http.StatusInternalServerError, err.Error())
			}
			return nil, toDataPipelineHTTPError(ctx, deps, "createDataPipeline", err)
		}
		body := toDataPipelineBody(item)
		if err := completeIdempotency(ctx, deps, attempt, http.StatusOK, body); err != nil {
			return nil, err
		}
		return &DataPipelineOutput{Body: body}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "getDataPipeline", Method: http.MethodGet, Path: "/api/v1/data-pipelines/{pipelinePublicId}", Summary: "data pipeline detail を返す", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineByPublicIDInput) (*DataPipelineDetailOutput, error) {
		_, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		detail, err := deps.DataPipelineService.Get(ctx, tenant.ID, input.PipelinePublicID)
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "getDataPipeline", err)
		}
		return &DataPipelineDetailOutput{Body: toDataPipelineDetailBody(detail)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "updateDataPipeline", Method: http.MethodPatch, Path: "/api/v1/data-pipelines/{pipelinePublicId}", Summary: "data pipeline の name / description を更新する", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineUpdateInput) (*DataPipelineOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DataPipelineService.Update(ctx, tenant.ID, current.User.ID, input.PipelinePublicID, service.DataPipelineInput{Name: input.Body.Name, Description: input.Body.Description}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "updateDataPipeline", err)
		}
		return &DataPipelineOutput{Body: toDataPipelineBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "saveDataPipelineVersion", Method: http.MethodPost, Path: "/api/v1/data-pipelines/{pipelinePublicId}/versions", Summary: "data pipeline draft graph を保存する", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineVersionSaveInput) (*DataPipelineVersionOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DataPipelineService.SaveDraftVersion(ctx, tenant.ID, current.User.ID, input.PipelinePublicID, input.Body.Graph, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "saveDataPipelineVersion", err)
		}
		return &DataPipelineVersionOutput{Body: toDataPipelineVersionBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "publishDataPipelineVersion", Method: http.MethodPost, Path: "/api/v1/data-pipeline-versions/{versionPublicId}/publish", Summary: "data pipeline version を publish する", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineVersionInput) (*DataPipelineVersionOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DataPipelineService.PublishVersion(ctx, tenant.ID, current.User.ID, input.VersionPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "publishDataPipelineVersion", err)
		}
		return &DataPipelineVersionOutput{Body: toDataPipelineVersionBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "previewDataPipelineVersion", Method: http.MethodPost, Path: "/api/v1/data-pipeline-versions/{versionPublicId}/preview", Summary: "selected node まで preview する", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelinePreviewInput) (*DataPipelinePreviewOutput, error) {
		_, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		preview, err := deps.DataPipelineService.Preview(ctx, tenant.ID, input.VersionPublicID, input.Body.NodeID, input.Body.Limit)
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "previewDataPipelineVersion", err)
		}
		return &DataPipelinePreviewOutput{Body: DataPipelinePreviewBody{NodeID: preview.NodeID, StepType: preview.StepType, Columns: preview.Columns, PreviewRows: preview.PreviewRows}}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "listDataPipelineRuns", Method: http.MethodGet, Path: "/api/v1/data-pipelines/{pipelinePublicId}/runs", Summary: "data pipeline run history を返す", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineRunsInput) (*DataPipelineRunListOutput, error) {
		_, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DataPipelineService.ListRuns(ctx, tenant.ID, input.PipelinePublicID, input.Limit)
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "listDataPipelineRuns", err)
		}
		out := &DataPipelineRunListOutput{}
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDataPipelineRunBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createDataPipelineRun", Method: http.MethodPost, Path: "/api/v1/data-pipeline-versions/{versionPublicId}/runs", Summary: "data pipeline manual run を request する", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineRunCreateInput) (*DataPipelineRunOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		bodyForHash := map[string]string{"versionPublicId": input.VersionPublicID}
		attempt, err := beginIdempotency(ctx, deps, input.IdempotencyKey, http.MethodPost, "/api/v1/data-pipeline-versions/{versionPublicId}/runs", current.User.ID, &tenant.ID, bodyForHash)
		if err != nil {
			return nil, toIdempotencyHTTPError(err)
		}
		if attempt.Replay {
			body, err := replayIdempotencyBody[DataPipelineRunBody](attempt)
			if err != nil {
				return nil, err
			}
			return &DataPipelineRunOutput{Body: body}, nil
		}
		userID := current.User.ID
		run, err := deps.DataPipelineService.RequestRun(ctx, tenant.ID, &userID, input.VersionPublicID, "manual", nil, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			if deps.IdempotencyService != nil {
				deps.IdempotencyService.Fail(ctx, attempt, http.StatusInternalServerError, err.Error())
			}
			return nil, toDataPipelineHTTPError(ctx, deps, "createDataPipelineRun", err)
		}
		body := toDataPipelineRunBody(run)
		if err := completeIdempotency(ctx, deps, attempt, http.StatusOK, body); err != nil {
			return nil, err
		}
		return &DataPipelineRunOutput{Body: body}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "listDataPipelineSchedules", Method: http.MethodGet, Path: "/api/v1/data-pipelines/{pipelinePublicId}/schedules", Summary: "data pipeline schedules を返す", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineScheduleListInput) (*DataPipelineScheduleListOutput, error) {
		_, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DataPipelineService.ListSchedules(ctx, tenant.ID, input.PipelinePublicID)
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "listDataPipelineSchedules", err)
		}
		out := &DataPipelineScheduleListOutput{}
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDataPipelineScheduleBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createDataPipelineSchedule", Method: http.MethodPost, Path: "/api/v1/data-pipelines/{pipelinePublicId}/schedules", Summary: "data pipeline schedule を作成する", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineScheduleCreateInput) (*DataPipelineScheduleOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DataPipelineService.CreateSchedule(ctx, tenant.ID, current.User.ID, input.PipelinePublicID, scheduleInputFromBody(input.Body), sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "createDataPipelineSchedule", err)
		}
		return &DataPipelineScheduleOutput{Body: toDataPipelineScheduleBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "updateDataPipelineSchedule", Method: http.MethodPatch, Path: "/api/v1/data-pipeline-schedules/{schedulePublicId}", Summary: "data pipeline schedule を更新する", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineScheduleUpdateInput) (*DataPipelineScheduleOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DataPipelineService.UpdateSchedule(ctx, tenant.ID, current.User.ID, input.SchedulePublicID, scheduleInputFromBody(input.Body), sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "updateDataPipelineSchedule", err)
		}
		return &DataPipelineScheduleOutput{Body: toDataPipelineScheduleBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "disableDataPipelineSchedule", Method: http.MethodDelete, Path: "/api/v1/data-pipeline-schedules/{schedulePublicId}", Summary: "data pipeline schedule を disable する", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineScheduleDeleteInput) (*DataPipelineScheduleOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DataPipelineService.DisableSchedule(ctx, tenant.ID, current.User.ID, input.SchedulePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "disableDataPipelineSchedule", err)
		}
		return &DataPipelineScheduleOutput{Body: toDataPipelineScheduleBody(item)}, nil
	})
}

func requireDataPipelineTenant(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, service.TenantAccess, error) {
	if deps.DataPipelineService == nil {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error503ServiceUnavailable("data pipeline service is not configured")
	}
	return requireActiveTenantRole(ctx, deps, sessionID, csrfToken, "data_pipeline_user", "data pipeline service")
}

func scheduleInputFromBody(body DataPipelineScheduleWriteBody) service.DataPipelineScheduleInput {
	return service.DataPipelineScheduleInput{Frequency: body.Frequency, Timezone: body.Timezone, RunTime: body.RunTime, Weekday: body.Weekday, MonthDay: body.MonthDay, Enabled: body.Enabled}
}

func toDataPipelineHTTPError(ctx context.Context, deps Dependencies, operation string, err error) error {
	switch {
	case errors.Is(err, service.ErrDataPipelineNotFound), errors.Is(err, service.ErrDataPipelineVersionNotFound), errors.Is(err, service.ErrDataPipelineRunNotFound), errors.Is(err, service.ErrDataPipelineScheduleNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, service.ErrInvalidDataPipelineInput), errors.Is(err, service.ErrInvalidDataPipelineGraph):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, service.ErrDataPipelineVersionUnpublished):
		return huma.Error409Conflict(err.Error())
	case errors.Is(err, service.ErrDatasetClickHouseNotReady):
		return huma.Error503ServiceUnavailable(err.Error())
	default:
		return internalHTTPError(ctx, deps, operation, err)
	}
}

func toDataPipelineDetailBody(detail service.DataPipelineDetail) DataPipelineDetailBody {
	body := DataPipelineDetailBody{
		Pipeline:  toDataPipelineBody(detail.Pipeline),
		Versions:  make([]DataPipelineVersionBody, 0, len(detail.Versions)),
		Runs:      make([]DataPipelineRunBody, 0, len(detail.Runs)),
		Schedules: make([]DataPipelineScheduleBody, 0, len(detail.Schedules)),
	}
	if detail.PublishedVersion != nil {
		item := toDataPipelineVersionBody(*detail.PublishedVersion)
		body.PublishedVersion = &item
	}
	for _, item := range detail.Versions {
		body.Versions = append(body.Versions, toDataPipelineVersionBody(item))
	}
	for _, item := range detail.Runs {
		body.Runs = append(body.Runs, toDataPipelineRunBody(item))
	}
	for _, item := range detail.Schedules {
		body.Schedules = append(body.Schedules, toDataPipelineScheduleBody(item))
	}
	return body
}

func toDataPipelineBody(item service.DataPipeline) DataPipelineBody {
	return DataPipelineBody{PublicID: item.PublicID, Name: item.Name, Description: item.Description, Status: item.Status, PublishedVersionID: item.PublishedVersionID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, ArchivedAt: item.ArchivedAt}
}

func toDataPipelineVersionBody(item service.DataPipelineVersion) DataPipelineVersionBody {
	return DataPipelineVersionBody{PublicID: item.PublicID, PipelineID: item.PipelineID, VersionNumber: item.VersionNumber, Status: item.Status, Graph: item.Graph, ValidationSummary: item.ValidationSummary, CreatedAt: item.CreatedAt, PublishedAt: item.PublishedAt}
}

func toDataPipelineRunBody(item service.DataPipelineRun) DataPipelineRunBody {
	body := DataPipelineRunBody{PublicID: item.PublicID, VersionID: item.VersionID, ScheduleID: item.ScheduleID, TriggerKind: item.TriggerKind, Status: item.Status, OutputWorkTableID: item.OutputWorkTableID, RowCount: item.RowCount, ErrorSummary: item.ErrorSummary, StartedAt: item.StartedAt, CompletedAt: item.CompletedAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, Steps: make([]DataPipelineRunStepBody, 0, len(item.Steps))}
	for _, step := range item.Steps {
		body.Steps = append(body.Steps, DataPipelineRunStepBody{NodeID: step.NodeID, StepType: step.StepType, Status: step.Status, RowCount: step.RowCount, ErrorSummary: step.ErrorSummary, ErrorSample: step.ErrorSample, Metadata: step.Metadata, StartedAt: step.StartedAt, CompletedAt: step.CompletedAt, CreatedAt: step.CreatedAt, UpdatedAt: step.UpdatedAt})
	}
	return body
}

func toDataPipelineScheduleBody(item service.DataPipelineSchedule) DataPipelineScheduleBody {
	return DataPipelineScheduleBody{PublicID: item.PublicID, VersionID: item.VersionID, Frequency: item.Frequency, Timezone: item.Timezone, RunTime: item.RunTime, Weekday: item.Weekday, MonthDay: item.MonthDay, Enabled: item.Enabled, NextRunAt: item.NextRunAt, LastRunAt: item.LastRunAt, LastStatus: item.LastStatus, LastErrorSummary: item.LastErrorSummary, LastRunID: item.LastRunID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
