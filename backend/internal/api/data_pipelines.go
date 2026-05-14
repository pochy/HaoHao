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
	PublicID              string     `json:"publicId" format:"uuid"`
	Name                  string     `json:"name" example:"Customer cleansing"`
	Description           string     `json:"description"`
	Status                string     `json:"status" example:"draft"`
	PublishedVersionID    *int64     `json:"publishedVersionId,omitempty"`
	CreatedAt             time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt             time.Time  `json:"updatedAt" format:"date-time"`
	ArchivedAt            *time.Time `json:"archivedAt,omitempty" format:"date-time"`
	LatestRunStatus       string     `json:"latestRunStatus,omitempty"`
	LatestRunAt           *time.Time `json:"latestRunAt,omitempty" format:"date-time"`
	LatestRunPublicID     string     `json:"latestRunPublicId,omitempty" format:"uuid"`
	ScheduleState         string     `json:"scheduleState,omitempty" enum:"enabled,disabled,none"`
	EnabledScheduleCount  int64      `json:"enabledScheduleCount,omitempty"`
	DisabledScheduleCount int64      `json:"disabledScheduleCount,omitempty"`
	NextRunAt             *time.Time `json:"nextRunAt,omitempty" format:"date-time"`
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

type DataPipelineRunOutputBody struct {
	NodeID            string         `json:"nodeId"`
	Status            string         `json:"status"`
	OutputWorkTableID *int64         `json:"outputWorkTableId,omitempty"`
	RowCount          int64          `json:"rowCount"`
	ErrorSummary      string         `json:"errorSummary,omitempty"`
	Metadata          map[string]any `json:"metadata"`
	StartedAt         *time.Time     `json:"startedAt,omitempty" format:"date-time"`
	CompletedAt       *time.Time     `json:"completedAt,omitempty" format:"date-time"`
	CreatedAt         time.Time      `json:"createdAt" format:"date-time"`
	UpdatedAt         time.Time      `json:"updatedAt" format:"date-time"`
}

