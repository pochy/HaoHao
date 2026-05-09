import { readFile } from 'node:fs/promises'
import { performance } from 'node:perf_hooks'

const baseURL = (process.env.HAOHAO_SMOKE_BASE_URL ?? 'http://127.0.0.1:18080').replace(/\/+$/, '')
const tenantSlug = process.env.HAOHAO_TENANT_SLUG ?? 'acme'
const email = process.env.HAOHAO_SMOKE_EMAIL ?? 'demo@example.com'
const password = process.env.HAOHAO_SMOKE_PASSWORD ?? 'changeme123'
const workspacePublicId = process.env.HAOHAO_DRIVE_WORKSPACE_PUBLIC_ID ?? '019dc8d1-14ff-7809-acdf-28249034d565'
const runtimeURL = process.env.HAOHAO_LMSTUDIO_PROXY_URL ?? 'http://127.0.0.1:11234'
const embeddingModel = process.env.HAOHAO_EVAL_EMBEDDING_MODEL ?? 'text-embedding-mxbai-embed-large-v1'
const generationModel = process.env.HAOHAO_RAG_GENERATION_MODEL ?? 'qwen/qwen3.5-9b'
const dimension = Number(process.env.HAOHAO_EVAL_EMBEDDING_DIMENSION ?? '1024')
const pollAttempts = Number(process.env.HAOHAO_SMOKE_POLL_ATTEMPTS ?? '80')
const pollDelayMs = Number(process.env.HAOHAO_SMOKE_POLL_DELAY_MS ?? '1500')
const broadMode = process.env.HAOHAO_SMOKE_RAG_BROAD === '1'
const broadQueryLimit = Number(process.env.HAOHAO_SMOKE_RAG_QUERY_LIMIT ?? '12')
const broadQueryIDs = (process.env.HAOHAO_SMOKE_RAG_QUERY_IDS ?? '')
  .split(',')
  .map((value) => value.trim())
  .filter(Boolean)
const minimumBroadCitationCoverage = Number(process.env.HAOHAO_SMOKE_RAG_MIN_CITATION_COVERAGE ?? '0.8')
const minimumBroadAnswerFactCoverage = Number(process.env.HAOHAO_SMOKE_RAG_MIN_ANSWER_FACT_COVERAGE ?? '0.8')
const maximumBroadNoCitationAnswerRate = Number(process.env.HAOHAO_SMOKE_RAG_MAX_NO_CITATION_ANSWER_RATE ?? '0')
const maximumBroadP95LatencyMs = Number(process.env.HAOHAO_SMOKE_RAG_MAX_P95_LATENCY_MS ?? '30000')
const requireBroadIndexCoverage = process.env.HAOHAO_SMOKE_RAG_REQUIRE_INDEX_COVERAGE === '1'
const triggerBroadRebuild = process.env.HAOHAO_SMOKE_RAG_REBUILD === '1'
const ragMaxContextChunks = Number(process.env.HAOHAO_SMOKE_RAG_MAX_CONTEXT_CHUNKS ?? '8')
const ragMaxContextRunes = Number(process.env.HAOHAO_SMOKE_RAG_MAX_CONTEXT_RUNES ?? '8000')
const broadMarkerPrefix = process.env.HAOHAO_SMOKE_RAG_MARKER_PREFIX ?? 'haohao-broad-rag'
const queryRewriteMode = process.env.HAOHAO_SMOKE_RAG_QUERY_REWRITE_MODE ?? 'deterministic'
const runQueryRewriteCase = process.env.HAOHAO_SMOKE_RAG_QUERY_REWRITE_CASE !== '0'

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

