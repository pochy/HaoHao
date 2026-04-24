import {
  deleteIntegrationGrant,
  listIntegrations,
  verifyIntegrationAccess,
} from './generated/sdk.gen'
import { readCookie } from './client'
import type { IntegrationStatusBody, VerifyIntegrationBody } from './generated/types.gen'

export async function fetchIntegrations(): Promise<IntegrationStatusBody[]> {
  const data = await listIntegrations({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items: IntegrationStatusBody[] | null }
  return data.items ?? []
}

export function startIntegrationConnect(resourceServer: string) {
  window.location.assign(`/api/v1/integrations/${encodeURIComponent(resourceServer)}/connect`)
}

export async function verifyIntegration(resourceServer: string): Promise<VerifyIntegrationBody> {
  return verifyIntegrationAccess({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    path: {
      resourceServer,
    },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<VerifyIntegrationBody>
}

export async function revokeIntegrationGrant(resourceServer: string): Promise<void> {
  await deleteIntegrationGrant({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    path: {
      resourceServer,
    },
    responseStyle: 'data',
    throwOnError: true,
  })
}
