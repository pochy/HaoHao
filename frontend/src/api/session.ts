import type { SessionBody } from './generated/types.gen'
import { readCookie } from './client'
import { getSession, login, logout } from './generated/sdk.gen'

export async function fetchCurrentSession(): Promise<SessionBody> {
  return getSession({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<SessionBody>
}

export async function loginWithPassword(email: string, password: string): Promise<SessionBody> {
  return login({
    body: {
      email,
      password,
    },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<SessionBody>
}

export async function logoutCurrentSession(): Promise<void> {
  await logout({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    responseStyle: 'data',
    throwOnError: true,
  })
}
