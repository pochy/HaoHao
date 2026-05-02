package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DriveSearchResultBody struct {
	Item      DriveItemBody                `json:"item"`
	Snippet   string                       `json:"snippet,omitempty"`
	IndexedAt *time.Time                   `json:"indexedAt,omitempty" format:"date-time"`
	Matches   []DriveSearchResultMatchBody `json:"matches,omitempty"`
}

type DriveSearchResultMatchBody struct {
	ResourceKind           string     `json:"resourceKind" enum:"drive_file,ocr_run,product_extraction,gold_table"`
	ResourcePublicID       string     `json:"resourcePublicId" format:"uuid"`
	MedallionAssetPublicID string     `json:"medallionAssetPublicId,omitempty" format:"uuid"`
	Layer                  string     `json:"layer,omitempty" enum:"bronze,silver,gold"`
	Snippet                string     `json:"snippet,omitempty"`
	IndexedAt              *time.Time `json:"indexedAt,omitempty" format:"date-time"`
}

type DriveSearchResultOutput struct {
	Body struct {
		Items []DriveSearchResultBody `json:"items"`
	}
}

type DriveIndexRebuildBody struct {
	Indexed int `json:"indexed"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

type DriveIndexRebuildOutput struct {
	Body DriveIndexRebuildBody
}

type DriveEditSessionBody struct {
	PublicID     string    `json:"publicId"`
	FilePublicID string    `json:"filePublicId"`
	Status       string    `json:"status"`
	BaseRevision int64     `json:"baseRevision"`
	Provider     string    `json:"provider"`
	ExpiresAt    time.Time `json:"expiresAt" format:"date-time"`
	CreatedAt    time.Time `json:"createdAt" format:"date-time"`
}

type DriveEditSessionOutput struct {
	Body DriveEditSessionBody
}

type DriveEditSaveBody struct {
	Content          string `json:"content" maxLength:"262144"`
	ExpectedRevision int64  `json:"expectedRevision"`
	Filename         string `json:"filename,omitempty" maxLength:"255"`
	ContentType      string `json:"contentType,omitempty"`
}

type DriveEditSaveOutput struct {
	Body struct {
		File     DriveFileBody `json:"file"`
		Revision int64         `json:"revision"`
		Conflict bool          `json:"conflict"`
	}
}

type DriveFilePathInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId" format:"uuid"`
}

type DriveEditSessionPathInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	CSRFToken       string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID    string      `path:"filePublicId" format:"uuid"`
	SessionPublicID string      `path:"sessionPublicId" format:"uuid"`
}

type DriveEditSaveInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	CSRFToken       string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID    string      `path:"filePublicId" format:"uuid"`
	SessionPublicID string      `path:"sessionPublicId" format:"uuid"`
	Body            DriveEditSaveBody
}

type DriveDeviceBody struct {
	PublicID           string     `json:"publicId"`
	DeviceName         string     `json:"deviceName"`
	Platform           string     `json:"platform"`
	Status             string     `json:"status"`
	RemoteWipeRequired bool       `json:"remoteWipeRequired"`
	LastSeenAt         *time.Time `json:"lastSeenAt,omitempty" format:"date-time"`
	CreatedAt          time.Time  `json:"createdAt" format:"date-time"`
	Token              string     `json:"token,omitempty"`
}

type DriveRegisterDeviceBody struct {
	DeviceName string `json:"deviceName" maxLength:"255"`
	Platform   string `json:"platform,omitempty" enum:"desktop,mobile,web"`
}

type DriveRegisterDeviceInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          DriveRegisterDeviceBody
}

type DriveDeviceOutput struct {
	Body DriveDeviceBody
}

type DriveSyncBearerInput struct {
	Authorization string `header:"Authorization" required:"true"`
	Cursor        string `query:"cursor"`
}

type DriveSyncDeltaEventBody struct {
	ID               int64          `json:"id"`
	PublicID         string         `json:"publicId"`
	ResourceType     string         `json:"resourceType"`
	ResourcePublicID string         `json:"resourcePublicId,omitempty"`
	Action           string         `json:"action"`
	ObjectVersion    string         `json:"objectVersion,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"createdAt" format:"date-time"`
}

type DriveSyncDeltaOutput struct {
	Body struct {
		Cursor      string                    `json:"cursor"`
		Events      []DriveSyncDeltaEventBody `json:"events"`
		RemoteWipe  bool                      `json:"remoteWipe"`
		FullResync  bool                      `json:"fullResync"`
		DeniedCount int                       `json:"deniedCount"`
	}
}

type DriveDeviceRevokeInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	DevicePublicID string      `path:"devicePublicId" format:"uuid"`
	Body           struct {
		Reason string `json:"reason,omitempty"`
	}
}

type DriveMobileOfflineOperationBody struct {
	OperationID      string `json:"operationId,omitempty"`
	OperationType    string `json:"operationType"`
	ResourceType     string `json:"resourceType"`
	ResourcePublicID string `json:"resourcePublicId" format:"uuid"`
	BaseRevision     int64  `json:"baseRevision"`
	Name             string `json:"name,omitempty"`
}

type DriveMobileOfflineReplayInput struct {
	Authorization string `header:"Authorization" required:"true"`
	Body          struct {
		Operations []DriveMobileOfflineOperationBody `json:"operations"`
	}
}

type DriveMobileOfflineReplayOutput struct {
	Body struct {
		Applied    int `json:"applied"`
		Denied     int `json:"denied"`
		Conflicted int `json:"conflicted"`
	}
}

func registerDriveSearchEditSyncRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "searchDriveDocuments",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/search/documents",
		Summary:     "Drive content index 付き検索結果を返す",
		Tags:        []string{DocTagDriveFilesFolders},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *SearchDriveItemsInput) (*DriveSearchResultOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		results, err := deps.DriveService.SearchDocuments(ctx, service.DriveSearchInput{
			TenantID:    tenant.ID,
			ActorUserID: current.User.ID,
			Query:       input.Query,
			ContentType: input.ContentType,
			Filter: service.DriveListItemsFilter{
				Type:      input.Type,
				Owner:     input.Owner,
				Source:    input.Source,
				Sort:      input.Sort,
				Direction: input.Direction,
			},
			Limit: input.Limit,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		out := &DriveSearchResultOutput{}
		for _, item := range results {
			matches := make([]DriveSearchResultMatchBody, 0, len(item.Matches))
			for _, match := range item.Matches {
				matches = append(matches, DriveSearchResultMatchBody{
					ResourceKind:           match.ResourceKind,
					ResourcePublicID:       match.ResourcePublicID,
					MedallionAssetPublicID: match.MedallionAssetPublicID,
					Layer:                  match.Layer,
					Snippet:                match.Snippet,
					IndexedAt:              match.IndexedAt,
				})
			}
			out.Body.Items = append(out.Body.Items, DriveSearchResultBody{
				Item:      toDriveItemBody(item.Item),
				Snippet:   item.Snippet,
				IndexedAt: item.IndexedAt,
				Matches:   matches,
			})
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "startDriveEditSession",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/files/{filePublicId}/edit-sessions",
		Summary:     "Drive file edit session を開始する",
		Tags:        []string{DocTagDriveCollaborationSync},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFilePathInput) (*DriveEditSessionOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		session, err := deps.DriveService.StartEditSession(ctx, tenant.ID, current.User.ID, input.FilePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveEditSessionOutput{Body: toDriveEditSessionBody(session)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "heartbeatDriveEditSession",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/files/{filePublicId}/edit-sessions/{sessionPublicId}/heartbeat",
		Summary:     "Drive file edit session lease を延長する",
		Tags:        []string{DocTagDriveCollaborationSync},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveEditSessionPathInput) (*DriveEditSessionOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		session, err := deps.DriveService.HeartbeatEditSession(ctx, tenant.ID, current.User.ID, input.FilePublicID, input.SessionPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveEditSessionOutput{Body: toDriveEditSessionBody(session)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "saveDriveEditSessionContent",
		Method:      http.MethodPut,
		Path:        "/api/v1/drive/files/{filePublicId}/edit-sessions/{sessionPublicId}/content",
		Summary:     "Drive edit session content を保存する",
		Tags:        []string{DocTagDriveCollaborationSync},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveEditSaveInput) (*DriveEditSaveOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		result, err := deps.DriveService.SaveEditSessionContent(ctx, service.DriveSaveEditInput{
			TenantID:         tenant.ID,
			ActorUserID:      current.User.ID,
			FilePublicID:     input.FilePublicID,
			SessionPublicID:  input.SessionPublicID,
			Content:          input.Body.Content,
			ExpectedRevision: input.Body.ExpectedRevision,
			Filename:         input.Body.Filename,
			ContentType:      input.Body.ContentType,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		out := &DriveEditSaveOutput{}
		out.Body.File = toDriveFileBody(result.File)
		out.Body.Revision = result.Revision
		out.Body.Conflict = result.Conflict
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "endDriveEditSession",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/files/{filePublicId}/edit-sessions/{sessionPublicId}",
		Summary:       "Drive edit session を終了する",
		Tags:          []string{DocTagDriveCollaborationSync},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveEditSessionPathInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.EndEditSession(ctx, tenant.ID, current.User.ID, input.FilePublicID, input.SessionPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "registerDriveSyncDevice",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive-sync/devices/register",
		Summary:     "Drive sync device を登録する",
		Tags:        []string{DocTagDriveCollaborationSync},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveRegisterDeviceInput) (*DriveDeviceOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		device, err := deps.DriveService.RegisterSyncDevice(ctx, service.DriveRegisterDeviceInput{
			TenantID:    tenant.ID,
			ActorUserID: current.User.ID,
			DeviceName:  input.Body.DeviceName,
			Platform:    input.Body.Platform,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveDeviceOutput{Body: toDriveDeviceBody(device)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDriveSyncDelta",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive-sync/delta",
		Summary:     "Drive sync delta を返す",
		Tags:        []string{DocTagDriveCollaborationSync},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, input *DriveSyncBearerInput) (*DriveSyncDeltaOutput, error) {
		delta, err := deps.DriveService.SyncDelta(ctx, bearerToken(input.Authorization), input.Cursor, "", "")
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return toDriveSyncDeltaOutput(delta), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "revokeDriveSyncDevice",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive-sync/devices/{devicePublicId}/revoke",
		Summary:     "Drive sync device を revoke する",
		Tags:        []string{DocTagDriveCollaborationSync},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveDeviceRevokeInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.RevokeSyncDevice(ctx, tenant.ID, current.User.ID, input.DevicePublicID, input.Body.Reason, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "replayDriveMobileOfflineOperations",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive-mobile/offline/replay",
		Summary:     "Drive mobile offline operation queue を replay する",
		Tags:        []string{DocTagDriveCollaborationSync},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, input *DriveMobileOfflineReplayInput) (*DriveMobileOfflineReplayOutput, error) {
		ops := make([]service.DriveOfflineOperationInput, 0, len(input.Body.Operations))
		for _, op := range input.Body.Operations {
			ops = append(ops, service.DriveOfflineOperationInput{
				OperationID:      op.OperationID,
				OperationType:    op.OperationType,
				ResourceType:     op.ResourceType,
				ResourcePublicID: op.ResourcePublicID,
				BaseRevision:     op.BaseRevision,
				Name:             op.Name,
			})
		}
		result, err := deps.DriveService.ReplayMobileOfflineOperations(ctx, bearerToken(input.Authorization), ops, service.AuditContext{ActorType: service.AuditActorSystem})
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		out := &DriveMobileOfflineReplayOutput{}
		out.Body.Applied = result.Applied
		out.Body.Denied = result.Denied
		out.Body.Conflicted = result.Conflicted
		return out, nil
	})
}

func bearerToken(header string) string {
	token := strings.TrimSpace(header)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return strings.TrimSpace(token[7:])
	}
	return token
}

func toDriveEditSessionBody(item service.DriveEditSession) DriveEditSessionBody {
	return DriveEditSessionBody{
		PublicID:     item.PublicID,
		FilePublicID: item.FilePublicID,
		Status:       item.Status,
		BaseRevision: item.BaseRevision,
		Provider:     item.Provider,
		ExpiresAt:    item.ExpiresAt,
		CreatedAt:    item.CreatedAt,
	}
}

func toDriveDeviceBody(item service.DriveSyncDevice) DriveDeviceBody {
	return DriveDeviceBody{
		PublicID:           item.PublicID,
		DeviceName:         item.DeviceName,
		Platform:           item.Platform,
		Status:             item.Status,
		RemoteWipeRequired: item.RemoteWipeRequired,
		LastSeenAt:         item.LastSeenAt,
		CreatedAt:          item.CreatedAt,
		Token:              item.RawToken,
	}
}

func toDriveSyncDeltaOutput(delta service.DriveSyncDelta) *DriveSyncDeltaOutput {
	out := &DriveSyncDeltaOutput{}
	out.Body.Cursor = delta.Cursor
	out.Body.RemoteWipe = delta.RemoteWipe
	out.Body.FullResync = delta.FullResync
	out.Body.DeniedCount = delta.DeniedCount
	for _, event := range delta.Events {
		out.Body.Events = append(out.Body.Events, DriveSyncDeltaEventBody{
			ID:               event.ID,
			PublicID:         event.PublicID,
			ResourceType:     event.ResourceType,
			ResourcePublicID: event.ResourcePublicID,
			Action:           event.Action,
			ObjectVersion:    event.ObjectVersion,
			Metadata:         event.Metadata,
			CreatedAt:        event.CreatedAt,
		})
	}
	return out
}
