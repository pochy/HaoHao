const baseURL = (process.env.HAOHAO_SMOKE_BASE_URL ?? process.env.HAOHAO_BASE_URL ?? 'http://127.0.0.1:8080').replace(/\/+$/, '')
const frontendURL = (process.env.HAOHAO_FRONTEND_BASE_URL ?? 'http://127.0.0.1:5173').replace(/\/+$/, '')
const tenantSlug = process.env.HAOHAO_TENANT_SLUG ?? 'acme'
const email = process.env.HAOHAO_SMOKE_EMAIL ?? process.env.HAOHAO_DEMO_EMAIL ?? 'demo@example.com'
const password = process.env.HAOHAO_SMOKE_PASSWORD ?? process.env.HAOHAO_DEMO_PASSWORD ?? 'changeme123'
const configuredWorkspacePublicID = process.env.HAOHAO_DRIVE_WORKSPACE_PUBLIC_ID ?? ''
const pollAttempts = Number(process.env.HAOHAO_SMOKE_POLL_ATTEMPTS ?? '60')
const pollDelayMs = Number(process.env.HAOHAO_SMOKE_POLL_DELAY_MS ?? '1500')
const cleanupDriveFile = process.env.HAOHAO_SMOKE_CLEANUP_DRIVE_FILE === '1'
const requestedScenario = (process.argv[2] ?? process.env.HAOHAO_SMOKE_SCENARIO ?? 'json').toLowerCase()

const cookies = new Map()
const created = {
  driveFilePublicIds: [],
  pipelinePublicIds: [],
  versionPublicIds: [],
  runPublicIds: [],
}

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
  return decodeURIComponent(cookies.get('XSRF-TOKEN') ?? '')
}

async function request(path, options = {}) {
  const headers = new Headers(options.headers ?? {})
  headers.set('Accept', 'application/json')
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

async function jsonRequest(path, method, body, headers = {}) {
  return request(path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken(),
      ...headers,
    },
    body: JSON.stringify(body),
  })
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
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

async function resolveWorkspacePublicID() {
  if (configuredWorkspacePublicID.trim()) {
    return configuredWorkspacePublicID.trim()
  }
  const workspaces = await request('/api/v1/drive/workspaces?limit=100')
  const workspace = workspaces.items?.[0]
  if (!workspace?.publicId) {
    throw new Error('no Drive workspace is available for smoke upload')
  }
  return workspace.publicId
}

async function uploadFile(workspacePublicId, filename, contentType, body) {
  const form = new FormData()
  form.set('workspacePublicId', workspacePublicId)
  form.set('file', new Blob([body], { type: contentType }), filename)
  const file = await request('/api/v1/drive/files', {
    method: 'POST',
    headers: { 'X-CSRF-Token': csrfToken() },
    body: form,
  })
  created.driveFilePublicIds.push(file.publicId)
  return file
}

async function uploadJSONFile(workspacePublicId) {
  const rows = [
    { id: '1', name: 'Alpha', amount: 10, status: 'ready', updated_at: '2026-05-01T00:00:00Z' },
    { id: '2', name: 'Beta', amount: 20, status: 'ready', updated_at: '2026-05-02T12:00:00Z' },
    { id: '3', name: '', amount: -5, status: 'hold', updated_at: '2026-05-03T00:00:00Z' },
  ]
  return uploadFile(workspacePublicId, `data-pipeline-smoke-${Date.now()}.json`, 'application/json', JSON.stringify(rows, null, 2))
}

async function uploadSnapshotJSONFile(workspacePublicId) {
  const rows = [
    { id: '1', name: 'Alpha', status: 'draft', updated_at: '2026-05-01T00:00:00Z' },
    { id: '1', name: 'Alpha', status: 'ready', updated_at: '2026-05-03T00:00:00Z' },
    { id: '2', name: 'Beta', status: 'ready', updated_at: '2026-05-02T00:00:00Z' },
  ]
  return uploadFile(workspacePublicId, `data-pipeline-snapshot-smoke-${Date.now()}.json`, 'application/json', JSON.stringify(rows, null, 2))
}

async function uploadSnapshotChangedJSONFile(workspacePublicId) {
  const rows = [
    { id: '1', name: 'Alpha', status: 'shipped', updated_at: '2026-05-04T00:00:00Z' },
    { id: '2', name: 'Beta', status: 'ready', updated_at: '2026-05-02T00:00:00Z' },
  ]
  return uploadFile(workspacePublicId, `data-pipeline-snapshot-changed-smoke-${Date.now()}.json`, 'application/json', JSON.stringify(rows, null, 2))
}

async function uploadSnapshotLateJSONFile(workspacePublicId) {
  const rows = [
    { id: '1', name: 'Alpha', status: 'review', updated_at: '2026-05-02T00:00:00Z' },
  ]
  return uploadFile(workspacePublicId, `data-pipeline-snapshot-late-smoke-${Date.now()}.json`, 'application/json', JSON.stringify(rows, null, 2))
}

async function uploadTextFile(workspacePublicId) {
  const text = [
    'invoice_id,customer,amount,confidence',
    'INV-1,Alpha,120,0.95',
    'INV-2,Beta,240,0.55',
    'INV-3,Gamma,360,0.72',
  ].join('\n')
  return uploadFile(workspacePublicId, `data-pipeline-smoke-${Date.now()}.txt`, 'text/plain', text)
}

async function uploadFieldReviewTextFile(workspacePublicId) {
  const text = [
    'Invoice ID: INV-100',
    'Customer: Alpha Trading',
    'Amount:',
  ].join('\n')
  return uploadFile(workspacePublicId, `data-pipeline-field-review-smoke-${Date.now()}.txt`, 'text/plain', text)
}

async function uploadTableReviewTextFile(workspacePublicId) {
  const text = [
    'invoice_id,customer,amount,status',
    'INV-1,Alpha,120,ready',
    'INV-2,Beta,,needs_review',
  ].join('\n')
  return uploadFile(workspacePublicId, `data-pipeline-table-review-smoke-${Date.now()}.txt`, 'text/plain', text)
}

async function uploadSchemaMappingReviewJSONFile(workspacePublicId) {
  const rows = [
    { invoice_number: 'INV-200', total: '345.67', state: 'ready' },
  ]
  return uploadFile(workspacePublicId, `data-pipeline-schema-mapping-review-smoke-${Date.now()}.json`, 'application/json', JSON.stringify(rows, null, 2))
}

async function uploadProductReviewTextFile(workspacePublicId) {
  const text = [
    '商品名: Smoke Low Confidence Product',
    'ブランド: SmokeBrand',
    '型番: SMK-100',
    'JANコード: 4901234567894',
    '価格: 1200円',
    'カテゴリ: Smoke Test',
  ].join('\n')
  return uploadFile(workspacePublicId, `data-pipeline-product-review-smoke-${Date.now()}.txt`, 'text/plain', text)
}

async function uploadXLSXFile(workspacePublicId) {
  const rows = [
    ['id', 'name', 'amount', 'status'],
    ['1', 'Alpha', '10', 'ready'],
    ['2', 'Beta', '20', 'ready'],
    ['3', '', '-5', 'hold'],
  ]
  return uploadFile(
    workspacePublicId,
    `data-pipeline-smoke-${Date.now()}.xlsx`,
    'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    createMinimalXLSX(rows),
  )
}

function node(id, stepType, label, x, y, config = {}) {
  return {
    id,
    type: 'pipelineStep',
    position: { x, y },
    data: { stepType, label, config },
  }
}

function outputNode(suffix, orderBy = ['id'], id = 'output', x = 1180, y = 120, config = {}) {
  return node(id, 'output', 'Smoke output', x, y, {
    displayName: `data pipeline smoke output ${suffix}`,
    tableName: `data_pipeline_smoke_output_${suffix}_${id}`,
    writeMode: 'replace',
    orderBy,
    ...config,
  })
}

function jsonGraph(filePublicId, suffix) {
  return {
    nodes: [
      node('input', 'input', 'Smoke Drive JSON', 60, 120, {
        sourceKind: 'drive_file',
        inputMode: 'json',
        filePublicIds: [filePublicId],
        recordPath: '$',
        includeSourceMetadataColumns: true,
        includeRawRecord: true,
        fields: [],
      }),
      node('json_extract', 'json_extract', 'Extract smoke fields', 340, 120, {
        sourceColumn: 'raw_record_json',
        recordPath: '$',
        includeSourceColumns: true,
        fields: [
          { column: 'id', path: 'id' },
          { column: 'name', path: 'name' },
          { column: 'amount', path: 'amount' },
          { column: 'status', path: 'status' },
          { column: 'updated_at', path: 'updated_at' },
        ],
      }),
      profileNode(),
      validateNode(),
      outputNode(suffix),
    ],
    edges: linearEdges(['input', 'json_extract', 'profile', 'validate', 'output']),
  }
}

