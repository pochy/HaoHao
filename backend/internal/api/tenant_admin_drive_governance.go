package api

import (
	"context"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type TenantAdminDriveIndexRebuildInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
}

type TenantAdminDriveLocalSearchIndexJobsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Status        string      `query:"status" enum:"queued,processing,completed,failed,skipped"`
	Limit         int32       `query:"limit" default:"50" minimum:"1" maximum:"200"`
}

type LocalSearchIndexJobBody struct {
	PublicID         string     `json:"publicId" format:"uuid"`
	ResourceKind     string     `json:"resourceKind,omitempty" enum:"drive_file,ocr_run,product_extraction,gold_table,schema_column,mapping_example"`
	ResourceID       *int64     `json:"resourceId,omitempty"`
	ResourcePublicID string     `json:"resourcePublicId,omitempty" format:"uuid"`
	Reason           string     `json:"reason"`
	Status           string     `json:"status" enum:"queued,processing,completed,failed,skipped"`
	Attempts         int32      `json:"attempts"`
	IndexedCount     int32      `json:"indexedCount"`
	SkippedCount     int32      `json:"skippedCount"`
	FailedCount      int32      `json:"failedCount"`
	LastError        string     `json:"lastError,omitempty"`
	StartedAt        *time.Time `json:"startedAt,omitempty" format:"date-time"`
	CompletedAt      *time.Time `json:"completedAt,omitempty" format:"date-time"`
	CreatedAt        time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt        time.Time  `json:"updatedAt" format:"date-time"`
}

type LocalSearchIndexJobOutput struct {
	Body LocalSearchIndexJobBody
}

type LocalSearchIndexJobListOutput struct {
	Body struct {
		Items []LocalSearchIndexJobBody `json:"items"`
	}
}

type TenantAdminDriveEncryptionPolicyBody struct {
	Mode         string    `json:"mode"`
	Scope        string    `json:"scope"`
	KeyPublicID  string    `json:"keyPublicId,omitempty"`
	Provider     string    `json:"provider,omitempty"`
	MaskedKeyRef string    `json:"maskedKeyRef,omitempty"`
	KeyStatus    string    `json:"keyStatus"`
	UpdatedAt    time.Time `json:"updatedAt,omitempty" format:"date-time"`
}

type TenantAdminDriveEncryptionPolicyInputBody struct {
	Mode     string `json:"mode" enum:"service_managed,tenant_managed"`
	Provider string `json:"provider,omitempty"`
	KeyRef   string `json:"keyRef,omitempty"`
}

type TenantAdminDriveEncryptionPolicyOutput struct {
	Body TenantAdminDriveEncryptionPolicyBody
}

type TenantAdminDriveEncryptionPolicyInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          TenantAdminDriveEncryptionPolicyInputBody
}

type TenantAdminDriveKMSKeyStatusInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	KeyPublicID   string      `path:"keyPublicId" format:"uuid"`
	Body          struct {
		Status string `json:"status" enum:"active,disabled,unavailable,deleted"`
	}
}

type TenantAdminDriveResidencyPolicyBody struct {
	PrimaryRegion   string    `json:"primaryRegion"`
	AllowedRegions  []string  `json:"allowedRegions"`
	ReplicationMode string    `json:"replicationMode"`
	IndexRegion     string    `json:"indexRegion"`
	BackupRegion    string    `json:"backupRegion"`
	Status          string    `json:"status"`
	UpdatedAt       time.Time `json:"updatedAt,omitempty" format:"date-time"`
}

type TenantAdminDriveResidencyPolicyOutput struct {
	Body TenantAdminDriveResidencyPolicyBody
}

type TenantAdminDriveResidencyPolicyInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          TenantAdminDriveResidencyPolicyBody
}

type DriveLegalCaseBody struct {
	PublicID    string    `json:"publicId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt" format:"date-time"`
}

type DriveLegalCaseOutput struct {
	Body DriveLegalCaseBody
}

type DriveLegalCaseCreateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          struct {
		Name        string `json:"name" maxLength:"255"`
		Description string `json:"description,omitempty" maxLength:"2000"`
	}
}

type DriveLegalCaseFileInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	CasePublicID  string      `path:"casePublicId" format:"uuid"`
	Body          struct {
		FilePublicID string `json:"filePublicId" format:"uuid"`
		Reason       string `json:"reason,omitempty" maxLength:"2000"`
	}
}

