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

function taxonomyGraph(file) {
  const nodes = [
    node('input', 'input', 'Drive Excel taxonomy', 60, 160, {
      sourceKind: 'drive_file',
      inputMode: 'spreadsheet',
      filePublicIds: [file.publicId],
      headerRow: 0,
      columns: ['taxonomy_id', 'level_1', 'level_2', 'level_3', 'level_4', 'level_5', 'level_6'],
      maxRows: 100000,
    }),
    node('filter_valid_id', 'transform', '空ID行を除外', 340, 160, {
      operation: 'filter',
      conditions: [
        { column: 'taxonomy_id', operator: 'regex', value: '^[0-9]+$' },
      ],
    }),
    node('clean', 'clean', '重複IDを整理', 620, 160, {
      rules: [
        { operation: 'dedupe', keys: ['taxonomy_id'], orderBy: 'row_number' },
      ],
    }),
    node('normalize', 'normalize', 'カテゴリ名の表記ゆれを整える', 900, 80, {
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
    node('schema_mapping', 'schema_mapping', 'taxonomy master schema', 1180, 160, {
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
    node('schema_inference', 'schema_inference', '分類マスタ schema inference', 1460, 160, {
      columns: ['taxonomy_id', 'category_l1', 'category_l2', 'category_l3', 'category_l4', 'category_l5', 'category_l6'],
      sampleLimit: 500,
    }),
    node('quality_report', 'quality_report', '分類階層品質レポート', 1740, 160, {
      columns: ['taxonomy_id', 'category_l1', 'category_l2', 'category_l3', 'category_l4', 'category_l5', 'category_l6'],
      outputMode: 'row_summary',
    }),
    node('output', 'output', 'taxonomy master ja-JP sample', 2020, 160, {
      displayName: 'taxonomy master ja-JP sample',
      tableName: 'sample_taxonomy_with_ids_ja_jp',
      writeMode: 'replace',
      orderBy: ['taxonomy_id'],
    }),
  ]
  return {
    nodes,
    edges: [
      { id: 'input-filter_valid_id', source: 'input', target: 'filter_valid_id' },
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
      name: 'サンプル: Excel分類マスタから taxonomy work table 作成',
      description: [
        'taxonomy-with-ids.ja-JP.xls を Drive Excel input として読み込み、ID付き階層分類マスタを構造化するサンプルです。',
        'EC 商品カテゴリ、検索ファセット、商品登録ルールなどで使う taxonomy master の整備を想定しています。',
      ].join('\n'),
    }),
  })
  const version = await request(`/api/v1/data-pipelines/${pipeline.publicId}/versions`, {
    method: 'POST',
    body: JSON.stringify({ graph: taxonomyGraph(file) }),
  })
  const published = await request(`/api/v1/data-pipeline-versions/${version.publicId}/publish`, {
    method: 'POST',
  })
  return { pipeline, version: published }
}

async function main() {
  await login()
  const file = await uploadTaxonomy()
  const created = await createPipeline(file)
  console.log(JSON.stringify({
    filePublicId: file.publicId,
    pipelinePublicId: created.pipeline.publicId,
    versionPublicId: created.version.publicId,
    pipelineName: created.pipeline.name,
    versionStatus: created.version.status,
  }, null, 2))
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
