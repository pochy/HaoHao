import { spawnSync } from 'node:child_process'
import { mkdir, readFile, stat, writeFile } from 'node:fs/promises'
import { basename, extname, resolve } from 'node:path'

const baseUrl = process.env.HAOHAO_BASE_URL ?? 'http://127.0.0.1:8080'
const email = process.env.HAOHAO_DEMO_EMAIL ?? 'demo@example.com'
const password = process.env.HAOHAO_DEMO_PASSWORD ?? 'changeme123'
const tenantSlug = process.env.HAOHAO_TENANT_SLUG ?? 'acme'
const sampleDir = resolve('samples')
const font = '/System/Library/Fonts/ヒラギノ角ゴシック W4.ttc'
const pdfOnly = process.argv.includes('--pdf-only')
const existingPDFOrderFilePublicId = process.env.HAOHAO_PDF_ORDER_FILE_PUBLIC_ID
const existingSharpCatalogFilePublicId = process.env.HAOHAO_SHARP_CATALOG_FILE_PUBLIC_ID

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

async function renderSample(name, body) {
  await mkdir(sampleDir, { recursive: true })
  const textPath = resolve(sampleDir, `${name}.txt`)
  const imagePath = resolve(sampleDir, `${name}.png`)
  await writeFile(textPath, body, 'utf8')
  const rendered = spawnSync('magick', [
    '-background', '#ffffff',
    '-fill', '#111827',
    '-font', font,
    '-pointsize', '34',
    '-size', '1400x1800',
    `caption:@${textPath}`,
    '-bordercolor', '#ffffff',
    '-border', '64',
    imagePath,
  ], { encoding: 'utf8' })
  if (rendered.status !== 0) {
    throw new Error(`failed to render ${imagePath}: ${rendered.stderr || rendered.stdout}`)
  }
  return { textPath, imagePath }
}

async function renderPDFSample(name, body) {
  const rendered = await renderSample(name, body)
  const jpegPath = resolve(sampleDir, `${name}.jpg`)
  const pdfPath = resolve(sampleDir, `${name}.pdf`)
  const converted = spawnSync('magick', [
    rendered.imagePath,
    '-background', '#ffffff',
    '-alpha', 'remove',
    '-alpha', 'off',
    '-quality', '92',
    jpegPath,
  ], { encoding: 'utf8' })
  if (converted.status !== 0) {
    throw new Error(`failed to convert ${rendered.imagePath} to jpeg: ${converted.stderr || converted.stdout}`)
  }
  const jpeg = await readFile(jpegPath)
  await writeFile(pdfPath, createSingleImagePDF(jpeg))
  return { ...rendered, jpegPath, pdfPath }
}

async function upload(path) {
  const form = new FormData()
  const file = await openAsBlob(path, { type: contentTypeForPath(path) })
  form.append('file', file, basename(path))
  return request('/api/v1/drive/files', { method: 'POST', body: form })
}

async function fileExists(path) {
  try {
    await stat(path)
    return true
  } catch {
    return false
  }
}

function contentTypeForPath(path) {
  switch (extname(path).toLowerCase()) {
    case '.pdf':
      return 'application/pdf'
    case '.jpg':
    case '.jpeg':
      return 'image/jpeg'
    case '.png':
    default:
      return 'image/png'
  }
}

function createSingleImagePDF(jpeg) {
  const { width, height } = jpegSize(jpeg)
  const content = Buffer.from(`q\n${width} 0 0 ${height} 0 0 cm\n/Im0 Do\nQ\n`, 'ascii')
  const objects = [
    Buffer.from('<< /Type /Catalog /Pages 2 0 R >>\n', 'ascii'),
    Buffer.from('<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n', 'ascii'),
    Buffer.from(`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${width} ${height}] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R >>\n`, 'ascii'),
    streamObject(content),
    streamObject(jpeg, `<< /Type /XObject /Subtype /Image /Width ${width} /Height ${height} /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length ${jpeg.length} >>`),
  ]
  const chunks = []
  const offsets = [0]
  let offset = 0
  const add = (chunk) => {
    const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk, 'binary')
    chunks.push(buffer)
    offset += buffer.length
  }
  add('%PDF-1.4\n%\xFF\xFF\xFF\xFF\n')
  objects.forEach((object, index) => {
    offsets.push(offset)
    add(`${index + 1} 0 obj\n`)
    add(object)
    add('endobj\n')
  })
  const xrefOffset = offset
  add(`xref\n0 ${objects.length + 1}\n`)
  add('0000000000 65535 f \n')
  for (let i = 1; i < offsets.length; i += 1) {
    add(`${String(offsets[i]).padStart(10, '0')} 00000 n \n`)
  }
  add(`trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${xrefOffset}\n%%EOF\n`)
  return Buffer.concat(chunks)
}

function streamObject(body, dictionary = `<< /Length ${body.length} >>`) {
  return Buffer.concat([
    Buffer.from(`${dictionary}\nstream\n`, 'ascii'),
    body,
    Buffer.from('\nendstream\n', 'ascii'),
  ])
}

function jpegSize(buffer) {
  if (buffer[0] !== 0xff || buffer[1] !== 0xd8) {
    throw new Error('not a jpeg image')
  }
  let offset = 2
  while (offset < buffer.length) {
    while (buffer[offset] === 0xff) offset += 1
    const marker = buffer[offset]
    offset += 1
    if (marker === 0xd9 || marker === 0xda) break
    const length = buffer.readUInt16BE(offset)
    if (marker >= 0xc0 && marker <= 0xc3) {
      return { height: buffer.readUInt16BE(offset + 3), width: buffer.readUInt16BE(offset + 5) }
    }
    offset += length
  }
  throw new Error('jpeg dimensions not found')
}

async function openAsBlob(path, options) {
  return globalThis.Blob
    ? await import('node:fs').then(({ openAsBlob }) => openAsBlob(path, options))
    : undefined
}

function node(id, stepType, label, x, y, config = {}) {
  return {
    id,
    type: 'pipelineStep',
    position: { x, y },
    data: { stepType, label, config },
  }
}

function linearGraph(files, steps, outputLabel, tableName) {
  const nodes = [
    node('input', 'input', 'Drive ファイル', 60, 120, {
      sourceKind: 'drive_file',
      filePublicIds: files.map((file) => file.publicId),
    }),
    ...steps.map((step, index) => node(step.id, step.type, step.label, 320 + index * 260, 120 + (index % 2) * 90, step.config ?? {})),
    node('output', 'output', outputLabel, 320 + steps.length * 260, 120, {
      displayName: outputLabel,
      tableName,
    }),
  ]
  const edges = []
  for (let i = 0; i < nodes.length - 1; i += 1) {
    edges.push({ id: `${nodes[i].id}-${nodes[i + 1].id}`, source: nodes[i].id, target: nodes[i + 1].id })
  }
  return { nodes, edges }
}

