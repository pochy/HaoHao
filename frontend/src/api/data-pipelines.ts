import { apiErrorFromResponse, readCookie } from './client'
import { randomID } from '../utils/id'

export type DataPipelineStepType =
  | 'input'
  | 'profile'
  | 'clean'
  | 'normalize'
  | 'validate'
  | 'schema_mapping'
  | 'schema_completion'
  | 'union'
  | 'join'
  | 'enrich_join'
  | 'transform'
  | 'output'
  | 'extract_text'
  | 'json_extract'
  | 'excel_extract'
  | 'classify_document'
  | 'extract_fields'
  | 'extract_table'
  | 'product_extraction'
  | 'confidence_gate'
  | 'quarantine'
  | 'route_by_condition'
  | 'deduplicate'
  | 'canonicalize'
  | 'redact_pii'
  | 'detect_language_encoding'
  | 'schema_inference'
  | 'entity_resolution'
  | 'unit_conversion'
  | 'relationship_extraction'
  | 'human_review'
  | 'sample_compare'
  | 'quality_report'

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
  latestRunStatus?: string
  latestRunAt?: string | null
  latestRunPublicId?: string
  scheduleState?: 'enabled' | 'disabled' | 'none' | string
  enabledScheduleCount?: number
  disabledScheduleCount?: number
  nextRunAt?: string | null
}

export type DataPipelineListParams = {
  q?: string
  status?: string
  publication?: string
  runStatus?: string
  scheduleState?: string
  sort?: string
  cursor?: string
  limit?: number
}

