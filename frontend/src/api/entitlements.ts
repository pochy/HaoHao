import { readCookie } from './client'
import { listTenantEntitlements, updateTenantEntitlements } from './generated/sdk.gen'
import type { EntitlementBody, EntitlementUpdateBody } from './generated/types.gen'

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export async function fetchTenantEntitlements(tenantSlug: string): Promise<EntitlementBody[]> {
  const data = await listTenantEntitlements({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: EntitlementBody[] | null }

  return data.items ?? []
}

export async function updateTenantEntitlementItems(
  tenantSlug: string,
  items: EntitlementUpdateBody[],
): Promise<EntitlementBody[]> {
  const data = await updateTenantEntitlements({
    headers: csrfHeaders(),
    path: { tenantSlug },
    body: { items },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: EntitlementBody[] | null }

  return data.items ?? []
}