function typedOutputGraph(filePublicId, suffix) {
  const graph = jsonGraph(filePublicId, suffix)
  graph.nodes = graph.nodes.map((item) => {
    if (item.id !== 'output') {
      return item
    }
    return outputNode(suffix, ['id'], 'output', 1180, 120, {
      columns: [
        { sourceColumn: 'id', name: 'id', type: 'string' },
        { sourceColumn: 'amount', name: 'amount_value', type: 'float64' },
        { sourceColumn: 'updated_at', name: 'updated_at', type: 'datetime' },
      ],
    })
  })
  return graph
}

function partitionGraph(filePublicId, suffix) {
  return {
    nodes: [
      node('input', 'input', 'Smoke Drive JSON', 60, 120, {
        sourceKind: 'drive_file',
        inputMode: 'json',
        filePublicIds: [filePublicId],
        recordPath: '$',
        includeSourceMetadataColumns: true,
        includeRawRecord: true,
        fields: [],
      }),
      node('json_extract', 'json_extract', 'Extract smoke fields', 320, 120, {
        sourceColumn: 'raw_record_json',
        recordPath: '$',
        includeSourceColumns: true,
        fields: [
          { column: 'id', path: 'id' },
          { column: 'name', path: 'name' },
          { column: 'amount', path: 'amount' },
          { column: 'status', path: 'status' },
          { column: 'updated_at', path: 'updated_at' },
        ],
      }),
      node('partition_filter', 'partition_filter', 'Partition filter', 580, 120, {
        dateColumn: 'updated_at',
        start: '2026-05-02T00:00:00Z',
        end: '2026-05-04T00:00:00Z',
        valueType: 'datetime',
      }),
      node('watermark_filter', 'watermark_filter', 'Watermark filter', 840, 120, {
        column: 'updated_at',
        watermarkValue: '2026-05-02T00:00:00Z',
        valueType: 'datetime',
      }),
      outputNode(suffix, ['id'], 'output', 1100, 120),
    ],
    edges: linearEdges(['input', 'json_extract', 'partition_filter', 'watermark_filter', 'output']),
  }
}

function watermarkPreviousGraph(filePublicId, suffix) {
  return {
    nodes: [
      node('input', 'input', 'Smoke Drive JSON', 60, 120, {
        sourceKind: 'drive_file',
        inputMode: 'json',
        filePublicIds: [filePublicId],
        recordPath: '$',
        includeSourceMetadataColumns: true,
        includeRawRecord: true,
        fields: [],
      }),
      node('json_extract', 'json_extract', 'Extract smoke fields', 340, 120, {
        sourceColumn: 'raw_record_json',
        recordPath: '$',
        includeSourceColumns: true,
        fields: [
          { column: 'id', path: 'id' },
          { column: 'updated_at', path: 'updated_at' },
        ],
      }),
      node('watermark_filter', 'watermark_filter', 'Previous watermark filter', 620, 120, {
        column: 'updated_at',
        watermarkSource: 'previous_success',
        watermarkValue: '2026-05-01T00:00:00Z',
        valueType: 'datetime',
      }),
      outputNode(suffix, ['id'], 'output', 900, 120),
    ],
    edges: linearEdges(['input', 'json_extract', 'watermark_filter', 'output']),
  }
}

function snapshotSCD2Graph(filePublicId, suffix) {
  return {
    nodes: [
      node('input', 'input', 'Smoke Drive JSON', 60, 120, {
        sourceKind: 'drive_file',
        inputMode: 'json',
        filePublicIds: [filePublicId],
        recordPath: '$',
        includeSourceMetadataColumns: true,
        includeRawRecord: true,
        fields: [],
      }),
      node('json_extract', 'json_extract', 'Extract snapshot fields', 340, 120, {
        sourceColumn: 'raw_record_json',
        recordPath: '$',
        includeSourceColumns: true,
        fields: [
          { column: 'id', path: 'id' },
          { column: 'name', path: 'name' },
          { column: 'status', path: 'status' },
          { column: 'updated_at', path: 'updated_at' },
        ],
      }),
      node('snapshot_scd2', 'snapshot_scd2', 'SCD2 snapshot', 620, 120, {
        uniqueKeys: ['id'],
        updatedAtColumn: 'updated_at',
        watchedColumns: ['name', 'status'],
      }),
      outputNode(suffix, ['id', 'valid_from'], 'output', 900, 120),
    ],
    edges: linearEdges(['input', 'json_extract', 'snapshot_scd2', 'output']),
  }
}

function snapshotAppendGraph(filePublicId, suffix) {
  const graph = snapshotSCD2Graph(filePublicId, suffix)
  graph.nodes = graph.nodes.map((item) => {
    if (item.id !== 'output') {
      return item
    }
    return outputNode(suffix, ['id', 'valid_from'], 'output', 900, 120, { writeMode: 'append' })
  })
  return graph
}

function snapshotMergeGraph(filePublicId, suffix) {
  const graph = snapshotSCD2Graph(filePublicId, suffix)
  graph.nodes = graph.nodes.map((item) => {
    if (item.id !== 'output') {
      return item
    }
    return outputNode(suffix, ['id', 'valid_from'], 'output', 900, 120, {
      writeMode: 'scd2_merge',
      uniqueKeys: ['id'],
      validFromColumn: 'valid_from',
      validToColumn: 'valid_to',
      isCurrentColumn: 'is_current',
      changeHashColumn: 'change_hash',
    })
  })
  return graph
}

function snapshotMergeBackfillGraph(filePublicId, suffix) {
  const graph = snapshotMergeGraph(filePublicId, suffix)
  graph.nodes = graph.nodes.map((item) => {
    if (item.id !== 'output') {
      return item
    }
    return {
      ...item,
      data: {
        ...item.data,
        config: {
          ...item.data.config,
          scd2MergePolicy: 'rebuild_key_history',
        },
      },
    }
  })
  return graph
}

function unionGraph(filePublicId, suffix) {
  const inputConfig = {
    sourceKind: 'drive_file',
    inputMode: 'json',
    filePublicIds: [filePublicId],
    recordPath: '$',
    includeSourceMetadataColumns: true,
    includeRawRecord: true,
    fields: [],
  }
  const extractConfig = {
    sourceColumn: 'raw_record_json',
    recordPath: '$',
    includeSourceColumns: true,
    fields: [
      { column: 'id', path: 'id' },
      { column: 'name', path: 'name' },
      { column: 'amount', path: 'amount' },
      { column: 'status', path: 'status' },
      { column: 'updated_at', path: 'updated_at' },
    ],
  }
  return {
    nodes: [
      node('input_a', 'input', 'Smoke Drive JSON A', 60, 80, inputConfig),
      node('extract_a', 'json_extract', 'Extract A', 320, 80, extractConfig),
      node('input_b', 'input', 'Smoke Drive JSON B', 60, 260, inputConfig),
      node('extract_b', 'json_extract', 'Extract B', 320, 260, extractConfig),
      node('union', 'union', 'Union rows', 620, 170, {
        columns: ['file_public_id', 'id', 'name', 'amount', 'status'],
        sourceLabelColumn: 'source_node_id',
      }),
      outputNode(suffix, ['id'], 'output', 900, 170),
    ],
    edges: [
      { id: 'input_a-extract_a', source: 'input_a', target: 'extract_a' },
      { id: 'extract_a-union', source: 'extract_a', target: 'union' },
      { id: 'input_b-extract_b', source: 'input_b', target: 'extract_b' },
      { id: 'extract_b-union', source: 'extract_b', target: 'union' },
      { id: 'union-output', source: 'union', target: 'output' },
    ],
  }
}

function excelGraph(filePublicId, suffix) {
  return {
    nodes: [
      node('input', 'input', 'Smoke Drive Excel', 60, 120, {
        sourceKind: 'drive_file',
        filePublicIds: [filePublicId],
      }),
      node('excel_extract', 'excel_extract', 'Extract smoke sheet', 340, 120, {
        sourceFileColumn: 'file_public_id',
        sheetIndex: 0,
        headerRow: 1,
        columns: ['id', 'name', 'amount', 'status'],
        includeSourceColumns: false,
        includeSourceMetadataColumns: true,
      }),
      profileNode(),
      validateNode(),
      outputNode(suffix),
    ],
    edges: linearEdges(['input', 'excel_extract', 'profile', 'validate', 'output']),
  }
}

function textGraph(filePublicId, suffix) {
  return {
    nodes: [
      node('input', 'input', 'Smoke Drive text', 60, 120, {
        sourceKind: 'drive_file',
        filePublicIds: [filePublicId],
      }),
      node('extract_text', 'extract_text', 'Extract text', 340, 120, {
        chunkMode: 'full_text',
      }),
      node('quality_report', 'quality_report', 'Quality report', 620, 120, {
        columns: ['text', 'confidence'],
      }),
      node('confidence_gate', 'confidence_gate', 'Confidence gate', 900, 120, {
        scoreColumns: ['confidence'],
        threshold: 0.8,
      }),
      outputNode(suffix, ['file_public_id']),
    ],
    edges: linearEdges(['input', 'extract_text', 'quality_report', 'confidence_gate', 'output']),
  }
}

