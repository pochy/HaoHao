import { getSession } from '@/api/generated/sdk.gen'
import type { SessionBody } from '@/api/generated/types.gen'

export async function fetchSession(): Promise<SessionBody> {
  const response = await getSession()

  if (response.error) {
    throw new Error(response.error.detail ?? 'failed to load session')
  }

  if (!response.data) {
    throw new Error('session response is empty')
  }

  return response.data
}

