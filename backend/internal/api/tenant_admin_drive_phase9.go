package api

import (
	"context"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DriveEDiscoveryConnectionBody struct {
	PublicID  string    `json:"publicId"`
	Provider  string    `json:"provider"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt" format:"date-time"`
}

type DriveEDiscoveryConnectionOutput struct {
	Body DriveEDiscoveryConnectionBody
}

type DriveEDiscoveryConnectionInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          struct {
		Provider string `json:"provider,omitempty"`
	}
}

type DriveEDiscoveryExportBody struct {
	PublicID         string    `json:"publicId"`
	CasePublicID     string    `json:"casePublicId"`
	Status           string    `json:"status"`
	ManifestHash     string    `json:"manifestHash,omitempty"`
	ProviderExportID string    `json:"providerExportId,omitempty"`
	ItemCount        int       `json:"itemCount"`
	CreatedAt        time.Time `json:"createdAt" format:"date-time"`
}

type DriveEDiscoveryExportOutput struct {
	Body DriveEDiscoveryExportBody
}

type DriveEDiscoveryExportInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          struct {
		ConnectionPublicID string `json:"connectionPublicId" format:"uuid"`
		CasePublicID       string `json:"casePublicId" format:"uuid"`
	}
}

type DriveEDiscoveryExportApproveInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug     string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	ExportPublicID string      `path:"exportPublicId" format:"uuid"`
}

type DriveHSMDeploymentBody struct {
	PublicID        string    `json:"publicId"`
	Provider        string    `json:"provider"`
	EndpointURL     string    `json:"endpointUrl"`
	Status          string    `json:"status"`
	HealthStatus    string    `json:"healthStatus"`
	AttestationHash string    `json:"attestationHash,omitempty"`
	KeyPublicID     string    `json:"keyPublicId"`
	KeyStatus       string    `json:"keyStatus"`
	CreatedAt       time.Time `json:"createdAt" format:"date-time"`
}

type DriveHSMDeploymentOutput struct {
	Body DriveHSMDeploymentBody
}

type DriveHSMDeploymentInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          struct {
		Provider    string `json:"provider,omitempty"`
		EndpointURL string `json:"endpointUrl,omitempty"`
	}
}

type DriveHSMBindInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          struct {
		FilePublicID string `json:"filePublicId" format:"uuid"`
		KeyPublicID  string `json:"keyPublicId" format:"uuid"`
	}
}

type DriveHSMKeyStatusInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	KeyPublicID   string      `path:"keyPublicId" format:"uuid"`
	Body          struct {
		Status string `json:"status" enum:"active,disabled,destroyed,unavailable"`
	}
}

type DriveGatewayBody struct {
	PublicID               string     `json:"publicId"`
	Name                   string     `json:"name"`
	Status                 string     `json:"status"`
	EndpointURL            string     `json:"endpointUrl"`
	CertificateFingerprint string     `json:"certificateFingerprint"`
	LastSeenAt             *time.Time `json:"lastSeenAt,omitempty" format:"date-time"`
	CreatedAt              time.Time  `json:"createdAt" format:"date-time"`
}

type DriveGatewayOutput struct {
	Body DriveGatewayBody
}

type DriveGatewayInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          struct {
		Name                   string `json:"name,omitempty"`
		EndpointURL            string `json:"endpointUrl,omitempty"`
		CertificateFingerprint string `json:"certificateFingerprint,omitempty"`
	}
}

type DriveGatewayBindInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	CSRFToken       string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug      string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	GatewayPublicID string      `path:"gatewayPublicId" format:"uuid"`
	Body            struct {
		FilePublicID string `json:"filePublicId" format:"uuid"`
	}
}

type DriveGatewayObjectBody struct {
	GatewayPublicID string `json:"gatewayPublicId"`
	FilePublicID    string `json:"filePublicId"`
	ManifestHash    string `json:"manifestHash"`
	Status          string `json:"status"`
}

type DriveGatewayObjectOutput struct {
	Body DriveGatewayObjectBody
}

type DriveGatewayStatusInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	CSRFToken       string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug      string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	GatewayPublicID string      `path:"gatewayPublicId" format:"uuid"`
	Body            struct {
		Status string `json:"status" enum:"active,disabled,disconnected"`
	}
}