async function updatePolicy(enabled, ragOverrides = {}) {
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
      model: embeddingModel,
      dimension,
    }
    drive.rag = {
      ...(drive.rag ?? {}),
      enabled: true,
      generationRuntime: 'lmstudio',
      generationRuntimeURL: runtimeURL,
      generationModel,
      maxContextChunks: ragMaxContextChunks,
      maxContextRunes: ragMaxContextRunes,
      queryRewriteEnabled: queryRewriteMode !== 'none',
      queryRewriteMode,
      queryRewriteMaxQueries: 6,
      ...ragOverrides,
    }
  } else {
    drive.localSearch = originalSettings.features?.drive?.localSearch ?? {
      vectorEnabled: false,
      embeddingRuntime: 'none',
      runtimeURL: '',
      model: '',
      dimension: 0,
    }
    drive.rag = originalSettings.features?.drive?.rag ?? {
      enabled: false,
      generationRuntime: 'none',
      generationRuntimeURL: '',
      generationModel: '',
      maxContextChunks: 6,
      maxContextRunes: 6000,
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

async function requestLocalSearchRebuild() {
  return jsonRequest(`/api/v1/admin/tenants/${tenantSlug}/drive/search/local-index/rebuilds`, 'POST', {})
}

async function searchDocuments(query, mode) {
  const params = new URLSearchParams({ q: query, mode, limit: '20' })
  return request(`/api/v1/drive/search/documents?${params.toString()}`)
}

function resultNames(result) {
  return (result.items ?? []).map((item) => item.item?.file?.originalFilename ?? item.item?.name ?? '').filter(Boolean)
}

function percentile(values, p) {
  if (values.length === 0) {
    return 0
  }
  const sorted = [...values].sort((a, b) => a - b)
  const index = Math.min(sorted.length - 1, Math.ceil((p / 100) * sorted.length) - 1)
  return sorted[index]
}

function normalizeText(value) {
  return String(value ?? '')
    .normalize('NFKC')
    .replace(/[,\s]/g, '')
    .toLowerCase()
}

function dateAliases(fact) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(fact)
  if (!match) {
    return [fact]
  }
  const [, year, month, day] = match
  return [
    fact,
    `${year}/${month}/${day}`,
    `${year}年${Number(month)}月${Number(day)}日`,
    `${year}年${month}月${day}日`,
  ]
}

function answerContainsFact(answer, fact) {
  const normalizedAnswer = normalizeText(answer)
  return dateAliases(fact).some((alias) => normalizedAnswer.includes(normalizeText(alias)))
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

async function waitForSearchHit(query, mode, expectedNames) {
  let last
  for (let attempt = 1; attempt <= pollAttempts; attempt += 1) {
    last = await searchDocuments(query, mode)
    const names = resultNames(last)
    if (expectedNames.every((name) => names.includes(name))) {
      return last
    }
    await new Promise((resolve) => setTimeout(resolve, pollDelayMs))
  }
  throw new Error(`search did not reach expected result for ${query}. last=${JSON.stringify(last)}`)
}

async function loadDriveRagDataset() {
  const body = await readFile(new URL('../samples/evaluation/drive-rag-retrieval-ja.json', import.meta.url), 'utf8')
  return JSON.parse(body)
}

function selectBroadQueries(dataset) {
  if (broadQueryIDs.length > 0) {
    const selected = dataset.queries.filter((query) => broadQueryIDs.includes(query.id))
    if (selected.length !== broadQueryIDs.length) {
      const found = new Set(selected.map((query) => query.id))
      const missing = broadQueryIDs.filter((id) => !found.has(id))
      throw new Error(`unknown broad query IDs: ${missing.join(', ')}`)
    }
    return selected
  }

  const defaultIDs = [
    'rag-query-001',
    'rag-query-003',
    'rag-query-005',
    'rag-query-006',
    'rag-query-007',
    'rag-query-025',
    'rag-query-030',
    'rag-query-033',
    'rag-query-036',
    'rag-query-039',
    'rag-query-041',
    'rag-query-042',
  ]
  const selected = dataset.queries.filter((query) => defaultIDs.includes(query.id))
  return selected.slice(0, Number.isFinite(broadQueryLimit) && broadQueryLimit > 0 ? broadQueryLimit : selected.length)
}

async function uploadDatasetDocuments(dataset, suffix, marker) {
  const uploadedByDocumentID = new Map()
  for (const document of dataset.documents) {
    if (document.visibility !== 'viewable') {
      continue
    }
    const safeTitle = document.title.replace(/\.txt$/i, '')
    const file = await uploadTextFile(`${safeTitle}-${suffix}.txt`, `${marker}\n${document.text}`)
    uploadedByDocumentID.set(document.id, {
      ...document,
      publicId: file.publicId,
      filename: file.originalFilename,
    })
  }
  return uploadedByDocumentID
}

async function waitForBroadIndex(queries, uploadedByDocumentID) {
  const expectedQueries = queries.filter((query) => (query.expectedDocumentIds ?? []).some((id) => uploadedByDocumentID.has(id)))
  let last = []
  for (let attempt = 1; attempt <= pollAttempts; attempt += 1) {
    last = []
    let passed = 0
    for (const query of expectedQueries) {
      const search = await searchDocuments(query.smokeQuery ?? query.query, query.mode === 'keyword' ? 'keyword' : query.mode)
      const names = resultNames(search)
      const expectedNames = query.expectedDocumentIds
        .map((id) => uploadedByDocumentID.get(id)?.filename)
        .filter(Boolean)
      const ok = expectedNames.every((name) => names.includes(name))
      last.push({ id: query.id, ok, expectedNames, names: names.slice(0, 8) })
      if (ok) {
        passed += 1
      }
    }
    if (passed === expectedQueries.length) {
      return { ready: true, passed, total: expectedQueries.length, results: last }
    }
    await new Promise((resolve) => setTimeout(resolve, pollDelayMs))
  }
  const passed = last.filter((item) => item.ok).length
  if (requireBroadIndexCoverage) {
    throw new Error(`broad search index did not reach expected coverage: ${JSON.stringify(last, null, 2)}`)
  }
  return { ready: false, passed, total: expectedQueries.length, results: last }
}

async function runBroadSmoke() {
  const dataset = await loadDriveRagDataset()
  const suffix = Date.now()
  const marker = `${broadMarkerPrefix}-${suffix}`
  const queries = selectBroadQueries(dataset).map((query) => ({
    ...query,
    smokeQuery: `${query.query} ${marker}`,
  }))
  const uploadedByDocumentID = await uploadDatasetDocuments(dataset, suffix, marker)

  if (triggerBroadRebuild) {
    await requestLocalSearchRebuild()
  }
  const indexReadiness = await waitForBroadIndex(queries, uploadedByDocumentID)

  const perQuery = []
  let expectedCitationChecks = 0
  let expectedCitationPasses = 0
  let answerFactChecks = 0
  let answerFactPasses = 0
  let noCitationAnswerViolations = 0
  let deniedQueries = 0
  let deniedBlocked = 0

  for (const query of queries) {
    const started = performance.now()
    const rag = await jsonRequest('/api/v1/drive/rag/query', 'POST', {
      query: query.smokeQuery ?? query.query,
      mode: query.mode === 'keyword' ? 'hybrid' : query.mode,
      limit: 8,
    })
    const latencyMs = Math.round(performance.now() - started)
    const answer = rag.answer ?? ''
    const citations = rag.citations ?? []
    const citedPublicIDs = new Set(citations.map((citation) => citation.filePublicId))
    const expectedPublicIDs = (query.expectedDocumentIds ?? [])
      .map((id) => uploadedByDocumentID.get(id)?.publicId)
      .filter(Boolean)
    const expectedCitationHit = expectedPublicIDs.length > 0 && expectedPublicIDs.some((id) => citedPublicIDs.has(id))
    if (expectedPublicIDs.length > 0) {
      expectedCitationChecks += 1
      if (expectedCitationHit) {
        expectedCitationPasses += 1
      }
    } else {
      deniedQueries += 1
      if (rag.blocked || citations.length === 0) {
        deniedBlocked += 1
      }
    }

    const expectedFacts = query.expectedAnswerFacts ?? []
    let factPasses = 0
    for (const fact of expectedFacts) {
      answerFactChecks += 1
      if (answerContainsFact(answer, fact)) {
        answerFactPasses += 1
        factPasses += 1
      }
    }

    if (!rag.blocked && citations.length === 0 && answer.trim().length > 0) {
      noCitationAnswerViolations += 1
    }

    perQuery.push({
      id: query.id,
      query: query.query,
      mode: query.mode,
      smokeQuery: query.smokeQuery,
      latencyMs,
      blocked: Boolean(rag.blocked),
      answer,
      expectedDocuments: query.expectedDocumentIds ?? [],
      citationFilenames: citations.map((citation) => citation.filename),
      expectedCitationHit,
      expectedFactPasses: `${factPasses}/${expectedFacts.length}`,
    })
  }

  const latencies = perQuery.map((item) => item.latencyMs)
  const metrics = {
    queryCount: queries.length,
    uploadedViewableDocuments: uploadedByDocumentID.size,
    citationCoverage: expectedCitationChecks === 0 ? 1 : expectedCitationPasses / expectedCitationChecks,
    answerFactCoverage: answerFactChecks === 0 ? 1 : answerFactPasses / answerFactChecks,
    noCitationAnswerRate: queries.length === 0 ? 0 : noCitationAnswerViolations / queries.length,
    deniedBlockRate: deniedQueries === 0 ? 1 : deniedBlocked / deniedQueries,
    p50LatencyMs: percentile(latencies, 50),
    p95LatencyMs: percentile(latencies, 95),
  }
  const checks = {
    citationCoverage: metrics.citationCoverage >= minimumBroadCitationCoverage,
    answerFactCoverage: metrics.answerFactCoverage >= minimumBroadAnswerFactCoverage,
    noCitationAnswerRate: metrics.noCitationAnswerRate <= maximumBroadNoCitationAnswerRate,
    deniedBlockRate: metrics.deniedBlockRate === 1,
    p95LatencyMs: metrics.p95LatencyMs <= maximumBroadP95LatencyMs,
  }
  const output = {
    ok: Object.values(checks).every(Boolean),
    broad: true,
    marker,
    runtimeURL,
    embeddingModel,
    generationModel,
    thresholds: {
      minimumBroadCitationCoverage,
      minimumBroadAnswerFactCoverage,
      maximumBroadNoCitationAnswerRate,
      maximumBroadP95LatencyMs,
      triggerBroadRebuild,
      ragMaxContextChunks,
      ragMaxContextRunes,
    },
    metrics,
    checks,
    indexReadiness,
    perQuery,
  }
  console.log(JSON.stringify(output, null, 2))
  if (!output.ok) {
    process.exitCode = 1
  }
}

async function runQueryRewriteSmoke(suffix) {
  const deskName = `haohao-rag-white-desk-${suffix}.txt`
  const chairName = `haohao-rag-white-chair-${suffix}.txt`
  const shelfName = `haohao-rag-interior-shelf-${suffix}.txt`
  const unrelatedName = `haohao-rag-unrelated-${suffix}.txt`
  const desk = await uploadTextFile(deskName, [
    '白い机',
    '木製デスクは明るいインテリアに合わせやすい。',
    '天板はホワイトで、観葉植物とも相性がよい。',
  ].join('\n'))
  const chair = await uploadTextFile(chairName, [
    '白い椅子',
    'ダイニングチェアはミニマルな部屋と白い家具に合う。',
  ].join('\n'))
  const shelf = await uploadTextFile(shelfName, [
    '収納棚',
    '棚とソファは白いインテリアの余白を崩さず、観葉植物と合わせられる。',
  ].join('\n'))
  await uploadTextFile(unrelatedName, [
    'ゲーム企画メモ',
    'ロボットの武装と宇宙戦闘について。',
  ].join('\n'))

  await waitForSearchHit('白い インテリア 家具', 'hybrid', [deskName])

  await updatePolicy(true, {
    queryRewriteEnabled: false,
    queryRewriteMode: 'none',
  })
  const disabledStarted = performance.now()
  const disabled = await jsonRequest('/api/v1/drive/rag/query', 'POST', {
    query: '白いインテリアに合う家具は？',
    mode: 'hybrid',
    limit: 8,
  })
  const disabledLatencyMs = Math.round(performance.now() - disabledStarted)

  await updatePolicy(true, {
    queryRewriteEnabled: true,
    queryRewriteMode,
    queryRewriteMaxQueries: 6,
  })
  const enabledStarted = performance.now()
  const enabled = await jsonRequest('/api/v1/drive/rag/query', 'POST', {
    query: '白いインテリアに合う家具は？',
    mode: 'hybrid',
    limit: 8,
  })
  const enabledLatencyMs = Math.round(performance.now() - enabledStarted)

  const expectedPublicIDs = new Set([desk.publicId, chair.publicId, shelf.publicId])
  const enabledCitedExpected = (enabled.citations ?? []).filter((citation) => expectedPublicIDs.has(citation.filePublicId))
  const enabledMatchedExpected = (enabled.matches ?? []).filter((citation) => expectedPublicIDs.has(citation.filePublicId))
  const disabledMatchedExpected = (disabled.matches ?? []).filter((citation) => expectedPublicIDs.has(citation.filePublicId))
  const traceQueries = (enabled.retrievalTrace ?? []).map((item) => item.query)
  const retryTrace = (enabled.retrievalTrace ?? []).filter((item) => item.retry)

  return {
    ok: !enabled.blocked && enabledMatchedExpected.length > 0 && traceQueries.length > 1,
    query: '白いインテリアに合う家具は？',
    queryRewriteMode,
    expectedFiles: [deskName, chairName, shelfName],
    enabled: {
      latencyMs: enabledLatencyMs,
      blocked: Boolean(enabled.blocked),
      citationFilenames: (enabled.citations ?? []).map((citation) => citation.filename),
      matchFilenames: (enabled.matches ?? []).map((citation) => citation.filename),
      expectedCitationCount: enabledCitedExpected.length,
      expectedMatchCount: enabledMatchedExpected.length,
      retrievalTrace: enabled.retrievalTrace ?? [],
      retryReasons: retryTrace.map((item) => ({
        query: item.query,
        reason: item.retryReason,
        missingSignals: item.missingSignals ?? [],
      })),
    },
    disabled: {
      latencyMs: disabledLatencyMs,
      blocked: Boolean(disabled.blocked),
      citationFilenames: (disabled.citations ?? []).map((citation) => citation.filename),
      matchFilenames: (disabled.matches ?? []).map((citation) => citation.filename),
      expectedMatchCount: disabledMatchedExpected.length,
      retrievalTrace: disabled.retrievalTrace ?? [],
    },
    checks: {
      enabledNotBlocked: !enabled.blocked,
      enabledHasExpectedMatch: enabledMatchedExpected.length > 0,
      enabledUsesMultipleRetrievalQueries: traceQueries.length > 1,
      enabledTraceContainsExpansion: traceQueries.some((query) => query.includes('デスク') || query.includes('椅子') || query.includes('棚')),
      retryTraceHasReason: retryTrace.length === 0 || retryTrace.every((item) => item.retryReason && (item.missingSignals ?? []).length > 0),
    },
  }
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
      await updatePolicy(false)
    } catch (error) {
      console.error(`cleanup settings: ${error.message}`)
    }
  }
}