function quarantineGraph(filePublicId, suffix) {
  return {
    nodes: [
      node('input', 'input', 'Smoke Drive text', 60, 180, {
        sourceKind: 'drive_file',
        filePublicIds: [filePublicId],
      }),
      node('extract_text', 'extract_text', 'Extract text', 320, 180, {
        chunkMode: 'full_text',
      }),
      node('quality_report', 'quality_report', 'Quality report', 580, 180, {
        columns: ['text', 'confidence'],
      }),
      node('confidence_gate', 'confidence_gate', 'Confidence gate', 840, 180, {
        scoreColumns: ['confidence'],
        threshold: 0.8,
      }),
      node('quarantine_pass', 'quarantine', 'Pass rows', 1100, 80, {
        statusColumn: 'gate_status',
        matchValues: ['needs_review'],
        outputMode: 'pass_only',
        mode: 'filter',
      }),
      outputNode(suffix, ['file_public_id'], 'output_pass', 1360, 80),
      node('quarantine_review', 'quarantine', 'Quarantine rows', 1100, 280, {
        statusColumn: 'gate_status',
        matchValues: ['needs_review'],
        outputMode: 'quarantine_only',
        mode: 'filter',
      }),
      outputNode(suffix, ['file_public_id'], 'output_quarantine', 1360, 280),
    ],
    edges: [
      ...linearEdges(['input', 'extract_text', 'quality_report', 'confidence_gate', 'quarantine_pass', 'output_pass']),
      { id: 'confidence_gate-quarantine_review', source: 'confidence_gate', target: 'quarantine_review' },
      { id: 'quarantine_review-output_quarantine', source: 'quarantine_review', target: 'output_quarantine' },
    ],
  }
}

function routeGraph(filePublicId, suffix) {
  return {
    nodes: [
      node('input', 'input', 'Smoke Drive text', 60, 180, {
        sourceKind: 'drive_file',
        filePublicIds: [filePublicId],
      }),
      node('extract_text', 'extract_text', 'Extract text', 320, 180, {
        chunkMode: 'full_text',
      }),
      node('quality_report', 'quality_report', 'Quality report', 580, 180, {
        columns: ['text', 'confidence'],
      }),
      node('confidence_gate', 'confidence_gate', 'Confidence gate', 840, 180, {
        scoreColumns: ['confidence'],
        threshold: 0.8,
      }),
      node('route', 'route_by_condition', 'Route by condition', 1100, 180, {
        routeColumn: 'route_key',
        defaultRoute: 'auto',
        mode: 'filter_route',
        route: 'review',
        rules: [
          { column: 'gate_status', operator: '=', value: 'needs_review', route: 'review' },
        ],
      }),
      outputNode(suffix, ['file_public_id'], 'output', 1360, 180),
    ],
    edges: linearEdges(['input', 'extract_text', 'quality_report', 'confidence_gate', 'route', 'output']),
  }
}

function reviewGraph(filePublicId, suffix) {
  return {
    nodes: [
      node('input', 'input', 'Smoke Drive text', 60, 180, {
        sourceKind: 'drive_file',
        filePublicIds: [filePublicId],
      }),
      node('extract_text', 'extract_text', 'Extract text', 320, 180, {
        chunkMode: 'full_text',
      }),
      node('quality_report', 'quality_report', 'Quality report', 580, 180, {
        columns: ['text', 'confidence'],
      }),
      node('confidence_gate', 'confidence_gate', 'Confidence gate', 840, 180, {
        scoreColumns: ['confidence'],
        threshold: 0.8,
      }),
      node('human_review', 'human_review', 'Human review', 1100, 180, {
        reasonColumns: ['gate_status'],
        queue: 'smoke-review',
        createReviewItems: true,
        mode: 'filter_review',
      }),
      outputNode(suffix, ['file_public_id'], 'output', 1360, 180),
    ],
    edges: linearEdges(['input', 'extract_text', 'quality_report', 'confidence_gate', 'human_review', 'output']),
  }
}

function fieldReviewGraph(filePublicId, suffix) {
  return {
    nodes: [
      node('input', 'input', 'Smoke Drive text', 60, 180, {
        sourceKind: 'drive_file',
        filePublicIds: [filePublicId],
      }),
      node('extract_text', 'extract_text', 'Extract text', 300, 180, {
        chunkMode: 'full_text',
      }),
      node('extract_fields', 'extract_fields', 'Extract fields', 540, 180, {
        fields: [
          { name: 'invoice_id', type: 'string', required: true, patterns: ['Invoice ID:\\s*([A-Z0-9-]+)'] },
          { name: 'customer', type: 'string', required: true, patterns: ['Customer:\\s*([^\\n]+)'] },
          { name: 'amount', type: 'number', required: true, patterns: ['Amount:\\s*([0-9.]+)'] },
        ],
      }),
      node('confidence_gate', 'confidence_gate', 'Confidence gate', 780, 180, {
        scoreColumns: ['field_confidence'],
        threshold: 0.9,
      }),
      node('human_review', 'human_review', 'Human review', 1020, 180, {
        reasonColumns: ['gate_status', 'gate_reason'],
        queue: 'field-review-smoke',
        createReviewItems: true,
        mode: 'filter_review',
      }),
      outputNode(suffix, ['file_public_id'], 'output', 1260, 180),
    ],
    edges: linearEdges(['input', 'extract_text', 'extract_fields', 'confidence_gate', 'human_review', 'output']),
  }
}

function tableReviewGraph(filePublicId, suffix) {
  return {
    nodes: [
      node('input', 'input', 'Smoke Drive table text', 60, 180, {
        sourceKind: 'drive_file',
        filePublicIds: [filePublicId],
      }),
      node('extract_text', 'extract_text', 'Extract text', 300, 180, {
        chunkMode: 'full_text',
      }),
      node('extract_table', 'extract_table', 'Extract table', 540, 180, {
        delimiter: ',',
        expectedColumnCount: 4,
      }),
      node('confidence_gate', 'confidence_gate', 'Confidence gate', 780, 180, {
        scoreColumns: ['table_confidence'],
        threshold: 0.9,
      }),
      node('human_review', 'human_review', 'Human review', 1020, 180, {
        reasonColumns: ['gate_status', 'gate_reason'],
        queue: 'table-review-smoke',
        createReviewItems: true,
        mode: 'filter_review',
      }),
      outputNode(suffix, ['file_public_id'], 'output', 1260, 180),
    ],
    edges: linearEdges(['input', 'extract_text', 'extract_table', 'confidence_gate', 'human_review', 'output']),
  }
}

function schemaMappingReviewGraph(filePublicId, suffix) {
  return {
    nodes: [
      node('input', 'input', 'Smoke Drive JSON', 60, 180, {
        sourceKind: 'drive_file',
        inputMode: 'json',
        filePublicIds: [filePublicId],
        recordPath: '$',
        includeSourceMetadataColumns: true,
        includeRawRecord: true,
        fields: [],
      }),
      node('json_extract', 'json_extract', 'Extract source columns', 300, 180, {
        sourceColumn: 'raw_record_json',
        recordPath: '$',
        includeSourceColumns: true,
        fields: [
          { column: 'invoice_number', path: 'invoice_number' },
          { column: 'total', path: 'total' },
          { column: 'state', path: 'state' },
        ],
      }),
      node('schema_mapping', 'schema_mapping', 'Map schema', 540, 180, {
        includeSourceColumns: true,
        confidenceThreshold: 0.9,
        mappings: [
          { sourceColumn: 'invoice_number', targetColumn: 'invoice_id', required: true, confidence: 0.96 },
          { sourceColumn: 'total', targetColumn: 'amount', required: true, confidence: 0.55 },
          { sourceColumn: 'state', targetColumn: 'status', required: true, confidence: 0.95 },
        ],
      }),
      node('confidence_gate', 'confidence_gate', 'Confidence gate', 780, 180, {
        scoreColumns: ['schema_mapping_confidence'],
        threshold: 0.9,
      }),
      node('human_review', 'human_review', 'Human review', 1020, 180, {
        reasonColumns: ['gate_status', 'gate_reason', 'schema_mapping_reason'],
        queue: 'schema-mapping-review-smoke',
        createReviewItems: true,
        mode: 'filter_review',
      }),
      outputNode(suffix, ['file_public_id'], 'output', 1260, 180),
    ],
    edges: linearEdges(['input', 'json_extract', 'schema_mapping', 'confidence_gate', 'human_review', 'output']),
  }
}

function productReviewGraph(filePublicId, suffix) {
  return {
    nodes: [
      node('input', 'input', 'Smoke Drive product file', 60, 180, {
        sourceKind: 'drive_file',
        filePublicIds: [filePublicId],
      }),
      node('product_extraction', 'product_extraction', 'Product extraction', 300, 180, {
        sourceFileColumn: 'file_public_id',
        includeSourceColumns: true,
        confidenceThreshold: 0.99,
      }),
      node('confidence_gate', 'confidence_gate', 'Confidence gate', 540, 180, {
        scoreColumns: ['product_confidence'],
        threshold: 0.99,
      }),
      node('human_review', 'human_review', 'Human review', 780, 180, {
        reasonColumns: ['gate_status', 'gate_reason', 'product_extraction_reason'],
        queue: 'product-review-smoke',
        createReviewItems: true,
        mode: 'filter_review',
      }),
      outputNode(suffix, ['file_public_id'], 'output', 1020, 180),
    ],
    edges: linearEdges(['input', 'product_extraction', 'confidence_gate', 'human_review', 'output']),
  }
}

