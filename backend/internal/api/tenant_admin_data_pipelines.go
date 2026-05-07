package api

import (
	"context"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type TenantAdminSchemaMappingSearchDocumentsRebuildInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          struct {
		Limit int32 `json:"limit,omitempty" minimum:"1" maximum:"1000"`
	}
}

type TenantAdminSchemaMappingExampleSharingInput struct {
	SessionCookie          http.Cookie `cookie:"SESSION_ID"`
	CSRFToken              string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug             string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	MappingExamplePublicID string      `path:"mappingExamplePublicId" format:"uuid"`
	Body                   struct {
		SharedScope string `json:"sharedScope" enum:"private,tenant"`
	}
}

type TenantAdminSchemaMappingExampleListInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Query         string      `query:"q" maxLength:"240"`
	SharedScope   string      `query:"sharedScope" enum:"private,tenant"`
	Decision      string      `query:"decision" enum:"accepted,rejected"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"200"`
}

type TenantAdminSchemaMappingSearchDocumentsRebuildOutput struct {
	Body struct {
		Indexed                int `json:"indexed"`
		SchemaColumnsIndexed   int `json:"schemaColumnsIndexed"`
		MappingExamplesIndexed int `json:"mappingExamplesIndexed"`
	}
}

type TenantAdminSchemaMappingExampleOutput struct {
	Body SchemaMappingExampleBody
}

type TenantAdminSchemaMappingExampleListOutput struct {
	Body struct {
		Items []TenantAdminSchemaMappingExampleListItemBody `json:"items"`
	}
}

type TenantAdminSchemaMappingExampleListItemBody struct {
	PublicID                   string     `json:"publicId" format:"uuid"`
	PipelinePublicID           string     `json:"pipelinePublicId" format:"uuid"`
	PipelineName               string     `json:"pipelineName"`
	SchemaColumnPublicID       string     `json:"schemaColumnPublicId" format:"uuid"`
	Domain                     string     `json:"domain"`
	SchemaType                 string     `json:"schemaType"`
	SourceColumn               string     `json:"sourceColumn"`
	SheetName                  string     `json:"sheetName,omitempty"`
	SampleValues               []string   `json:"sampleValues"`
	NeighborColumns            []string   `json:"neighborColumns"`
	TargetColumn               string     `json:"targetColumn"`
	Decision                   string     `json:"decision"`
	SharedScope                string     `json:"sharedScope"`
	SearchDocumentMaterialized bool       `json:"searchDocumentMaterialized"`
	DecidedAt                  time.Time  `json:"decidedAt"`
	SharedAt                   *time.Time `json:"sharedAt,omitempty"`
	CreatedAt                  time.Time  `json:"createdAt"`
	UpdatedAt                  time.Time  `json:"updatedAt"`
}

func registerTenantAdminDataPipelineRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listTenantAdminSchemaMappingExamples",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/data-pipelines/schema-mapping/examples",
		Tags:        []string{DocTagDataDatasets},
		Summary:     "tenant admin 用 schema mapping example 一覧を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminSchemaMappingExampleListInput) (*TenantAdminSchemaMappingExampleListOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if deps.DataPipelineService == nil {
			return nil, huma.Error503ServiceUnavailable("data pipeline service is not configured")
		}
		items, err := deps.DataPipelineService.ListSchemaMappingExamplesForAdmin(ctx, tenant.ID, service.DataPipelineSchemaMappingExampleListInput{
			Query:       input.Query,
			SharedScope: input.SharedScope,
			Decision:    input.Decision,
			Limit:       input.Limit,
		})
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "listTenantAdminSchemaMappingExamples", err)
		}
		out := &TenantAdminSchemaMappingExampleListOutput{}
		out.Body.Items = make([]TenantAdminSchemaMappingExampleListItemBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toTenantAdminSchemaMappingExampleListItemBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "rebuildTenantAdminSchemaMappingSearchDocuments",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/data-pipelines/schema-mapping/search-documents/rebuild",
		Tags:        []string{DocTagDataDatasets},
		Summary:     "schema mapping search documents を rebuild する",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminSchemaMappingSearchDocumentsRebuildInput) (*TenantAdminSchemaMappingSearchDocumentsRebuildOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if deps.DataPipelineService == nil {
			return nil, huma.Error503ServiceUnavailable("data pipeline service is not configured")
		}
		result, err := deps.DataPipelineService.RebuildSchemaMappingSearchDocuments(ctx, tenant.ID, input.Body.Limit)
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "rebuildTenantAdminSchemaMappingSearchDocuments", err)
		}
		out := &TenantAdminSchemaMappingSearchDocumentsRebuildOutput{}
		out.Body.Indexed = result.Indexed
		out.Body.SchemaColumnsIndexed = result.SchemaColumnsIndexed
		out.Body.MappingExamplesIndexed = result.MappingExamplesIndexed
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateTenantAdminSchemaMappingExampleSharing",
		Method:      http.MethodPatch,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/data-pipelines/schema-mapping/examples/{mappingExamplePublicId}/sharing",
		Tags:        []string{DocTagDataDatasets},
		Summary:     "schema mapping example の tenant 共有を切り替える",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminSchemaMappingExampleSharingInput) (*TenantAdminSchemaMappingExampleOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if deps.DataPipelineService == nil {
			return nil, huma.Error503ServiceUnavailable("data pipeline service is not configured")
		}
		result, err := deps.DataPipelineService.UpdateSchemaMappingExampleSharing(ctx, tenant.ID, current.User.ID, service.DataPipelineSchemaMappingExampleSharingInput{
			ExamplePublicID: input.MappingExamplePublicID,
			SharedScope:     input.Body.SharedScope,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDataPipelineHTTPError(ctx, deps, "updateTenantAdminSchemaMappingExampleSharing", err)
		}
		return &TenantAdminSchemaMappingExampleOutput{Body: toSchemaMappingExampleBody(result)}, nil
	})
}

func toTenantAdminSchemaMappingExampleListItemBody(item service.DataPipelineSchemaMappingExampleListItem) TenantAdminSchemaMappingExampleListItemBody {
	return TenantAdminSchemaMappingExampleListItemBody{
		PublicID:                   item.PublicID,
		PipelinePublicID:           item.PipelinePublicID,
		PipelineName:               item.PipelineName,
		SchemaColumnPublicID:       item.SchemaColumnPublicID,
		Domain:                     item.Domain,
		SchemaType:                 item.SchemaType,
		SourceColumn:               item.SourceColumn,
		SheetName:                  item.SheetName,
		SampleValues:               item.SampleValues,
		NeighborColumns:            item.NeighborColumns,
		TargetColumn:               item.TargetColumn,
		Decision:                   item.Decision,
		SharedScope:                item.SharedScope,
		SearchDocumentMaterialized: item.SearchDocumentMaterialized,
		DecidedAt:                  item.DecidedAt,
		SharedAt:                   item.SharedAt,
		CreatedAt:                  item.CreatedAt,
		UpdatedAt:                  item.UpdatedAt,
	}
}