async function main() {
  await login()
  await updatePolicy(true)

  if (broadMode) {
    await runBroadSmoke()
    return
  }

  const suffix = Date.now()
  const invoiceName = `haohao-lmstudio-rag-invoice-${suffix}.txt`
  const noteName = `haohao-lmstudio-rag-note-${suffix}.txt`
  const invoice = await uploadTextFile(invoiceName, [
    '請求書',
    '請求番号: INV-RAG-2026-0001',
    '請求日: 2026-05-07',
    '振込期限: 2026-06-30',
    '支払先: 株式会社青葉商事',
    '税込合計: 128000円',
    '登録番号: T1234567890123',
  ].join('\n'))
  await uploadTextFile(noteName, [
    'プロダクト週報',
    '今週は管理画面のナビゲーション改善を進めた。',
    '来週はアクセシビリティ確認と表示速度の計測を行う。',
  ].join('\n'))

  await waitForSemanticHit(invoiceName, noteName)
  const rag = await jsonRequest('/api/v1/drive/rag/query', 'POST', {
    query: 'この請求書の支払期限と税込合計は？',
    mode: 'hybrid',
    limit: 8,
  })

  const queryRewrite = runQueryRewriteCase ? await runQueryRewriteSmoke(suffix) : null

  const answer = rag.answer ?? ''
  const output = {
    ok: true,
    runtimeURL,
    embeddingModel,
    generationModel,
    invoiceFile: invoice.publicId,
    answer,
    citations: rag.citations ?? [],
    matches: rag.matches ?? [],
    retrievalTrace: rag.retrievalTrace ?? [],
    queryRewrite,
    blocked: Boolean(rag.blocked),
    checks: {
      notBlocked: !rag.blocked,
      hasAnswer: answer.length > 0,
      citesInvoice: (rag.citations ?? []).some((item) => item.filePublicId === invoice.publicId),
      answerMentionsDeadline: answer.includes('2026-06-30') || answer.includes('6月30日'),
      answerMentionsTotal: answer.includes('128000') || answer.includes('128,000') || answer.includes('128,000円'),
      queryRewriteCase: !runQueryRewriteCase || Boolean(queryRewrite?.ok),
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
