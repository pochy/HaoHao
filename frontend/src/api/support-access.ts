import { readCookie } from './client'
import { endSupportAccess, getCurrentSupportAccess, startSupportAccess } from './generated/sdk.gen'
import type { StartSupportAccessBodyWritable, SupportAccessOutputBody } from './generated/types.gen'

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export async function fetchCurrentSupportAccess(): Promise<SupportAccessOutputBody> {
  return getCurrentSupportAccess({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<SupportAccessOutputBody>
}

export async function startSupportAccessSession(
  body: StartSupportAccessBodyWritable,
): Promise<SupportAccessOutputBody> {
  return startSupportAccess({
    headers: csrfHeaders(),
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<SupportAccessOutputBody>
}

export async function endSupportAccessSession(): Promise<void> {
  await endSupportAccess({
    headers: csrfHeaders(),
    responseStyle: 'data',
    throwOnError: true,
  })
}