type DriveMarketplaceInstallBody struct {
	PublicID  string    `json:"publicId"`
	AppSlug   string    `json:"appSlug"`
	AppName   string    `json:"appName"`
	Status    string    `json:"status"`
	Scopes    []string  `json:"scopes"`
	CreatedAt time.Time `json:"createdAt" format:"date-time"`
}

type DriveMarketplaceInstallOutput struct {
	Body DriveMarketplaceInstallBody
}

type DriveMarketplaceInstallInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          struct {
		AppSlug string `json:"appSlug"`
	}
}

type DriveMarketplaceApproveInput struct {
	SessionCookie        http.Cookie `cookie:"SESSION_ID"`
	CSRFToken            string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug           string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	InstallationPublicID string      `path:"installationPublicId" format:"uuid"`
}

type DriveMarketplaceScopeInput struct {
	SessionCookie        http.Cookie `cookie:"SESSION_ID"`
	TenantSlug           string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	InstallationPublicID string      `path:"installationPublicId" format:"uuid"`
	Scope                string      `query:"scope"`
	FilePublicID         string      `query:"filePublicId" format:"uuid"`
}

type DriveMarketplaceScopeOutput struct {
	Body struct {
		Allowed bool `json:"allowed"`
	}
}

func registerTenantAdminDrivePhase9Routes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{OperationID: "createDriveEDiscoveryConnection", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/ediscovery/connections", Tags: []string{"drive-ediscovery"}, Summary: "Drive eDiscovery provider connection を作成する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveEDiscoveryConnectionInput) (*DriveEDiscoveryConnectionOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.CreateEDiscoveryConnection(ctx, tenant.ID, current.User.ID, input.Body.Provider, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveEDiscoveryConnectionOutput{Body: DriveEDiscoveryConnectionBody{PublicID: item.PublicID, Provider: item.Provider, Status: item.Status, CreatedAt: item.CreatedAt}}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "requestDriveEDiscoveryExport", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/ediscovery/exports", Tags: []string{"drive-ediscovery"}, Summary: "Drive eDiscovery export を request する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveEDiscoveryExportInput) (*DriveEDiscoveryExportOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.RequestEDiscoveryExport(ctx, tenant.ID, current.User.ID, input.Body.ConnectionPublicID, input.Body.CasePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveEDiscoveryExportOutput{Body: toDriveEDiscoveryExportBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "approveDriveEDiscoveryExport", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/ediscovery/exports/{exportPublicId}/approve", Tags: []string{"drive-ediscovery"}, Summary: "Drive eDiscovery export を approve する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveEDiscoveryExportApproveInput) (*DriveEDiscoveryExportOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.ApproveEDiscoveryExport(ctx, tenant.ID, current.User.ID, input.ExportPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveEDiscoveryExportOutput{Body: toDriveEDiscoveryExportBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createDriveHSMDeployment", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/hsm/deployments", Tags: []string{"drive-hsm"}, Summary: "Drive HSM deployment を作成する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveHSMDeploymentInput) (*DriveHSMDeploymentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.CreateHSMDeployment(ctx, tenant.ID, current.User.ID, input.Body.Provider, input.Body.EndpointURL, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveHSMDeploymentOutput{Body: toDriveHSMDeploymentBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "bindDriveHSMKey", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/hsm/bindings", Tags: []string{"drive-hsm"}, Summary: "Drive HSM key を file に bind する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveHSMBindInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.BindHSMKeyToFile(ctx, tenant.ID, current.User.ID, input.Body.FilePublicID, input.Body.KeyPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "updateDriveHSMKeyStatus", Method: http.MethodPatch, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/hsm/keys/{keyPublicId}", Tags: []string{"drive-hsm"}, Summary: "Drive HSM key status を更新する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveHSMKeyStatusInput) (*DriveHSMDeploymentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.SetHSMKeyStatus(ctx, tenant.ID, current.User.ID, input.KeyPublicID, input.Body.Status, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveHSMDeploymentOutput{Body: toDriveHSMDeploymentBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "registerDriveGateway", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/gateways", Tags: []string{"drive-gateway"}, Summary: "Drive on-prem gateway を登録する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveGatewayInput) (*DriveGatewayOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.RegisterGateway(ctx, tenant.ID, current.User.ID, input.Body.Name, input.Body.EndpointURL, input.Body.CertificateFingerprint, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveGatewayOutput{Body: toDriveGatewayBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "bindDriveGatewayFile", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/gateways/{gatewayPublicId}/objects", Tags: []string{"drive-gateway"}, Summary: "Drive file を gateway object として bind する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveGatewayBindInput) (*DriveGatewayObjectOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.BindGatewayFile(ctx, tenant.ID, current.User.ID, input.GatewayPublicID, input.Body.FilePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveGatewayObjectOutput{Body: DriveGatewayObjectBody{GatewayPublicID: item.GatewayPublicID, FilePublicID: item.FilePublicID, ManifestHash: item.ManifestHash, Status: item.Status}}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "updateDriveGatewayStatus", Method: http.MethodPatch, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/gateways/{gatewayPublicId}", Tags: []string{"drive-gateway"}, Summary: "Drive gateway status を更新する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveGatewayStatusInput) (*DriveGatewayOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.SetGatewayStatus(ctx, tenant.ID, current.User.ID, input.GatewayPublicID, input.Body.Status, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveGatewayOutput{Body: toDriveGatewayBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "installDriveMarketplaceApp", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/marketplace/installations", Tags: []string{"drive-marketplace"}, Summary: "Drive marketplace app install を request する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveMarketplaceInstallInput) (*DriveMarketplaceInstallOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.InstallMarketplaceApp(ctx, tenant.ID, current.User.ID, input.Body.AppSlug, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveMarketplaceInstallOutput{Body: toDriveMarketplaceInstallBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "approveDriveMarketplaceInstallation", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/marketplace/installations/{installationPublicId}/approve", Tags: []string{"drive-marketplace"}, Summary: "Drive marketplace app install を approve する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveMarketplaceApproveInput) (*DriveMarketplaceInstallOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.ApproveMarketplaceInstallation(ctx, tenant.ID, current.User.ID, input.InstallationPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveMarketplaceInstallOutput{Body: toDriveMarketplaceInstallBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "uninstallDriveMarketplaceInstallation", Method: http.MethodDelete, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/marketplace/installations/{installationPublicId}", Tags: []string{"drive-marketplace"}, Summary: "Drive marketplace app install を uninstall する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveMarketplaceApproveInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.UninstallMarketplaceInstallation(ctx, tenant.ID, current.User.ID, input.InstallationPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "checkDriveMarketplaceScope", Method: http.MethodGet, Path: "/api/v1/admin/tenants/{tenantSlug}/drive/marketplace/installations/{installationPublicId}/scope-check", Tags: []string{"drive-marketplace"}, Summary: "Drive marketplace scope と OpenFGA permission を確認する", Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DriveMarketplaceScopeInput) (*DriveMarketplaceScopeOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.CheckMarketplaceScope(ctx, tenant.ID, current.User.ID, input.InstallationPublicID, input.Scope, input.FilePublicID); err != nil {
			return nil, toDriveHTTPError(err)
		}
		out := &DriveMarketplaceScopeOutput{}
		out.Body.Allowed = true
		return out, nil
	})
}

func toDriveEDiscoveryExportBody(item service.DriveEDiscoveryExport) DriveEDiscoveryExportBody {
	return DriveEDiscoveryExportBody{PublicID: item.PublicID, CasePublicID: item.CasePublicID, Status: item.Status, ManifestHash: item.ManifestHash, ProviderExportID: item.ProviderExportID, ItemCount: item.ItemCount, CreatedAt: item.CreatedAt}
}

func toDriveHSMDeploymentBody(item service.DriveHSMDeployment) DriveHSMDeploymentBody {
	return DriveHSMDeploymentBody{PublicID: item.PublicID, Provider: item.Provider, EndpointURL: item.EndpointURL, Status: item.Status, HealthStatus: item.HealthStatus, AttestationHash: item.AttestationHash, KeyPublicID: item.KeyPublicID, KeyStatus: item.KeyStatus, CreatedAt: item.CreatedAt}
}

func toDriveGatewayBody(item service.DriveGateway) DriveGatewayBody {
	return DriveGatewayBody{PublicID: item.PublicID, Name: item.Name, Status: item.Status, EndpointURL: item.EndpointURL, CertificateFingerprint: item.CertificateFingerprint, LastSeenAt: item.LastSeenAt, CreatedAt: item.CreatedAt}
}

func toDriveMarketplaceInstallBody(item service.DriveMarketplaceInstallation) DriveMarketplaceInstallBody {
	return DriveMarketplaceInstallBody{PublicID: item.PublicID, AppSlug: item.AppSlug, AppName: item.AppName, Status: item.Status, Scopes: item.Scopes, CreatedAt: item.CreatedAt}
}
