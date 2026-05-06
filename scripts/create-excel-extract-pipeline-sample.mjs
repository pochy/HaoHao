import { openAsBlob } from 'node:fs'
import { basename, resolve } from 'node:path'

const baseUrl = process.env.HAOHAO_BASE_URL ?? 'http://127.0.0.1:8080'
const email = process.env.HAOHAO_DEMO_EMAIL ?? 'demo@example.com'
const password = process.env.HAOHAO_DEMO_PASSWORD ?? 'changeme123'
const tenantSlug = process.env.HAOHAO_TENANT_SLUG ?? 'acme'
const taxonomyPath = resolve(process.env.HAOHAO_TAXONOMY_XLS ?? 'samples/taxonomy-with-ids.ja-JP.xls')

const cookieJar = new Map()

function rememberCookies(response) {
  const setCookies = response.headers.getSetCookie?.() ?? []
  for (const item of setCookies) {
    const [pair] = item.split(';')
    const index = pair.indexOf('=')
    if (index > 0) {
      cookieJar.set(pair.slice(0, index), pair.slice(index + 1))
    }
  }
}

function cookieHeader() {
  return [...cookieJar.entries()].map(([key, value]) => `${key}=${value}`).join('; ')
}

function csrfToken() {
  return decodeURIComponent(cookieJar.get('XSRF-TOKEN') ?? '')
}