function graphFromSpecs(files, specs, edgePairs, outputLabel, tableName) {
  const outputX = Math.max(580, ...specs.map((spec) => spec.x)) + 260
  return {
    nodes: [
      node('input', 'input', 'Drive ファイル', 60, 160, {
        sourceKind: 'drive_file',
        filePublicIds: files.map((file) => file.publicId),
      }),
      ...specs.map((spec) => node(spec.id, spec.type, spec.label, spec.x, spec.y, spec.config ?? {})),
      node('output', 'output', outputLabel, outputX, 160, {
        displayName: outputLabel,
        tableName,
      }),
    ],
    edges: edgePairs.map(([source, target]) => ({ id: `${source}-${target}`, source, target })),
  }
}

function estimateProcurementGraph(files, options) {
  return graphFromSpecs(files, [
    { id: 'extract_text', type: 'extract_text', label: '見積書 OCR テキスト抽出', x: 320, y: 160, config: { languages: ['japanese'], chunkMode: 'full_text', includeBoxes: true } },
    { id: 'detect_language', type: 'detect_language_encoding', label: '日本語/文字化け確認', x: 580, y: 80, config: { textColumn: 'text', outputTextColumn: 'normalized_text', languageColumn: 'language', mojibakeScoreColumn: 'mojibake_score' } },
    { id: 'classify_document', type: 'classify_document', label: '見積カテゴリ分類', x: 840, y: 80, config: { outputColumn: 'document_type', classes: options.classes } },
    { id: 'extract_fields', type: 'extract_fields', label: '購買ヘッダー抽出', x: 1100, y: 80, config: { fields: options.fields } },
    { id: 'canonicalize', type: 'canonicalize', label: '会社名/日付正規化', x: 1360, y: 80, config: { rules: [
      { column: 'customer_name', outputColumn: 'customer_canonical', operations: ['trim', 'normalize_spaces', 'zenkaku_to_hankaku_basic'] },
      { column: 'supplier_name', outputColumn: 'supplier_canonical', operations: ['trim', 'normalize_spaces', 'zenkaku_to_hankaku_basic'], mappings: { '城南オフィスサービス株式会社': 'jonan_office_service' } },
      { column: 'valid_until', outputColumn: 'valid_until_canonical', operations: ['trim', 'zenkaku_to_hankaku_basic', 'normalize_date'] },
    ] } },
    { id: 'extract_table', type: 'extract_table', label: '見積明細表抽出', x: 580, y: 260, config: { source: 'text_delimited', delimiter: ',', headerRow: true } },
    { id: 'join_header_lines', type: 'join', label: 'ヘッダーと明細を結合', x: 1620, y: 160, config: { joinType: 'left', joinStrictness: 'all', leftKeys: ['file_public_id'], rightKeys: ['file_public_id'], selectColumns: ['document_type', 'estimate_number', 'customer_canonical', 'supplier_canonical', 'valid_until_canonical', 'field_confidence'] } },
    { id: 'confidence_gate', type: 'confidence_gate', label: '購買レビュー判定', x: 1880, y: 160, config: { scoreColumns: ['confidence', 'field_confidence'], threshold: 0.82, mode: 'annotate' } },
    { id: 'quality_report', type: 'quality_report', label: '見積品質レポート', x: 2140, y: 160, config: { columns: options.qualityColumns, outputMode: 'row_summary' } },
  ], [
    ['input', 'extract_text'],
    ['extract_text', 'detect_language'],
    ['detect_language', 'classify_document'],
    ['classify_document', 'extract_fields'],
    ['extract_fields', 'canonicalize'],
    ['extract_text', 'extract_table'],
    ['extract_table', 'join_header_lines'],
    ['canonicalize', 'join_header_lines'],
    ['join_header_lines', 'confidence_gate'],
    ['confidence_gate', 'quality_report'],
    ['quality_report', 'output'],
  ], options.outputLabel, options.tableName)
}

function medicalClaimTriageGraph(files, options) {
  return graphFromSpecs(files, [
    { id: 'extract_text', type: 'extract_text', label: '診療明細 OCR', x: 320, y: 160, config: { languages: ['japanese'], chunkMode: 'full_text', includeBoxes: true } },
    { id: 'redact_pii', type: 'redact_pii', label: '連絡先情報マスキング', x: 580, y: 80, config: { columns: ['text'], types: ['email', 'phone'], mode: 'mask', outputSuffix: '_redacted' } },
    { id: 'classify_document', type: 'classify_document', label: '医療帳票分類', x: 840, y: 80, config: { outputColumn: 'document_type', classes: options.classes } },
    { id: 'extract_fields', type: 'extract_fields', label: '請求ヘッダー抽出', x: 1100, y: 80, config: { fields: options.fields } },
    { id: 'extract_table', type: 'extract_table', label: '診療行抽出', x: 580, y: 260, config: { source: 'text_delimited', delimiter: ',', headerRow: true } },
    { id: 'confidence_gate', type: 'confidence_gate', label: '低信頼度行チェック', x: 840, y: 260, config: { scoreColumns: ['confidence'], threshold: 0.78, mode: 'annotate' } },
    { id: 'join_claim_lines', type: 'join', label: '請求ヘッダー付与', x: 1360, y: 160, config: { joinType: 'left', joinStrictness: 'all', leftKeys: ['file_public_id'], rightKeys: ['file_public_id'], selectColumns: ['document_type', 'claim_month', 'clinic_name', 'insurer_number', 'patient_id', 'field_confidence'] } },
    { id: 'human_review', type: 'human_review', label: '医療事務レビュー振分', x: 1620, y: 160, config: { reasonColumns: ['gate_status', 'field_confidence'], statusColumn: 'review_status', queueColumn: 'review_queue', queue: 'medical_billing_ops', mode: 'annotate' } },
    { id: 'quality_report', type: 'quality_report', label: '点検品質レポート', x: 1880, y: 160, config: { columns: options.qualityColumns, outputMode: 'row_summary' } },
  ], [
    ['input', 'extract_text'],
    ['extract_text', 'redact_pii'],
    ['redact_pii', 'classify_document'],
    ['classify_document', 'extract_fields'],
    ['extract_text', 'extract_table'],
    ['extract_table', 'confidence_gate'],
    ['confidence_gate', 'join_claim_lines'],
    ['extract_fields', 'join_claim_lines'],
    ['join_claim_lines', 'human_review'],
    ['human_review', 'quality_report'],
    ['quality_report', 'output'],
  ], options.outputLabel, options.tableName)
}