type DataPipelineRunBody struct {
	PublicID          string                      `json:"publicId" format:"uuid"`
	VersionID         int64                       `json:"versionId"`
	ScheduleID        *int64                      `json:"scheduleId,omitempty"`
	TriggerKind       string                      `json:"triggerKind" example:"manual"`
	Status            string                      `json:"status" example:"pending"`
	OutputWorkTableID *int64                      `json:"outputWorkTableId,omitempty"`
	RowCount          int64                       `json:"rowCount"`
	ErrorSummary      string                      `json:"errorSummary,omitempty"`
	StartedAt         *time.Time                  `json:"startedAt,omitempty" format:"date-time"`
	CompletedAt       *time.Time                  `json:"completedAt,omitempty" format:"date-time"`
	CreatedAt         time.Time                   `json:"createdAt" format:"date-time"`
	UpdatedAt         time.Time                   `json:"updatedAt" format:"date-time"`
	Steps             []DataPipelineRunStepBody   `json:"steps"`
	Outputs           []DataPipelineRunOutputBody `json:"outputs"`
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
	Items      []DataPipelineBody `json:"items"`
	NextCursor string             `json:"nextCursor,omitempty"`
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

type DataPipelineReviewItemBody struct {
	PublicID          string                          `json:"publicId" format:"uuid"`
	VersionID         int64                           `json:"versionId"`
	RunID             int64                           `json:"runId"`
	NodeID            string                          `json:"nodeId"`
	Queue             string                          `json:"queue"`
	Status            string                          `json:"status" enum:"open,approved,rejected,needs_changes,closed"`
	Reason            []map[string]any                `json:"reason"`
	SourceSnapshot    map[string]any                  `json:"sourceSnapshot"`
	SourceFingerprint string                          `json:"sourceFingerprint"`
	CreatedByUserID   *int64                          `json:"createdByUserId,omitempty"`
	UpdatedByUserID   *int64                          `json:"updatedByUserId,omitempty"`
	AssignedToUserID  *int64                          `json:"assignedToUserId,omitempty"`
	DecisionComment   string                          `json:"decisionComment,omitempty"`
	DecidedAt         *time.Time                      `json:"decidedAt,omitempty" format:"date-time"`
	CreatedAt         time.Time                       `json:"createdAt" format:"date-time"`
	UpdatedAt         time.Time                       `json:"updatedAt" format:"date-time"`
	Comments          []DataPipelineReviewCommentBody `json:"comments,omitempty"`
}

type DataPipelineReviewCommentBody struct {
	PublicID     string    `json:"publicId" format:"uuid"`
	AuthorUserID *int64    `json:"authorUserId,omitempty"`
	Body         string    `json:"body"`
	CreatedAt    time.Time `json:"createdAt" format:"date-time"`
}

type DataPipelineReviewItemListBody struct {
	Items []DataPipelineReviewItemBody `json:"items"`
}

type DataPipelineReviewItemTransitionBody struct {
	Status  string `json:"status" enum:"open,approved,rejected,needs_changes,closed"`
	Comment string `json:"comment,omitempty" maxLength:"4000"`
}

type DataPipelineReviewItemCommentWriteBody struct {
	Body string `json:"body" maxLength:"4000"`
}

type SchemaMappingCandidateRequestBody struct {
	PipelinePublicID string                             `json:"pipelinePublicId,omitempty" format:"uuid"`
	VersionPublicID  string                             `json:"versionPublicId,omitempty" format:"uuid"`
	Domain           string                             `json:"domain,omitempty" maxLength:"120"`
	SchemaType       string                             `json:"schemaType,omitempty" maxLength:"120"`
	Columns          []SchemaMappingCandidateColumnBody `json:"columns" minItems:"1" maxItems:"100"`
	Limit            int32                              `json:"limit,omitempty" minimum:"1" maximum:"10"`
}

type SchemaMappingCandidateColumnBody struct {
	SourceColumn    string   `json:"sourceColumn" maxLength:"240"`
	SheetName       string   `json:"sheetName,omitempty" maxLength:"240"`
	SampleValues    []string `json:"sampleValues,omitempty" maxItems:"20"`
	NeighborColumns []string `json:"neighborColumns,omitempty" maxItems:"40"`
}

type SchemaMappingCandidateListBody struct {
	Items []SchemaMappingCandidateItemBody `json:"items"`
}

type SchemaMappingCandidateItemBody struct {
	SourceColumn string                       `json:"sourceColumn"`
	Candidates   []SchemaMappingCandidateBody `json:"candidates"`
}

type SchemaMappingCandidateBody struct {
	SchemaColumnPublicID string  `json:"schemaColumnPublicId" format:"uuid"`
	TargetColumn         string  `json:"targetColumn"`
	Score                float64 `json:"score"`
	MatchMethod          string  `json:"matchMethod" enum:"keyword,vector,hybrid,strict"`
	Reason               string  `json:"reason"`
	Snippet              string  `json:"snippet,omitempty"`
	AcceptedEvidence     int64   `json:"acceptedEvidence"`
	RejectedEvidence     int64   `json:"rejectedEvidence"`
}

type SchemaMappingExampleWriteBody struct {
	PipelinePublicID     string   `json:"pipelinePublicId" format:"uuid"`
	VersionPublicID      string   `json:"versionPublicId,omitempty" format:"uuid"`
	SchemaColumnPublicID string   `json:"schemaColumnPublicId" format:"uuid"`
	SourceColumn         string   `json:"sourceColumn" maxLength:"240"`
	SheetName            string   `json:"sheetName,omitempty" maxLength:"240"`
	SampleValues         []string `json:"sampleValues,omitempty" maxItems:"20"`
	NeighborColumns      []string `json:"neighborColumns,omitempty" maxItems:"40"`
	Decision             string   `json:"decision" enum:"accepted,rejected"`
}

type SchemaMappingExampleBody struct {
	PublicID             string `json:"publicId" format:"uuid"`
	SchemaColumnPublicID string `json:"schemaColumnPublicId" format:"uuid"`
	SourceColumn         string `json:"sourceColumn"`
	TargetColumn         string `json:"targetColumn"`
	Decision             string `json:"decision"`
	SharedScope          string `json:"sharedScope"`
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

type DataPipelineDraftPreviewRequestBody struct {
	Graph  service.DataPipelineGraph `json:"graph"`
	NodeID string                    `json:"nodeId,omitempty"`
	Limit  int32                     `json:"limit,omitempty" minimum:"1" maximum:"1000"`
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

type DataPipelineReviewItemOutput struct {
	Body DataPipelineReviewItemBody
}

type DataPipelineReviewItemListOutput struct {
	Body DataPipelineReviewItemListBody
}

type DataPipelineReviewCommentOutput struct {
	Body DataPipelineReviewCommentBody
}

type SchemaMappingCandidateOutput struct {
	Body SchemaMappingCandidateListBody
}

type SchemaMappingExampleOutput struct {
	Body SchemaMappingExampleBody
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
	Query         string      `query:"q"`
	Status        string      `query:"status" enum:"draft,published"`
	Publication   string      `query:"publication" enum:"all,published,unpublished" default:"all"`
	RunStatus     string      `query:"runStatus" enum:"pending,processing,completed,failed,skipped"`
	ScheduleState string      `query:"scheduleState" enum:"all,enabled,disabled,none" default:"all"`
	Sort          string      `query:"sort" enum:"updated_desc,updated_asc,created_desc,created_asc,name_asc,name_desc,latest_run_desc" default:"updated_desc"`
	Cursor        string      `query:"cursor"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"100" default:"25"`
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

type DataPipelineDraftPreviewInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	CSRFToken        string      `header:"X-CSRF-Token" required:"true"`
	PipelinePublicID string      `path:"pipelinePublicId" format:"uuid"`
	Body             DataPipelineDraftPreviewRequestBody
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

type DataPipelineReviewItemListInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	PipelinePublicID string      `path:"pipelinePublicId" format:"uuid"`
	Status           string      `query:"status" enum:"open,approved,rejected,needs_changes,closed"`
	Limit            int32       `query:"limit" minimum:"1" maximum:"100" default:"25"`
}

type DataPipelineReviewItemInput struct {
	SessionCookie      http.Cookie `cookie:"SESSION_ID"`
	ReviewItemPublicID string      `path:"reviewItemPublicId" format:"uuid"`
}

type DataPipelineReviewItemTransitionInput struct {
	SessionCookie      http.Cookie `cookie:"SESSION_ID"`
	CSRFToken          string      `header:"X-CSRF-Token" required:"true"`
	ReviewItemPublicID string      `path:"reviewItemPublicId" format:"uuid"`
	Body               DataPipelineReviewItemTransitionBody
}

type DataPipelineReviewItemCommentInput struct {
	SessionCookie      http.Cookie `cookie:"SESSION_ID"`
	CSRFToken          string      `header:"X-CSRF-Token" required:"true"`
	ReviewItemPublicID string      `path:"reviewItemPublicId" format:"uuid"`
	Body               DataPipelineReviewItemCommentWriteBody
}

type SchemaMappingCandidateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          SchemaMappingCandidateRequestBody
}

type SchemaMappingExampleInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          SchemaMappingExampleWriteBody
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
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		result, err := deps.DataPipelineService.List(ctx, tenant.ID, current.User.ID, service.DataPipelineListInput{
			Query:         input.Query,
			Status:        input.Status,
			Publication:   input.Publication,
			RunStatus:     input.RunStatus,
			ScheduleState: input.ScheduleState,
			Sort:          input.Sort,
			Cursor:        input.Cursor,
			Limit:         input.Limit,
		})
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "listDataPipelines", err)
		}
		out := &DataPipelineListOutput{}
		out.Body.NextCursor = result.NextCursor
		out.Body.Items = make([]DataPipelineBody, 0, len(result.Items))
		for _, item := range result.Items {
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
		if err := checkDatasetScopeAction(ctx, deps, tenant.ID, current.User.ID, service.DataActionCreatePipeline); err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "createDataPipeline", err)
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
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		detail, err := deps.DataPipelineService.Get(ctx, tenant.ID, input.PipelinePublicID)
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "getDataPipeline", err)
		}
		if err := checkDatasetResourceAction(ctx, deps, tenant.ID, current.User.ID, service.DataResourceDataPipeline, detail.Pipeline.PublicID, service.DataActionView); err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "getDataPipeline", err)
		}
		return &DataPipelineDetailOutput{Body: toDataPipelineDetailBody(detail)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "updateDataPipeline", Method: http.MethodPatch, Path: "/api/v1/data-pipelines/{pipelinePublicId}", Summary: "data pipeline の name / description を更新する", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineUpdateInput) (*DataPipelineOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := checkDatasetResourceAction(ctx, deps, tenant.ID, current.User.ID, service.DataResourceDataPipeline, input.PipelinePublicID, service.DataActionUpdate); err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "updateDataPipeline", err)
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
		if err := checkDatasetResourceAction(ctx, deps, tenant.ID, current.User.ID, service.DataResourceDataPipeline, input.PipelinePublicID, service.DataActionSaveVersion); err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "saveDataPipelineVersion", err)
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
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		preview, err := deps.DataPipelineService.Preview(ctx, tenant.ID, current.User.ID, input.VersionPublicID, input.Body.NodeID, input.Body.Limit)
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "previewDataPipelineVersion", err)
		}
		return &DataPipelinePreviewOutput{Body: DataPipelinePreviewBody{NodeID: preview.NodeID, StepType: preview.StepType, Columns: preview.Columns, PreviewRows: preview.PreviewRows}}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "previewDataPipelineDraft", Method: http.MethodPost, Path: "/api/v1/data-pipelines/{pipelinePublicId}/preview", Summary: "未保存 draft graph の selected node まで preview する", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineDraftPreviewInput) (*DataPipelinePreviewOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := checkDatasetResourceAction(ctx, deps, tenant.ID, current.User.ID, service.DataResourceDataPipeline, input.PipelinePublicID, service.DataActionPreview); err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "previewDataPipelineDraft", err)
		}
		preview, err := deps.DataPipelineService.PreviewDraft(ctx, tenant.ID, current.User.ID, input.PipelinePublicID, input.Body.Graph, input.Body.NodeID, input.Body.Limit)
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "previewDataPipelineDraft", err)
		}
		return &DataPipelinePreviewOutput{Body: DataPipelinePreviewBody{NodeID: preview.NodeID, StepType: preview.StepType, Columns: preview.Columns, PreviewRows: preview.PreviewRows}}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "schemaMappingCandidates", Method: http.MethodPost, Path: "/api/v1/data-pipelines/schema-mapping/candidates", Summary: "schema mapping 候補を返す", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *SchemaMappingCandidateInput) (*SchemaMappingCandidateOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		result, err := deps.DataPipelineService.SchemaMappingCandidates(ctx, tenant.ID, current.User.ID, schemaMappingCandidateInputFromBody(input.Body))
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "schemaMappingCandidates", err)
		}
		return &SchemaMappingCandidateOutput{Body: toSchemaMappingCandidateListBody(result)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "recordSchemaMappingExample", Method: http.MethodPost, Path: "/api/v1/data-pipelines/schema-mapping/examples", Summary: "schema mapping の採用/却下履歴を記録する", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *SchemaMappingExampleInput) (*SchemaMappingExampleOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		result, err := deps.DataPipelineService.RecordSchemaMappingExample(ctx, tenant.ID, current.User.ID, schemaMappingExampleInputFromBody(input.Body), sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "recordSchemaMappingExample", err)
		}
		return &SchemaMappingExampleOutput{Body: toSchemaMappingExampleBody(result)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "listDataPipelineRuns", Method: http.MethodGet, Path: "/api/v1/data-pipelines/{pipelinePublicId}/runs", Summary: "data pipeline run history を返す", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineRunsInput) (*DataPipelineRunListOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		if err := checkDatasetResourceAction(ctx, deps, tenant.ID, current.User.ID, service.DataResourceDataPipeline, input.PipelinePublicID, service.DataActionView); err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "listDataPipelineRuns", err)
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

	huma.Register(api, huma.Operation{OperationID: "listDataPipelineReviewItems", Method: http.MethodGet, Path: "/api/v1/data-pipelines/{pipelinePublicId}/review-items", Summary: "data pipeline review item 一覧を返す", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineReviewItemListInput) (*DataPipelineReviewItemListOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		if err := checkDatasetResourceAction(ctx, deps, tenant.ID, current.User.ID, service.DataResourceDataPipeline, input.PipelinePublicID, service.DataActionView); err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "listDataPipelineReviewItems", err)
		}
		items, err := deps.DataPipelineService.ListReviewItems(ctx, tenant.ID, input.PipelinePublicID, service.DataPipelineReviewItemListInput{Status: input.Status, Limit: input.Limit})
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "listDataPipelineReviewItems", err)
		}
		out := &DataPipelineReviewItemListOutput{}
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDataPipelineReviewItemBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{OperationID: "getDataPipelineReviewItem", Method: http.MethodGet, Path: "/api/v1/data-pipeline-review-items/{reviewItemPublicId}", Summary: "data pipeline review item detail を返す", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineReviewItemInput) (*DataPipelineReviewItemOutput, error) {
		_, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.DataPipelineService.GetReviewItem(ctx, tenant.ID, input.ReviewItemPublicID)
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "getDataPipelineReviewItem", err)
		}
		return &DataPipelineReviewItemOutput{Body: toDataPipelineReviewItemBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "transitionDataPipelineReviewItem", Method: http.MethodPost, Path: "/api/v1/data-pipeline-review-items/{reviewItemPublicId}/transition", Summary: "data pipeline review item の status を更新する", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineReviewItemTransitionInput) (*DataPipelineReviewItemOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DataPipelineService.TransitionReviewItem(ctx, tenant.ID, current.User.ID, input.ReviewItemPublicID, service.DataPipelineReviewItemTransitionInput{Status: input.Body.Status, Comment: input.Body.Comment}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "transitionDataPipelineReviewItem", err)
		}
		return &DataPipelineReviewItemOutput{Body: toDataPipelineReviewItemBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "commentDataPipelineReviewItem", Method: http.MethodPost, Path: "/api/v1/data-pipeline-review-items/{reviewItemPublicId}/comments", Summary: "data pipeline review item に comment を追加する", Tags: []string{DocTagDataDatasets}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DataPipelineReviewItemCommentInput) (*DataPipelineReviewCommentOutput, error) {
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		comment, err := deps.DataPipelineService.CreateReviewItemComment(ctx, tenant.ID, current.User.ID, input.ReviewItemPublicID, input.Body.Body, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "commentDataPipelineReviewItem", err)
		}
		return &DataPipelineReviewCommentOutput{Body: toDataPipelineReviewCommentBody(comment)}, nil
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
		current, tenant, err := requireDataPipelineTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		if err := checkDatasetResourceAction(ctx, deps, tenant.ID, current.User.ID, service.DataResourceDataPipeline, input.PipelinePublicID, service.DataActionView); err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "listDataPipelineSchedules", err)
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
		if err := checkDatasetResourceAction(ctx, deps, tenant.ID, current.User.ID, service.DataResourceDataPipeline, input.PipelinePublicID, service.DataActionManageSchedule); err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "createDataPipelineSchedule", err)
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
	return requireActiveTenantAnyRole(ctx, deps, sessionID, csrfToken, []string{"data_pipeline_user", "tenant_admin"}, "data pipeline service")
}

