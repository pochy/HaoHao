package api

import (
	"context"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DriveOfficeSessionBody struct {
	PublicID          string    `json:"publicId"`
	FilePublicID      string    `json:"filePublicId"`
	Provider          string    `json:"provider"`
	ProviderSessionID string    `json:"providerSessionId"`
	AccessLevel       string    `json:"accessLevel"`
	LaunchURL         string    `json:"launchUrl"`
	ExpiresAt         time.Time `json:"expiresAt" format:"date-time"`
	CreatedAt         time.Time `json:"createdAt" format:"date-time"`
}

type DriveOfficeSessionOutput struct {
	Body DriveOfficeSessionBody
}

type DriveOfficeSessionCreateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId" format:"uuid"`
	Body          struct {
		AccessLevel string `json:"accessLevel,omitempty" enum:"view,edit"`
	}
}

type DriveOfficeSessionRevokeInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	CSRFToken       string      `header:"X-CSRF-Token" required:"true"`
	SessionPublicID string      `path:"sessionPublicId" format:"uuid"`
}

type DriveOfficeWebhookInput struct {
	Provider string `path:"provider"`
	Body     struct {
		ProviderEventID string `json:"providerEventId"`
		ProviderFileID  string `json:"providerFileId"`
		Revision        string `json:"revision"`
		Checksum        string `json:"checksum,omitempty"`
	}
}

type DriveOfficeWebhookOutput struct {
	Body struct {
		ProviderEventID string `json:"providerEventId"`
		ProviderFileID  string `json:"providerFileId"`
		Revision        string `json:"revision"`
		Result          string `json:"result"`
	}
}

type DriveE2EEUserKeyBody struct {
	PublicID     string    `json:"publicId"`
	UserPublicID string    `json:"userPublicId"`
	Algorithm    string    `json:"algorithm"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt" format:"date-time"`
}

type DriveE2EEUserKeyOutput struct {
	Body DriveE2EEUserKeyBody
}

type DriveE2EEUserKeyCreateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          struct {
		Algorithm string         `json:"algorithm,omitempty"`
		PublicKey map[string]any `json:"publicKey,omitempty"`
	}
}

type DriveE2EEFileKeyBody struct {
	PublicID         string    `json:"publicId"`
	FilePublicID     string    `json:"filePublicId"`
	KeyVersion       int       `json:"keyVersion"`
	Algorithm        string    `json:"algorithm"`
	CiphertextSHA256 string    `json:"ciphertextSha256"`
	CreatedAt        time.Time `json:"createdAt" format:"date-time"`
}

type DriveE2EEFileKeyOutput struct {
	Body DriveE2EEFileKeyBody
}

type DriveE2EEFileKeyCreateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId" format:"uuid"`
	Body          struct {
		Algorithm        string         `json:"algorithm,omitempty"`
		CiphertextSHA256 string         `json:"ciphertextSha256,omitempty"`
		WrappedFileKey   string         `json:"wrappedFileKey"`
		WrapAlgorithm    string         `json:"wrapAlgorithm,omitempty"`
		Metadata         map[string]any `json:"metadata,omitempty"`
	}
}

type DriveE2EEEnvelopeBody struct {
	FileKeyPublicID string    `json:"fileKeyPublicId"`
	RecipientUserID string    `json:"recipientUserId"`
	WrappedFileKey  string    `json:"wrappedFileKey"`
	WrapAlgorithm   string    `json:"wrapAlgorithm"`
	CreatedAt       time.Time `json:"createdAt" format:"date-time"`
}

type DriveE2EEEnvelopeOutput struct {
	Body DriveE2EEEnvelopeBody
}

type DriveE2EEEnvelopeCreateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId" format:"uuid"`
	Body          struct {
		RecipientUserPublicID string `json:"recipientUserPublicId" format:"uuid"`
		WrappedFileKey        string `json:"wrappedFileKey"`
		WrapAlgorithm         string `json:"wrapAlgorithm,omitempty"`
	}
}

type DriveE2EEEnvelopeGetInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	FilePublicID  string      `path:"filePublicId" format:"uuid"`
}

type DriveE2EEEnvelopeRevokeInput struct {
	SessionCookie         http.Cookie `cookie:"SESSION_ID"`
	CSRFToken             string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID          string      `path:"filePublicId" format:"uuid"`
	RecipientUserPublicID string      `path:"recipientUserPublicId" format:"uuid"`
}

type DriveAIJobBody struct {
	PublicID     string    `json:"publicId"`
	FilePublicID string    `json:"filePublicId"`
	JobType      string    `json:"jobType"`
	Provider     string    `json:"provider"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt" format:"date-time"`
}

type DriveAIJobOutput struct {
	Body DriveAIJobBody
}

type DriveAIJobCreateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId" format:"uuid"`
	Body          struct {
		JobType string `json:"jobType" enum:"summary,classification"`
	}
}