async function request(path, options = {}) {
  const headers = new Headers(options.headers ?? {})
  headers.set('Accept', 'application/json')
  if (cookieJar.size > 0) {
    headers.set('Cookie', cookieHeader())
  }
  if (options.body && !(options.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  if (!['GET', 'HEAD', 'OPTIONS'].includes((options.method ?? 'GET').toUpperCase())) {
    headers.set('X-CSRF-Token', csrfToken())
  }
  const response = await fetch(`${baseUrl}${path}`, { ...options, headers })
  rememberCookies(response)
  if (!response.ok) {
    const text = await response.text()
    throw new Error(`${options.method ?? 'GET'} ${path} failed: ${response.status} ${text}`)
  }
  if (response.status === 204) {
    return undefined
  }
  return response.json()
}

function sleep(ms) {
  return new Promise((resolveSleep) => setTimeout(resolveSleep, ms))
}

async function login() {
  await request('/api/v1/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
  await request('/api/v1/session/tenant', {
    method: 'POST',
    body: JSON.stringify({ tenantSlug }),
  })
}

async function uploadTaxonomy() {
  const form = new FormData()
  const file = await openAsBlob(taxonomyPath, { type: 'application/vnd.ms-excel' })
  form.append('file', file, basename(taxonomyPath))
  return request('/api/v1/drive/files', { method: 'POST', body: form })
}

function node(id, stepType, label, x, y, config = {}) {
  return {
    id,
    type: 'pipelineStep',
    position: { x, y },
    data: { stepType, label, config },
  }
}

function taxonomyExcelExtractGraph(file) {
  const taxonomyColumns = ['taxonomy_id', 'level_1', 'level_2', 'level_3', 'level_4', 'level_5', 'level_6']
  const nodes = [
    node('input', 'input', 'Drive Excel file', 60, 75, {
      sourceKind: 'drive_file',
      filePublicIds: [file.publicId],
    }),
    node('excel_extract', 'excel_extract', 'Excel taxonomy rows', 360, 75, {
      sourceFileColumn: 'file_public_id',
      sheetIndex: 0,
      headerRow: 0,
      columns: taxonomyColumns,
      includeSourceColumns: false,
      includeSourceMetadataColumns: true,
      maxRows: 100000,
    }),
    node('filter_valid_id', 'transform', '空ID行を除外', 660, 75, {
      operation: 'filter',
      conditions: [
        { column: 'taxonomy_id', operator: 'regex', value: '^[0-9]+$' },
      ],
    }),
    node('clean', 'clean', '重複IDを整理', 930, 75, {
      rules: [
        { operation: 'dedupe', keys: ['taxonomy_id'], orderBy: 'row_number' },
      ],
    }),
    node('normalize', 'normalize', 'カテゴリ名の表記ゆれを整える', 1185, 75, {
      rules: [
        { column: 'taxonomy_id', operation: 'trim' },
        { column: 'level_1', operation: 'normalize_spaces' },
        { column: 'level_2', operation: 'normalize_spaces' },
        { column: 'level_3', operation: 'normalize_spaces' },
        { column: 'level_4', operation: 'normalize_spaces' },
        { column: 'level_5', operation: 'normalize_spaces' },
        { column: 'level_6', operation: 'normalize_spaces' },
      ],
    }),
    node('schema_mapping', 'schema_mapping', 'taxonomy master schema', 1545, 75, {
      mappings: [
        { sourceColumn: 'taxonomy_id', targetColumn: 'taxonomy_id', cast: 'string', required: true },
        { sourceColumn: 'level_1', targetColumn: 'category_l1', cast: 'string', required: true },
        { sourceColumn: 'level_2', targetColumn: 'category_l2', cast: 'string' },
        { sourceColumn: 'level_3', targetColumn: 'category_l3', cast: 'string' },
        { sourceColumn: 'level_4', targetColumn: 'category_l4', cast: 'string' },
        { sourceColumn: 'level_5', targetColumn: 'category_l5', cast: 'string' },
        { sourceColumn: 'level_6', targetColumn: 'category_l6', cast: 'string' },
        { sourceColumn: 'sheet_name', targetColumn: 'source_sheet', cast: 'string' },
        { sourceColumn: 'row_number', targetColumn: 'source_row_number', cast: 'string' },
      ],
    }),
    node('schema_inference', 'schema_inference', '分類マスタ schema inference', 1875, 75, {
      columns: ['taxonomy_id', 'category_l1', 'category_l2', 'category_l3', 'category_l4', 'category_l5', 'category_l6'],
      sampleLimit: 500,
    }),
    node('quality_report', 'quality_report', '分類階層品質レポート', 2220, 75, {
      columns: ['taxonomy_id', 'category_l1', 'category_l2', 'category_l3', 'category_l4', 'category_l5', 'category_l6'],
      outputMode: 'row_summary',
    }),
    node('output', 'output', 'taxonomy master ja-JP excel_extract sample', 2520, 75, {
      displayName: 'taxonomy master ja-JP excel_extract sample',
      tableName: 'sample_taxonomy_excel_extract_ja_jp',
      writeMode: 'replace',
      orderBy: ['taxonomy_id'],
    }),
  ]
  return {
    nodes,
    edges: [
      { id: 'input-excel_extract', source: 'input', target: 'excel_extract' },
      { id: 'excel_extract-filter_valid_id', source: 'excel_extract', target: 'filter_valid_id' },
      { id: 'filter_valid_id-clean', source: 'filter_valid_id', target: 'clean' },
      { id: 'clean-normalize', source: 'clean', target: 'normalize' },
      { id: 'normalize-schema_mapping', source: 'normalize', target: 'schema_mapping' },
      { id: 'schema_mapping-schema_inference', source: 'schema_mapping', target: 'schema_inference' },
      { id: 'schema_inference-quality_report', source: 'schema_inference', target: 'quality_report' },
      { id: 'quality_report-output', source: 'quality_report', target: 'output' },
    ],
  }
}

async function createPipeline(file) {
  const pipeline = await request('/api/v1/data-pipelines', {
    method: 'POST',
    headers: { 'Idempotency-Key': crypto.randomUUID() },
    body: JSON.stringify({
      name: 'サンプル: excel_extract で Excel 分類マスタを構造化',
      description: [
        'taxonomy-with-ids.ja-JP.xls を Drive file input として受け取り、後段の excel_extract step で行と列を抽出するサンプルです。',
        '参照元の Drive Excel input サンプルと同じ taxonomy master 整備フローを、独立した excel_extract step で構成しています。',
      ].join('\n'),
    }),
  })
  const version = await request(`/api/v1/data-pipelines/${pipeline.publicId}/versions`, {
    method: 'POST',
    body: JSON.stringify({ graph: taxonomyExcelExtractGraph(file) }),
  })
  const published = await request(`/api/v1/data-pipeline-versions/${version.publicId}/publish`, {
    method: 'POST',
  })
  return { pipeline, version: published }
}

async function createRun(versionPublicId) {
  return request(`/api/v1/data-pipeline-versions/${versionPublicId}/runs`, {
    method: 'POST',
    headers: { 'Idempotency-Key': crypto.randomUUID() },
  })
}

async function waitForRun(pipelinePublicId, runPublicId) {
  for (let attempt = 0; attempt < 120; attempt += 1) {
    const runs = await request(`/api/v1/data-pipelines/${pipelinePublicId}/runs?limit=20`)
    const run = (runs.items ?? []).find((item) => item.publicId === runPublicId)
    if (run && ['completed', 'failed'].includes(run.status)) {
      return run
    }
    await sleep(1000)
  }
  throw new Error(`Timed out waiting for run ${runPublicId}`)
}

async function main() {
  await login()
  const file = await uploadTaxonomy()
  const created = await createPipeline(file)
  const requestedRun = await createRun(created.version.publicId)
  const run = await waitForRun(created.pipeline.publicId, requestedRun.publicId)
  console.log(JSON.stringify({
    filePublicId: file.publicId,
    pipelinePublicId: created.pipeline.publicId,
    versionPublicId: created.version.publicId,
    pipelineName: created.pipeline.name,
    versionStatus: created.version.status,
    runPublicId: run.publicId,
    runStatus: run.status,
    rowCount: run.rowCount,
    errorSummary: run.errorSummary,
  }, null, 2))
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
