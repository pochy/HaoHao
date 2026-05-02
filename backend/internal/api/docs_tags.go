package api

import "github.com/danielgtaylor/huma/v2"

const (
	DocTagAuthSession             = "Auth & Session"
	DocTagTenantWorkspace         = "Tenant Workspace"
	DocTagTenantAdministration    = "Tenant Administration"
	DocTagCustomerSignals         = "Customer Signals"
	DocTagDataDatasets            = "Data & Datasets"
	DocTagPlatformIntegrations    = "Platform Integrations"
	DocTagDriveFilesFolders       = "Drive Files & Folders"
	DocTagDriveSharingPermissions = "Drive Sharing & Permissions"
	DocTagDriveCollaborationSync  = "Drive Collaboration & Sync"
	DocTagDriveAIOCR              = "Drive AI & OCR"
	DocTagDriveSecurityCompliance = "Drive Security & Compliance"
	DocTagDriveAdminGovernance    = "Drive Admin & Governance"
	DocTagExternalAPIs            = "External APIs"
)

type docTagDefinition struct {
	name        string
	description string
}

var openAPIDocTagDefinitions = []docTagDefinition{
	{DocTagAuthSession, "Authentication settings, login, logout, session refresh, and CSRF endpoints."},
	{DocTagTenantWorkspace, "Tenant selection, tenant-scoped settings, notifications, invitations, and workspace utilities."},
	{DocTagTenantAdministration, "Tenant lifecycle administration, support access, and admin role management."},
	{DocTagCustomerSignals, "Customer signal records, saved filters, and import jobs."},
	{DocTagDataDatasets, "File metadata, dataset creation, work tables, query jobs, and export workflows."},
	{DocTagPlatformIntegrations, "External integrations, machine clients, entitlements, and webhooks."},
	{DocTagDriveFilesFolders, "Drive file, folder, item listing, search, metadata, and trash operations."},
	{DocTagDriveSharingPermissions, "Drive share links, invitations, groups, public access, and permission views."},
	{DocTagDriveCollaborationSync, "Drive workspaces, collaborative editing, office sessions, sync, and offline operations."},
	{DocTagDriveAIOCR, "Drive OCR, product extraction, AI summary, and AI classification operations."},
	{DocTagDriveSecurityCompliance, "Drive encryption, HSM, legal hold, eDiscovery, clean room, and gateway controls."},
	{DocTagDriveAdminGovernance, "Tenant-admin Drive policies, audit, search rebuild, OCR status, and operations health."},
	{DocTagExternalAPIs, "Bearer, M2M, SCIM, and external Drive integration APIs."},
}

func OpenAPIDocTags(surface Surface) []*huma.Tag {
	definitions := openAPIDocTagDefinitionsForSurface(surface)
	tags := make([]*huma.Tag, 0, len(definitions))
	for _, definition := range definitions {
		tags = append(tags, &huma.Tag{Name: definition.name, Description: definition.description})
	}
	return tags
}

func OpenAPIDocTagNames(surface Surface) []string {
	definitions := openAPIDocTagDefinitionsForSurface(surface)
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.name)
	}
	return names
}

func openAPIDocTagDefinitionsForSurface(surface Surface) []docTagDefinition {
	definitions := make([]docTagDefinition, 0, len(openAPIDocTagDefinitions))
	for _, definition := range openAPIDocTagDefinitions {
		if openAPIDocTagIncludedInSurface(surface, definition.name) {
			definitions = append(definitions, definition)
		}
	}
	return definitions
}

func openAPIDocTagIncludedInSurface(surface Surface, name string) bool {
	switch surface {
	case SurfaceFull:
		return true
	case SurfaceBrowser:
		return name != DocTagExternalAPIs
	case SurfaceExternal:
		return name == DocTagExternalAPIs
	default:
		return false
	}
}