type DriveAISummaryBody struct {
	PublicID     string    `json:"publicId"`
	FilePublicID string    `json:"filePublicId"`
	SummaryText  string    `json:"summaryText"`
	Provider     string    `json:"provider"`
	CreatedAt    time.Time `json:"createdAt" format:"date-time"`
}

type DriveAISummaryOutput struct {
	Body DriveAISummaryBody
}

type DriveAIClassificationBody struct {
	Label      string    `json:"label"`
	Confidence float64   `json:"confidence"`
	Provider   string    `json:"provider"`
	CreatedAt  time.Time `json:"createdAt" format:"date-time"`
}

type DriveAIClassificationsOutput struct {
	Body struct {
		Items []DriveAIClassificationBody `json:"items"`
	}
}

type DriveAIFileInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	FilePublicID  string      `path:"filePublicId" format:"uuid"`
}

type DriveMarketplaceAppBody struct {
	PublicID      string   `json:"publicId"`
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	PublisherName string   `json:"publisherName"`
	Version       string   `json:"version"`
	Scopes        []string `json:"scopes"`
}

type DriveMarketplaceAppsOutput struct {
	Body struct {
		Items []DriveMarketplaceAppBody `json:"items"`
	}
}

type DriveMarketplaceAppsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

func registerDrivePhase9Routes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{OperationID: "createDriveOfficeSession", Method: http.MethodPost, Path: "/api/v1/drive/files/{filePublicId}/office/sessions", Tags: []string{"drive-office"}, Summary: "Drive Office edit session を作成する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveOfficeSessionCreateInput) (*DriveOfficeSessionOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.CreateOfficeSession(ctx, tenant.ID, current.User.ID, input.FilePublicID, input.Body.AccessLevel, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveOfficeSessionOutput{Body: toDriveOfficeSessionBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "revokeDriveOfficeSession", Method: http.MethodDelete, Path: "/api/v1/drive/office/sessions/{sessionPublicId}", Tags: []string{"drive-office"}, Summary: "Drive Office edit session を revoke する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveOfficeSessionRevokeInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.RevokeOfficeSession(ctx, tenant.ID, current.User.ID, input.SessionPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "acceptDriveOfficeWebhook", Method: http.MethodPost, Path: "/api/office/webhooks/{provider}", Tags: []string{"drive-office"}, Summary: "Drive Office provider webhook を受け取る"}, func(ctx context.Context, input *DriveOfficeWebhookInput) (*DriveOfficeWebhookOutput, error) {
		item, err := deps.DriveService.AcceptOfficeWebhook(ctx, service.DriveOfficeWebhookInput{
			Provider:        input.Provider,
			ProviderEventID: input.Body.ProviderEventID,
			ProviderFileID:  input.Body.ProviderFileID,
			Revision:        input.Body.Revision,
			Checksum:        input.Body.Checksum,
		})
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		out := &DriveOfficeWebhookOutput{}
		out.Body.ProviderEventID = item.ProviderEventID
		out.Body.ProviderFileID = item.ProviderFileID
		out.Body.Revision = item.Revision
		out.Body.Result = item.Result
		return out, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createDriveE2EEUserKey", Method: http.MethodPost, Path: "/api/v1/drive/e2ee/user-keys", Tags: []string{"drive-e2ee"}, Summary: "Drive E2EE public key を登録する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveE2EEUserKeyCreateInput) (*DriveE2EEUserKeyOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.CreateE2EEUserKey(ctx, tenant.ID, current.User.ID, input.Body.Algorithm, input.Body.PublicKey, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveE2EEUserKeyOutput{Body: DriveE2EEUserKeyBody{PublicID: item.PublicID, UserPublicID: item.UserPublicID, Algorithm: item.Algorithm, Status: item.Status, CreatedAt: item.CreatedAt}}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createDriveE2EEFileKey", Method: http.MethodPost, Path: "/api/v1/drive/files/{filePublicId}/e2ee/keys", Tags: []string{"drive-e2ee"}, Summary: "Drive E2EE file key metadata を作成する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveE2EEFileKeyCreateInput) (*DriveE2EEFileKeyOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.CreateE2EEFileKey(ctx, tenant.ID, current.User.ID, input.FilePublicID, input.Body.Algorithm, input.Body.CiphertextSHA256, input.Body.WrappedFileKey, input.Body.WrapAlgorithm, input.Body.Metadata, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveE2EEFileKeyOutput{Body: DriveE2EEFileKeyBody{PublicID: item.PublicID, FilePublicID: item.FilePublicID, KeyVersion: item.KeyVersion, Algorithm: item.Algorithm, CiphertextSHA256: item.CiphertextSHA256, CreatedAt: item.CreatedAt}}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createDriveE2EEEnvelope", Method: http.MethodPost, Path: "/api/v1/drive/files/{filePublicId}/e2ee/envelopes", Tags: []string{"drive-e2ee"}, Summary: "Drive E2EE recipient envelope を作成する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveE2EEEnvelopeCreateInput) (*DriveE2EEEnvelopeOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.CreateE2EERecipientEnvelope(ctx, tenant.ID, current.User.ID, input.FilePublicID, input.Body.RecipientUserPublicID, input.Body.WrappedFileKey, input.Body.WrapAlgorithm, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveE2EEEnvelopeOutput{Body: toDriveE2EEEnvelopeBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "getDriveE2EEEnvelope", Method: http.MethodGet, Path: "/api/v1/drive/files/{filePublicId}/e2ee/envelope", Tags: []string{"drive-e2ee"}, Summary: "Drive E2EE actor envelope を返す", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveE2EEEnvelopeGetInput) (*DriveE2EEEnvelopeOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.GetE2EEEnvelope(ctx, tenant.ID, current.User.ID, input.FilePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveE2EEEnvelopeOutput{Body: toDriveE2EEEnvelopeBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "revokeDriveE2EEEnvelope", Method: http.MethodDelete, Path: "/api/v1/drive/files/{filePublicId}/e2ee/envelopes/{recipientUserPublicId}", Tags: []string{"drive-e2ee"}, Summary: "Drive E2EE recipient envelope を revoke する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveE2EEEnvelopeRevokeInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.RevokeE2EEEnvelope(ctx, tenant.ID, current.User.ID, input.FilePublicID, input.RecipientUserPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createDriveAIJob", Method: http.MethodPost, Path: "/api/v1/drive/files/{filePublicId}/ai/jobs", Tags: []string{"drive-ai"}, Summary: "Drive AI job を作成する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveAIJobCreateInput) (*DriveAIJobOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.CreateAIJob(ctx, tenant.ID, current.User.ID, input.FilePublicID, input.Body.JobType, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveAIJobOutput{Body: DriveAIJobBody{PublicID: item.PublicID, FilePublicID: item.FilePublicID, JobType: item.JobType, Provider: item.Provider, Status: item.Status, CreatedAt: item.CreatedAt}}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "getDriveAISummary", Method: http.MethodGet, Path: "/api/v1/drive/files/{filePublicId}/ai/summary", Tags: []string{"drive-ai"}, Summary: "Drive AI summary を返す", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveAIFileInput) (*DriveAISummaryOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.GetAISummary(ctx, tenant.ID, current.User.ID, input.FilePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveAISummaryOutput{Body: DriveAISummaryBody{PublicID: item.PublicID, FilePublicID: item.FilePublicID, SummaryText: item.SummaryText, Provider: item.Provider, CreatedAt: item.CreatedAt}}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "listDriveAIClassifications", Method: http.MethodGet, Path: "/api/v1/drive/files/{filePublicId}/ai/classifications", Tags: []string{"drive-ai"}, Summary: "Drive AI classifications を返す", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveAIFileInput) (*DriveAIClassificationsOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListAIClassifications(ctx, tenant.ID, current.User.ID, input.FilePublicID)
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		out := &DriveAIClassificationsOutput{}
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, DriveAIClassificationBody{Label: item.Label, Confidence: item.Confidence, Provider: item.Provider, CreatedAt: item.CreatedAt})
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{OperationID: "listDriveMarketplaceApps", Method: http.MethodGet, Path: "/api/v1/drive/marketplace/apps", Tags: []string{"drive-marketplace"}, Summary: "Drive marketplace catalog を返す", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveMarketplaceAppsInput) (*DriveMarketplaceAppsOutput, error) {
		if _, _, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, ""); err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListMarketplaceApps(ctx)
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		out := &DriveMarketplaceAppsOutput{}
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, DriveMarketplaceAppBody{PublicID: item.PublicID, Slug: item.Slug, Name: item.Name, PublisherName: item.PublisherName, Version: item.Version, Scopes: item.Scopes})
		}
		return out, nil
	})
}

func toDriveOfficeSessionBody(item service.DriveOfficeSession) DriveOfficeSessionBody {
	return DriveOfficeSessionBody{PublicID: item.PublicID, FilePublicID: item.FilePublicID, Provider: item.Provider, ProviderSessionID: item.ProviderSessionID, AccessLevel: item.AccessLevel, LaunchURL: item.LaunchURL, ExpiresAt: item.ExpiresAt, CreatedAt: item.CreatedAt}
}

func toDriveE2EEEnvelopeBody(item service.DriveE2EEEnvelope) DriveE2EEEnvelopeBody {
	return DriveE2EEEnvelopeBody{FileKeyPublicID: item.FileKeyPublicID, RecipientUserID: item.RecipientUserID, WrappedFileKey: item.WrappedFileKey, WrapAlgorithm: item.WrapAlgorithm, CreatedAt: item.CreatedAt}
}