type DriveLegalExportBody struct {
	PublicID     string    `json:"publicId"`
	CasePublicID string    `json:"casePublicId"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt" format:"date-time"`
}

type DriveLegalExportOutput struct {
	Body DriveLegalExportBody
}

type DriveLegalExportCreateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	CasePublicID  string      `path:"casePublicId" format:"uuid"`
}

type DriveCleanRoomBody struct {
	PublicID  string    `json:"publicId"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt" format:"date-time"`
}

type DriveCleanRoomOutput struct {
	Body DriveCleanRoomBody
}

type DriveCleanRoomCreateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          struct {
		Name string `json:"name" maxLength:"255"`
	}
}

type DriveCleanRoomParticipantInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	RoomPublicID  string      `path:"roomPublicId" format:"uuid"`
	Body          struct {
		UserPublicID string `json:"userPublicId" format:"uuid"`
		Role         string `json:"role" enum:"participant,reviewer,owner"`
	}
}

type DriveCleanRoomDatasetBody struct {
	PublicID           string    `json:"publicId"`
	CleanRoomPublicID  string    `json:"cleanRoomPublicId"`
	SourceFilePublicID string    `json:"sourceFilePublicId"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"createdAt" format:"date-time"`
}

type DriveCleanRoomDatasetOutput struct {
	Body DriveCleanRoomDatasetBody
}

type DriveCleanRoomDatasetInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	RoomPublicID  string      `path:"roomPublicId" format:"uuid"`
	Body          struct {
		FilePublicID string `json:"filePublicId" format:"uuid"`
	}
}

type DriveCleanRoomExportBody struct {
	PublicID     string    `json:"publicId"`
	Status       string    `json:"status"`
	DeniedReason string    `json:"deniedReason,omitempty"`
	CreatedAt    time.Time `json:"createdAt" format:"date-time"`
}

type DriveCleanRoomExportOutput struct {
	Body DriveCleanRoomExportBody
}

type DriveCleanRoomExportInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	RoomPublicID  string      `path:"roomPublicId" format:"uuid"`
	Body          struct {
		RawDatasetExport bool `json:"rawDatasetExport"`
	}
}

func registerTenantAdminDriveGovernanceRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "rebuildTenantAdminDriveSearchIndex",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/search/index/rebuild",
		Tags:        []string{DocTagDriveAdminGovernance},
		Summary:     "Drive search index を rebuild する",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveIndexRebuildInput) (*DriveIndexRebuildOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		result, err := deps.DriveService.RebuildDriveSearchIndex(ctx, tenant.ID, current.User.ID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveIndexRebuildOutput{Body: DriveIndexRebuildBody{Indexed: result.Indexed, Skipped: result.Skipped, Failed: result.Failed}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createTenantAdminDriveLocalSearchRebuild",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/search/local-index/rebuilds",
		Tags:        []string{DocTagDriveAdminGovernance},
		Summary:     "Drive local search index rebuild を enqueue する",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveIndexRebuildInput) (*LocalSearchIndexJobOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if deps.LocalSearchService == nil {
			return nil, huma.Error503ServiceUnavailable("local search service is not configured")
		}
		job, err := deps.LocalSearchService.RequestRebuild(ctx, tenant.ID, "admin_rebuild")
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &LocalSearchIndexJobOutput{Body: toLocalSearchIndexJobBody(job)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listTenantAdminDriveLocalSearchIndexJobs",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/search/local-index/jobs",
		Tags:        []string{DocTagDriveAdminGovernance},
		Summary:     "Drive local search index job 一覧を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveLocalSearchIndexJobsInput) (*LocalSearchIndexJobListOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if deps.LocalSearchService == nil {
			return nil, huma.Error503ServiceUnavailable("local search service is not configured")
		}
		jobs, err := deps.LocalSearchService.ListIndexJobs(ctx, tenant.ID, input.Status, input.Limit)
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		out := &LocalSearchIndexJobListOutput{}
		out.Body.Items = make([]LocalSearchIndexJobBody, 0, len(jobs))
		for _, job := range jobs {
			out.Body.Items = append(out.Body.Items, toLocalSearchIndexJobBody(job))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{OperationID: "getTenantAdminDriveEncryptionPolicy", Method: http.MethodGet, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/security/encryption-policy", Tags: []string{DocTagDriveAdminGovernance}, Summary: "Drive encryption policy を返す", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDriveBySlugInput) (*TenantAdminDriveEncryptionPolicyOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		policy, err := deps.DriveService.GetEncryptionPolicy(ctx, tenant.ID)
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &TenantAdminDriveEncryptionPolicyOutput{Body: toTenantAdminDriveEncryptionPolicyBody(policy)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "updateTenantAdminDriveEncryptionPolicy", Method: http.MethodPut, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/security/encryption-policy", Tags: []string{DocTagDriveAdminGovernance}, Summary: "Drive encryption policy を更新する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDriveEncryptionPolicyInput) (*TenantAdminDriveEncryptionPolicyOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		policy, err := deps.DriveService.UpsertEncryptionPolicy(ctx, tenant.ID, current.User.ID, input.Body.Mode, input.Body.Provider, input.Body.KeyRef, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &TenantAdminDriveEncryptionPolicyOutput{Body: toTenantAdminDriveEncryptionPolicyBody(policy)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "updateTenantAdminDriveKmsKeyStatus", Method: http.MethodPatch, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/security/kms-keys/{keyPublicId}", Tags: []string{DocTagDriveAdminGovernance}, Summary: "Drive KMS key status を更新する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDriveKMSKeyStatusInput) (*TenantAdminDriveEncryptionPolicyOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		policy, err := deps.DriveService.SetEncryptionKeyStatus(ctx, tenant.ID, current.User.ID, input.KeyPublicID, input.Body.Status, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &TenantAdminDriveEncryptionPolicyOutput{Body: toTenantAdminDriveEncryptionPolicyBody(policy)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "getTenantAdminDriveResidencyPolicy", Method: http.MethodGet, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/residency-policy", Tags: []string{DocTagDriveAdminGovernance}, Summary: "Drive residency policy を返す", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDriveBySlugInput) (*TenantAdminDriveResidencyPolicyOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		policy, err := deps.DriveService.GetResidencyPolicy(ctx, tenant.ID)
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &TenantAdminDriveResidencyPolicyOutput{Body: toTenantAdminDriveResidencyPolicyBody(policy)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "updateTenantAdminDriveResidencyPolicy", Method: http.MethodPut, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/residency-policy", Tags: []string{DocTagDriveAdminGovernance}, Summary: "Drive residency policy を更新する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDriveResidencyPolicyInput) (*TenantAdminDriveResidencyPolicyOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		policy, err := deps.DriveService.UpsertResidencyPolicy(ctx, tenant.ID, current.User.ID, service.DriveResidencyPolicy{
			PrimaryRegion:   input.Body.PrimaryRegion,
			AllowedRegions:  input.Body.AllowedRegions,
			ReplicationMode: input.Body.ReplicationMode,
			IndexRegion:     input.Body.IndexRegion,
			BackupRegion:    input.Body.BackupRegion,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &TenantAdminDriveResidencyPolicyOutput{Body: toTenantAdminDriveResidencyPolicyBody(policy)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createDriveLegalCase", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/legal/cases", Tags: []string{DocTagDriveSecurityCompliance}, Summary: "Drive legal case を作成する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveLegalCaseCreateInput) (*DriveLegalCaseOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.CreateLegalCase(ctx, tenant.ID, current.User.ID, input.Body.Name, input.Body.Description, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveLegalCaseOutput{Body: toDriveLegalCaseBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "addDriveLegalCaseFile", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/legal/cases/{casePublicId}/files", Tags: []string{DocTagDriveSecurityCompliance}, Summary: "Drive legal case に file hold を追加する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveLegalCaseFileInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.AddLegalCaseFile(ctx, tenant.ID, current.User.ID, input.CasePublicID, input.Body.FilePublicID, input.Body.Reason, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createDriveLegalExport", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/legal/cases/{casePublicId}/exports", Tags: []string{DocTagDriveSecurityCompliance}, Summary: "Drive legal export を作成する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveLegalExportCreateInput) (*DriveLegalExportOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.CreateLegalExport(ctx, tenant.ID, current.User.ID, input.CasePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveLegalExportOutput{Body: DriveLegalExportBody{PublicID: item.PublicID, CasePublicID: item.CasePublicID, Status: item.Status, CreatedAt: item.CreatedAt}}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createDriveCleanRoom", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/clean-rooms", Tags: []string{DocTagDriveSecurityCompliance}, Summary: "Drive clean room を作成する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveCleanRoomCreateInput) (*DriveCleanRoomOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		room, err := deps.DriveService.CreateCleanRoom(ctx, tenant.ID, current.User.ID, input.Body.Name, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveCleanRoomOutput{Body: toDriveCleanRoomBody(room)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "addDriveCleanRoomParticipant", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/clean-rooms/{roomPublicId}/participants", Tags: []string{DocTagDriveSecurityCompliance}, Summary: "Drive clean room participant を追加する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveCleanRoomParticipantInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.AddCleanRoomParticipant(ctx, tenant.ID, current.User.ID, input.RoomPublicID, input.Body.UserPublicID, input.Body.Role, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "submitDriveCleanRoomDataset", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/clean-rooms/{roomPublicId}/datasets", Tags: []string{DocTagDriveSecurityCompliance}, Summary: "Drive clean room dataset を投入する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveCleanRoomDatasetInput) (*DriveCleanRoomDatasetOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		dataset, err := deps.DriveService.SubmitCleanRoomDataset(ctx, tenant.ID, current.User.ID, input.RoomPublicID, input.Body.FilePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveCleanRoomDatasetOutput{Body: DriveCleanRoomDatasetBody{
			PublicID: dataset.PublicID, CleanRoomPublicID: dataset.CleanRoomPublicID, SourceFilePublicID: dataset.SourceFilePublicID, Status: dataset.Status, CreatedAt: dataset.CreatedAt,
		}}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "requestDriveCleanRoomExport", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/clean-rooms/{roomPublicId}/exports", Tags: []string{DocTagDriveSecurityCompliance}, Summary: "Drive clean room export を要求する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveCleanRoomExportInput) (*DriveCleanRoomExportOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		export, err := deps.DriveService.RequestCleanRoomExport(ctx, tenant.ID, current.User.ID, input.RoomPublicID, input.Body.RawDatasetExport, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			if export.PublicID != "" {
				return &DriveCleanRoomExportOutput{Body: toDriveCleanRoomExportBody(export)}, nil
			}
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveCleanRoomExportOutput{Body: toDriveCleanRoomExportBody(export)}, nil
	})
}

func toTenantAdminDriveEncryptionPolicyBody(item service.DriveEncryptionPolicy) TenantAdminDriveEncryptionPolicyBody {
	return TenantAdminDriveEncryptionPolicyBody{
		Mode: item.Mode, Scope: item.Scope, KeyPublicID: item.KeyPublicID,
		Provider: item.Provider, MaskedKeyRef: item.MaskedKeyRef, KeyStatus: item.KeyStatus, UpdatedAt: item.UpdatedAt,
	}
}

func toTenantAdminDriveResidencyPolicyBody(item service.DriveResidencyPolicy) TenantAdminDriveResidencyPolicyBody {
	return TenantAdminDriveResidencyPolicyBody{
		PrimaryRegion: item.PrimaryRegion, AllowedRegions: item.AllowedRegions,
		ReplicationMode: item.ReplicationMode, IndexRegion: item.IndexRegion, BackupRegion: item.BackupRegion,
		Status: item.Status, UpdatedAt: item.UpdatedAt,
	}
}

func toLocalSearchIndexJobBody(item service.LocalSearchIndexJob) LocalSearchIndexJobBody {
	return LocalSearchIndexJobBody{
		PublicID:         item.PublicID,
		ResourceKind:     item.ResourceKind,
		ResourceID:       item.ResourceID,
		ResourcePublicID: item.ResourcePublicID,
		Reason:           item.Reason,
		Status:           item.Status,
		Attempts:         item.Attempts,
		IndexedCount:     item.IndexedCount,
		SkippedCount:     item.SkippedCount,
		FailedCount:      item.FailedCount,
		LastError:        item.LastError,
		StartedAt:        item.StartedAt,
		CompletedAt:      item.CompletedAt,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func toDriveLegalCaseBody(item service.DriveLegalCase) DriveLegalCaseBody {
	return DriveLegalCaseBody{PublicID: item.PublicID, Name: item.Name, Description: item.Description, Status: item.Status, CreatedAt: item.CreatedAt}
}

func toDriveCleanRoomBody(item service.DriveCleanRoom) DriveCleanRoomBody {
	return DriveCleanRoomBody{PublicID: item.PublicID, Name: item.Name, Status: item.Status, CreatedAt: item.CreatedAt}
}

func toDriveCleanRoomExportBody(item service.DriveCleanRoomExport) DriveCleanRoomExportBody {
	return DriveCleanRoomExportBody{PublicID: item.PublicID, Status: item.Status, DeniedReason: item.DeniedReason, CreatedAt: item.CreatedAt}
}