function deliveryExceptionGraph(files, options) {
  return graphFromSpecs(files, [
    { id: 'extract_text', type: 'extract_text', label: '検収書 OCR', x: 320, y: 160, config: { languages: ['japanese'], chunkMode: 'full_text', includeBoxes: true } },
    { id: 'extract_table', type: 'extract_table', label: '納品明細表抽出', x: 580, y: 260, config: { source: 'text_delimited', delimiter: ',', headerRow: true } },
    { id: 'deduplicate', type: 'deduplicate', label: '重複明細検出', x: 840, y: 260, config: { keyColumns: ['file_public_id', 'source_text'], mode: 'annotate', statusColumn: 'duplicate_status', groupColumn: 'duplicate_group_id' } },
    { id: 'classify_document', type: 'classify_document', label: '検収/納品書分類', x: 580, y: 80, config: { outputColumn: 'document_type', classes: options.classes } },
    { id: 'extract_fields', type: 'extract_fields', label: '検収ヘッダー抽出', x: 840, y: 80, config: { fields: options.fields } },
    { id: 'entity_resolution', type: 'entity_resolution', label: '納入業者名寄せ', x: 1100, y: 80, config: { column: 'supplier_name', outputPrefix: 'supplier', dictionary: [
      { entityId: 'supplier_tozai_packaging', name: '東西梱包株式会社', aliases: ['東西梱包株式会社', '東西梱包'] },
    ] } },
    { id: 'join_header_lines', type: 'join', label: '差異明細へヘッダー付与', x: 1360, y: 160, config: { joinType: 'left', joinStrictness: 'all', leftKeys: ['file_public_id'], rightKeys: ['file_public_id'], selectColumns: ['document_type', 'inspection_number', 'purchase_order_number', 'supplier_name', 'supplier_entity_id', 'supplier_match_score', 'field_confidence'] } },
    { id: 'sample_compare', type: 'sample_compare', label: '納品行確認サンプル', x: 1620, y: 160, config: { pairs: [
      { field: 'line', beforeColumn: 'source_text', afterColumn: 'row_json' },
      { field: 'supplier', beforeColumn: 'supplier_name', afterColumn: 'supplier_entity_id' },
    ] } },
    { id: 'quality_report', type: 'quality_report', label: '検収品質レポート', x: 1880, y: 160, config: { columns: options.qualityColumns, outputMode: 'row_summary' } },
  ], [
    ['input', 'extract_text'],
    ['extract_text', 'extract_table'],
    ['extract_table', 'deduplicate'],
    ['extract_text', 'classify_document'],
    ['classify_document', 'extract_fields'],
    ['extract_fields', 'entity_resolution'],
    ['deduplicate', 'join_header_lines'],
    ['entity_resolution', 'join_header_lines'],
    ['join_header_lines', 'sample_compare'],
    ['sample_compare', 'quality_report'],
    ['quality_report', 'output'],
  ], options.outputLabel, options.tableName)
}

function utilityEnergyGraph(files, options) {
  return graphFromSpecs(files, [
    { id: 'extract_text', type: 'extract_text', label: '電力請求 OCR', x: 320, y: 160, config: { languages: ['japanese'], chunkMode: 'full_text', includeBoxes: true } },
    { id: 'classify_document', type: 'classify_document', label: '請求/検針票分類', x: 580, y: 160, config: { outputColumn: 'document_type', classes: options.classes } },
    { id: 'extract_table', type: 'extract_table', label: '期間別使用量抽出', x: 840, y: 240, config: { source: 'text_delimited', delimiter: ',', headerRow: true } },
    { id: 'unit_conversion', type: 'unit_conversion', label: '料金単位を JPY 化', x: 1100, y: 240, config: { rules: [
      { valueColumn: 'row_number', inputUnit: '行', outputUnit: 'row', outputValueColumn: 'line_number_normalized', outputUnitColumn: 'line_number_unit', conversions: [{ from: '行', to: 'row', rate: 1 }] },
    ] } },
    { id: 'extract_fields', type: 'extract_fields', label: '施設/契約情報抽出', x: 840, y: 80, config: { fields: options.fields } },
    { id: 'canonicalize', type: 'canonicalize', label: '施設名/契約者正規化', x: 1100, y: 80, config: { rules: [
      { column: 'account_name', outputColumn: 'account_canonical', operations: ['trim', 'normalize_spaces', 'zenkaku_to_hankaku_basic'] },
      { column: 'facility_name', outputColumn: 'facility_canonical', operations: ['trim', 'normalize_spaces', 'zenkaku_to_hankaku_basic'] },
    ] } },
    { id: 'join_usage_header', type: 'join', label: '使用量に施設情報付与', x: 1360, y: 160, config: { joinType: 'left', joinStrictness: 'all', leftKeys: ['file_public_id'], rightKeys: ['file_public_id'], selectColumns: ['document_type', 'bill_number', 'account_canonical', 'facility_canonical', 'billing_amount_jpy', 'field_confidence'] } },
    { id: 'schema_inference', type: 'schema_inference', label: 'エネルギー台帳スキーマ推定', x: 1620, y: 160, config: { columns: ['bill_number', 'facility_canonical', 'billing_amount_jpy', 'row_json'], sampleLimit: 100 } },
    { id: 'confidence_gate', type: 'confidence_gate', label: '施設台帳登録判定', x: 1880, y: 160, config: { scoreColumns: ['confidence', 'field_confidence'], threshold: 0.8, mode: 'annotate' } },
    { id: 'quality_report', type: 'quality_report', label: '電力データ品質レポート', x: 2140, y: 160, config: { columns: options.qualityColumns, outputMode: 'row_summary' } },
  ], [
    ['input', 'extract_text'],
    ['extract_text', 'classify_document'],
    ['classify_document', 'extract_table'],
    ['extract_table', 'unit_conversion'],
    ['classify_document', 'extract_fields'],
    ['extract_fields', 'canonicalize'],
    ['unit_conversion', 'join_usage_header'],
    ['canonicalize', 'join_usage_header'],
    ['join_usage_header', 'schema_inference'],
    ['schema_inference', 'confidence_gate'],
    ['confidence_gate', 'quality_report'],
    ['quality_report', 'output'],
  ], options.outputLabel, options.tableName)
}

async function createPipeline(definition) {
  const pipeline = await request('/api/v1/data-pipelines', {
    method: 'POST',
    headers: { 'Idempotency-Key': crypto.randomUUID() },
    body: JSON.stringify({ name: definition.name, description: definition.description }),
  })
  const version = await request(`/api/v1/data-pipelines/${pipeline.publicId}/versions`, {
    method: 'POST',
    body: JSON.stringify({ graph: definition.graph }),
  })
  const published = await request(`/api/v1/data-pipeline-versions/${version.publicId}/publish`, {
    method: 'POST',
  })
  return { pipeline, version: published }
}

function shouldCreatePipeline(key) {
  return !pdfOnly || key === 'pdf_order' || key === 'sharp_catalog'
}

