import { readCookie } from './client'
import {
  createTenantAdminTenant,
  deactivateTenantAdminTenant,
  getTenantAdminTenant,
  grantTenantAdminRole,
  listTenantAdminTenants,
  revokeTenantAdminRole,
  updateTenantAdminTenant,
} from './generated/sdk.gen'
import type {
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
