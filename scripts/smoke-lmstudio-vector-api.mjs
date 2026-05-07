const baseURL = (process.env.HAOHAO_SMOKE_BASE_URL ?? 'http://127.0.0.1:18080').replace(/\/+$/, '')
const tenantSlug = process.env.HAOHAO_TENANT_SLUG ?? 'acme'
const email = process.env.HAOHAO_SMOKE_EMAIL ?? 'demo@example.com'
const password = process.env.HAOHAO_SMOKE_PASSWORD ?? 'changeme123'
const workspacePublicId = process.env.HAOHAO_DRIVE_WORKSPACE_PUBLIC_ID ?? '019dc8d1-14ff-7809-acdf-28249034d565'
const runtimeURL = process.env.HAOHAO_LMSTUDIO_PROXY_URL ?? 'http://127.0.0.1:11234'
const model = process.env.HAOHAO_EVAL_EMBEDDING_MODEL ?? 'text-embedding-mxbai-embed-large-v1'
const dimension = Number(process.env.HAOHAO_EVAL_EMBEDDING_DIMENSION ?? '1024')
const pollAttempts = Number(process.env.HAOHAO_SMOKE_POLL_ATTEMPTS ?? '45')
const pollDelayMs = Number(process.env.HAOHAO_SMOKE_POLL_DELAY_MS ?? '1500')
const rebuild = process.env.HAOHAO_SMOKE_REBUILD === '1'

const cookies = new Map()
const uploadedFileIDs = []
let originalSettings

function cookieHeader() {
  return [...cookies.entries()].map(([key, value]) => `${key}=${value}`).join('; ')
}

function rememberCookies(headers) {
  for (const raw of headers.getSetCookie?.() ?? []) {
    const first = raw.split(';', 1)[0]
    const index = first.indexOf('=')
    if (index > 0) {
      cookies.set(first.slice(0, index), first.slice(index + 1))
    }
  }
}

function csrfToken() {
  return cookies.get('XSRF-TOKEN') ?? ''
}

async function request(path, options = {}) {
  const headers = new Headers(options.headers ?? {})
  if (cookies.size > 0) {
    headers.set('Cookie', cookieHeader())
  }
  const response = await fetch(`${baseURL}${path}`, { ...options, headers })
  rememberCookies(response.headers)
  const contentType = response.headers.get('Content-Type') ?? ''
  const body = contentType.includes('application/json') ? await response.json() : await response.text()
  if (!response.ok) {
    throw new Error(`${options.method ?? 'GET'} ${path} returned ${response.status}: ${typeof body === 'string' ? body : JSON.stringify(body)}`)
  }
  return body
}

async function jsonRequest(path, method, body) {
  return request(path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken(),
    },
    body: JSON.stringify(body),
  })
}

async function login() {
  await request('/api/v1/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  if (!cookies.has('SESSION_ID')) {
    throw new Error('login did not set SESSION_ID')
  }
  await jsonRequest('/api/v1/session/tenant', 'POST', { tenantSlug })
}

async function updateLocalSearchPolicy(enabled) {
  const settings = await request(`/api/v1/admin/tenants/${tenantSlug}/settings`)
  if (!originalSettings) {
    originalSettings = structuredClone(settings)
  }
  const features = structuredClone(settings.features ?? {})
  const drive = structuredClone(features.drive ?? {})
  if (enabled) {
    drive.localSearch = {
      ...(drive.localSearch ?? {}),
      vectorEnabled: true,
      embeddingRuntime: 'lmstudio',
      runtimeURL,
      model,
      dimension,
    }
  } else {
    drive.localSearch = originalSettings.features?.drive?.localSearch ?? {
      vectorEnabled: false,
      embeddingRuntime: 'none',
      runtimeURL: '',
      model: '',
      dimension: 0,
    }
  }
  features.drive = drive
  return jsonRequest(`/api/v1/admin/tenants/${tenantSlug}/settings`, 'PUT', {
    fileQuotaBytes: settings.fileQuotaBytes,
    rateLimitLoginPerMinute: settings.rateLimitLoginPerMinute,
    rateLimitBrowserApiPerMinute: settings.rateLimitBrowserApiPerMinute,
    rateLimitExternalApiPerMinute: settings.rateLimitExternalApiPerMinute,
    notificationsEnabled: settings.notificationsEnabled,
    features,
  })
}