export type DataPipelineListResult = {
  items: DataPipelineBody[]
  nextCursor: string
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

export type DataPipelineRunOutputBody = {
  nodeId: string
  status: string
  outputWorkTableId?: number | null
  rowCount: number
  errorSummary?: string
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
  outputs: DataPipelineRunOutputBody[]
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
  outputSchemas?: DataPipelineOutputSchemaBody[]
}

export type DataPipelineOutputSchemaBody = {
  nodeId: string
  stepType: string
  columns: string[]
  warnings?: string[]
}

export type DataPipelineGraphValidationBody = {
  validationSummary: { valid: boolean, errors: string[] }
  outputSchemas: DataPipelineOutputSchemaBody[]
  nodeWarnings: DataPipelineNodeWarningBody[]
}

export type DataPipelineNodeWarningBody = {
  nodeId: string
  stepType: string
  code: 'missing_upstream_columns' | 'missing_right_upstream_columns' | string
  severity: 'warning' | 'error' | string
  message: string
  columns: string[]
  configKeys?: string[]
}

export type DataPipelineReviewCommentBody = {
  publicId: string
  authorUserId?: number | null
  body: string
  createdAt: string
}

export type DataPipelineReviewItemBody = {
  publicId: string
  pipelinePublicId?: string
  pipelineName?: string
  versionId: number
  runId: number
  runPublicId?: string
  nodeId: string
  queue: string
  status: 'open' | 'approved' | 'rejected' | 'needs_changes' | 'closed' | string
  reason: Array<Record<string, unknown>>
  sourceSnapshot: Record<string, unknown>
  sourceFingerprint: string
  createdByUserId?: number | null
  updatedByUserId?: number | null
  assignedToUserId?: number | null
  decisionComment?: string
  decidedAt?: string | null
  createdAt: string
  updatedAt: string
  comments?: DataPipelineReviewCommentBody[]
}

export type DataPipelineReviewItemListParams = {
  status?: string
  limit?: number
}

export type DataPipelineReviewItemTransitionBody = {
  status: 'open' | 'approved' | 'rejected' | 'needs_changes' | 'closed'
  comment?: string
}

export type DataPipelineScheduleWriteBody = {
  frequency?: 'daily' | 'weekly' | 'monthly'
  timezone?: string
  runTime?: string
  weekday?: number | null
  monthDay?: number | null
  enabled?: boolean
}

export type SchemaMappingCandidateColumnInput = {
  sourceColumn: string
  sheetName?: string
  sampleValues?: string[]
  neighborColumns?: string[]
}

export type SchemaMappingCandidateBody = {
  schemaColumnPublicId: string
  targetColumn: string
  score: number
  matchMethod: string
  reason: string
  snippet?: string
  acceptedEvidence: number
  rejectedEvidence: number
}

export type SchemaMappingCandidateItem = {
  sourceColumn: string
  candidates: SchemaMappingCandidateBody[]
}

export type SchemaMappingCandidateResult = {
  items: SchemaMappingCandidateItem[]
}

export type SchemaMappingCandidateRequest = {
  pipelinePublicId?: string
  versionPublicId?: string
  domain?: string
  schemaType?: string
  columns: SchemaMappingCandidateColumnInput[]
  limit?: number
}

export type SchemaMappingExampleRequest = {
  pipelinePublicId: string
  versionPublicId?: string
  schemaColumnPublicId: string
  sourceColumn: string
  sheetName?: string
  sampleValues?: string[]
  neighborColumns?: string[]
  decision: 'accepted' | 'rejected'
}

export type SchemaMappingExampleBody = {
  publicId: string
  schemaColumnPublicId: string
  sourceColumn: string
  targetColumn: string
  decision: string
  sharedScope: string
}

export function sanitizeDataPipelineGraph(graph: DataPipelineGraph): DataPipelineGraph {
  return {
    nodes: (graph.nodes ?? []).map((node) => {
      const sanitized: DataPipelineNode = {
        id: stringValue(node.id),
        position: {
          x: numberValue(node.position?.x),
          y: numberValue(node.position?.y),
        },
        data: {
          stepType: stringValue(node.data?.stepType) as DataPipelineStepType,
        },
      }

      const type = normalizeDataPipelineNodeType(stringValue(node.type))
      if (type) {
        sanitized.type = type
      }

      const label = stringValue(node.data?.label)
      if (label) {
        sanitized.data.label = label
      }

      const config = recordValue(node.data?.config)
      if (config) {
        sanitized.data.config = config
      }

      return sanitized
    }),
    edges: (graph.edges ?? []).map((edge) => ({
      id: stringValue(edge.id),
      source: stringValue(edge.source),
      target: stringValue(edge.target),
    })),
  }
}

function normalizeDataPipelineNodeType(value: string): string {
  if (!value || value === 'dataPipelineNode') {
    return 'pipelineStep'
  }
  return value
}

export function isDataPipelineAutoPreviewEnabled(data?: Partial<DataPipelineNode['data']> | null): boolean {
  if (!data?.stepType) {
    return false
  }
  if (manualPreviewStepTypes.has(String(data.stepType))) {
    return false
  }
  return !configUsesManualPreview(data.config)
}

export function isDataPipelinePreviewSupported(graph: DataPipelineGraph): boolean {
  return Boolean(graph)
}

export function isDataPipelineDraftRunPreviewGraph(graph: DataPipelineGraph): boolean {
  return graph.nodes.some((node) => (
    manualPreviewStepTypes.has(String(node.data.stepType))
    || (node.data.stepType === 'input' && stringValue(node.data.config?.sourceKind) === 'drive_file')
  ))
}

const manualPreviewStepTypes = new Set([
  'extract_text',
  'excel_extract',
  'classify_document',
  'extract_fields',
  'extract_table',
  'confidence_gate',
  'deduplicate',
  'canonicalize',
  'redact_pii',
  'detect_language_encoding',
  'schema_inference',
  'entity_resolution',
  'unit_conversion',
  'relationship_extraction',
  'human_review',
  'sample_compare',
  'quality_report',
  'llm_enrichment',
  'api_enrichment',
  'external_enrichment',
])

const manualPreviewConfigKeys = new Set([
  'completionKind',
  'completionMethod',
  'completionProvider',
  'enrichmentKind',
  'enrichmentMethod',
  'enrichmentProvider',
  'executor',
  'method',
  'mode',
  'provider',
  'runtime',
].map(normalizeConfigKey))

const manualPreviewConfigValues = new Set([
  'ai',
  'api',
  'bedrock',
  'external',
  'external_api',
  'external_lookup',
  'gemini',
  'llm',
  'ollama',
  'openai',
  'anthropic',
  'vertex_ai',
  'webhook',
])

function configUsesManualPreview(value: unknown): boolean {
  if (Array.isArray(value)) {
    return value.some((item) => configUsesManualPreview(item))
  }
  if (!value || typeof value !== 'object') {
    return false
  }
  return Object.entries(value as Record<string, unknown>).some(([key, item]) => {
    if (manualPreviewPresenceKey(key)) {
      return item !== undefined && item !== null && item !== ''
    }
    if (manualPreviewConfigKeys.has(normalizeConfigKey(key)) && valueDisablesAutoPreview(item)) {
      return true
    }
    return configUsesManualPreview(item)
  })
}

function manualPreviewPresenceKey(key: string): boolean {
  const normalized = key.toLowerCase()
  return normalized.includes('llm') || normalized.includes('prompt')
}

function valueDisablesAutoPreview(value: unknown): boolean {
  if (typeof value === 'string') {
    return manualPreviewConfigValues.has(normalizeConfigValue(value))
  }
  if (Array.isArray(value)) {
    return value.some((item) => valueDisablesAutoPreview(item))
  }
  if (value && typeof value === 'object') {
    return configUsesManualPreview(value)
  }
  return false
}

function normalizeConfigKey(value: string): string {
  return value.replace(/[-_\s]/g, '').toLowerCase()
}

function normalizeConfigValue(value: string): string {
  return value.trim().replace(/[-\s]/g, '_').toLowerCase()
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

export async function fetchDataPipelines(params: DataPipelineListParams = {}): Promise<DataPipelineListResult> {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && String(value).trim() !== '') {
      search.set(key, String(value))
    }
  }
  const query = search.toString()
  const data = await request<{ items?: DataPipelineBody[], nextCursor?: string }>(`/api/v1/data-pipelines${query ? `?${query}` : ''}`)
  return {
    items: data.items ?? [],
    nextCursor: data.nextCursor ?? '',
  }
}

