import { openAsBlob } from 'node:fs'
import { basename, resolve } from 'node:path'

const baseUrl = process.env.HAOHAO_BASE_URL ?? 'http://127.0.0.1:8080'
const email = process.env.HAOHAO_DEMO_EMAIL ?? 'demo@example.com'
const password = process.env.HAOHAO_DEMO_PASSWORD ?? 'changeme123'
const tenantSlug = process.env.HAOHAO_TENANT_SLUG ?? 'acme'
const pokedexPath = resolve(process.env.HAOHAO_POKEDEX_JSON ?? 'scripts/pokedex.json')

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

async function uploadPokedex() {
  const form = new FormData()
  const file = await openAsBlob(pokedexPath, { type: 'application/json' })
  form.append('file', file, basename(pokedexPath))
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

function jsonField(column, pathSegments, extra = {}) {
  return { column, pathSegments, ...extra }
}

function pokedexGraph(file) {
  const tableColumns = [
    'pokemon_id',
    'name_en',
    'name_ja',
    'name_zh',
    'name_fr',
    'primary_type',
    'secondary_type',
    'types',
    'species',
    'description',
    'height',
    'weight',
    'egg_groups',
    'ability_1',
    'ability_1_hidden',
    'ability_2',
    'ability_2_hidden',
    'gender_ratio',
    'hp',
    'attack',
    'defense',
    'sp_attack',
    'sp_defense',
    'speed',
    'evolution_prev_id',
    'evolution_prev_condition',
    'evolution_next_id',
    'evolution_next_condition',
    'sprite_url',
    'thumbnail_url',
    'hires_url',
  ]
  const nodes = [
    node('input', 'input', 'Drive JSON pokedex', 60, 160, {
      sourceKind: 'drive_file',
      inputMode: 'json',
      filePublicIds: [file.publicId],
      recordPath: '$',
      maxRows: 100000,
      includeSourceMetadataColumns: true,
      fields: [
        jsonField('pokemon_id', ['id']),
        jsonField('name_en', ['name', 'english']),
        jsonField('name_ja', ['name', 'japanese']),
        jsonField('name_zh', ['name', 'chinese']),
        jsonField('name_fr', ['name', 'french']),
        jsonField('primary_type', ['type', '0']),
        jsonField('secondary_type', ['type', '1']),
        jsonField('types', ['type'], { join: '|' }),
        jsonField('species', ['species']),
        jsonField('description', ['description']),
        jsonField('height', ['profile', 'height']),
        jsonField('weight', ['profile', 'weight']),
        jsonField('egg_groups', ['profile', 'egg'], { join: '|' }),
        jsonField('ability_1', ['profile', 'ability', '0', '0']),
        jsonField('ability_1_hidden', ['profile', 'ability', '0', '1']),
        jsonField('ability_2', ['profile', 'ability', '1', '0']),
        jsonField('ability_2_hidden', ['profile', 'ability', '1', '1']),
        jsonField('gender_ratio', ['profile', 'gender']),
        jsonField('hp', ['base', 'HP']),
        jsonField('attack', ['base', 'Attack']),
        jsonField('defense', ['base', 'Defense']),
        jsonField('sp_attack', ['base', 'Sp. Attack']),
        jsonField('sp_defense', ['base', 'Sp. Defense']),
        jsonField('speed', ['base', 'Speed']),
        jsonField('evolution_prev_id', ['evolution', 'prev', '0']),
        jsonField('evolution_prev_condition', ['evolution', 'prev', '1']),
        jsonField('evolution_next_id', ['evolution', 'next', '0', '0']),
        jsonField('evolution_next_condition', ['evolution', 'next', '0', '1']),
        jsonField('sprite_url', ['image', 'sprite']),
        jsonField('thumbnail_url', ['image', 'thumbnail']),
        jsonField('hires_url', ['image', 'hires']),
      ],
    }),
    node('clean', 'clean', 'pokemon id dedupe', 340, 160, {
      rules: [
        { operation: 'dedupe', keys: ['pokemon_id'], orderBy: 'pokemon_id' },
      ],
    }),
    node('normalize', 'normalize', 'text normalization', 620, 160, {
      rules: [
        { column: 'name_en', operation: 'normalize_spaces' },
        { column: 'species', operation: 'normalize_spaces' },
        { column: 'height', operation: 'normalize_spaces' },
        { column: 'weight', operation: 'normalize_spaces' },
      ],
    }),
    node('schema_mapping', 'schema_mapping', 'pokedex structured schema', 900, 160, {
      mappings: tableColumns.map((column) => ({
        sourceColumn: column,
        targetColumn: column,
        cast: ['pokemon_id', 'hp', 'attack', 'defense', 'sp_attack', 'sp_defense', 'speed'].includes(column) ? 'int64' : 'string',
        required: ['pokemon_id', 'name_en'].includes(column),
      })),
    }),
    node('schema_inference', 'schema_inference', 'schema inference', 1180, 80, {
      columns: tableColumns,
      sampleLimit: 100000,
    }),
    node('quality_report', 'quality_report', 'quality report', 1460, 160, {
      columns: ['pokemon_id', 'name_en', 'primary_type', 'hp', 'attack', 'defense', 'speed'],
      outputMode: 'row_summary',
    }),
    node('output', 'output', 'pokedex structured sample', 1740, 160, {
      displayName: 'pokedex structured sample',
      tableName: 'sample_pokedex_structured',
      writeMode: 'replace',
      orderBy: ['pokemon_id'],
    }),
  ]
  return {
    nodes,
    edges: [
      { id: 'input-clean', source: 'input', target: 'clean' },
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
      name: 'サンプル: ネストJSON Pokedex を構造化テーブル化',
      description: [
        'scripts/pokedex.json を Drive JSON input として読み込み、name/base/profile/evolution/image などのネストした値を列へ展開します。',
        '大量の配列 JSON を Bronze の Drive file から Silver/Work table へ整形するサンプル pipeline です。',
      ].join('\n'),
    }),
  })
  const version = await request(`/api/v1/data-pipelines/${pipeline.publicId}/versions`, {
    method: 'POST',
    body: JSON.stringify({ graph: pokedexGraph(file) }),
  })
  const published = await request(`/api/v1/data-pipeline-versions/${version.publicId}/publish`, {
    method: 'POST',
  })
  return { pipeline, version: published }
}

async function main() {
  await login()
  const file = await uploadPokedex()
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