function profileNode() {
  return node('profile', 'profile', 'Profile smoke rows', 620, 120, {})
}

function validateNode() {
  return node('validate', 'validate', 'Validate smoke rows', 900, 120, {
    rules: [
      { column: 'name', operator: 'required', severity: 'error' },
      { column: 'amount', operator: 'range', min: 0, max: 100, severity: 'warning' },
      { column: 'status', operator: 'in', values: ['ready'], severity: 'warning' },
    ],
  })
}

function linearEdges(ids) {
  return ids.slice(0, -1).map((source, index) => {
    const target = ids[index + 1]
    return { id: `${source}-${target}`, source, target }
  })
}

async function createPipeline(scenario, filePublicId) {
  const suffix = `${scenario}_${Date.now()}`
  const graphs = {
    json: jsonGraph,
    typed_output: typedOutputGraph,
    partition: partitionGraph,
    watermark_previous: watermarkPreviousGraph,
    snapshot_scd2: snapshotSCD2Graph,
    snapshot_append: snapshotAppendGraph,
    snapshot_merge: snapshotMergeGraph,
    snapshot_merge_backfill: snapshotMergeBackfillGraph,
    union: unionGraph,
    excel: excelGraph,
    text: textGraph,
    quarantine: quarantineGraph,
    route: routeGraph,
    review: reviewGraph,
    field_review: fieldReviewGraph,
    table_review: tableReviewGraph,
    schema_mapping_review: schemaMappingReviewGraph,
    product_review: productReviewGraph,
  }
  const pipeline = await jsonRequest('/api/v1/data-pipelines', 'POST', {
    name: `Smoke: ${scenario} metadata ${new Date().toISOString()}`,
    description: `Smoke test for Data Pipeline ${scenario} metadata and output registration.`,
  }, { 'Idempotency-Key': crypto.randomUUID() })
  created.pipelinePublicIds.push(pipeline.publicId)

  const version = await jsonRequest(`/api/v1/data-pipelines/${encodeURIComponent(pipeline.publicId)}/versions`, 'POST', {
    graph: graphs[scenario](filePublicId, suffix),
  })
  const published = await jsonRequest(`/api/v1/data-pipeline-versions/${encodeURIComponent(version.publicId)}/publish`, 'POST', {})
  created.versionPublicIds.push(published.publicId)
  return { pipeline, version: published }
}

async function createDraftPipeline(name) {
  const pipeline = await jsonRequest('/api/v1/data-pipelines', 'POST', {
    name,
    description: 'Smoke test for Data Pipeline draft validation endpoint.',
  }, { 'Idempotency-Key': crypto.randomUUID() })
  created.pipelinePublicIds.push(pipeline.publicId)
  return pipeline
}

async function validateDraftGraph(pipelinePublicId, graph) {
  return jsonRequest(`/api/v1/data-pipelines/${encodeURIComponent(pipelinePublicId)}/validate`, 'POST', { graph })
}

async function requestRun(versionPublicId) {
  const run = await jsonRequest(`/api/v1/data-pipeline-versions/${encodeURIComponent(versionPublicId)}/runs`, 'POST', {}, {
    'Idempotency-Key': crypto.randomUUID(),
  })
  created.runPublicIds.push(run.publicId)
  return run
}

async function ensureProductExtractionReady(filePublicId) {
  await jsonRequest(`/api/v1/drive/files/${encodeURIComponent(filePublicId)}/ocr/jobs`, 'POST', {})
  let ocr
  for (let attempt = 1; attempt <= pollAttempts; attempt += 1) {
    ocr = await request(`/api/v1/drive/files/${encodeURIComponent(filePublicId)}/ocr`)
    if (ocr.run?.status === 'completed') {
      break
    }
    if (ocr.run && !['pending', 'processing'].includes(ocr.run.status)) {
      throw new Error(`OCR did not complete: ${JSON.stringify(ocr.run)}`)
    }
    await sleep(pollDelayMs)
  }
  if (ocr?.run?.status !== 'completed') {
    throw new Error(`OCR did not complete after ${pollAttempts} attempts: ${JSON.stringify(ocr?.run)}`)
  }
  await jsonRequest(`/api/v1/drive/files/${encodeURIComponent(filePublicId)}/product-extractions/jobs`, 'POST', {})
  let productExtractions
  for (let attempt = 1; attempt <= pollAttempts; attempt += 1) {
    productExtractions = await request(`/api/v1/drive/files/${encodeURIComponent(filePublicId)}/product-extractions`)
    if ((productExtractions.items ?? []).length > 0) {
      return productExtractions.items
    }
    await sleep(pollDelayMs)
  }
  throw new Error(`product extraction items did not appear after ${pollAttempts} attempts: ${JSON.stringify(productExtractions)}`)
}

function findStep(run, nodeId) {
  return (run.steps ?? []).find((step) => step.nodeId === nodeId)
}

function assertCommonRun(run, expectedRows) {
  if (run.status !== 'completed') {
    throw new Error(`run did not complete: ${run.status} ${run.errorSummary ?? ''}`)
  }
  if (run.rowCount !== expectedRows) {
    throw new Error(`run rowCount = ${run.rowCount}, want ${expectedRows}`)
  }
  const output = (run.outputs ?? []).find((item) => item.nodeId === 'output')
  if (!output || output.status !== 'completed' || output.rowCount !== expectedRows) {
    throw new Error(`unexpected output: ${JSON.stringify(output)}`)
  }
  for (const step of run.steps ?? []) {
    if (!step.metadata?.queryStats?.queryId) {
      throw new Error(`queryStats missing for step ${step.nodeId}: ${JSON.stringify(step.metadata)}`)
    }
    if (typeof step.metadata.outputRows !== 'number') {
      throw new Error(`outputRows missing for step ${step.nodeId}: ${JSON.stringify(step.metadata)}`)
    }
  }
}

function assertProfileValidateRun(run, expectedRows) {
  assertCommonRun(run, expectedRows)
  const profileStep = findStep(run, 'profile')
  const validateStep = findStep(run, 'validate')
  if (!profileStep?.metadata?.profile) {
    throw new Error(`profile metadata missing: ${JSON.stringify(profileStep)}`)
  }
  if (profileStep.metadata.profile.rowCount !== expectedRows || profileStep.metadata.profile.columnCount < 4) {
    throw new Error(`unexpected profile metadata: ${JSON.stringify(profileStep.metadata.profile)}`)
  }
  if (!validateStep?.metadata?.validation) {
    throw new Error(`validation metadata missing: ${JSON.stringify(validateStep)}`)
  }
  const validation = validateStep.metadata.validation
  if (validation.warningCount < 2 || validation.failedRows < 3 || !Array.isArray(validation.samples) || validation.samples.length === 0) {
    throw new Error(`unexpected validation metadata: ${JSON.stringify(validation)}`)
  }
}

function assertUnionRun(run) {
  assertCommonRun(run, 6)
  const unionStep = findStep(run, 'union')
  if (!unionStep || unionStep.rowCount !== 6) {
    throw new Error(`unexpected union step: ${JSON.stringify(unionStep)}`)
  }
}

function assertPartitionRun(run) {
  assertCommonRun(run, 2)
  const partitionStep = findStep(run, 'partition_filter')
  const watermarkStep = findStep(run, 'watermark_filter')
  if (partitionStep?.rowCount !== 2 || partitionStep?.metadata?.partitionFilter?.dateColumn !== 'updated_at') {
    throw new Error(`unexpected partition filter metadata: ${JSON.stringify(partitionStep)}`)
  }
  if (watermarkStep?.rowCount !== 2 || watermarkStep?.metadata?.watermarkFilter?.column !== 'updated_at') {
    throw new Error(`unexpected watermark filter metadata: ${JSON.stringify(watermarkStep)}`)
  }
}

function assertSnapshotSCD2Run(run) {
  assertCommonRun(run, 3)
  const snapshotStep = findStep(run, 'snapshot_scd2')
  const metadata = snapshotStep?.metadata?.snapshotSCD2
  if (!metadata || !Array.isArray(metadata.uniqueKeys) || !metadata.uniqueKeys.includes('id')) {
    throw new Error(`snapshot metadata missing: ${JSON.stringify(snapshotStep)}`)
  }
  for (const column of ['valid_from', 'valid_to', 'is_current', 'change_hash']) {
    if (!metadata[`${column.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase())}Column`] && column !== 'valid_from' && column !== 'valid_to') {
      throw new Error(`snapshot metadata missing ${column}: ${JSON.stringify(metadata)}`)
    }
  }
  if (metadata.updatedAtColumn !== 'updated_at' || !metadata.watchedColumns?.includes('status')) {
    throw new Error(`unexpected snapshot metadata: ${JSON.stringify(metadata)}`)
  }
}

