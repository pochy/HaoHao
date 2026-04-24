import './client'

import type { AuthSettingsBody } from './generated/types.gen'
import { getAuthSettings } from './generated/sdk.gen'

export async function fetchAuthSettings(): Promise<AuthSettingsBody> {
  return getAuthSettings({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<AuthSettingsBody>
}
