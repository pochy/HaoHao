const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS'])
const CSRF_COOKIE_NAME = 'XSRF-TOKEN'
const CSRF_HEADER_NAME = 'X-CSRF-Token'

export async function apiFetch(
  input: RequestInfo | URL,
  init: RequestInit = {},
): Promise<Response> {
  const headers = new Headers(init.headers)
  const method = (init.method ?? 'GET').toUpperCase()

  if (!SAFE_METHODS.has(method)) {
    const csrfToken = readCookie(CSRF_COOKIE_NAME)
    if (csrfToken) {
      headers.set(CSRF_HEADER_NAME, csrfToken)
    }
  }

  return fetch(input, {
    ...init,
    credentials: 'include',
    headers,
  })
}

export function readCookie(name: string): string | null {
  if (typeof document === 'undefined' || document.cookie === '') {
    return null
  }

  const encodedName = encodeURIComponent(name)
  const match = document.cookie
    .split('; ')
    .find((cookie) => cookie.startsWith(`${encodedName}=`))

  if (!match) {
    return null
  }

  return decodeURIComponent(match.slice(encodedName.length + 1))
}