function assertTextRun(run) {
  assertCommonRun(run, 1)
  const qualityStep = findStep(run, 'quality_report')
  const gateStep = findStep(run, 'confidence_gate')
  if (!qualityStep?.metadata?.quality) {
    throw new Error(`quality metadata missing: ${JSON.stringify(qualityStep)}`)
  }
  if (!Array.isArray(qualityStep.metadata.quality.warnings) || qualityStep.metadata.quality.warnings.length === 0) {
    throw new Error(`quality warnings missing: ${JSON.stringify(qualityStep.metadata.quality)}`)
  }
  if (!gateStep?.metadata?.confidenceGate) {
    throw new Error(`confidenceGate metadata missing: ${JSON.stringify(gateStep)}`)
  }
  const samples = gateStep.metadata.confidenceGate.lowConfidenceSamples
  if (!Array.isArray(samples) || samples.length === 0 || !samples[0].gate_reason) {
    throw new Error(`confidenceGate low confidence samples missing: ${JSON.stringify(gateStep.metadata.confidenceGate)}`)
  }
}

function assertQuarantineRun(run) {
  if (run.status !== 'completed') {
    throw new Error(`run did not complete: ${run.status} ${run.errorSummary ?? ''}`)
  }
  const passOutput = (run.outputs ?? []).find((item) => item.nodeId === 'output_pass')
  const quarantineOutput = (run.outputs ?? []).find((item) => item.nodeId === 'output_quarantine')
  if (!passOutput || passOutput.status !== 'completed' || passOutput.rowCount !== 0) {
    throw new Error(`unexpected pass output: ${JSON.stringify(passOutput)}`)
  }
  if (!quarantineOutput || quarantineOutput.status !== 'completed' || quarantineOutput.rowCount !== 1) {
    throw new Error(`unexpected quarantine output: ${JSON.stringify(quarantineOutput)}`)
  }
  const passStep = findStep(run, 'quarantine_pass')
  const reviewStep = findStep(run, 'quarantine_review')
  for (const step of [passStep, reviewStep]) {
    if (!step?.metadata?.queryStats?.queryId) {
      throw new Error(`queryStats missing for quarantine step: ${JSON.stringify(step)}`)
    }
    if (step.metadata.quarantinedRows !== 1 || step.metadata.passedRows !== 0) {
      throw new Error(`unexpected quarantine metadata: ${JSON.stringify(step.metadata)}`)
    }
  }
}

function assertRouteRun(run) {
  assertCommonRun(run, 1)
  const routeStep = findStep(run, 'route')
  if (!Array.isArray(routeStep?.metadata?.routeCounts) || routeStep.metadata.routeCounts.length !== 1) {
    throw new Error(`route counts missing: ${JSON.stringify(routeStep)}`)
  }
  const reviewRoute = routeStep.metadata.routeCounts.find((item) => item.route === 'review')
  if (!reviewRoute || reviewRoute.count !== 1 || routeStep.metadata.routeColumn !== 'route_key') {
    throw new Error(`unexpected route metadata: ${JSON.stringify(routeStep.metadata)}`)
  }
}

function assertReviewRun(run) {
  assertCommonRun(run, 1)
  const reviewStep = findStep(run, 'human_review')
  if (!reviewStep?.metadata?.queryStats?.queryId) {
    throw new Error(`queryStats missing for human_review step: ${JSON.stringify(reviewStep)}`)
  }
  if (reviewStep.metadata.reviewItemCount !== 1) {
    throw new Error(`unexpected review item metadata: ${JSON.stringify(reviewStep.metadata)}`)
  }
}

function assertFieldReviewRun(run) {
  assertCommonRun(run, 1)
  const extractStep = findStep(run, 'extract_fields')
  if (!extractStep?.metadata?.fieldExtraction) {
    throw new Error(`fieldExtraction metadata missing: ${JSON.stringify(extractStep)}`)
  }
  if (extractStep.metadata.fieldExtraction.lowConfidenceRows !== 1 || extractStep.metadata.fieldExtraction.missingRequiredRows !== 1) {
    throw new Error(`unexpected fieldExtraction metadata: ${JSON.stringify(extractStep.metadata.fieldExtraction)}`)
  }
  const gateStep = findStep(run, 'confidence_gate')
  if (gateStep?.metadata?.confidenceGate?.needsReviewRows !== 1) {
    throw new Error(`unexpected confidence gate metadata: ${JSON.stringify(gateStep?.metadata?.confidenceGate)}`)
  }
  const reviewStep = findStep(run, 'human_review')
  if (reviewStep?.metadata?.reviewItemCount !== 1) {
    throw new Error(`unexpected human review metadata: ${JSON.stringify(reviewStep?.metadata)}`)
  }
}

function assertTableReviewRun(run) {
  assertCommonRun(run, 1)
  const extractStep = findStep(run, 'extract_table')
  if (!extractStep?.metadata?.tableExtraction) {
    throw new Error(`tableExtraction metadata missing: ${JSON.stringify(extractStep)}`)
  }
  if (extractStep.metadata.tableExtraction.lowConfidenceRows !== 1 || extractStep.metadata.tableExtraction.rowCount !== 3) {
    throw new Error(`unexpected tableExtraction metadata: ${JSON.stringify(extractStep.metadata.tableExtraction)}`)
  }
  const gateStep = findStep(run, 'confidence_gate')
  if (gateStep?.metadata?.confidenceGate?.needsReviewRows !== 1) {
    throw new Error(`unexpected confidence gate metadata: ${JSON.stringify(gateStep?.metadata?.confidenceGate)}`)
  }
  const reviewStep = findStep(run, 'human_review')
  if (reviewStep?.metadata?.reviewItemCount !== 1) {
    throw new Error(`unexpected human review metadata: ${JSON.stringify(reviewStep?.metadata)}`)
  }
}

function assertSchemaMappingReviewRun(run) {
  assertCommonRun(run, 1)
  const mappingStep = findStep(run, 'schema_mapping')
  if (!mappingStep?.metadata?.schemaMapping) {
    throw new Error(`schemaMapping metadata missing: ${JSON.stringify(mappingStep)}`)
  }
  if (mappingStep.metadata.schemaMapping.lowConfidenceRows !== 1 || mappingStep.metadata.schemaMapping.mappingCount !== 3) {
    throw new Error(`unexpected schemaMapping metadata: ${JSON.stringify(mappingStep.metadata.schemaMapping)}`)
  }
  const gateStep = findStep(run, 'confidence_gate')
  if (gateStep?.metadata?.confidenceGate?.needsReviewRows !== 1) {
    throw new Error(`unexpected confidence gate metadata: ${JSON.stringify(gateStep?.metadata?.confidenceGate)}`)
  }
  const reviewStep = findStep(run, 'human_review')
  if (reviewStep?.metadata?.reviewItemCount !== 1) {
    throw new Error(`unexpected human review metadata: ${JSON.stringify(reviewStep?.metadata)}`)
  }
}

function assertProductReviewRun(run) {
  assertCommonRun(run, 1)
  const productStep = findStep(run, 'product_extraction')
  if (!productStep?.metadata?.productExtraction) {
    throw new Error(`productExtraction metadata missing: ${JSON.stringify(productStep)}`)
  }
  if (productStep.metadata.productExtraction.itemCount < 1 || productStep.metadata.productExtraction.lowConfidenceItems < 1) {
    throw new Error(`unexpected productExtraction metadata: ${JSON.stringify(productStep.metadata.productExtraction)}`)
  }
  const gateStep = findStep(run, 'confidence_gate')
  if (gateStep?.metadata?.confidenceGate?.needsReviewRows !== 1) {
    throw new Error(`unexpected confidence gate metadata: ${JSON.stringify(gateStep?.metadata?.confidenceGate)}`)
  }
  const reviewStep = findStep(run, 'human_review')
  if (reviewStep?.metadata?.reviewItemCount !== 1) {
    throw new Error(`unexpected human review metadata: ${JSON.stringify(reviewStep?.metadata)}`)
  }
}

async function assertReviewItems(pipelinePublicId) {
  const listed = await request(`/api/v1/data-pipelines/${encodeURIComponent(pipelinePublicId)}/review-items?status=open&limit=10`)
  const items = listed.items ?? []
  if (items.length !== 1) {
    throw new Error(`review item count = ${items.length}, want 1: ${JSON.stringify(listed)}`)
  }
  const item = items[0]
  if (item.status !== 'open' || item.queue !== 'smoke-review' || item.nodeId !== 'human_review') {
    throw new Error(`unexpected review item: ${JSON.stringify(item)}`)
  }
  const detail = await request(`/api/v1/data-pipeline-review-items/${encodeURIComponent(item.publicId)}`)
  if (!detail.sourceSnapshot || !Array.isArray(detail.reason) || detail.reason.length === 0) {
    throw new Error(`review item detail missing source snapshot or reason: ${JSON.stringify(detail)}`)
  }
  const transitioned = await jsonRequest(`/api/v1/data-pipeline-review-items/${encodeURIComponent(item.publicId)}/transition`, 'POST', {
    status: 'approved',
    comment: 'smoke approved',
  })
  if (transitioned.status !== 'approved' || transitioned.decisionComment !== 'smoke approved') {
    throw new Error(`review item transition failed: ${JSON.stringify(transitioned)}`)
  }
  return { item, transitioned }
}

