import { readCookie } from './client'
import {
  approveTenantAdminDriveShareApproval,
  createTenantAdminDriveLocalSearchRebuild,
  createTenantAdminTenant,
  deactivateTenantAdminTenant,
  getTenantAdminDriveOpenFgaDrift,
  getTenantAdminDriveOcrStatus,
  getTenantAdminDriveOperationsHealth,
  getTenantAdminTenant,
  grantTenantAdminRole,
  listTenantAdminDriveAuditEvents,
  listTenantAdminDriveInvitations,
  listTenantAdminDriveLocalSearchIndexJobs,
  listTenantAdminDriveShareApprovals,
  listTenantAdminDriveShareLinks,
  listTenantAdminDriveShares,
  listTenantAdminTenants,
  rejectTenantAdminDriveShareApproval,
  repairTenantAdminDriveOpenFgaSync,
  revokeTenantAdminRole,
  updateTenantAdminTenant,
} from './generated/sdk.gen'
import type {
  DriveShareInvitationBody,
  LocalSearchIndexJobBody,
  TenantAdminDriveAuditEventBody,
  TenantAdminDriveShareLinkStateBody,
  TenantAdminDriveShareStateBody,
  TenantAdminDriveSyncOutputBody,
  TenantAdminDriveOcrStatusBody,
  TenantAdminDriveOperationsHealthBody,
  TenantAdminMembershipRequestBody,
  TenantAdminTenantBody,
  TenantAdminTenantDetailBody,
  TenantAdminTenantRequestBody,
} from './generated/types.gen'

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export async function fetchTenantAdminDriveShares(tenantSlug: string): Promise<TenantAdminDriveShareStateBody[]> {
  const data = await listTenantAdminDriveShares({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items: TenantAdminDriveShareStateBody[] | null }
  return data.items ?? []
}

export async function fetchTenantAdminDriveShareLinks(tenantSlug: string): Promise<TenantAdminDriveShareLinkStateBody[]> {
  const data = await listTenantAdminDriveShareLinks({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items: TenantAdminDriveShareLinkStateBody[] | null }
  return data.items ?? []
}

export async function fetchTenantAdminDriveInvitations(tenantSlug: string): Promise<DriveShareInvitationBody[]> {
  const data = await listTenantAdminDriveInvitations({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items: DriveShareInvitationBody[] | null }
  return data.items ?? []
}

export async function fetchTenantAdminDriveApprovals(tenantSlug: string): Promise<DriveShareInvitationBody[]> {
  const data = await listTenantAdminDriveShareApprovals({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items: DriveShareInvitationBody[] | null }
  return data.items ?? []
}

export async function approveTenantAdminDriveApproval(tenantSlug: string, invitationPublicId: string): Promise<void> {
  await approveTenantAdminDriveShareApproval({
    headers: csrfHeaders(),
    path: { tenantSlug, invitationPublicId },
    responseStyle: 'data',
    throwOnError: true,
  })
}

export async function rejectTenantAdminDriveApproval(tenantSlug: string, invitationPublicId: string): Promise<void> {
  await rejectTenantAdminDriveShareApproval({
    headers: csrfHeaders(),
    path: { tenantSlug, invitationPublicId },
    responseStyle: 'data',
    throwOnError: true,
  })
}

export async function fetchTenantAdminDriveAuditEvents(tenantSlug: string): Promise<TenantAdminDriveAuditEventBody[]> {
  const data = await listTenantAdminDriveAuditEvents({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items: TenantAdminDriveAuditEventBody[] | null }
  return data.items ?? []
}

export async function fetchTenantAdminDriveDrift(tenantSlug: string): Promise<TenantAdminDriveSyncOutputBody> {
  return getTenantAdminDriveOpenFgaDrift({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantAdminDriveSyncOutputBody>
}

export async function fetchTenantAdminDriveOperationsHealth(tenantSlug: string): Promise<TenantAdminDriveOperationsHealthBody> {
  return getTenantAdminDriveOperationsHealth({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantAdminDriveOperationsHealthBody>
}

export async function fetchTenantAdminDriveOCRStatus(tenantSlug: string): Promise<TenantAdminDriveOcrStatusBody> {
  return getTenantAdminDriveOcrStatus({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantAdminDriveOcrStatusBody>
}

export async function fetchTenantAdminDriveLocalSearchJobs(tenantSlug: string): Promise<LocalSearchIndexJobBody[]> {
  const data = await listTenantAdminDriveLocalSearchIndexJobs({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items: LocalSearchIndexJobBody[] | null }
  return data.items ?? []
}

export async function createTenantAdminDriveLocalSearchRebuildJob(tenantSlug: string): Promise<LocalSearchIndexJobBody> {
  return createTenantAdminDriveLocalSearchRebuild({
    headers: csrfHeaders(),
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<LocalSearchIndexJobBody>
}

export async function repairTenantAdminDriveSync(tenantSlug: string): Promise<TenantAdminDriveSyncOutputBody> {
  return repairTenantAdminDriveOpenFgaSync({
    headers: csrfHeaders(),
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantAdminDriveSyncOutputBody>
}

export async function fetchTenantAdminTenants(): Promise<TenantAdminTenantBody[]> {
  const data = await listTenantAdminTenants({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items: TenantAdminTenantBody[] | null }

  return data.items ?? []
}

export async function fetchTenantAdminTenant(tenantSlug: string): Promise<TenantAdminTenantDetailBody> {
  return getTenantAdminTenant({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantAdminTenantDetailBody>
}

export async function createTenantFromForm(
  body: TenantAdminTenantRequestBody,
): Promise<TenantAdminTenantBody> {
  return createTenantAdminTenant({
    headers: csrfHeaders(),
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantAdminTenantBody>
}

export async function updateTenantFromForm(
  tenantSlug: string,
  body: TenantAdminTenantRequestBody,
): Promise<TenantAdminTenantBody> {
  return updateTenantAdminTenant({
    headers: csrfHeaders(),
    path: { tenantSlug },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantAdminTenantBody>
}

export async function deactivateTenant(tenantSlug: string): Promise<void> {
  await deactivateTenantAdminTenant({
    headers: csrfHeaders(),
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  })
}

export async function grantTenantRole(
  tenantSlug: string,
  body: TenantAdminMembershipRequestBody,
): Promise<void> {
  await grantTenantAdminRole({
    headers: csrfHeaders(),
    path: { tenantSlug },
    body,
    responseStyle: 'data',
    throwOnError: true,
  })
}

export async function revokeTenantRole(
  tenantSlug: string,
  userPublicId: string,
  roleCode: string,
): Promise<void> {
  await revokeTenantAdminRole({
    headers: csrfHeaders(),
    path: { tenantSlug, userPublicId, roleCode },
    responseStyle: 'data',
    throwOnError: true,
  })
}
