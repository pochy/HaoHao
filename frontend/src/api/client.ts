import type { ErrorModel } from './generated/types.gen'
import { client } from './generated/client.gen'

type ProblemLike = Partial<Pick<ErrorModel, 'detail' | 'title'>> & {
  message?: string
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
    if (!['GET', 'HEAD', 'OPTIONS'].includes(method) && !headers.has('X-CSRF-Token')) {
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