async function assertDriveFileReviewItemLink(filePublicId, expectedItem, expectedPipelinePublicId) {
  const listed = await request(`/api/v1/drive/files/${encodeURIComponent(filePublicId)}/data-pipeline-review-items?status=open&limit=10`)
  const items = listed.items ?? []
  const item = items.find((candidate) => candidate.publicId === expectedItem.publicId)
  if (!item) {
    throw new Error(`drive file review link missing ${expectedItem.publicId}: ${JSON.stringify(listed)}`)
  }
  if (item.pipelinePublicId !== expectedPipelinePublicId || !item.runPublicId) {
    throw new Error(`drive file review link missing pipeline/run public IDs: ${JSON.stringify(item)}`)
  }
  return item
}

async function assertFieldReviewItems(pipelinePublicId, filePublicId) {
  const listed = await request(`/api/v1/data-pipelines/${encodeURIComponent(pipelinePublicId)}/review-items?status=open&limit=10`)
  const items = listed.items ?? []
  if (items.length !== 1) {
    throw new Error(`field review item count = ${items.length}, want 1: ${JSON.stringify(listed)}`)
  }
  const item = items[0]
  if (item.queue !== 'field-review-smoke' || item.nodeId !== 'human_review') {
    throw new Error(`unexpected field review item: ${JSON.stringify(item)}`)
  }
  if (String(item.sourceSnapshot?.field_confidence) !== '0.6667' || item.sourceSnapshot?.amount !== '') {
    throw new Error(`field review snapshot missing extraction confidence: ${JSON.stringify(item.sourceSnapshot)}`)
  }
  const driveLink = await assertDriveFileReviewItemLink(filePublicId, item, pipelinePublicId)
  return { item, driveLink }
}

async function assertTableReviewItems(pipelinePublicId, filePublicId) {
  const listed = await request(`/api/v1/data-pipelines/${encodeURIComponent(pipelinePublicId)}/review-items?status=open&limit=10`)
  const items = listed.items ?? []
  if (items.length !== 1) {
    throw new Error(`table review item count = ${items.length}, want 1: ${JSON.stringify(listed)}`)
  }
  const item = items[0]
  if (item.queue !== 'table-review-smoke' || item.nodeId !== 'human_review') {
    throw new Error(`unexpected table review item: ${JSON.stringify(item)}`)
  }
  if (String(item.sourceSnapshot?.table_confidence) !== '0.7500' || String(item.sourceSnapshot?.table_missing_cell_count) !== '1') {
    throw new Error(`table review snapshot missing table confidence: ${JSON.stringify(item.sourceSnapshot)}`)
  }
  const driveLink = await assertDriveFileReviewItemLink(filePublicId, item, pipelinePublicId)
  return { item, driveLink }
}

async function assertSchemaMappingReviewItems(pipelinePublicId, filePublicId) {
  const listed = await request(`/api/v1/data-pipelines/${encodeURIComponent(pipelinePublicId)}/review-items?status=open&limit=10`)
  const items = listed.items ?? []
  if (items.length !== 1) {
    throw new Error(`schema mapping review item count = ${items.length}, want 1: ${JSON.stringify(listed)}`)
  }
  const item = items[0]
  if (item.queue !== 'schema-mapping-review-smoke' || item.nodeId !== 'human_review') {
    throw new Error(`unexpected schema mapping review item: ${JSON.stringify(item)}`)
  }
  if (String(item.sourceSnapshot?.schema_mapping_confidence) !== '0.8200' || item.sourceSnapshot?.amount !== '345.67') {
    throw new Error(`schema mapping review snapshot missing confidence or mapped columns: ${JSON.stringify(item.sourceSnapshot)}`)
  }
  const driveLink = await assertDriveFileReviewItemLink(filePublicId, item, pipelinePublicId)
  return { item, driveLink }
}

async function assertProductReviewItems(pipelinePublicId, filePublicId) {
  const listed = await request(`/api/v1/data-pipelines/${encodeURIComponent(pipelinePublicId)}/review-items?status=open&limit=10`)
  const items = listed.items ?? []
  if (items.length !== 1) {
    throw new Error(`product review item count = ${items.length}, want 1: ${JSON.stringify(listed)}`)
  }
  const item = items[0]
  if (item.queue !== 'product-review-smoke' || item.nodeId !== 'human_review') {
    throw new Error(`unexpected product review item: ${JSON.stringify(item)}`)
  }
  if (!item.sourceSnapshot?.product_extraction_item_public_id || item.sourceSnapshot?.product_confidence === undefined || !item.sourceSnapshot?.product_name) {
    throw new Error(`product review snapshot missing product extraction context: ${JSON.stringify(item.sourceSnapshot)}`)
  }
  const driveLink = await assertDriveFileReviewItemLink(filePublicId, item, pipelinePublicId)
  return { item, driveLink }
}

async function waitForRun(pipelinePublicId, runPublicId, assertRun) {
  let last
  for (let attempt = 1; attempt <= pollAttempts; attempt += 1) {
    const runs = await request(`/api/v1/data-pipelines/${encodeURIComponent(pipelinePublicId)}/runs?limit=10`)
    last = (runs.items ?? []).find((item) => item.publicId === runPublicId)
    if (last && !['pending', 'processing'].includes(last.status)) {
      assertRun(last)
      return last
    }
    await sleep(pollDelayMs)
  }
  throw new Error(`run did not finish after ${pollAttempts} attempts: ${JSON.stringify(last)}`)
}

async function runScenario(workspacePublicId, scenario) {
  const uploaders = {
    json: uploadJSONFile,
    typed_output: uploadJSONFile,
    partition: uploadJSONFile,
    watermark_previous: uploadJSONFile,
    snapshot_scd2: uploadSnapshotJSONFile,
    snapshot_append: uploadSnapshotJSONFile,
    snapshot_merge: uploadSnapshotJSONFile,
    snapshot_merge_backfill: uploadSnapshotJSONFile,
    union: uploadJSONFile,
    excel: uploadXLSXFile,
    text: uploadTextFile,
    quarantine: uploadTextFile,
    route: uploadTextFile,
    review: uploadTextFile,
    field_review: uploadFieldReviewTextFile,
    table_review: uploadTableReviewTextFile,
    schema_mapping_review: uploadSchemaMappingReviewJSONFile,
    product_review: uploadProductReviewTextFile,
  }
  const assertions = {
    json: (run) => assertProfileValidateRun(run, 3),
    typed_output: (run) => assertProfileValidateRun(run, 3),
    partition: assertPartitionRun,
    watermark_previous: (run) => assertCommonRun(run, 2),
    snapshot_scd2: assertSnapshotSCD2Run,
    snapshot_append: assertSnapshotSCD2Run,
    snapshot_merge: assertSnapshotSCD2Run,
    snapshot_merge_backfill: assertSnapshotSCD2Run,
    union: assertUnionRun,
    excel: (run) => assertProfileValidateRun(run, 3),
    text: assertTextRun,
    quarantine: assertQuarantineRun,
    route: assertRouteRun,
    review: assertReviewRun,
    field_review: assertFieldReviewRun,
    table_review: assertTableReviewRun,
    schema_mapping_review: assertSchemaMappingReviewRun,
    product_review: assertProductReviewRun,
  }
  const file = await uploaders[scenario](workspacePublicId)
  if (scenario === 'product_review') {
    await ensureProductExtractionReady(file.publicId)
  }
  const { pipeline, version } = await createPipeline(scenario, file.publicId)
  const run = await requestRun(version.publicId)
  const completed = await waitForRun(pipeline.publicId, run.publicId, assertions[scenario])
  const reviewItems = scenario === 'review'
    ? await assertReviewItems(pipeline.publicId)
    : scenario === 'field_review'
      ? await assertFieldReviewItems(pipeline.publicId, file.publicId)
    : scenario === 'table_review'
      ? await assertTableReviewItems(pipeline.publicId, file.publicId)
      : scenario === 'schema_mapping_review'
        ? await assertSchemaMappingReviewItems(pipeline.publicId, file.publicId)
        : scenario === 'product_review'
          ? await assertProductReviewItems(pipeline.publicId, file.publicId)
          : undefined
  return {
    scenario,
    workspacePublicId,
    driveFilePublicId: file.publicId,
    pipelinePublicId: pipeline.publicId,
    versionPublicId: version.publicId,
    runPublicId: completed.publicId,
    status: completed.status,
    rowCount: completed.rowCount,
    detailUrl: `${frontendURL}/data-pipelines/${pipeline.publicId}`,
    profile: findStep(completed, 'profile')?.metadata?.profile,
    validation: findStep(completed, 'validate')?.metadata?.validation,
    quality: findStep(completed, 'quality_report')?.metadata?.quality,
    confidenceGate: findStep(completed, 'confidence_gate')?.metadata?.confidenceGate,
    reviewItems,
    outputs: completed.outputs,
  }
}