func scheduleInputFromBody(body DataPipelineScheduleWriteBody) service.DataPipelineScheduleInput {
	return service.DataPipelineScheduleInput{Frequency: body.Frequency, Timezone: body.Timezone, RunTime: body.RunTime, Weekday: body.Weekday, MonthDay: body.MonthDay, Enabled: body.Enabled}
}

func schemaMappingCandidateInputFromBody(body SchemaMappingCandidateRequestBody) service.DataPipelineSchemaMappingCandidateInput {
	columns := make([]service.DataPipelineSchemaMappingSourceColumn, 0, len(body.Columns))
	for _, column := range body.Columns {
		columns = append(columns, service.DataPipelineSchemaMappingSourceColumn{
			SourceColumn:    column.SourceColumn,
			SheetName:       column.SheetName,
			SampleValues:    column.SampleValues,
			NeighborColumns: column.NeighborColumns,
		})
	}
	return service.DataPipelineSchemaMappingCandidateInput{
		PipelinePublicID: body.PipelinePublicID,
		VersionPublicID:  body.VersionPublicID,
		Domain:           body.Domain,
		SchemaType:       body.SchemaType,
		Columns:          columns,
		Limit:            body.Limit,
	}
}

func schemaMappingExampleInputFromBody(body SchemaMappingExampleWriteBody) service.DataPipelineSchemaMappingExampleInput {
	return service.DataPipelineSchemaMappingExampleInput{
		PipelinePublicID:     body.PipelinePublicID,
		VersionPublicID:      body.VersionPublicID,
		SchemaColumnPublicID: body.SchemaColumnPublicID,
		SourceColumn:         body.SourceColumn,
		SheetName:            body.SheetName,
		SampleValues:         body.SampleValues,
		NeighborColumns:      body.NeighborColumns,
		Decision:             body.Decision,
	}
}

