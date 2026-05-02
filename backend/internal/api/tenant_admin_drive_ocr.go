package api

import (
	"context"
	"net/http"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type TenantAdminDriveOCRDependencyBody struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
}

type TenantAdminDriveOCROllamaBody struct {
	Configured     bool `json:"configured"`
	Reachable      bool `json:"reachable"`
	ModelAvailable bool `json:"modelAvailable"`
}

type TenantAdminDriveOCRLMStudioBody struct {
	Configured     bool `json:"configured"`
	Reachable      bool `json:"reachable"`
	ModelAvailable bool `json:"modelAvailable"`
}

type TenantAdminDriveOCRLocalCommandBody struct {
	Name       string `json:"name"`
	Command    string `json:"command"`
	Configured bool   `json:"configured"`
	Available  bool   `json:"available"`
	Version    string `json:"version,omitempty"`
}

type TenantAdminDriveOCRStatusBody struct {
	Enabled             bool                                  `json:"enabled"`
	OCREngine           string                                `json:"ocrEngine"`
	StructuredExtractor string                                `json:"structuredExtractor"`
	Dependencies        []TenantAdminDriveOCRDependencyBody   `json:"dependencies"`
	Ollama              TenantAdminDriveOCROllamaBody         `json:"ollama"`
	LMStudio            TenantAdminDriveOCRLMStudioBody       `json:"lmStudio"`
	LocalCommands       []TenantAdminDriveOCRLocalCommandBody `json:"localCommands"`
	StatusCounts        map[string]int64                      `json:"statusCounts"`
}

type TenantAdminDriveOCRStatusOutput struct {
	Body TenantAdminDriveOCRStatusBody
}

func registerTenantAdminDriveOCRRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "getTenantAdminDriveOCRStatus",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/ocr/status",
		Tags:        []string{DocTagDriveAdminGovernance},
		Summary:     "tenant admin 用 Drive OCR runtime status を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveBySlugInput) (*TenantAdminDriveOCRStatusOutput, error) {
		if deps.DriveOCRService == nil {
			return nil, huma.Error503ServiceUnavailable("drive ocr service is not configured")
		}
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		status, err := deps.DriveOCRService.RuntimeStatus(ctx, tenant.ID)
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &TenantAdminDriveOCRStatusOutput{Body: toTenantAdminDriveOCRStatusBody(status)}, nil
	})
}

func toTenantAdminDriveOCRStatusBody(status service.DriveOCRRuntimeStatus) TenantAdminDriveOCRStatusBody {
	out := TenantAdminDriveOCRStatusBody{
		Enabled:             status.Enabled,
		OCREngine:           status.OCREngine,
		StructuredExtractor: status.StructuredExtractor,
		Ollama: TenantAdminDriveOCROllamaBody{
			Configured:     status.Ollama.Configured,
			Reachable:      status.Ollama.Reachable,
			ModelAvailable: status.Ollama.ModelAvailable,
		},
		LMStudio: TenantAdminDriveOCRLMStudioBody{
			Configured:     status.LMStudio.Configured,
			Reachable:      status.LMStudio.Reachable,
			ModelAvailable: status.LMStudio.ModelAvailable,
		},
		StatusCounts: status.StatusCounts,
	}
	for _, dep := range status.Dependencies {
		out.Dependencies = append(out.Dependencies, TenantAdminDriveOCRDependencyBody{
			Name:      dep.Name,
			Available: dep.Available,
			Version:   dep.Version,
		})
	}
	for _, command := range status.LocalCommands {
		out.LocalCommands = append(out.LocalCommands, TenantAdminDriveOCRLocalCommandBody{
			Name:       command.Name,
			Command:    command.Command,
			Configured: command.Configured,
			Available:  command.Available,
			Version:    command.Version,
		})
	}
	return out
}
