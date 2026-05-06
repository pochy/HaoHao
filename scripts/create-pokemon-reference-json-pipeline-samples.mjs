import { openAsBlob } from 'node:fs'
import { basename, resolve } from 'node:path'

const baseUrl = process.env.HAOHAO_BASE_URL ?? 'http://127.0.0.1:8080'
const email = process.env.HAOHAO_DEMO_EMAIL ?? 'demo@example.com'
const password = process.env.HAOHAO_DEMO_PASSWORD ?? 'changeme123'
const tenantSlug = process.env.HAOHAO_TENANT_SLUG ?? 'acme'

const samplePaths = {
  items: resolve(process.env.HAOHAO_ITEMS_JSON ?? 'samples/items.json'),
  moves: resolve(process.env.HAOHAO_MOVES_JSON ?? 'samples/moves.json'),
  types: resolve(process.env.HAOHAO_TYPES_JSON ?? 'samples/types.json'),
}

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

function jsonField(column, pathSegments, extra = {}) {
  return { column, pathSegments, ...extra }
}

function jsonInputConfig(file, fields) {
  return {
    sourceKind: 'drive_file',
    inputMode: 'json',
    filePublicIds: [file.publicId],
    recordPath: '$',
    maxRows: 100000,
    includeSourceMetadataColumns: true,
    fields,
  }
}

