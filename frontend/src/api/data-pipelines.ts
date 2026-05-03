import { apiErrorFromResponse, readCookie } from './client'

export type DataPipelineStepType =
  | 'input'
  | 'profile'
  | 'clean'
  | 'normalize'
  | 'validate'
  | 'schema_mapping'
  | 'schema_completion'
  | 'enrich_join'
  | 'transform'
  | 'output'

export type DataPipelineGraph = {
  nodes: DataPipelineNode[]
  edges: DataPipelineEdge[]
}

export type DataPipelineNode = {
  id: string
  type?: string
  position: { x: number, y: number }
  data: {
    label?: string
    stepType: DataPipelineStepType
    config?: Record<string, unknown>
  }
}

export type DataPipelineEdge = {
  id: string
  source: string
  target: string
}

export type DataPipelineBody = {
  publicId: string
  name: string
  description: string
  status: string
  publishedVersionId?: number | null
  createdAt: string
  updatedAt: string
  archivedAt?: string | null
}

export type DataPipelineVersionBody = {
  publicId: string
  pipelineId: number
  versionNumber: number
  status: string
  graph: DataPipelineGraph
  validationSummary: { valid: boolean, errors: string[] }
  createdAt: string
  publishedAt?: string | null
}

export type DataPipelineRunStepBody = {
  nodeId: string
  stepType: string
  status: string
  rowCount: number
  errorSummary?: string
  errorSample: Array<Record<string, unknown>>
  metadata: Record<string, unknown>
  startedAt?: string | null
  completedAt?: string | null
  createdAt: string
  updatedAt: string
}

export type DataPipelineRunBody = {
  publicId: string
  versionId: number
  scheduleId?: number | null
  triggerKind: string
  status: string
  outputWorkTableId?: number | null
  rowCount: number
  errorSummary?: string
  startedAt?: string | null
  completedAt?: string | null
  createdAt: string
  updatedAt: string
  steps: DataPipelineRunStepBody[]
}

export type DataPipelineScheduleBody = {
  publicId: string
  versionId: number
  frequency: 'daily' | 'weekly' | 'monthly'
  timezone: string
  runTime: string
  weekday?: number | null
  monthDay?: number | null
  enabled: boolean
  nextRunAt: string
  lastRunAt?: string | null
  lastStatus?: string
  lastErrorSummary?: string
  lastRunId?: number | null
  createdAt: string
  updatedAt: string
}

export type DataPipelineDetailBody = {
  pipeline: DataPipelineBody
  publishedVersion?: DataPipelineVersionBody | null
  versions: DataPipelineVersionBody[]
  runs: DataPipelineRunBody[]
  schedules: DataPipelineScheduleBody[]
}

export type DataPipelinePreviewBody = {
  nodeId: string
  stepType: string
  columns: string[]
  previewRows: Array<Record<string, unknown>>
}

export type DataPipelineScheduleWriteBody = {
  frequency?: 'daily' | 'weekly' | 'monthly'
  timezone?: string
  runTime?: string
  weekday?: number | null
  monthDay?: number | null
  enabled?: boolean
}

async function ensureCSRFCookie() {
  if (readCookie('XSRF-TOKEN')) {
    return
  }
  await request('/api/v1/csrf', { method: 'GET' }, false)
}

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
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
  if (init.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
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

export async function fetchDataPipelines(): Promise<DataPipelineBody[]> {
  const data = await request<{ items?: DataPipelineBody[] }>('/api/v1/data-pipelines?limit=100')
  return data.items ?? []
}

export async function createDataPipeline(body: { name: string, description?: string }): Promise<DataPipelineBody> {
  return request<DataPipelineBody>('/api/v1/data-pipelines', {
    method: 'POST',
    headers: {
      ...csrfHeaders(),
      'Idempotency-Key': crypto.randomUUID(),
    },
    body: JSON.stringify(body),
  })
}

export async function fetchDataPipeline(publicId: string): Promise<DataPipelineDetailBody> {
  return request<DataPipelineDetailBody>(`/api/v1/data-pipelines/${encodeURIComponent(publicId)}`)
}

export async function updateDataPipeline(publicId: string, body: { name: string, description?: string }): Promise<DataPipelineBody> {
  return request<DataPipelineBody>(`/api/v1/data-pipelines/${encodeURIComponent(publicId)}`, {
    method: 'PATCH',
    headers: csrfHeaders(),
    body: JSON.stringify(body),
  })
}

export async function saveDataPipelineVersion(publicId: string, graph: DataPipelineGraph): Promise<DataPipelineVersionBody> {
  return request<DataPipelineVersionBody>(`/api/v1/data-pipelines/${encodeURIComponent(publicId)}/versions`, {
    method: 'POST',
    headers: csrfHeaders(),
    body: JSON.stringify({ graph }),
  })
}

export async function publishDataPipelineVersion(versionPublicId: string): Promise<DataPipelineVersionBody> {
  return request<DataPipelineVersionBody>(`/api/v1/data-pipeline-versions/${encodeURIComponent(versionPublicId)}/publish`, {
    method: 'POST',
    headers: csrfHeaders(),
  })
}

export async function previewDataPipelineVersion(versionPublicId: string, nodeId: string, limit = 100): Promise<DataPipelinePreviewBody> {
  return request<DataPipelinePreviewBody>(`/api/v1/data-pipeline-versions/${encodeURIComponent(versionPublicId)}/preview`, {
    method: 'POST',
    headers: csrfHeaders(),
    body: JSON.stringify({ nodeId, limit }),
  })
}

export async function createDataPipelineRun(versionPublicId: string): Promise<DataPipelineRunBody> {
  return request<DataPipelineRunBody>(`/api/v1/data-pipeline-versions/${encodeURIComponent(versionPublicId)}/runs`, {
    method: 'POST',
    headers: {
      ...csrfHeaders(),
      'Idempotency-Key': crypto.randomUUID(),
    },
  })
}

export async function fetchDataPipelineRuns(publicId: string): Promise<DataPipelineRunBody[]> {
  const data = await request<{ items?: DataPipelineRunBody[] }>(`/api/v1/data-pipelines/${encodeURIComponent(publicId)}/runs?limit=25`)
  return data.items ?? []
}

export async function createDataPipelineSchedule(publicId: string, body: DataPipelineScheduleWriteBody): Promise<DataPipelineScheduleBody> {
  return request<DataPipelineScheduleBody>(`/api/v1/data-pipelines/${encodeURIComponent(publicId)}/schedules`, {
    method: 'POST',
    headers: csrfHeaders(),
    body: JSON.stringify(body),
  })
}

export async function updateDataPipelineSchedule(publicId: string, body: DataPipelineScheduleWriteBody): Promise<DataPipelineScheduleBody> {
  return request<DataPipelineScheduleBody>(`/api/v1/data-pipeline-schedules/${encodeURIComponent(publicId)}`, {
    method: 'PATCH',
    headers: csrfHeaders(),
    body: JSON.stringify(body),
  })
}

export async function disableDataPipelineSchedule(publicId: string): Promise<DataPipelineScheduleBody> {
  return request<DataPipelineScheduleBody>(`/api/v1/data-pipeline-schedules/${encodeURIComponent(publicId)}`, {
    method: 'DELETE',
    headers: csrfHeaders(),
  })
}
