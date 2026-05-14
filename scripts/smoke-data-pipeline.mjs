const baseURL = (process.env.HAOHAO_SMOKE_BASE_URL ?? process.env.HAOHAO_BASE_URL ?? 'http://127.0.0.1:8080').replace(/\/+$/, '')
const frontendURL = (process.env.HAOHAO_FRONTEND_BASE_URL ?? 'http://127.0.0.1:5173').replace(/\/+$/, '')
const tenantSlug = process.env.HAOHAO_TENANT_SLUG ?? 'acme'
const email = process.env.HAOHAO_SMOKE_EMAIL ?? process.env.HAOHAO_DEMO_EMAIL ?? 'demo@example.com'
const password = process.env.HAOHAO_SMOKE_PASSWORD ?? process.env.HAOHAO_DEMO_PASSWORD ?? 'changeme123'
const configuredWorkspacePublicID = process.env.HAOHAO_DRIVE_WORKSPACE_PUBLIC_ID ?? ''
const pollAttempts = Number(process.env.HAOHAO_SMOKE_POLL_ATTEMPTS ?? '60')
const pollDelayMs = Number(process.env.HAOHAO_SMOKE_POLL_DELAY_MS ?? '1500')
const cleanupDriveFile = process.env.HAOHAO_SMOKE_CLEANUP_DRIVE_FILE === '1'

const cookies = new Map()
const created = {
  driveFilePublicId: '',
  pipelinePublicId: '',
  versionPublicId: '',
  runPublicId: '',
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

async function uploadJSONFile(workspacePublicId) {
  const rows = [
    { id: '1', name: 'Alpha', amount: 10, status: 'ready' },
    { id: '2', name: 'Beta', amount: 20, status: 'ready' },
    { id: '3', name: '', amount: -5, status: 'hold' },
  ]
  const form = new FormData()
  form.set('workspacePublicId', workspacePublicId)
  form.set('file', new Blob([JSON.stringify(rows, null, 2)], { type: 'application/json' }), `data-pipeline-smoke-${Date.now()}.json`)
  const file = await request('/api/v1/drive/files', {
    method: 'POST',
    headers: { 'X-CSRF-Token': csrfToken() },
    body: form,
  })
  created.driveFilePublicId = file.publicId
  return file
}

function node(id, stepType, label, x, y, config = {}) {
  return {
    id,
    type: 'pipelineStep',
    position: { x, y },
    data: { stepType, label, config },
  }
}

function graph(filePublicId, suffix) {
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
        ],
      }),
      node('profile', 'profile', 'Profile smoke rows', 620, 120, {}),
      node('validate', 'validate', 'Validate smoke rows', 900, 120, {
        rules: [
          { column: 'name', operator: 'required', severity: 'error' },
          { column: 'amount', operator: 'range', min: 0, max: 100, severity: 'warning' },
          { column: 'status', operator: 'in', values: ['ready'], severity: 'warning' },
        ],
      }),
      node('output', 'output', 'Smoke output', 1180, 120, {
        displayName: `data pipeline smoke output ${suffix}`,
        tableName: `data_pipeline_smoke_output_${suffix}`,
        writeMode: 'replace',
        orderBy: ['id'],
      }),
    ],
    edges: [
      { id: 'input-json_extract', source: 'input', target: 'json_extract' },
      { id: 'json_extract-profile', source: 'json_extract', target: 'profile' },
      { id: 'profile-validate', source: 'profile', target: 'validate' },
      { id: 'validate-output', source: 'validate', target: 'output' },
    ],
  }
}

async function createPipeline(filePublicId) {
  const suffix = String(Date.now())
  const pipeline = await jsonRequest('/api/v1/data-pipelines', 'POST', {
    name: `Smoke: Drive JSON metadata ${new Date().toISOString()}`,
    description: 'Smoke test for Drive file input, JSON extract, profile metadata, validation metadata, and output registration.',
  }, { 'Idempotency-Key': crypto.randomUUID() })
  created.pipelinePublicId = pipeline.publicId

  const version = await jsonRequest(`/api/v1/data-pipelines/${encodeURIComponent(pipeline.publicId)}/versions`, 'POST', {
    graph: graph(filePublicId, suffix),
  })
  const published = await jsonRequest(`/api/v1/data-pipeline-versions/${encodeURIComponent(version.publicId)}/publish`, 'POST', {})
  created.versionPublicId = published.publicId
  return { pipeline, version: published }
}

async function requestRun(versionPublicId) {
  const run = await jsonRequest(`/api/v1/data-pipeline-versions/${encodeURIComponent(versionPublicId)}/runs`, 'POST', {}, {
    'Idempotency-Key': crypto.randomUUID(),
  })
  created.runPublicId = run.publicId
  return run
}

function findStep(run, nodeId) {
  return (run.steps ?? []).find((step) => step.nodeId === nodeId)
}

function assertRun(run) {
  if (run.status !== 'completed') {
    throw new Error(`run did not complete: ${run.status} ${run.errorSummary ?? ''}`)
  }
  if (run.rowCount !== 3) {
    throw new Error(`run rowCount = ${run.rowCount}, want 3`)
  }
  const profileStep = findStep(run, 'profile')
  const validateStep = findStep(run, 'validate')
  const output = (run.outputs ?? []).find((item) => item.nodeId === 'output')
  if (!profileStep?.metadata?.profile) {
    throw new Error(`profile metadata missing: ${JSON.stringify(profileStep)}`)
  }
  if (profileStep.metadata.profile.rowCount !== 3 || profileStep.metadata.profile.columnCount < 4) {
    throw new Error(`unexpected profile metadata: ${JSON.stringify(profileStep.metadata.profile)}`)
  }
  if (!validateStep?.metadata?.validation) {
    throw new Error(`validation metadata missing: ${JSON.stringify(validateStep)}`)
  }
  const validation = validateStep.metadata.validation
  if (validation.warningCount < 2 || validation.failedRows < 2) {
    throw new Error(`unexpected validation metadata: ${JSON.stringify(validation)}`)
  }
  if (!output || output.status !== 'completed' || output.rowCount !== 3) {
    throw new Error(`unexpected output: ${JSON.stringify(output)}`)
  }
}

async function waitForRun(pipelinePublicId, runPublicId) {
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

async function cleanup() {
  if (!cleanupDriveFile || !created.driveFilePublicId) {
    return
  }
  try {
    await request(`/api/v1/drive/files/${encodeURIComponent(created.driveFilePublicId)}`, {
      method: 'DELETE',
      headers: { 'X-CSRF-Token': csrfToken() },
    })
  } catch (error) {
    console.error(`cleanup Drive file failed: ${error.message}`)
  }
}

async function main() {
  await login()
  const workspacePublicId = await resolveWorkspacePublicID()
  const file = await uploadJSONFile(workspacePublicId)
  const { pipeline, version } = await createPipeline(file.publicId)
  const run = await requestRun(version.publicId)
  const completed = await waitForRun(pipeline.publicId, run.publicId)
  console.log(JSON.stringify({
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
    outputs: completed.outputs,
  }, null, 2))
}

main()
  .catch((error) => {
    console.error(error)
    process.exitCode = 1
  })
  .finally(async () => {
    await cleanup()
  })
