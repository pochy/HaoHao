import { openAsBlob } from 'node:fs'
import { basename, resolve } from 'node:path'

const baseUrl = process.env.HAOHAO_BASE_URL ?? 'http://127.0.0.1:8080'
const email = process.env.HAOHAO_DEMO_EMAIL ?? 'demo@example.com'
const password = process.env.HAOHAO_DEMO_PASSWORD ?? 'changeme123'
const tenantSlug = process.env.HAOHAO_TENANT_SLUG ?? 'acme'
const pokedexPath = resolve(process.env.HAOHAO_POKEDEX_JSON ?? 'samples/pokedex.json')

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

async function uploadJSON(path) {
  const form = new FormData()
  const file = await openAsBlob(path, { type: 'application/json' })
  form.append('file', file, basename(path))
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

function jsonField(column, path, extra = {}) {
  return { column, path, ...extra }
}

function mapping(sourceColumn, targetColumn = sourceColumn, cast = 'string', required = false) {
  return { sourceColumn, targetColumn, cast, required }
}

function graph(file) {
  const columns = [
    'pokemon_id',
    'name_en',
    'primary_type',
    'secondary_type',
    'types',
    'species',
    'height',
    'weight',
    'hp',
    'attack',
    'defense',
    'sp_attack',
    'sp_defense',
    'speed',
  ]
  return {
    nodes: [
      node('input', 'input', 'Drive JSON raw records', 60, 160, {
        sourceKind: 'drive_file',
        inputMode: 'json',
        filePublicIds: [file.publicId],
        recordPath: '$',
        maxRows: 100000,
        includeSourceMetadataColumns: true,
        includeRawRecord: true,
        fields: [],
      }),
      node('json_extract', 'json_extract', 'Extract nested Pokemon JSON', 340, 160, {
        sourceColumn: 'raw_record_json',
        recordPath: '$',
        maxRows: 100000,
        includeSourceColumns: true,
        includeRawRecord: false,
        fields: [
          jsonField('pokemon_id', 'id'),
          jsonField('name_en', 'name.english'),
          jsonField('primary_type', 'type.0'),
          jsonField('secondary_type', 'type.1'),
          jsonField('types', 'type', { join: '|' }),
          jsonField('species', 'species'),
          jsonField('height', 'profile.height'),
          jsonField('weight', 'profile.weight'),
          jsonField('hp', 'base.HP'),
          jsonField('attack', 'base.Attack'),
          jsonField('defense', 'base.Defense'),
          jsonField('sp_attack', 'base.Sp. Attack'),
          jsonField('sp_defense', 'base.Sp. Defense'),
          jsonField('speed', 'base.Speed'),
        ],
      }),
      node('clean', 'clean', 'pokemon id dedupe', 620, 160, {
        rules: [
          { operation: 'dedupe', keys: ['pokemon_id'], orderBy: 'pokemon_id' },
        ],
      }),
      node('schema_mapping', 'schema_mapping', 'json extract schema', 900, 160, {
        mappings: columns.map((column) => mapping(
          column,
          column,
          ['pokemon_id', 'hp', 'attack', 'defense', 'sp_attack', 'sp_defense', 'speed'].includes(column) ? 'int64' : 'string',
          ['pokemon_id', 'name_en'].includes(column),
        )),
      }),
      node('quality_report', 'quality_report', 'quality report', 1180, 160, {
        columns: ['pokemon_id', 'name_en', 'primary_type', 'hp', 'attack', 'defense', 'speed'],
        outputMode: 'row_summary',
      }),
      node('output', 'output', 'json extract pokedex sample', 1460, 160, {
        displayName: 'json extract pokedex sample',
        tableName: 'sample_pokedex_json_extract',
        writeMode: 'replace',
        orderBy: ['pokemon_id'],
      }),
    ],
    edges: [
      { id: 'input-json_extract', source: 'input', target: 'json_extract' },
      { id: 'json_extract-clean', source: 'json_extract', target: 'clean' },
      { id: 'clean-schema_mapping', source: 'clean', target: 'schema_mapping' },
      { id: 'schema_mapping-quality_report', source: 'schema_mapping', target: 'quality_report' },
      { id: 'quality_report-output', source: 'quality_report', target: 'output' },
    ],
  }
}

async function createPipeline(file) {
  const pipeline = await request('/api/v1/data-pipelines', {
    method: 'POST',
    headers: { 'Idempotency-Key': crypto.randomUUID() },
    body: JSON.stringify({
      name: 'サンプル: json_extract で Pokedex JSON を構造化',
      description: [
        'Drive JSON input は raw_record_json だけを生成し、後段の json_extract step で name/type/profile/base のネスト値を列へ展開します。',
        'JSON 抽出を input node ではなく独立 step として使うサンプル pipeline です。',
      ].join('\n'),
    }),
  })
  const version = await request(`/api/v1/data-pipelines/${pipeline.publicId}/versions`, {
    method: 'POST',
    body: JSON.stringify({ graph: graph(file) }),
  })
  const published = await request(`/api/v1/data-pipeline-versions/${version.publicId}/publish`, {
    method: 'POST',
  })
  return { pipeline, version: published }
}

async function main() {
  await login()
  const file = await uploadJSON(pokedexPath)
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
