import type { ErrorModel } from './generated/types.gen'
import { client } from './generated/client.gen'

type ProblemLike = Partial<Pick<ErrorModel, 'detail' | 'title'>> & {
  message?: string
  status?: number
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

  return '認証処理に失敗しました'
}

export function toApiErrorStatus(error: unknown): number | undefined {
  if (error && typeof error === 'object' && 'status' in error) {
    const status = (error as ProblemLike).status
    return typeof status === 'number' ? status : undefined
  }

  return undefined
}

export function isApiForbidden(error: unknown): boolean {
  if (toApiErrorStatus(error) === 403) {
    return true
  }

  const message = toApiErrorMessage(error)
  return /forbidden|machine_client_admin|docs_reader|todo_user/i.test(message)
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

    return fetch(input, {
      ...init,
      credentials: 'include',
      headers,
    })
  },
})
