import { getHealth } from '@/api/generated/sdk.gen'
import type { HealthBody } from '@/api/generated/types.gen'

export async function fetchHealth(): Promise<HealthBody> {
  const response = await getHealth()

  if (response.error) {
    throw new Error(response.error.detail ?? 'failed to load health')
  }

  if (!response.data) {
    throw new Error('health response is empty')
  }

  return response.data
}

