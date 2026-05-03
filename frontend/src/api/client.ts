import type { ErrorModel } from './generated/types.gen'
import { client } from './generated/client.gen'

type ProblemLike = Partial<Pick<ErrorModel, 'detail' | 'instance' | 'status' | 'title' | 'type'>> & {
  message?: string
}

type ApiErrorInit = {
  title?: string
  detail?: string
  type?: string
  status?: number
  instance?: string
  requestId?: string
  fallbackMessage?: string
}

export class ApiError extends Error {
  title?: string
  detail?: string
  type?: string
  status?: number
  instance?: string
  requestId?: string

  constructor(init: ApiErrorInit) {
    super(init.detail || init.title || init.fallbackMessage || 'リクエストに失敗しました')
    this.name = 'ApiError'
    this.title = init.title
    this.detail = init.detail
    this.type = init.type
    this.status = init.status
    this.instance = init.instance
    this.requestId = init.requestId
  }
}

export function readCookie(name: string): string | undefined {
  const prefix = `${name}=`
  return document.cookie
    .split(';')
    .map((part) => part.trim())
    .find((part) => part.startsWith(prefix))
    ?.slice(prefix.length)
}

export function toApiErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.detail || error.title || error.message
  }

  if (error instanceof Error && error.message) {
    return error.message
  }

  if (error && typeof error === 'object') {
    const problem = error as ProblemLike
    if (problem.detail) {
      return problem.detail
    }
    if (problem.title) {
      return problem.title
    }
    if (problem.message) {
      return problem.message
    }
  }

  return 'リクエストに失敗しました'
}

export function toApiErrorStatus(error: unknown): number | undefined {
  if (error && typeof error === 'object' && 'status' in error) {
    const status = (error as ProblemLike).status
    return typeof status === 'number' ? status : undefined
  }

  return undefined
}

export function toApiErrorType(error: unknown): string {
  if (error instanceof ApiError) {
    return error.type ?? ''
  }
  if (error && typeof error === 'object' && 'type' in error) {
    const value = (error as ProblemLike).type
    return typeof value === 'string' ? value : ''
  }
  return ''
}

export function toApiErrorRequestId(error: unknown): string {
  if (error instanceof ApiError) {
    return error.requestId ?? requestIDFromInstance(error.instance)
  }
  if (error && typeof error === 'object' && 'instance' in error) {
    return requestIDFromInstance((error as ProblemLike).instance)
  }
  return ''
}

export function isRetryableApiError(error: unknown): boolean {
  const status = toApiErrorStatus(error)
  return status === undefined || status === 0 || status === 408 || status >= 500
}

export async function apiErrorFromResponse(response: Response, fallbackMessage: string): Promise<ApiError> {
  let problem: ProblemLike = {}
  try {
    const body = await response.json()
    if (body && typeof body === 'object') {
      problem = body as ProblemLike
    }
  } catch {
    // Non-JSON responses keep the HTTP status text.
  }
  const requestId = response.headers.get('X-Request-ID') || requestIDFromInstance(problem.instance)
  return apiErrorFromProblem(problem, response.status, requestId, fallbackMessage || response.statusText || `Request failed (${response.status})`)
}

function requestIDFromInstance(instance?: string): string {
  const prefix = 'urn:haohao:request:'
  return instance?.startsWith(prefix) ? instance.slice(prefix.length) : ''
}

function apiErrorFromProblem(problem: ProblemLike, status: number | undefined, requestId: string, fallbackMessage: string): ApiError {
  return new ApiError({
    title: problem.title,
    detail: problem.detail,
    type: problem.type,
    status: typeof problem.status === 'number' ? problem.status : status,
    instance: problem.instance,
    requestId: requestId || requestIDFromInstance(problem.instance),
    fallbackMessage,
  })
}

export function isApiForbidden(error: unknown): boolean {
  if (toApiErrorStatus(error) === 403) {
    return true
  }

  const message = toApiErrorMessage(error)
  return /forbidden|customer_signal_user|data_pipeline_user|machine_client_admin|tenant_admin|docs_reader|todo_user/i.test(message)
}

let csrfBootstrapPromise: Promise<void> | null = null

async function ensureCSRFCookie() {
  if (typeof document === 'undefined' || readCookie('XSRF-TOKEN')) {
    return
  }

  if (!csrfBootstrapPromise) {
    csrfBootstrapPromise = (async () => {
      try {
        await fetch('/api/v1/csrf', {
          method: 'GET',
          credentials: 'include',
          headers: {
            Accept: 'application/json',
          },
        })
      } catch {
        // The original mutating request will surface the real failure.
      }
    })().finally(() => {
      csrfBootstrapPromise = null
    })
  }

  await csrfBootstrapPromise
}

client.setConfig({
  baseUrl: '',
  credentials: 'include',
  responseStyle: 'data',
  throwOnError: true,
  fetch: async (input, init) => {
    const request = input instanceof Request ? input : undefined
    const headers = new Headers(request?.headers ?? init?.headers ?? {})
    headers.set('Accept', 'application/json')

    const method = (init?.method ?? request?.method ?? 'GET').toUpperCase()
    const csrfHeader = headers.get('X-CSRF-Token')
    if (!['GET', 'HEAD', 'OPTIONS'].includes(method) && !csrfHeader) {
      await ensureCSRFCookie()
      const token = readCookie('XSRF-TOKEN')
      if (token) {
        headers.set('X-CSRF-Token', token)
      }
    }
    if (method === 'POST' && !headers.get('Idempotency-Key')) {
      headers.set('Idempotency-Key', crypto.randomUUID())
    }

    return fetch(input, {
      ...init,
      credentials: 'include',
      headers,
    })
  },
})

client.interceptors.error.use((error, response) => {
  if (error instanceof ApiError) {
    return error
  }
  if (response instanceof Response) {
    const problem = error && typeof error === 'object' ? error as ProblemLike : {}
    const requestId = response.headers.get('X-Request-ID') || requestIDFromInstance(problem.instance)
    return apiErrorFromProblem(problem, response.status, requestId, response.statusText || `Request failed (${response.status})`)
  }
  if (error instanceof Error) {
    return new ApiError({ detail: error.message, status: 0, fallbackMessage: error.message })
  }
  return error
})
