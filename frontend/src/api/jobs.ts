import { apiErrorFromResponse, readCookie } from './client'

export type SystemJobBody = {
  type: string
  publicId: string
  title: string
  subjectType?: string
  subjectPublicId?: string
  requestedByDisplayName?: string
  requestedByEmail?: string
  status: string
  statusGroup: string
  action?: string
  errorMessage?: string
  outboxEventPublicId?: string
  createdAt: string
  updatedAt: string
  startedAt?: string
  completedAt?: string
  metadata: Record<string, unknown>
  canStop: boolean
}

export type SystemJobListBody = {
  items: SystemJobBody[]
  total: number
  limit: number
  offset: number
}

export type SystemJobFilters = {
  query?: string
  type?: string
  status?: string
  statusGroup?: string
  limit?: number
  offset?: number
}

async function ensureCSRFCookie() {
  if (readCookie('XSRF-TOKEN')) {
    return
  }
  await request('/api/v1/csrf', { method: 'GET' }, false)
}

async function request<T>(path: string, init: RequestInit = {}, withCSRF = true): Promise<T> {
  const method = (init.method ?? 'GET').toUpperCase()
  const needsCSRF = withCSRF && !['GET', 'HEAD', 'OPTIONS'].includes(method)
  if (needsCSRF) {
    await ensureCSRFCookie()
  }
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  if (needsCSRF) {
    const token = readCookie('XSRF-TOKEN')
    if (token) {
      headers.set('X-CSRF-Token', token)
    }
  }
  const response = await fetch(path, {
    ...init,
    credentials: 'include',
    headers,
  })
  if (!response.ok) {
    throw await apiErrorFromResponse(response, response.statusText)
  }
  if (response.status === 204) {
    return undefined as T
  }
  return await response.json() as T
}

export async function fetchSystemJobs(filters: SystemJobFilters): Promise<SystemJobListBody> {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(filters)) {
    if (value !== undefined && value !== null && String(value).trim() !== '') {
      params.set(key, String(value))
    }
  }
  const query = params.toString()
  return request<SystemJobListBody>(`/api/v1/jobs${query ? `?${query}` : ''}`)
}

export async function fetchSystemJob(type: string, publicId: string): Promise<SystemJobBody> {
  return request<SystemJobBody>(`/api/v1/jobs/${encodeURIComponent(type)}/${encodeURIComponent(publicId)}`)
}

export async function stopSystemJob(type: string, publicId: string): Promise<SystemJobBody> {
  return request<SystemJobBody>(`/api/v1/jobs/${encodeURIComponent(type)}/${encodeURIComponent(publicId)}/stop`, {
    method: 'POST',
  })
}
