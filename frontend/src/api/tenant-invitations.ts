import { apiErrorFromResponse, readCookie } from './client'
import {
  acceptTenantInvitation,
  createTenantInvitation,
  listTenantInvitations,
  provisionTenantInvitationIdentity,
  revokeTenantInvitation,
  setupTenantInvitationIdentity,
} from './generated/sdk.gen'
import type {
  CreateTenantInvitationRequestBodyWritable,
  TenantInvitationBody,
} from './generated/types.gen'

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export async function fetchTenantInvitations(tenantSlug: string): Promise<TenantInvitationBody[]> {
  const data = await listTenantInvitations({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: TenantInvitationBody[] | null }

  return data.items ?? []
}

export async function createTenantInvitationItem(
  tenantSlug: string,
  body: CreateTenantInvitationRequestBodyWritable,
): Promise<TenantInvitationBody> {
  return createTenantInvitation({
    headers: csrfHeaders(),
    path: { tenantSlug },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantInvitationBody>
}

export async function revokeTenantInvitationItem(
  tenantSlug: string,
  invitationPublicId: string,
): Promise<void> {
  await revokeTenantInvitation({
    headers: csrfHeaders(),
    path: { tenantSlug, invitationPublicId },
    responseStyle: 'data',
    throwOnError: true,
  })
}

export async function provisionTenantInvitationIdentityItem(
  tenantSlug: string,
  invitationPublicId: string,
): Promise<TenantInvitationBody> {
  return provisionTenantInvitationIdentity({
    headers: csrfHeaders(),
    path: { tenantSlug, invitationPublicId },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantInvitationBody>
}

export async function resolveTenantInvitationItem(token: string): Promise<TenantInvitationBody> {
  const response = await fetch(`/api/v1/invitations/resolve?token=${encodeURIComponent(token)}`, {
    credentials: 'include',
  })
  if (!response.ok) {
    throw await apiErrorFromResponse(response, 'Invitation could not be loaded')
  }
  return response.json() as Promise<TenantInvitationBody>
}

export async function setupTenantInvitationIdentityItem(
  invitationPublicId: string,
  token: string,
): Promise<TenantInvitationBody> {
  return setupTenantInvitationIdentity({
    path: { invitationPublicId },
    body: { token },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantInvitationBody>
}

export async function acceptTenantInvitationItem(token: string): Promise<TenantInvitationBody> {
  return acceptTenantInvitation({
    headers: csrfHeaders(),
    body: { token },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantInvitationBody>
}
