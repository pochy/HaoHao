export async function checkDocsAccess(): Promise<void> {
  const response = await fetch('/docs/openapi', {
    method: 'GET',
    credentials: 'include',
    headers: {
      Accept: 'text/html',
    },
  })

  if (response.ok) {
    return
  }

  if (response.status === 401) {
    throw new Error('Login is required to open docs.')
  }
  if (response.status === 403) {
    throw new Error('docs_reader role is required to open docs.')
  }

  throw new Error(`Docs are unavailable: HTTP ${response.status}`)
}