async function runWatermarkPreviousScenario(workspacePublicId) {
  const file = await uploadJSONFile(workspacePublicId)
  const { pipeline, version } = await createPipeline('watermark_previous', file.publicId)

  const firstRun = await requestRun(version.publicId)
  const firstCompleted = await waitForRun(pipeline.publicId, firstRun.publicId, (run) => assertCommonRun(run, 2))
  const firstWatermark = findStep(firstCompleted, 'watermark_filter')?.metadata?.watermarkFilter
  if (!firstWatermark || firstWatermark.resolvedSource !== 'initial' || !firstWatermark.nextWatermarkValue) {
    throw new Error(`unexpected first watermark metadata: ${JSON.stringify(firstWatermark)}`)
  }

  const secondRun = await requestRun(version.publicId)
  const secondCompleted = await waitForRun(pipeline.publicId, secondRun.publicId, (run) => assertCommonRun(run, 0))
  const secondWatermark = findStep(secondCompleted, 'watermark_filter')?.metadata?.watermarkFilter
  if (!secondWatermark || secondWatermark.resolvedSource !== 'previous_success' || secondWatermark.previousRunPublicId !== firstCompleted.publicId) {
    throw new Error(`unexpected second watermark metadata: ${JSON.stringify(secondWatermark)}`)
  }
  if (secondWatermark.watermarkValue !== firstWatermark.nextWatermarkValue || secondWatermark.nextWatermarkValue !== firstWatermark.nextWatermarkValue) {
    throw new Error(`watermark did not carry forward: first=${JSON.stringify(firstWatermark)} second=${JSON.stringify(secondWatermark)}`)
  }

  return {
    scenario: 'watermark_previous',
    workspacePublicId,
    driveFilePublicId: file.publicId,
    pipelinePublicId: pipeline.publicId,
    versionPublicId: version.publicId,
    firstRunPublicId: firstCompleted.publicId,
    secondRunPublicId: secondCompleted.publicId,
    detailUrl: `${frontendURL}/data-pipelines/${pipeline.publicId}`,
    firstWatermark,
    secondWatermark,
    outputs: secondCompleted.outputs,
  }
}

async function runSnapshotAppendScenario(workspacePublicId) {
  const file = await uploadSnapshotJSONFile(workspacePublicId)
  const { pipeline, version } = await createPipeline('snapshot_append', file.publicId)
  const firstRun = await requestRun(version.publicId)
  const firstCompleted = await waitForRun(pipeline.publicId, firstRun.publicId, assertSnapshotSCD2Run)
  const secondRun = await requestRun(version.publicId)
  const secondCompleted = await waitForRun(pipeline.publicId, secondRun.publicId, (run) => assertCommonRun(run, 6))
  return {
    scenario: 'snapshot_append',
    workspacePublicId,
    driveFilePublicId: file.publicId,
    pipelinePublicId: pipeline.publicId,
    versionPublicId: version.publicId,
    firstRunPublicId: firstCompleted.publicId,
    secondRunPublicId: secondCompleted.publicId,
    detailUrl: `${frontendURL}/data-pipelines/${pipeline.publicId}`,
    outputs: secondCompleted.outputs,
  }
}

async function createPublishedVersion(pipelinePublicId, graph) {
  const version = await jsonRequest(`/api/v1/data-pipelines/${encodeURIComponent(pipelinePublicId)}/versions`, 'POST', { graph })
  const published = await jsonRequest(`/api/v1/data-pipeline-versions/${encodeURIComponent(version.publicId)}/publish`, 'POST', {})
  created.versionPublicIds.push(published.publicId)
  return published
}

async function runSnapshotMergeScenario(workspacePublicId) {
  const suffix = `snapshot_merge_${Date.now()}`
  const file = await uploadSnapshotJSONFile(workspacePublicId)
  const pipeline = await jsonRequest('/api/v1/data-pipelines', 'POST', {
    name: `Smoke: snapshot_merge metadata ${new Date().toISOString()}`,
    description: 'Smoke test for Data Pipeline SCD2 merge output registration.',
  }, { 'Idempotency-Key': crypto.randomUUID() })
  created.pipelinePublicIds.push(pipeline.publicId)
  const version = await createPublishedVersion(pipeline.publicId, snapshotMergeGraph(file.publicId, suffix))

  const firstRun = await requestRun(version.publicId)
  const firstCompleted = await waitForRun(pipeline.publicId, firstRun.publicId, assertSnapshotSCD2Run)
  const secondRun = await requestRun(version.publicId)
  const secondCompleted = await waitForRun(pipeline.publicId, secondRun.publicId, (run) => assertCommonRun(run, 3))

  const changedFile = await uploadSnapshotChangedJSONFile(workspacePublicId)
  const changedVersion = await createPublishedVersion(pipeline.publicId, snapshotMergeGraph(changedFile.publicId, suffix))
  const changedRun = await requestRun(changedVersion.publicId)
  const changedCompleted = await waitForRun(pipeline.publicId, changedRun.publicId, (run) => assertCommonRun(run, 4))

  return {
    scenario: 'snapshot_merge',
    workspacePublicId,
    driveFilePublicId: file.publicId,
    changedDriveFilePublicId: changedFile.publicId,
    pipelinePublicId: pipeline.publicId,
    versionPublicId: version.publicId,
    changedVersionPublicId: changedVersion.publicId,
    firstRunPublicId: firstCompleted.publicId,
    secondRunPublicId: secondCompleted.publicId,
    changedRunPublicId: changedCompleted.publicId,
    detailUrl: `${frontendURL}/data-pipelines/${pipeline.publicId}`,
    outputs: changedCompleted.outputs,
  }
}

async function runSnapshotMergeBackfillScenario(workspacePublicId) {
  const suffix = `snapshot_merge_backfill_${Date.now()}`
  const file = await uploadSnapshotJSONFile(workspacePublicId)
  const pipeline = await jsonRequest('/api/v1/data-pipelines', 'POST', {
    name: `Smoke: snapshot_merge_backfill metadata ${new Date().toISOString()}`,
    description: 'Smoke test for Data Pipeline SCD2 merge backfill output registration.',
  }, { 'Idempotency-Key': crypto.randomUUID() })
  created.pipelinePublicIds.push(pipeline.publicId)
  const version = await createPublishedVersion(pipeline.publicId, snapshotMergeBackfillGraph(file.publicId, suffix))

  const firstRun = await requestRun(version.publicId)
  const firstCompleted = await waitForRun(pipeline.publicId, firstRun.publicId, assertSnapshotSCD2Run)

  const lateFile = await uploadSnapshotLateJSONFile(workspacePublicId)
  const lateVersion = await createPublishedVersion(pipeline.publicId, snapshotMergeBackfillGraph(lateFile.publicId, suffix))
  const lateRun = await requestRun(lateVersion.publicId)
  const lateCompleted = await waitForRun(pipeline.publicId, lateRun.publicId, (run) => assertCommonRun(run, 4))
  const repeatedRun = await requestRun(lateVersion.publicId)
  const repeatedCompleted = await waitForRun(pipeline.publicId, repeatedRun.publicId, (run) => assertCommonRun(run, 4))

  return {
    scenario: 'snapshot_merge_backfill',
    workspacePublicId,
    driveFilePublicId: file.publicId,
    lateDriveFilePublicId: lateFile.publicId,
    pipelinePublicId: pipeline.publicId,
    versionPublicId: version.publicId,
    lateVersionPublicId: lateVersion.publicId,
    firstRunPublicId: firstCompleted.publicId,
    lateRunPublicId: lateCompleted.publicId,
    repeatedRunPublicId: repeatedCompleted.publicId,
    detailUrl: `${frontendURL}/data-pipelines/${pipeline.publicId}`,
    outputs: repeatedCompleted.outputs,
  }
}

function schemaForNode(validation, nodeId) {
  return (validation.outputSchemas ?? []).find((schema) => schema.nodeId === nodeId)
}

function assertValidationHasColumns(validation, nodeId, columns) {
  const schema = schemaForNode(validation, nodeId)
  if (!schema) {
    throw new Error(`validation schema missing for node ${nodeId}: ${JSON.stringify(validation)}`)
  }
  for (const column of columns) {
    if (!(schema.columns ?? []).includes(column)) {
      throw new Error(`validation schema for ${nodeId} missing ${column}: ${JSON.stringify(schema)}`)
    }
  }
}

function assertNoValidationWarnings(label, validation) {
  if (!validation.validationSummary?.valid) {
    throw new Error(`${label} validation summary is invalid: ${JSON.stringify(validation.validationSummary)}`)
  }
  if ((validation.nodeWarnings ?? []).length > 0) {
    throw new Error(`${label} unexpected validation warnings: ${JSON.stringify(validation.nodeWarnings)}`)
  }
}