func toSchemaMappingCandidateListBody(result service.DataPipelineSchemaMappingCandidateResult) SchemaMappingCandidateListBody {
	body := SchemaMappingCandidateListBody{Items: make([]SchemaMappingCandidateItemBody, 0, len(result.Items))}
	for _, item := range result.Items {
		out := SchemaMappingCandidateItemBody{
			SourceColumn: item.SourceColumn,
			Candidates:   make([]SchemaMappingCandidateBody, 0, len(item.Candidates)),
		}
		for _, candidate := range item.Candidates {
			out.Candidates = append(out.Candidates, SchemaMappingCandidateBody{
				SchemaColumnPublicID: candidate.SchemaColumnPublicID,
				TargetColumn:         candidate.TargetColumn,
				Score:                candidate.Score,
				MatchMethod:          candidate.MatchMethod,
				Reason:               candidate.Reason,
				Snippet:              candidate.Snippet,
				AcceptedEvidence:     candidate.AcceptedEvidence,
				RejectedEvidence:     candidate.RejectedEvidence,
			})
		}
		body.Items = append(body.Items, out)
	}
	return body
}

func toSchemaMappingExampleBody(result service.DataPipelineSchemaMappingExample) SchemaMappingExampleBody {
	return SchemaMappingExampleBody{
		PublicID:             result.PublicID,
		SchemaColumnPublicID: result.SchemaColumnPublicID,
		SourceColumn:         result.SourceColumn,
		TargetColumn:         result.TargetColumn,
		Decision:             result.Decision,
		SharedScope:          result.SharedScope,
	}
}