async function main() {
  await login()

  let invoice
  let contract
  let expenseA
  let expenseB
  let expenseC
  let leaseEstimate
  let medicalClaim
  let deliveryInspection
  let utilityBill

  if (!pdfOnly) {
    invoice = await renderSample('ja-unstructured-invoice-aoba-20260503', `請求書
発行日: 2026年05月03日
請求書番号: INV-2026-0503-014
取引先: 株式会社青葉商事
支払期日: 2026年05月31日

明細
データ入力代行費, 120,000円
紙伝票スキャン費, 45,000円
消費税, 16,500円
合計金額: 181,500円

担当: 経理部 佐藤
備考: 月末締め翌月末払い。`)

    contract = await renderSample('ja-unstructured-contract-review-20260503', `業務委託契約 確認メモ
作成日: 2026年05月03日

株式会社桜食品は、株式会社北斗物流に冷蔵配送業務を委託する。
株式会社北斗物流は、毎営業日 09:00 までに配送計画を株式会社桜食品へ提出する。
契約期間: 2026年06月01日 から 2027年05月31日

担当者連絡先:
桜食品 購買部 山田 花子 hanako.yamada@sakura-foods.example
北斗物流 営業部 鈴木 一郎 03-1234-5678

注意: 個人情報を含むため、外部共有前にマスキングを行う。`)

    expenseA = await renderSample('ja-unstructured-expense-tanaka-1-20260503', `経費精算申請 控え
申請日: 2026年05月03日
社員名: 田中 太郎
利用日: 2026年04月28日
支払先: JR東日本
用途: 顧客訪問 交通費
区間: 東京駅 から 横浜駅
金額: 960円
承認者: 営業部 課長`)

    expenseB = await renderSample('ja-unstructured-expense-tanaka-duplicate-20260503', `経費精算申請 控え
申請日: 2026年05月03日
社員名: 田中　太郎
利用日: 2026/04/28
支払先: ＪＲ東日本
用途: 顧客訪問 交通費
区間: 東京駅 から 横浜駅
金額: 960 円
承認者: 営業部 課長`)

    expenseC = await renderSample('ja-unstructured-expense-suzuki-20260503', `経費精算申請 控え
申請日: 2026年05月03日
社員名: 鈴木 一郎
利用日: 2026年04月30日
支払先: 日本交通
用途: 顧客訪問 タクシー代
区間: 品川 から 大手町
金額: 3,420円
承認者: 営業部 課長`)

    leaseEstimate = await renderSample('ja-unstructured-lease-estimate-20260503', `オフィス移転見積書
見積日: 2026年05月03日
見積番号: EST-2026-0503-22
顧客名: 株式会社南青山デザイン
提出会社: 城南オフィスサービス株式会社
有効期限: 2026年05月17日

明細
品目,数量,単価,金額
デスク搬入,24,18000,432000
会議室什器組立,3,52000,156000
旧オフィス養生,1,88000,88000
合計,28,0,676000

備考: 休日作業の場合は別途15%の割増。`)

    medicalClaim = await renderSample('ja-unstructured-medical-claim-20260503', `診療報酬明細 確認票
作成日: 2026年05月03日
請求月: 2026年04月
医療機関: 青葉内科クリニック
保険者番号: 06123456
患者ID: P-20493
患者名: 山本 恵

明細
診療日,診療区分,点数,自己負担額
2026-04-08,初診,288,860
2026-04-08,検査,1250,3750
2026-04-15,投薬,410,1230
合計,件数3,1948,5840

注意: 患者情報を含むため、共有前に確認が必要。`)

    deliveryInspection = await renderSample('ja-unstructured-delivery-inspection-20260503', `納品検収書
検収日: 2026年05月03日
検収番号: ACC-2026-0503-18
発注番号: PO-2026-0418-44
納入先: 株式会社北浜フーズ 大阪物流センター
納入業者: 東西梱包株式会社
担当者: 物流部 森

明細
商品コード,商品名,発注数量,納品数量,不良数量
FD-1001,冷凍餃子 20個入,120,120,0
FD-2030,冷凍焼売 30個入,80,78,2
FD-3302,春巻き 業務用,60,60,0

判定: 一部不良あり。差分は交換手配。`)

    utilityBill = await renderSample('ja-unstructured-utility-bill-20260503', `電力使用量のお知らせ
発行日: 2026年05月03日
請求番号: ELEC-2026-0503-901
契約者: 株式会社晴海ロジスティクス
供給地点特定番号: 03-1234-5678-9012-3456
対象施設: 晴海第2倉庫
請求金額: 312,480円

使用量明細
期間,区分,使用量kWh,料金円
2026-04-01〜2026-04-10,昼間,4820,144600
2026-04-11〜2026-04-20,昼間,4510,135300
2026-04-21〜2026-04-30,夜間,1086,32580

備考: 冷蔵設備の増設により前年比 12% 増。`)
  }

  const pdfOrder = await renderPDFSample('ja-unstructured-pdf-purchase-order-20260503', `発注書
発注日: 2026年05月03日
発注番号: PO-2026-0503-07
発注元: 株式会社青葉商事
仕入先: 株式会社北斗物流
納品希望日: 2026年05月20日

明細
冷蔵配送チャーター, 2便, 85,000円
荷役作業費, 1式, 18,000円
消費税, 10,300円
発注金額: 113,300円

承認者: 購買部 部長
備考: 納品後、検収完了をもって請求可能。`)

  const uploaded = {}
  if (!pdfOnly) {
    uploaded.invoice = await upload(invoice.imagePath)
    uploaded.contract = await upload(contract.imagePath)
    uploaded.expenses = [
      await upload(expenseA.imagePath),
      await upload(expenseB.imagePath),
      await upload(expenseC.imagePath),
    ]
    uploaded.leaseEstimate = await upload(leaseEstimate.imagePath)
    uploaded.medicalClaim = await upload(medicalClaim.imagePath)
    uploaded.deliveryInspection = await upload(deliveryInspection.imagePath)
    uploaded.utilityBill = await upload(utilityBill.imagePath)
  }
  uploaded.pdfOrder = existingPDFOrderFilePublicId
    ? { publicId: existingPDFOrderFilePublicId }
    : await upload(pdfOrder.pdfPath)
  const sharpCatalogPath = resolve(sampleDir, 'SHARP-bd-recorder-2025-06.pdf')
  if (existingSharpCatalogFilePublicId) {
    uploaded.sharpCatalog = { publicId: existingSharpCatalogFilePublicId }
  } else if (await fileExists(sharpCatalogPath)) {
    uploaded.sharpCatalog = await upload(sharpCatalogPath)
  } else {
    console.warn(`Skipping SHARP catalog sample because ${sharpCatalogPath} does not exist. Set HAOHAO_SHARP_CATALOG_FILE_PUBLIC_ID to use an existing Drive file.`)
  }

  const pipelines = []
  const invoiceDescription = [
    '目的: 日本語の請求書スキャン画像を、支払処理やレビューに使える標準化済みデータへ変換するサンプルです。',
    '元データ: Drive にアップロードされた請求書画像。請求書番号、発行日、取引先、合計金額などが画像内に記載されています。',
    'ゴール: invoice_number、issue_date、vendor_canonical、amount_normalized、amount_unit、document_type、品質情報を持つ請求書支払データを作成します。',
    '処理: OCR でテキスト化し、言語と文字化け傾向を検出し、請求書として分類します。その後、正規表現で支払項目を抽出し、取引先表記を正規化し、円金額を JPY 単位の数値へ標準化し、スキーマ推定と品質レポートを付与します。',
  ].join('\n')
  const contractDescription = [
    '目的: 日本語の契約確認メモ画像から、法務レビューに必要な機密情報保護済みデータを作るサンプルです。',
    '元データ: Drive にアップロードされた契約確認メモ画像。会社名、委託関係、提出先、メールアドレスや電話番号などの PII が含まれる想定です。',
    'ゴール: OCR テキストから PII をマスキングし、委託・提出先などの関係情報、レビュー判定、品質情報を持つ契約メモレビュー用データを作成します。',
    '処理: OCR でテキスト化し、言語と文字化け傾向を検出します。メールアドレスや電話番号を検出してマスクし、マスク後テキストから関係パターンを抽出し、抽出件数や文字化けスコアをもとに人手レビュー対象を判定し、品質レポートを付与します。',
  ].join('\n')
  const expenseDescription = [
    '目的: 日本語の経費精算控え画像を、経費申請レビュー、監査、重複チェックに使える構造化データへ変換するサンプルです。',
    '元データ: Drive にアップロードされた経費精算控え画像3枚。社員名、利用日、支払先、用途、区間、金額、承認者などが画像内に記載されています。',
    'ゴール: employee_name、expense_date_canonical、vendor_canonical、amount_jpy、duplicate_status、survivor_flag、vendor_entity_id、品質情報を持つ経費精算レビュー用データを作成します。田中太郎の同一内容2件は同じ重複グループに入り、片方が duplicate になります。',
    '処理: OCR で日本語テキストを抽出し、正規表現で社員名、利用日、支払先、金額を抽出します。全角英数字、スペース、日付表記、支払先表記を正規化し、社員名、正規化済み利用日、金額をキーに重複申請を検出します。さらに支払先を辞書で JR東日本 や 日本交通 の標準エンティティIDへ名寄せし、正規化前後の差分と品質レポートを付与します。',
  ].join('\n')
  const pdfOrderDescription = [
    '目的: 日本語のPDF発注書を、購買レビューや発注管理に使える構造化データへ変換するサンプルです。',
    '元データ: Drive にアップロードされた1ページPDF。発注番号、発注日、発注元、仕入先、納品希望日、発注金額、承認者などがPDF内に記載されています。',
    'ゴール: purchase_order_number、order_date_canonical、buyer_canonical、supplier_canonical、delivery_date_canonical、amount_jpy、amount_normalized、品質情報を持つPDF発注書レビュー用データを作成します。',
    '処理: PDFをDrive OCRでページ画像として読み取り、日本語テキストを抽出します。正規表現で発注項目を抽出し、日付、会社名、金額を正規化し、円金額を JPY 単位の数値へ標準化し、必須項目の品質レポートを付与します。',
  ].join('\n')
  const sharpCatalogDescription = [
    '目的: SHARP のBD/4KレコーダーPDFカタログを、商品比較や販売資料レビューに使える構造化データへ変換するサンプルです。',
    '元データ: samples/SHARP-bd-recorder-2025-06.pdf。AQUOS 4Kレコーダー、ブルーレイディスクレコーダーの総合カタログPDFで、モデル名、HDD容量、録画機能、消費電力、URLなどが複数ページに記載されています。',
    'ゴール: catalog_title、catalog_issue、site_url、primary_4k_model、standard_model、hdd_capacity、power_watts、document_type、品質情報を持つPDFカタログレビュー用データを作成します。',
    '処理: PDFからDrive OCR/テキスト抽出を行い、言語検出とカタログ分類を実施します。正規表現で代表モデルや仕様値を抽出し、型番やURLを正規化し、抽出信頼度をもとに品質レポートを付与します。',
  ].join('\n')
  const leaseEstimateDescription = [
    '目的: オフィス移転見積書から、見積ヘッダーと明細表を抽出し、購買比較に使える見積明細データを作成するサンプルです。',
    '元データ: Drive にアップロードされた見積書画像。見積番号、顧客、提出会社、有効期限、作業別明細が記載されています。',
    'ゴール: document_type、estimate_number、customer_name、supplier_name、valid_until と、品目/数量/単価/金額の明細行を持つ見積レビュー用データを作成します。',
    '処理: OCR 後に言語/文字化けを確認し、見積書として分類します。ヘッダー抽出と会社名/日付正規化の流れ、明細表抽出の流れを分け、file_public_id で JOIN して購買レビュー判定と品質情報を付与します。',
  ].join('\n')
  const medicalClaimDescription = [
    '目的: 診療報酬明細の確認票から、請求ヘッダーと診療行を抽出し、医療事務の点検に使えるデータを作るサンプルです。',
    '元データ: Drive にアップロードされた診療報酬明細画像。請求月、医療機関、保険者番号、患者ID、診療日別の点数と自己負担額が記載されています。',
    'ゴール: claim_month、clinic_name、insurer_number、patient_id と、診療日/診療区分/点数/自己負担額の明細行を持つ点検用データを作成します。',
    '処理: OCR 後に外部共有前の連絡先マスキングを通してから医療帳票分類と請求ヘッダー抽出を行います。診療行抽出は別経路で先に低信頼度判定し、ヘッダーと JOIN したあと医療事務レビューキューへ振り分けます。',
  ].join('\n')
  const deliveryInspectionDescription = [
    '目的: 納品検収書から、発注番号と納品差異のある商品明細を構造化し、交換手配や仕入先評価に使えるデータを作るサンプルです。',
    '元データ: Drive にアップロードされた納品検収書画像。検収番号、発注番号、納入先、納入業者、商品別の発注数量/納品数量/不良数量が記載されています。',
    'ゴール: inspection_number、purchase_order_number、delivery_destination、supplier_name と、商品コード/商品名/数量/不良数の明細行を持つ検収データを作成します。',
    '処理: OCR 直後に明細表抽出と重複行検出を行う経路、帳票分類/ヘッダー抽出/納入業者名寄せを行う経路に分けます。最後に JOIN して、差異確認用のサンプル比較と品質情報を付与します。',
  ].join('\n')
  const utilityBillDescription = [
    '目的: 電力使用量のお知らせから、請求ヘッダーと期間別使用量を抽出し、施設別のエネルギー管理に使えるデータを作るサンプルです。',
    '元データ: Drive にアップロードされた電力請求書画像。契約者、供給地点、対象施設、請求金額、期間別の使用量と料金が記載されています。',
    'ゴール: bill_number、account_name、supply_point_number、facility_name、billing_amount_jpy と、期間/区分/使用量kWh/料金円の明細行を持つエネルギー管理データを作成します。',
    '処理: OCR と請求/検針票分類の後、期間別使用量の表抽出と施設/契約情報抽出を並列化します。使用量行へ施設情報を JOIN し、台帳スキーマ推定、登録判定、品質レポートを付与します。',
  ].join('\n')

  if (shouldCreatePipeline('lease_estimate')) pipelines.push(await createPipeline({
    name: 'サンプル: オフィス移転見積書 OCR から見積明細データ作成',
    description: leaseEstimateDescription,
    graph: estimateProcurementGraph([uploaded.leaseEstimate], {
      classes: [{ label: 'office_move_estimate', keywords: ['オフィス移転見積書', '見積番号', '有効期限', '明細'], priority: 10 }],
      fields: [
        { name: 'estimate_number', type: 'string', required: true, patterns: ['見積番号[:：]\\s*([A-Z0-9\\-]+)'] },
        { name: 'estimate_date', type: 'string', required: true, patterns: ['見積日[:：]\\s*([0-9]{4}年[0-9]{2}月[0-9]{2}日)'] },
        { name: 'customer_name', type: 'string', required: true, patterns: ['顧客名[:：]\\s*([^\\n]+)'] },
        { name: 'supplier_name', type: 'string', required: true, patterns: ['提出会社[:：]\\s*([^\\n]+)'] },
        { name: 'valid_until', type: 'string', patterns: ['有効期限[:：]\\s*([0-9]{4}年[0-9]{2}月[0-9]{2}日)'] },
      ],
      qualityColumns: ['document_type', 'estimate_number', 'customer_canonical', 'supplier_canonical', 'valid_until_canonical', 'field_confidence', 'row_json', 'gate_status'],
      outputLabel: 'オフィス移転見積明細 サンプル',
      tableName: 'sample_office_move_estimate_lines',
    }),
  }))

  if (shouldCreatePipeline('medical_claim')) pipelines.push(await createPipeline({
    name: 'サンプル: 診療報酬明細 OCR から医療請求点検データ作成',
    description: medicalClaimDescription,
    graph: medicalClaimTriageGraph([uploaded.medicalClaim], {
      classes: [{ label: 'medical_claim_statement', keywords: ['診療報酬明細', '保険者番号', '患者ID', '点数'], priority: 10 }],
      fields: [
        { name: 'claim_month', type: 'string', required: true, patterns: ['請求月[:：]\\s*([0-9]{4}年[0-9]{2}月)'] },
        { name: 'clinic_name', type: 'string', required: true, patterns: ['医療機関[:：]\\s*([^\\n]+)'] },
        { name: 'insurer_number', type: 'string', required: true, patterns: ['保険者番号[:：]\\s*([0-9]+)'] },
        { name: 'patient_id', type: 'string', required: true, patterns: ['患者ID[:：]\\s*([A-Z0-9\\-]+)'] },
      ],
      qualityColumns: ['document_type', 'claim_month', 'clinic_name', 'insurer_number', 'patient_id', 'field_confidence', 'row_json', 'gate_status', 'review_status'],
      outputLabel: '診療報酬点検明細 サンプル',
      tableName: 'sample_medical_claim_review_lines',
    }),
  }))

  if (shouldCreatePipeline('delivery_inspection')) pipelines.push(await createPipeline({
    name: 'サンプル: 納品検収書 OCR から納品差異データ作成',
    description: deliveryInspectionDescription,
    graph: deliveryExceptionGraph([uploaded.deliveryInspection], {
      classes: [{ label: 'delivery_inspection_report', keywords: ['納品検収書', '検収番号', '発注番号', '不良数量'], priority: 10 }],
      fields: [
        { name: 'inspection_number', type: 'string', required: true, patterns: ['検収番号[:：]\\s*([A-Z0-9\\-]+)'] },
        { name: 'inspection_date', type: 'string', required: true, patterns: ['検収日[:：]\\s*([0-9]{4}年[0-9]{2}月[0-9]{2}日)'] },
        { name: 'purchase_order_number', type: 'string', required: true, patterns: ['発注番号[:：]\\s*([A-Z0-9\\-]+)'] },
        { name: 'delivery_destination', type: 'string', required: true, patterns: ['納入先[:：]\\s*([^\\n]+)'] },
        { name: 'supplier_name', type: 'string', required: true, patterns: ['納入業者[:：]\\s*([^\\n]+)'] },
      ],
      qualityColumns: ['document_type', 'inspection_number', 'purchase_order_number', 'supplier_entity_id', 'supplier_match_score', 'duplicate_status', 'field_confidence', 'row_json', 'changed_field_count'],
      outputLabel: '納品検収明細 サンプル',
      tableName: 'sample_delivery_inspection_lines',
    }),
  }))

  if (shouldCreatePipeline('utility_bill')) pipelines.push(await createPipeline({
    name: 'サンプル: 電力請求書 OCR から施設別使用量データ作成',
    description: utilityBillDescription,
    graph: utilityEnergyGraph([uploaded.utilityBill], {
      classes: [{ label: 'utility_bill', keywords: ['電力使用量のお知らせ', '供給地点特定番号', '使用量明細', '請求金額'], priority: 10 }],
      fields: [
        { name: 'bill_number', type: 'string', required: true, patterns: ['請求番号[:：]\\s*([A-Z0-9\\-]+)'] },
        { name: 'issue_date', type: 'string', required: true, patterns: ['発行日[:：]\\s*([0-9]{4}年[0-9]{2}月[0-9]{2}日)'] },
        { name: 'account_name', type: 'string', required: true, patterns: ['契約者[:：]\\s*([^\\n]+)'] },
        { name: 'supply_point_number', type: 'string', required: true, patterns: ['供給地点特定番号[:：]\\s*([0-9\\-]+)'] },
        { name: 'facility_name', type: 'string', required: true, patterns: ['対象施設[:：]\\s*([^\\n]+)'] },
        { name: 'billing_amount_jpy', type: 'number', required: true, patterns: ['請求金額[:：]\\s*([0-9,]+)\\s*円'] },
      ],
      qualityColumns: ['document_type', 'bill_number', 'account_canonical', 'facility_canonical', 'billing_amount_jpy', 'field_confidence', 'row_json', 'gate_status'],
      outputLabel: '施設別電力使用量明細 サンプル',
      tableName: 'sample_utility_bill_usage_lines',
    }),
  }))

  if (shouldCreatePipeline('invoice')) pipelines.push(await createPipeline({
    name: 'サンプル: 日本語請求書 OCR から支払データ標準化',
    description: invoiceDescription,
    graph: linearGraph([uploaded.invoice], [
      { id: 'extract_text', type: 'extract_text', label: 'OCR テキスト抽出', config: { languages: ['japanese'], chunkMode: 'full_text', includeBoxes: true } },
      { id: 'detect_language', type: 'detect_language_encoding', label: '言語と文字化け検出', config: { textColumn: 'text', outputTextColumn: 'normalized_text', languageColumn: 'language', mojibakeScoreColumn: 'mojibake_score' } },
      { id: 'classify_document', type: 'classify_document', label: '請求書分類', config: { outputColumn: 'document_type', classes: [{ label: 'invoice', keywords: ['請求書', '請求書番号', '合計金額'], priority: 10 }] } },
      { id: 'extract_fields', type: 'extract_fields', label: '支払項目抽出', config: { fields: [
        { name: 'invoice_number', type: 'string', patterns: ['請求書番号[:：]\\s*([A-Z0-9\\-]+)'] },
        { name: 'issue_date', type: 'string', patterns: ['発行日[:：]\\s*([0-9]{4}年[0-9]{2}月[0-9]{2}日)'] },
        { name: 'vendor', type: 'string', patterns: ['取引先[:：]\\s*([^\\n]+)'] },
        { name: 'amount_jpy', type: 'number', patterns: ['合計金額[:：]\\s*([0-9,]+)\\s*円'] },
      ] } },
      { id: 'canonicalize', type: 'canonicalize', label: '表記ゆれ正規化', config: { rules: [
        { column: 'vendor', outputColumn: 'vendor_canonical', operations: ['trim', 'normalize_spaces', 'zenkaku_to_hankaku_basic'], mappings: { '株式会社青葉商事': 'aoba_trading' } },
      ] } },
      { id: 'unit_conversion', type: 'unit_conversion', label: '金額単位標準化', config: { rules: [
        { valueColumn: 'amount_jpy', inputUnit: '円', outputUnit: 'JPY', outputValueColumn: 'amount_normalized', outputUnitColumn: 'amount_unit', conversions: [{ from: '円', to: 'JPY', rate: 1 }] },
      ] } },
      { id: 'schema_inference', type: 'schema_inference', label: 'スキーマ推定', config: { columns: ['invoice_number', 'issue_date', 'vendor_canonical', 'amount_normalized'], sampleLimit: 100 } },
      { id: 'quality_report', type: 'quality_report', label: '品質レポート', config: { columns: ['invoice_number', 'vendor_canonical', 'amount_normalized', 'language'], outputMode: 'row_summary' } },
    ], '請求書支払データ サンプル', 'sample_ja_invoice_payables'),
  }))

  if (shouldCreatePipeline('contract')) pipelines.push(await createPipeline({
    name: 'サンプル: 日本語契約メモの PII マスキングと関係抽出',
    description: contractDescription,
    graph: linearGraph([uploaded.contract], [
      { id: 'extract_text', type: 'extract_text', label: 'OCR テキスト抽出', config: { languages: ['japanese'], chunkMode: 'full_text', includeBoxes: true } },
      { id: 'detect_language', type: 'detect_language_encoding', label: '言語と文字化け検出', config: { textColumn: 'text', outputTextColumn: 'normalized_text', languageColumn: 'language', mojibakeScoreColumn: 'mojibake_score' } },
      { id: 'redact_pii', type: 'redact_pii', label: 'PII マスキング', config: { columns: ['normalized_text'], types: ['email', 'phone'], mode: 'mask', outputSuffix: '_redacted' } },
      { id: 'relationship_extraction', type: 'relationship_extraction', label: '委託関係抽出', config: { textColumn: 'normalized_text_redacted', patterns: [
        { relationType: '委託', pattern: '(株式会社[^\\s、。]+)は、?(株式会社[^\\s、。]+)に[^。]*委託する' },
        { relationType: '提出先', pattern: '(株式会社[^\\s、。]+)は、?[^。]*を(株式会社[^\\s、。]+)へ提出する' },
      ] } },
      { id: 'human_review', type: 'human_review', label: '人手レビュー判定', config: { reasonColumns: ['mojibake_score', 'relationship_count'], statusColumn: 'review_status', queueColumn: 'review_queue', queue: 'legal_ops', mode: 'annotate' } },
      { id: 'quality_report', type: 'quality_report', label: '品質レポート', config: { columns: ['language', 'pii_detected', 'relationship_count', 'review_status'], outputMode: 'row_summary' } },
    ], '契約メモレビュー サンプル', 'sample_ja_contract_review'),
  }))

  if (shouldCreatePipeline('expenses')) pipelines.push(await createPipeline({
    name: 'サンプル: 日本語経費精算 OCR の重複検出と支払先名寄せ',
    description: expenseDescription,
    graph: linearGraph(uploaded.expenses, [
      { id: 'extract_text', type: 'extract_text', label: 'OCR テキスト抽出', config: { languages: ['japanese'], chunkMode: 'full_text', includeBoxes: true } },
      { id: 'extract_fields', type: 'extract_fields', label: '経費項目抽出', config: { fields: [
        { name: 'employee_name', type: 'string', patterns: ['社員名[:：]\\s*([^\\n]+)'] },
        { name: 'expense_date', type: 'string', patterns: ['利用日[:：]\\s*([0-9]{4}[年/][0-9]{2}[月/][0-9]{2}日?)'] },
        { name: 'vendor', type: 'string', patterns: ['支払先[:：]\\s*([^\\n]+)'] },
        { name: 'amount_jpy', type: 'number', patterns: ['金額[:：]\\s*([0-9,]+)\\s*円'] },
      ] } },
      { id: 'canonicalize', type: 'canonicalize', label: '申請内容正規化', config: { rules: [
        { column: 'employee_name', outputColumn: 'employee_name_canonical', operations: ['trim', 'normalize_spaces', 'zenkaku_to_hankaku_basic'], mappings: { '田中 太郎': '田中 太郎', '田中　太郎': '田中 太郎' } },
        { column: 'expense_date', outputColumn: 'expense_date_canonical', operations: ['trim', 'zenkaku_to_hankaku_basic', 'normalize_date'] },
        { column: 'vendor', outputColumn: 'vendor_canonical', operations: ['trim', 'normalize_spaces', 'zenkaku_to_hankaku_basic'], mappings: { 'ＪＲ東日本': 'JR東日本', 'JR東日本': 'JR東日本', '日本交通': '日本交通' } },
      ] } },
      { id: 'deduplicate', type: 'deduplicate', label: '重複申請検出', config: { keyColumns: ['employee_name_canonical', 'expense_date_canonical', 'amount_jpy'], mode: 'annotate', statusColumn: 'duplicate_status', groupColumn: 'duplicate_group_id' } },
      { id: 'entity_resolution', type: 'entity_resolution', label: '支払先名寄せ', config: { column: 'vendor_canonical', outputPrefix: 'vendor', dictionary: [
        { entityId: 'vendor_jreast', name: 'JR東日本', aliases: ['JR東日本', 'ＪＲ東日本', '東日本旅客鉄道'] },
        { entityId: 'vendor_nihon_kotsu', name: '日本交通', aliases: ['日本交通', '日本交通株式会社'] },
      ] } },
      { id: 'sample_compare', type: 'sample_compare', label: '正規化前後比較', config: { pairs: [
        { field: 'employee_name', beforeColumn: 'employee_name', afterColumn: 'employee_name_canonical' },
        { field: 'vendor', beforeColumn: 'vendor', afterColumn: 'vendor_canonical' },
      ] } },
      { id: 'quality_report', type: 'quality_report', label: '品質レポート', config: { columns: ['employee_name_canonical', 'vendor_entity_id', 'duplicate_status', 'changed_field_count'], outputMode: 'row_summary' } },
    ], '経費精算レビュー サンプル', 'sample_ja_expense_review'),
  }))

  if (shouldCreatePipeline('pdf_order')) pipelines.push(await createPipeline({
    name: 'サンプル: PDF発注書 OCR から購買データ標準化',
    description: pdfOrderDescription,
    graph: linearGraph([uploaded.pdfOrder], [
      { id: 'extract_text', type: 'extract_text', label: 'PDF OCR テキスト抽出', config: { languages: ['japanese'], chunkMode: 'full_text', includeBoxes: true } },
      { id: 'extract_fields', type: 'extract_fields', label: '発注項目抽出', config: { fields: [
        { name: 'purchase_order_number', type: 'string', patterns: ['発注番号[:：]\\s*([A-Z0-9\\-]+)'] },
        { name: 'order_date', type: 'string', patterns: ['発注日[:：]\\s*([0-9]{4}[年/][0-9]{2}[月/][0-9]{2}日?)'] },
        { name: 'buyer', type: 'string', patterns: ['発注元[:：]\\s*([^\\n]+)'] },
        { name: 'supplier', type: 'string', patterns: ['仕入先[:：]\\s*([^\\n]+)'] },
        { name: 'delivery_date', type: 'string', patterns: ['納品希望日[:：]\\s*([0-9]{4}[年/][0-9]{2}[月/][0-9]{2}日?)'] },
        { name: 'amount_jpy', type: 'number', patterns: ['発注金額[:：]\\s*([0-9,]+)\\s*円'] },
      ] } },
      { id: 'canonicalize', type: 'canonicalize', label: '発注内容正規化', config: { rules: [
        { column: 'order_date', outputColumn: 'order_date_canonical', operations: ['trim', 'zenkaku_to_hankaku_basic', 'normalize_date'] },
        { column: 'delivery_date', outputColumn: 'delivery_date_canonical', operations: ['trim', 'zenkaku_to_hankaku_basic', 'normalize_date'] },
        { column: 'buyer', outputColumn: 'buyer_canonical', operations: ['trim', 'normalize_spaces', 'zenkaku_to_hankaku_basic'], mappings: { '株式会社青葉商事': 'aoba_trading' } },
        { column: 'supplier', outputColumn: 'supplier_canonical', operations: ['trim', 'normalize_spaces', 'zenkaku_to_hankaku_basic'], mappings: { '株式会社北斗物流': 'hokuto_logistics' } },
      ] } },
      { id: 'unit_conversion', type: 'unit_conversion', label: '発注金額標準化', config: { rules: [
        { valueColumn: 'amount_jpy', inputUnit: '円', outputUnit: 'JPY', outputValueColumn: 'amount_normalized', outputUnitColumn: 'amount_unit', conversions: [{ from: '円', to: 'JPY', rate: 1 }] },
      ] } },
      { id: 'quality_report', type: 'quality_report', label: '品質レポート', config: { columns: ['purchase_order_number', 'order_date_canonical', 'supplier_canonical', 'delivery_date_canonical', 'amount_normalized'], outputMode: 'row_summary' } },
    ], 'PDF発注書レビュー サンプル', 'sample_pdf_purchase_orders'),
  }))

  if (shouldCreatePipeline('sharp_catalog') && uploaded.sharpCatalog) pipelines.push(await createPipeline({
    name: 'サンプル: SHARP PDFカタログ OCR から商品比較データ作成',
    description: sharpCatalogDescription,
    graph: linearGraph([uploaded.sharpCatalog], [
      { id: 'extract_text', type: 'extract_text', label: 'PDFカタログ テキスト抽出', config: { languages: ['japanese'], chunkMode: 'full_text', includeBoxes: false } },
      { id: 'detect_language', type: 'detect_language_encoding', label: '言語と文字化け検出', config: { textColumn: 'text', outputTextColumn: 'normalized_text', languageColumn: 'language', mojibakeScoreColumn: 'mojibake_score' } },
      { id: 'classify_document', type: 'classify_document', label: 'カタログ分類', config: { outputColumn: 'document_type', classes: [
        { label: 'bd_recorder_catalog', keywords: ['AQUOS', 'ブルーレイディスクレコーダー', '4Kレコーダー', '総合カタログ'], priority: 10 },
      ] } },
      { id: 'extract_fields', type: 'extract_fields', label: 'カタログ項目抽出', config: { fields: [
        { name: 'catalog_title', type: 'string', required: true, patterns: ['(4Kレコーダー\\s*ブルーレイディスクレコーダー)', '(レコーダー総合カタログ[^\\n]+)'] },
        { name: 'catalog_issue', type: 'string', patterns: ['総合カタログ\\s*([0-9]{4}\\s*-\\s*[0-9]+号)', 'カタログ([0-9]{4}[^\\n]+号)'] },
        { name: 'site_url', type: 'string', patterns: ['(https://jp\\.sharp/bd/)'] },
        { name: 'primary_4k_model', type: 'string', required: true, patterns: ['(4B-C40GT3)', '(4B-C20GT3)'] },
        { name: 'standard_model', type: 'string', patterns: ['(2B-C20GT1)', '(2B-C20GW1)', '(2B-C10GW1)'] },
        { name: 'hdd_capacity', type: 'string', patterns: ['内蔵HDD\\s*([0-9]+\\s*TB)', '(4TB)', '(2\\s*TB)'] },
        { name: 'power_watts', type: 'number', patterns: ['消費電力\\s*約?([0-9]+)W'] },
      ] } },
      { id: 'canonicalize', type: 'canonicalize', label: 'カタログ値正規化', config: { rules: [
        { column: 'catalog_issue', outputColumn: 'catalog_issue_canonical', operations: ['trim', 'normalize_spaces', 'zenkaku_to_hankaku_basic'] },
        { column: 'site_url', outputColumn: 'site_url_canonical', operations: ['trim'] },
        { column: 'primary_4k_model', outputColumn: 'primary_4k_model_canonical', operations: ['trim', 'uppercase', 'zenkaku_to_hankaku_basic'] },
        { column: 'standard_model', outputColumn: 'standard_model_canonical', operations: ['trim', 'uppercase', 'zenkaku_to_hankaku_basic'] },
        { column: 'hdd_capacity', outputColumn: 'hdd_capacity_canonical', operations: ['trim', 'normalize_spaces', 'uppercase', 'zenkaku_to_hankaku_basic'] },
      ] } },
      { id: 'quality_report', type: 'quality_report', label: '品質レポート', config: { columns: ['document_type', 'catalog_title', 'primary_4k_model_canonical', 'standard_model_canonical', 'hdd_capacity_canonical', 'power_watts'], outputMode: 'row_summary' } },
    ], 'SHARP PDFカタログ商品比較 サンプル', 'sample_sharp_bd_catalog_review'),
  }))

  console.log(JSON.stringify({
    baseUrl,
    files: uploaded,
    pipelines: pipelines.map(({ pipeline, version }) => ({
      name: pipeline.name,
      publicId: pipeline.publicId,
      versionPublicId: version.publicId,
      url: `http://localhost:5173/data-pipelines/${pipeline.publicId}`,
    })),
  }, null, 2))
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})
