import { readCookie } from './client'
import { getTenantSettings, updateTenantSettings } from './generated/sdk.gen'
import type { TenantSettingsBody, TenantSettingsRequestBodyWritable } from './generated/types.gen'

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export async function fetchTenantSettings(tenantSlug: string): Promise<TenantSettingsBody> {
  return getTenantSettings({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantSettingsBody>
}

export async function updateTenantSettingsItem(
  tenantSlug: string,
  body: TenantSettingsRequestBodyWritable,
): Promise<TenantSettingsBody> {
  return updateTenantSettings({
    headers: csrfHeaders(),
    path: { tenantSlug },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantSettingsBody>
}
