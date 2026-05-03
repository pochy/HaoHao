import { spawnSync } from 'node:child_process'
import { mkdir, writeFile } from 'node:fs/promises'
import { basename, resolve } from 'node:path'

const baseUrl = process.env.HAOHAO_BASE_URL ?? 'http://127.0.0.1:8080'
const email = process.env.HAOHAO_DEMO_EMAIL ?? 'demo@example.com'
const password = process.env.HAOHAO_DEMO_PASSWORD ?? 'changeme123'
const tenantSlug = process.env.HAOHAO_TENANT_SLUG ?? 'acme'
const sampleDir = resolve('samples')
const font = '/System/Library/Fonts/ヒラギノ角ゴシック W4.ttc'

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

async function upload(path) {
  const form = new FormData()
  const file = await openAsBlob(path, { type: 'image/png' })
  form.append('file', file, basename(path))
  return request('/api/v1/drive/files', { method: 'POST', body: form })
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

async function main() {
  await login()

  const invoice = await renderSample('ja-unstructured-invoice-aoba-20260503', `請求書
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

  const contract = await renderSample('ja-unstructured-contract-review-20260503', `業務委託契約 確認メモ
作成日: 2026年05月03日

株式会社桜食品は、株式会社北斗物流に冷蔵配送業務を委託する。
株式会社北斗物流は、毎営業日 09:00 までに配送計画を株式会社桜食品へ提出する。
契約期間: 2026年06月01日 から 2027年05月31日

担当者連絡先:
桜食品 購買部 山田 花子 hanako.yamada@sakura-foods.example
北斗物流 営業部 鈴木 一郎 03-1234-5678

注意: 個人情報を含むため、外部共有前にマスキングを行う。`)

  const expenseA = await renderSample('ja-unstructured-expense-tanaka-1-20260503', `経費精算申請 控え
申請日: 2026年05月03日
社員名: 田中 太郎
利用日: 2026年04月28日
支払先: JR東日本
用途: 顧客訪問 交通費
区間: 東京駅 から 横浜駅
金額: 960円
承認者: 営業部 課長`)

  const expenseB = await renderSample('ja-unstructured-expense-tanaka-duplicate-20260503', `経費精算申請 控え
申請日: 2026年05月03日
社員名: 田中　太郎
利用日: 2026/04/28
支払先: ＪＲ東日本
用途: 顧客訪問 交通費
区間: 東京駅 から 横浜駅
金額: 960 円
承認者: 営業部 課長`)

  const expenseC = await renderSample('ja-unstructured-expense-suzuki-20260503', `経費精算申請 控え
申請日: 2026年05月03日
社員名: 鈴木 一郎
利用日: 2026年04月30日
支払先: 日本交通
用途: 顧客訪問 タクシー代
区間: 品川 から 大手町
金額: 3,420円
承認者: 営業部 課長`)

  const uploaded = {
    invoice: await upload(invoice.imagePath),
    contract: await upload(contract.imagePath),
    expenses: [
      await upload(expenseA.imagePath),
      await upload(expenseB.imagePath),
      await upload(expenseC.imagePath),
    ],
  }

  const pipelines = []

  pipelines.push(await createPipeline({
    name: 'サンプル: 日本語請求書 OCR から支払データ標準化',
    description: '日本語の請求書スキャンから OCR、言語検出、文書分類、項目抽出、取引先名の正規化、円金額の単位標準化、スキーマ推定、品質レポートまで行うサンプルです。',
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

  pipelines.push(await createPipeline({
    name: 'サンプル: 日本語契約メモの PII マスキングと関係抽出',
    description: '契約確認メモから OCR したテキストに対して、PII 検出・マスキング、委託関係の抽出、レビュー対象判定、品質レポートを行うサンプルです。',
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

  pipelines.push(await createPipeline({
    name: 'サンプル: 日本語経費精算 OCR の重複検出と支払先名寄せ',
    description: '経費精算控えのスキャン複数枚から項目を抽出し、表記ゆれ正規化、重複申請の検出、支払先の名寄せ、正規化前後の比較、品質レポートを行うサンプルです。',
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
        { column: 'vendor', outputColumn: 'vendor_canonical', operations: ['trim', 'normalize_spaces', 'zenkaku_to_hankaku_basic'], mappings: { 'ＪＲ東日本': 'JR東日本', 'JR東日本': 'JR東日本', '日本交通': '日本交通' } },
      ] } },
      { id: 'deduplicate', type: 'deduplicate', label: '重複申請検出', config: { keyColumns: ['employee_name_canonical', 'expense_date', 'amount_jpy'], mode: 'annotate', statusColumn: 'duplicate_status', groupColumn: 'duplicate_group_id' } },
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
