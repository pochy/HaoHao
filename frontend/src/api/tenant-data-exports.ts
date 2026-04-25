import { readCookie } from './client'
import { createTenantDataExport, listTenantDataExports } from './generated/sdk.gen'
import type { TenantDataExportBody } from './generated/types.gen'

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export async function fetchTenantDataExports(tenantSlug: string): Promise<TenantDataExportBody[]> {
  const data = await listTenantDataExports({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: TenantDataExportBody[] | null }

  return data.items ?? []
}

export async function createTenantDataExportItem(tenantSlug: string, format: 'json' | 'csv' = 'json'): Promise<TenantDataExportBody> {
  return createTenantDataExport({
    headers: csrfHeaders(),
    path: { tenantSlug },
    body: { format },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantDataExportBody>
}