async function runValidationScenario(workspacePublicId) {
  const suffix = `validation_${Date.now()}`
  const textFile = await uploadTextFile(workspacePublicId)
  const jsonFile = await uploadSchemaMappingReviewJSONFile(workspacePublicId)
  const productFile = await uploadProductReviewTextFile(workspacePublicId)
  const pipeline = await createDraftPipeline(`Smoke: validation endpoint ${new Date().toISOString()}`)

  const textValidation = await validateDraftGraph(pipeline.publicId, textGraph(textFile.publicId, `${suffix}_text`))
  assertNoValidationWarnings('text', textValidation)
  assertValidationHasColumns(textValidation, 'extract_text', ['file_public_id', 'text', 'confidence'])
  assertValidationHasColumns(textValidation, 'quality_report', ['text', 'confidence', 'quality_report_json'])
  assertValidationHasColumns(textValidation, 'output', ['file_public_id'])

  const schemaMappingValidation = await validateDraftGraph(pipeline.publicId, schemaMappingReviewGraph(jsonFile.publicId, `${suffix}_schema_mapping`))
  assertNoValidationWarnings('schema_mapping_review', schemaMappingValidation)
  assertValidationHasColumns(schemaMappingValidation, 'schema_mapping', ['file_public_id', 'invoice_id', 'amount', 'status', 'schema_mapping_confidence'])
  assertValidationHasColumns(schemaMappingValidation, 'output', ['file_public_id'])

  const productValidation = await validateDraftGraph(pipeline.publicId, productReviewGraph(productFile.publicId, `${suffix}_product`))
  assertNoValidationWarnings('product_review', productValidation)
  assertValidationHasColumns(productValidation, 'product_extraction', ['file_public_id', 'product_name', 'product_confidence', 'product_extraction_reason'])
  assertValidationHasColumns(productValidation, 'output', ['file_public_id'])

  const brokenGraph = textGraph(textFile.publicId, `${suffix}_broken`)
  const qualityNode = brokenGraph.nodes.find((item) => item.id === 'quality_report')
  qualityNode.data.config.columns = ['missing_validation_column']
  const brokenValidation = await validateDraftGraph(pipeline.publicId, brokenGraph)
  const missingWarning = (brokenValidation.nodeWarnings ?? []).find((warning) => warning.nodeId === 'quality_report' && warning.code === 'missing_upstream_columns')
  if (!missingWarning || !(missingWarning.columns ?? []).includes('missing_validation_column')) {
    throw new Error(`missing-column warning was not returned for broken graph: ${JSON.stringify(brokenValidation.nodeWarnings)}`)
  }

  return {
    scenario: 'validation',
    pipelinePublicId: pipeline.publicId,
    detailUrl: `${frontendURL}/data-pipelines/${pipeline.publicId}`,
    checked: [
      { label: 'text', schemas: textValidation.outputSchemas.length, warnings: textValidation.nodeWarnings.length },
      { label: 'schema_mapping_review', schemas: schemaMappingValidation.outputSchemas.length, warnings: schemaMappingValidation.nodeWarnings.length },
      { label: 'product_review', schemas: productValidation.outputSchemas.length, warnings: productValidation.nodeWarnings.length },
      { label: 'broken_quality_report', warning: missingWarning },
    ],
  }
}

async function cleanup() {
  if (!cleanupDriveFile) {
    return
  }
  for (const publicId of created.driveFilePublicIds) {
    try {
      await request(`/api/v1/drive/files/${encodeURIComponent(publicId)}`, {
        method: 'DELETE',
        headers: { 'X-CSRF-Token': csrfToken() },
      })
    } catch (error) {
      console.error(`cleanup Drive file failed: ${error.message}`)
    }
  }
}

async function main() {
  const scenarioNames = ['json', 'typed_output', 'partition', 'watermark_previous', 'snapshot_scd2', 'snapshot_append', 'snapshot_merge', 'snapshot_merge_backfill', 'union', 'excel', 'text', 'quarantine', 'route', 'review', 'field_review', 'table_review', 'schema_mapping_review', 'product_review', 'validation']
  const scenarios = requestedScenario === 'suite' ? ['json', 'typed_output', 'partition', 'watermark_previous', 'snapshot_scd2', 'snapshot_append', 'snapshot_merge', 'snapshot_merge_backfill', 'union', 'excel', 'text', 'quarantine', 'route', 'review', 'field_review', 'table_review', 'schema_mapping_review', 'product_review'] : [requestedScenario]
  for (const scenario of scenarios) {
    if (!scenarioNames.includes(scenario)) {
      throw new Error(`unknown smoke scenario: ${scenario}`)
    }
  }
  await login()
  const workspacePublicId = await resolveWorkspacePublicID()
  const results = []
  for (const scenario of scenarios) {
    results.push(scenario === 'validation'
      ? await runValidationScenario(workspacePublicId)
      : scenario === 'watermark_previous'
        ? await runWatermarkPreviousScenario(workspacePublicId)
        : scenario === 'snapshot_append'
          ? await runSnapshotAppendScenario(workspacePublicId)
          : scenario === 'snapshot_merge'
            ? await runSnapshotMergeScenario(workspacePublicId)
            : scenario === 'snapshot_merge_backfill'
              ? await runSnapshotMergeBackfillScenario(workspacePublicId)
      : await runScenario(workspacePublicId, scenario))
  }
  console.log(JSON.stringify(requestedScenario === 'suite' ? { scenarios: results } : results[0], null, 2))
}

const crcTable = new Uint32Array(256)
for (let i = 0; i < 256; i += 1) {
  let c = i
  for (let k = 0; k < 8; k += 1) {
    c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
  }
  crcTable[i] = c >>> 0
}

function crc32(buffer) {
  let crc = 0xffffffff
  for (const byte of buffer) {
    crc = crcTable[(crc ^ byte) & 0xff] ^ (crc >>> 8)
  }
  return (crc ^ 0xffffffff) >>> 0
}

function xmlEscape(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&apos;')
}

function columnName(index) {
  let name = ''
  let value = index + 1
  while (value > 0) {
    const mod = (value - 1) % 26
    name = String.fromCharCode(65 + mod) + name
    value = Math.floor((value - mod) / 26)
  }
  return name
}

function createMinimalXLSX(rows) {
  const sheetRows = rows.map((row, rowIndex) => {
    const cells = row.map((value, colIndex) => {
      const ref = `${columnName(colIndex)}${rowIndex + 1}`
      return `<c r="${ref}" t="inlineStr"><is><t>${xmlEscape(value)}</t></is></c>`
    }).join('')
    return `<row r="${rowIndex + 1}">${cells}</row>`
  }).join('')
  const files = {
    '[Content_Types].xml': '<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>',
    '_rels/.rels': '<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>',
    'xl/workbook.xml': '<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>',
    'xl/_rels/workbook.xml.rels': '<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>',
    'xl/worksheets/sheet1.xml': `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>${sheetRows}</sheetData></worksheet>`,
  }
  return zipStore(files)
}

function writeUInt16LE(value) {
  const buffer = Buffer.alloc(2)
  buffer.writeUInt16LE(value)
  return buffer
}

function writeUInt32LE(value) {
  const buffer = Buffer.alloc(4)
  buffer.writeUInt32LE(value >>> 0)
  return buffer
}

function zipStore(files) {
  const localParts = []
  const centralParts = []
  let offset = 0
  for (const [name, content] of Object.entries(files)) {
    const nameBuffer = Buffer.from(name)
    const data = Buffer.from(content)
    const crc = crc32(data)
    const local = Buffer.concat([
      writeUInt32LE(0x04034b50),
      writeUInt16LE(20),
      writeUInt16LE(0),
      writeUInt16LE(0),
      writeUInt16LE(0),
      writeUInt16LE(0),
      writeUInt32LE(crc),
      writeUInt32LE(data.length),
      writeUInt32LE(data.length),
      writeUInt16LE(nameBuffer.length),
      writeUInt16LE(0),
      nameBuffer,
      data,
    ])
    const central = Buffer.concat([
      writeUInt32LE(0x02014b50),
      writeUInt16LE(20),
      writeUInt16LE(20),
      writeUInt16LE(0),
      writeUInt16LE(0),
      writeUInt16LE(0),
      writeUInt16LE(0),
      writeUInt32LE(crc),
      writeUInt32LE(data.length),
      writeUInt32LE(data.length),
      writeUInt16LE(nameBuffer.length),
      writeUInt16LE(0),
      writeUInt16LE(0),
      writeUInt16LE(0),
      writeUInt16LE(0),
      writeUInt32LE(0),
      writeUInt32LE(offset),
      nameBuffer,
    ])
    localParts.push(local)
    centralParts.push(central)
    offset += local.length
  }
  const centralDirectory = Buffer.concat(centralParts)
  const end = Buffer.concat([
    writeUInt32LE(0x06054b50),
    writeUInt16LE(0),
    writeUInt16LE(0),
    writeUInt16LE(centralParts.length),
    writeUInt16LE(centralParts.length),
    writeUInt32LE(centralDirectory.length),
    writeUInt32LE(offset),
    writeUInt16LE(0),
  ])
  return Buffer.concat([...localParts, centralDirectory, end])
}

main()
  .catch((error) => {
    console.error(error)
    process.exitCode = 1
  })
  .finally(async () => {
    await cleanup()
  })