function simpleJSONGraph({ file, inputLabel, outputLabel, tableName, orderBy, fields, mappings, qualityColumns }) {
  const nodes = [
    node('input', 'input', inputLabel, 60, 160, jsonInputConfig(file, fields)),
    node('clean', 'clean', 'dedupe by key', 340, 160, {
      rules: [
        { operation: 'dedupe', keys: orderBy, orderBy: orderBy[0] },
      ],
    }),
    node('normalize', 'normalize', 'normalize labels', 620, 160, {
      rules: [
        { column: mappings[1]?.sourceColumn ?? mappings[0].sourceColumn, operation: 'normalize_spaces' },
      ],
    }),
    node('schema_mapping', 'schema_mapping', 'structured schema', 900, 160, { mappings }),
    node('schema_inference', 'schema_inference', 'schema inference', 1180, 80, {
      columns: mappings.map((item) => item.targetColumn),
      sampleLimit: 100000,
    }),
    node('quality_report', 'quality_report', 'quality report', 1460, 160, {
      columns: qualityColumns,
      outputMode: 'row_summary',
    }),
    node('output', 'output', outputLabel, 1740, 160, {
      displayName: outputLabel,
      tableName,
      writeMode: 'replace',
      orderBy,
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

function mapping(sourceColumn, targetColumn = sourceColumn, cast = 'string', required = false) {
  return { sourceColumn, targetColumn, cast, required }
}

function itemsGraph(file) {
  const mappings = [
    mapping('item_id', 'item_id', 'int64', true),
    mapping('name_en', 'name_en', 'string', true),
    mapping('name_ja'),
    mapping('name_zh'),
    mapping('item_type'),
    mapping('description'),
  ]
  return simpleJSONGraph({
    file,
    inputLabel: 'Drive JSON items',
    outputLabel: 'pokemon items structured sample',
    tableName: 'sample_pokemon_items_structured',
    orderBy: ['item_id'],
    fields: [
      jsonField('item_id', ['id']),
      jsonField('item_type', ['type']),
      jsonField('description', ['description']),
      jsonField('name_en', ['name', 'english']),
      jsonField('name_ja', ['name', 'japanese']),
      jsonField('name_zh', ['name', 'chinese']),
    ],
    mappings,
    qualityColumns: ['item_id', 'name_en', 'item_type'],
  })
}

function movesGraph(file) {
  const mappings = [
    mapping('move_id', 'move_id', 'int64', true),
    mapping('name_en', 'name_en', 'string', true),
    mapping('name_ja'),
    mapping('name_zh'),
    mapping('name_fr'),
    mapping('move_type'),
    mapping('category'),
    mapping('pp'),
    mapping('power'),
    mapping('accuracy'),
  ]
  return simpleJSONGraph({
    file,
    inputLabel: 'Drive JSON moves',
    outputLabel: 'pokemon moves structured sample',
    tableName: 'sample_pokemon_moves_structured',
    orderBy: ['move_id'],
    fields: [
      jsonField('move_id', ['id']),
      jsonField('name_en', ['name', 'english']),
      jsonField('name_ja', ['name', 'japanese']),
      jsonField('name_zh', ['name', 'chinese']),
      jsonField('name_fr', ['name', 'french']),
      jsonField('move_type', ['type']),
      jsonField('category', ['category']),
      jsonField('pp', ['pp']),
      jsonField('power', ['power']),
      jsonField('accuracy', ['accuracy']),
    ],
    mappings,
    qualityColumns: ['move_id', 'name_en', 'move_type', 'category', 'pp'],
  })
}

function typesGraph(file) {
  const mappings = [
    mapping('type_en', 'type_en', 'string', true),
    mapping('type_ja'),
    mapping('type_zh'),
    mapping('effective_types'),
    mapping('ineffective_types'),
    mapping('no_effect_types'),
  ]
  return simpleJSONGraph({
    file,
    inputLabel: 'Drive JSON type chart',
    outputLabel: 'pokemon type chart structured sample',
    tableName: 'sample_pokemon_type_chart_structured',
    orderBy: ['type_en'],
    fields: [
      jsonField('type_en', ['english']),
      jsonField('type_ja', ['japanese']),
      jsonField('type_zh', ['chinese']),
      jsonField('effective_types', ['effective'], { join: '|' }),
      jsonField('ineffective_types', ['ineffective'], { join: '|' }),
      jsonField('no_effect_types', ['no_effect'], { join: '|' }),
    ],
    mappings,
    qualityColumns: ['type_en', 'effective_types', 'ineffective_types', 'no_effect_types'],
  })
}

async function createPipeline(name, description, graph) {
  const pipeline = await request('/api/v1/data-pipelines', {
    method: 'POST',
    headers: { 'Idempotency-Key': crypto.randomUUID() },
    body: JSON.stringify({ name, description }),
  })
  const version = await request(`/api/v1/data-pipelines/${pipeline.publicId}/versions`, {
    method: 'POST',
    body: JSON.stringify({ graph }),
  })
  const published = await request(`/api/v1/data-pipeline-versions/${version.publicId}/publish`, {
    method: 'POST',
  })
  return { pipeline, version: published }
}

async function createSample(key, path, graphFactory, name, description) {
  const file = await uploadJSON(path)
  const created = await createPipeline(name, description, graphFactory(file))
  return {
    key,
    filePublicId: file.publicId,
    pipelinePublicId: created.pipeline.publicId,
    versionPublicId: created.version.publicId,
    pipelineName: created.pipeline.name,
    versionStatus: created.version.status,
  }
}

async function main() {
  await login()
  const results = []
  results.push(await createSample(
    'items',
    samplePaths.items,
    itemsGraph,
    'サンプル: Items JSON を構造化テーブル化',
    'samples/items.json を Drive JSON input として読み込み、item id、type、多言語名、説明を列へ展開する sample pipeline です。',
  ))
  results.push(await createSample(
    'moves',
    samplePaths.moves,
    movesGraph,
    'サンプル: Moves JSON を構造化テーブル化',
    'samples/moves.json を Drive JSON input として読み込み、move id、多言語名、type、category、pp、power、accuracy を列へ展開する sample pipeline です。',
  ))
  results.push(await createSample(
    'types',
    samplePaths.types,
    typesGraph,
    'サンプル: Types JSON を構造化テーブル化',
    'samples/types.json を Drive JSON input として読み込み、type 名と effective / ineffective / no_effect の配列をテーブル列へ展開する sample pipeline です。',
  ))
  console.log(JSON.stringify({ samples: results }, null, 2))
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