export async function createDataPipeline(body: { name: string, description?: string }): Promise<DataPipelineBody> {
  return request<DataPipelineBody>('/api/v1/data-pipelines', {
    method: 'POST',
    headers: {
      ...csrfHeaders(),
      'Idempotency-Key': randomID(),
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
    body: JSON.stringify({ graph: sanitizeDataPipelineGraph(graph) }),
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

export async function previewDataPipelineDraft(publicId: string, graph: DataPipelineGraph, nodeId: string, limit = 100): Promise<DataPipelinePreviewBody> {
  return request<DataPipelinePreviewBody>(`/api/v1/data-pipelines/${encodeURIComponent(publicId)}/preview`, {
    method: 'POST',
    headers: csrfHeaders(),
    body: JSON.stringify({ graph: sanitizeDataPipelineGraph(graph), nodeId, limit }),
  })
}

export async function validateDataPipelineDraft(publicId: string, graph: DataPipelineGraph): Promise<DataPipelineGraphValidationBody> {
  return request<DataPipelineGraphValidationBody>(`/api/v1/data-pipelines/${encodeURIComponent(publicId)}/validate`, {
    method: 'POST',
    headers: csrfHeaders(),
    body: JSON.stringify({ graph: sanitizeDataPipelineGraph(graph) }),
  })
}

export async function createDataPipelineRun(versionPublicId: string): Promise<DataPipelineRunBody> {
  return request<DataPipelineRunBody>(`/api/v1/data-pipeline-versions/${encodeURIComponent(versionPublicId)}/runs`, {
    method: 'POST',
    headers: {
      ...csrfHeaders(),
      'Idempotency-Key': randomID(),
    },
  })
}

export async function fetchDataPipelineRuns(publicId: string): Promise<DataPipelineRunBody[]> {
  const data = await request<{ items?: DataPipelineRunBody[] }>(`/api/v1/data-pipelines/${encodeURIComponent(publicId)}/runs?limit=25`)
  return data.items ?? []
}

export async function fetchDataPipelineReviewItems(publicId: string, params: DataPipelineReviewItemListParams = {}): Promise<DataPipelineReviewItemBody[]> {
  const search = new URLSearchParams()
  if (params.status) search.set('status', params.status)
  if (params.limit) search.set('limit', String(params.limit))
  const query = search.toString()
  const data = await request<{ items?: DataPipelineReviewItemBody[] }>(`/api/v1/data-pipelines/${encodeURIComponent(publicId)}/review-items${query ? `?${query}` : ''}`)
  return data.items ?? []
}

export async function fetchDriveFileDataPipelineReviewItems(filePublicId: string, params: DataPipelineReviewItemListParams = {}): Promise<DataPipelineReviewItemBody[]> {
  const search = new URLSearchParams()
  if (params.status) search.set('status', params.status)
  if (params.limit) search.set('limit', String(params.limit))
  const query = search.toString()
  const data = await request<{ items?: DataPipelineReviewItemBody[] }>(`/api/v1/drive/files/${encodeURIComponent(filePublicId)}/data-pipeline-review-items${query ? `?${query}` : ''}`)
  return data.items ?? []
}

export async function fetchDataPipelineReviewItem(publicId: string): Promise<DataPipelineReviewItemBody> {
  return request<DataPipelineReviewItemBody>(`/api/v1/data-pipeline-review-items/${encodeURIComponent(publicId)}`)
}

export async function transitionDataPipelineReviewItem(publicId: string, body: DataPipelineReviewItemTransitionBody): Promise<DataPipelineReviewItemBody> {
  return request<DataPipelineReviewItemBody>(`/api/v1/data-pipeline-review-items/${encodeURIComponent(publicId)}/transition`, {
    method: 'POST',
    headers: csrfHeaders(),
    body: JSON.stringify(body),
  })
}

export async function commentDataPipelineReviewItem(publicId: string, body: string): Promise<DataPipelineReviewCommentBody> {
  return request<DataPipelineReviewCommentBody>(`/api/v1/data-pipeline-review-items/${encodeURIComponent(publicId)}/comments`, {
    method: 'POST',
    headers: csrfHeaders(),
    body: JSON.stringify({ body }),
  })
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

export async function fetchSchemaMappingCandidates(body: SchemaMappingCandidateRequest): Promise<SchemaMappingCandidateResult> {
  const data = await request<{ items?: SchemaMappingCandidateItem[] }>('/api/v1/data-pipelines/schema-mapping/candidates', {
    method: 'POST',
    headers: csrfHeaders(),
    body: JSON.stringify(body),
  })
  return { items: data.items ?? [] }
}

export async function recordSchemaMappingExample(body: SchemaMappingExampleRequest): Promise<SchemaMappingExampleBody> {
  return request<SchemaMappingExampleBody>('/api/v1/data-pipelines/schema-mapping/examples', {
    method: 'POST',
    headers: csrfHeaders(),
    body: JSON.stringify(body),
  })
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function numberValue(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function recordValue(value: unknown): Record<string, unknown> | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return undefined
  }
  return JSON.parse(JSON.stringify(value)) as Record<string, unknown>
}