func toDataPipelineHTTPError(ctx context.Context, deps Dependencies, operation string, err error) error {
	switch {
	case errors.Is(err, service.ErrDataPipelineNotFound), errors.Is(err, service.ErrDataPipelineVersionNotFound), errors.Is(err, service.ErrDataPipelineRunNotFound), errors.Is(err, service.ErrDataPipelineScheduleNotFound), errors.Is(err, service.ErrDataPipelineReviewItemNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, service.ErrInvalidDataPipelineInput), errors.Is(err, service.ErrInvalidDataPipelineGraph), errors.Is(err, service.ErrInvalidCursor):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, service.ErrDataPipelineVersionUnpublished):
		return huma.Error409Conflict(err.Error())
	case errors.Is(err, service.ErrDataPermissionDenied):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, service.ErrDataAuthzUnavailable):
		return dataAccessAuthorizationUnavailableHTTPError(ctx, deps, operation, err)
	case errors.Is(err, service.ErrDatasetClickHouseNotReady):
		return huma.Error503ServiceUnavailable(err.Error())
	default:
		return internalHTTPError(ctx, deps, operation, err)
	}
}

func filterDataPipelinesForAction(ctx context.Context, deps Dependencies, actorUserID int64, items []service.DataPipeline, action string) ([]service.DataPipeline, error) {
	if deps.DatasetAuthorizationService == nil {
		return nil, service.ErrDataAuthzUnavailable
	}
	publicIDs := make([]string, 0, len(items))
	for _, item := range items {
		publicIDs = append(publicIDs, item.PublicID)
	}
	allowed, err := deps.DatasetAuthorizationService.FilterResourcePublicIDs(ctx, actorUserID, service.DataResourceDataPipeline, action, publicIDs)
	if err != nil {
		return nil, err
	}
	filtered := make([]service.DataPipeline, 0, len(items))
	for _, item := range items {
		if allowed[item.PublicID] {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
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
	return DataPipelineBody{
		PublicID:              item.PublicID,
		Name:                  item.Name,
		Description:           item.Description,
		Status:                item.Status,
		PublishedVersionID:    item.PublishedVersionID,
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
		ArchivedAt:            item.ArchivedAt,
		LatestRunStatus:       item.LatestRunStatus,
		LatestRunAt:           item.LatestRunAt,
		LatestRunPublicID:     item.LatestRunPublicID,
		ScheduleState:         item.ScheduleState,
		EnabledScheduleCount:  item.EnabledScheduleCount,
		DisabledScheduleCount: item.DisabledScheduleCount,
		NextRunAt:             item.NextRunAt,
	}
}

func toDataPipelineVersionBody(item service.DataPipelineVersion) DataPipelineVersionBody {
	return DataPipelineVersionBody{PublicID: item.PublicID, PipelineID: item.PipelineID, VersionNumber: item.VersionNumber, Status: item.Status, Graph: item.Graph, ValidationSummary: item.ValidationSummary, CreatedAt: item.CreatedAt, PublishedAt: item.PublishedAt}
}

func toDataPipelineRunBody(item service.DataPipelineRun) DataPipelineRunBody {
	body := DataPipelineRunBody{PublicID: item.PublicID, VersionID: item.VersionID, ScheduleID: item.ScheduleID, TriggerKind: item.TriggerKind, Status: item.Status, OutputWorkTableID: item.OutputWorkTableID, RowCount: item.RowCount, ErrorSummary: item.ErrorSummary, StartedAt: item.StartedAt, CompletedAt: item.CompletedAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, Steps: make([]DataPipelineRunStepBody, 0, len(item.Steps)), Outputs: make([]DataPipelineRunOutputBody, 0, len(item.Outputs))}
	for _, step := range item.Steps {
		body.Steps = append(body.Steps, DataPipelineRunStepBody{NodeID: step.NodeID, StepType: step.StepType, Status: step.Status, RowCount: step.RowCount, ErrorSummary: step.ErrorSummary, ErrorSample: step.ErrorSample, Metadata: step.Metadata, StartedAt: step.StartedAt, CompletedAt: step.CompletedAt, CreatedAt: step.CreatedAt, UpdatedAt: step.UpdatedAt})
	}
	for _, output := range item.Outputs {
		body.Outputs = append(body.Outputs, DataPipelineRunOutputBody{NodeID: output.NodeID, Status: output.Status, OutputWorkTableID: output.OutputWorkTableID, RowCount: output.RowCount, ErrorSummary: output.ErrorSummary, Metadata: output.Metadata, StartedAt: output.StartedAt, CompletedAt: output.CompletedAt, CreatedAt: output.CreatedAt, UpdatedAt: output.UpdatedAt})
	}
	return body
}

func toDataPipelineReviewItemBody(item service.DataPipelineReviewItem) DataPipelineReviewItemBody {
	body := DataPipelineReviewItemBody{
		PublicID:          item.PublicID,
		VersionID:         item.VersionID,
		RunID:             item.RunID,
		NodeID:            item.NodeID,
		Queue:             item.Queue,
		Status:            item.Status,
		Reason:            item.Reason,
		SourceSnapshot:    item.SourceSnapshot,
		SourceFingerprint: item.SourceFingerprint,
		CreatedByUserID:   item.CreatedByUserID,
		UpdatedByUserID:   item.UpdatedByUserID,
		AssignedToUserID:  item.AssignedToUserID,
		DecisionComment:   item.DecisionComment,
		DecidedAt:         item.DecidedAt,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
		Comments:          make([]DataPipelineReviewCommentBody, 0, len(item.Comments)),
	}
	for _, comment := range item.Comments {
		body.Comments = append(body.Comments, toDataPipelineReviewCommentBody(comment))
	}
	return body
}

func toDataPipelineReviewCommentBody(item service.DataPipelineReviewItemComment) DataPipelineReviewCommentBody {
	return DataPipelineReviewCommentBody{PublicID: item.PublicID, AuthorUserID: item.AuthorUserID, Body: item.Body, CreatedAt: item.CreatedAt}
}

func toDataPipelineScheduleBody(item service.DataPipelineSchedule) DataPipelineScheduleBody {
	return DataPipelineScheduleBody{PublicID: item.PublicID, VersionID: item.VersionID, Frequency: item.Frequency, Timezone: item.Timezone, RunTime: item.RunTime, Weekday: item.Weekday, MonthDay: item.MonthDay, Enabled: item.Enabled, NextRunAt: item.NextRunAt, LastRunAt: item.LastRunAt, LastStatus: item.LastStatus, LastErrorSummary: item.LastErrorSummary, LastRunID: item.LastRunID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
