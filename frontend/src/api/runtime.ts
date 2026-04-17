import type { CreateClientConfig } from './generated/client.gen'

import { apiFetch } from '@/shared/lib/http/transport'

export const createClientConfig: CreateClientConfig = (config) => ({
  ...config,
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? '',
  fetch: (input, init) => apiFetch(input, init),
})

