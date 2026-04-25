import { readCookie } from './client'
import { listTenants, selectTenant } from './generated/sdk.gen'
import type { ListTenantsBody, TenantBody } from './generated/types.gen'

export async function fetchTenants(): Promise<ListTenantsBody> {
  const data = await listTenants({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as ListTenantsBody

  return {
    ...data,
    items: data.items ?? [],
  }
}

export async function switchActiveTenant(tenantSlug: string): Promise<TenantBody> {
  const data = await selectTenant({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    body: {
      tenantSlug,
    },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { activeTenant: TenantBody }

  return data.activeTenant
}