async function uploadTextFile(filename, text) {
  const form = new FormData()
  form.set('workspacePublicId', workspacePublicId)
  form.set('file', new Blob([text], { type: 'text/plain' }), filename)
  const response = await request('/api/v1/drive/files', {
    method: 'POST',
    headers: { 'X-CSRF-Token': csrfToken() },
    body: form,
  })
  uploadedFileIDs.push(response.publicId)
  return response
}

async function rebuildLocalSearch() {
  return jsonRequest(`/api/v1/admin/tenants/${tenantSlug}/drive/search/local-index/rebuilds`, 'POST', {})
}

async function searchDocuments(query, mode) {
  const params = new URLSearchParams({ q: query, mode, limit: '20' })
  return request(`/api/v1/drive/search/documents?${params.toString()}`)
}

function resultNames(result) {
  return (result.items ?? []).map((item) => item.item?.file?.originalFilename ?? item.item?.name ?? '').filter(Boolean)
}

async function waitForSemanticHit(expectedName, excludedName) {
  let last
  for (let attempt = 1; attempt <= pollAttempts; attempt += 1) {
    last = await searchDocuments('支払期限', 'semantic')
    const names = resultNames(last)
    if (names.includes(expectedName) && !names.includes(excludedName)) {
      return last
    }
    await new Promise((resolve) => setTimeout(resolve, pollDelayMs))
  }
  throw new Error(`semantic search did not reach expected result. last=${JSON.stringify(last)}`)
}

async function cleanup() {
  for (const publicId of uploadedFileIDs.reverse()) {
    try {
      await request(`/api/v1/drive/files/${encodeURIComponent(publicId)}`, {
        method: 'DELETE',
        headers: { 'X-CSRF-Token': csrfToken() },
      })
    } catch (error) {
      console.error(`cleanup delete ${publicId}: ${error.message}`)
    }
  }
  if (originalSettings) {
    try {
      await updateLocalSearchPolicy(false)
    } catch (error) {
      console.error(`cleanup settings: ${error.message}`)
    }
  }
}

async function main() {
  await login()
  await updateLocalSearchPolicy(true)

  const suffix = Date.now()
  const invoiceName = `haohao-lmstudio-vector-invoice-${suffix}.txt`
  const noteName = `haohao-lmstudio-vector-note-${suffix}.txt`
  const invoice = await uploadTextFile(invoiceName, [
    '請求書',
    '請求番号: INV-LMSTUDIO-2026-0001',
    '請求日: 2026-05-07',
    '振込期限: 2026-06-30',
    '支払先: 株式会社青葉商事',
    '税込合計: 128000円',
    '登録番号: T1234567890123',
  ].join('\n'))
  const note = await uploadTextFile(noteName, [
    'プロダクト週報',
    '今週は管理画面のナビゲーション改善を進めた。',
    '来週はアクセシビリティ確認と表示速度の計測を行う。',
    '対象はUI文言、画面遷移、操作ログの確認である。',
  ].join('\n'))

  if (rebuild) {
    await rebuildLocalSearch()
  }
  const semantic = await waitForSemanticHit(invoiceName, noteName)
  const hybrid = await searchDocuments('支払期限', 'hybrid')
  const keyword = await searchDocuments('支払期限', 'keyword')
  const defaultKeyword = await request(`/api/v1/drive/search/documents?${new URLSearchParams({ q: '振込期限', limit: '20' }).toString()}`)

  const output = {
    ok: true,
    runtimeURL,
    model,
    dimension,
    invoiceFile: invoice.publicId,
    noteFile: note.publicId,
    keywordNames: resultNames(keyword),
    semanticNames: resultNames(semantic),
    hybridNames: resultNames(hybrid),
    defaultNames: resultNames(defaultKeyword),
    checks: {
      keywordDoesNotFindSynonym: !resultNames(keyword).includes(invoiceName),
      semanticFindsInvoice: resultNames(semantic).includes(invoiceName),
      semanticExcludesNote: !resultNames(semantic).includes(noteName),
      hybridFindsInvoice: resultNames(hybrid).includes(invoiceName),
      defaultKeywordFindsExactTerm: resultNames(defaultKeyword).includes(invoiceName),
    },
  }
  output.ok = Object.values(output.checks).every(Boolean)
  console.log(JSON.stringify(output, null, 2))
  if (!output.ok) {
    process.exitCode = 1
  }
}

try {
  await main()
} finally {
  await cleanup()
}
