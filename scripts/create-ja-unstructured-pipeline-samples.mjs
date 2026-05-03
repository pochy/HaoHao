import { spawnSync } from 'node:child_process'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
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
  }
  uploaded.pdfOrder = existingPDFOrderFilePublicId
    ? { publicId: existingPDFOrderFilePublicId }
    : await upload(pdfOrder.pdfPath)
  uploaded.sharpCatalog = existingSharpCatalogFilePublicId
    ? { publicId: existingSharpCatalogFilePublicId }
    : await upload(resolve(sampleDir, 'SHARP-bd-recorder-2025-06.pdf'))

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

  if (shouldCreatePipeline('sharp_catalog')) pipelines.push(await createPipeline({
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
