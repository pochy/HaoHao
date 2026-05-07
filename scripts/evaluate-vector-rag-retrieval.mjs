import { readFileSync } from 'node:fs'
import { performance } from 'node:perf_hooks'
import { resolve } from 'node:path'

const schemaMappingPath = resolve(process.env.HAOHAO_SCHEMA_MAPPING_EVAL ?? 'samples/evaluation/schema-mapping-invoice-ja.json')
const schemaColumnsPath = resolve(process.env.HAOHAO_SCHEMA_MAPPING_SEED ?? 'samples/schema-mapping/invoice-columns.json')
const driveRagPath = resolve(process.env.HAOHAO_DRIVE_RAG_EVAL ?? 'samples/evaluation/drive-rag-retrieval-ja.json')
const runtime = (process.env.HAOHAO_EVAL_EMBEDDING_RUNTIME ?? 'fake').trim().toLowerCase()
const model = process.env.HAOHAO_EVAL_EMBEDDING_MODEL ?? (runtime === 'fake' ? 'fake-eval-embedding' : '')
const runtimeURL = (process.env.HAOHAO_EVAL_EMBEDDING_URL ?? defaultRuntimeURL(runtime)).replace(/\/+$/, '')
const scoreThreshold = Number(process.env.HAOHAO_EVAL_SCORE_THRESHOLD ?? '0.05')
const schemaScoreThreshold = Number(process.env.HAOHAO_EVAL_SCHEMA_SCORE_THRESHOLD ?? scoreThreshold)
const driveScoreThreshold = Number(process.env.HAOHAO_EVAL_DRIVE_SCORE_THRESHOLD ?? scoreThreshold)
const schemaKeywordWeight = Number(process.env.HAOHAO_EVAL_SCHEMA_KEYWORD_WEIGHT ?? '0.8')
const topK = Number(process.env.HAOHAO_EVAL_TOP_K ?? '5')
const enforceGates = process.env.HAOHAO_EVAL_ENFORCE_GATES === '1'

function defaultRuntimeURL(value) {
  if (value === 'ollama') {
    return 'http://127.0.0.1:11434'
  }
  if (value === 'lmstudio') {
    return 'http://127.0.0.1:1234'
  }
  return ''
}

function readJSON(path) {
  return JSON.parse(readFileSync(path, 'utf8'))
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

function asText(parts) {
  return parts.flat().filter((item) => item !== undefined && item !== null && String(item).trim() !== '').join('\n')
}

function cosine(a, b) {
  let dot = 0
  let an = 0
  let bn = 0
  for (let i = 0; i < Math.min(a.length, b.length); i += 1) {
    dot += a[i] * b[i]
    an += a[i] * a[i]
    bn += b[i] * b[i]
  }
  if (an === 0 || bn === 0) {
    return 0
  }
  return dot / (Math.sqrt(an) * Math.sqrt(bn))
}

function tokenize(text) {
  const normalized = String(text)
    .toLowerCase()
    .replace(/[^\p{L}\p{N}_-]+/gu, ' ')
  const tokens = normalized
    .split(/\s+/)
    .filter(Boolean)
  for (const run of normalized.match(/[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}0-9]+/gu) ?? []) {
    for (const size of [2, 3, 4]) {
      for (let index = 0; index + size <= run.length; index += 1) {
        tokens.push(run.slice(index, index + size))
      }
    }
  }
  return tokens
}

const fakeConcepts = [
  ['invoice_number', '請求no', '請求番号', 'invoice number', 'invoice no', 'invoice_no', '伝票番号', 'bill'],
  ['invoice_date', '請求日', '発行日', 'invoice date', 'billing_date'],
  ['due_date', '支払期限', '支払期日', '振込期限', '支払予定日', 'payment due', 'due date', 'net 30'],
  ['vendor_name', '仕入先', '支払先', '請求元', '請求元会社', '取引先名', 'vendor', 'supplier', 'supplier name'],
  ['vendor_registration_number', '登録番号', '適格番号', 'インボイス登録', 'registration no', 'tax_registration_number'],
  ['subtotal_amount', '小計', '税抜', 'subtotal', 'net amount'],
  ['tax_amount', '消費税', '税額', 'vat', 'tax amount'],
  ['total_amount', '税込合計', '請求金額', '合計金額', '支払金額', 'お支払金額', 'amount due', 'total amount'],
  ['currency', '通貨', 'currency', 'currency_code', 'jpy', 'usd', '円'],
  ['purchase_order_number', '注文番号', '発注番号', '発注no', '注文書参照', 'po number', 'purchase order', 'po-'],
  ['contract', '保守契約', '契約', '解約通知期限', '自動更新'],
  ['expense', '経費', '交通費', '顧客訪問', '申請者'],
  ['sales_note', '営業週報', '商談数', '新規提案'],
  ['payroll', '給与', '基本給', '控除', '従業員'],
  ['card', 'カード明細', 'クレジットカード', '加盟店']
]

function fakeEmbed(text) {
  const vector = new Array(1024).fill(0)
  const normalized = String(text).toLowerCase()
  for (let index = 0; index < fakeConcepts.length; index += 1) {
    const terms = fakeConcepts[index]
    for (const term of terms) {
      if (normalized.includes(term)) {
        vector[index] += 1
      }
    }
  }
  for (const token of tokenize(text)) {
    let hash = 2166136261
    for (let index = 0; index < token.length; index += 1) {
      hash ^= token.charCodeAt(index)
      hash = Math.imul(hash, 16777619)
    }
    vector[32 + (Math.abs(hash) % 992)] += 0.05
  }
  return vector
}

async function embed(texts) {
  assert(Array.isArray(texts) && texts.length > 0, 'embed requires texts')
  if (runtime === 'fake') {
    return texts.map(fakeEmbed)
  }
  assert(model.trim() !== '', 'HAOHAO_EVAL_EMBEDDING_MODEL is required for real embedding runtime')
  assert(runtimeURL.trim() !== '', 'HAOHAO_EVAL_EMBEDDING_URL is required for real embedding runtime')
  if (runtime === 'ollama') {
    const response = await postJSON(`${runtimeURL}/api/embed`, { model, input: texts })
    assert(Array.isArray(response.embeddings), 'Ollama response.embeddings must be an array')
    return response.embeddings
  }
  if (runtime === 'lmstudio') {
    const response = await postJSON(`${runtimeURL}/v1/embeddings`, { model, input: texts })
    assert(Array.isArray(response.data), 'LM Studio response.data must be an array')
    return response.data.map((item) => item.embedding)
  }
  throw new Error(`unsupported HAOHAO_EVAL_EMBEDDING_RUNTIME: ${runtime}`)
}

async function postJSON(url, payload) {
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!response.ok) {
    throw new Error(`${url} returned ${response.status}: ${await response.text()}`)
  }
  return response.json()
}

function rankByVector(queryVector, candidates) {
  return candidates
    .map((candidate) => ({ ...candidate, score: cosine(queryVector, candidate.embedding) }))
    .sort((a, b) => b.score - a.score)
}

function keywordScore(query, text) {
  const queryTokens = new Set(tokenize(query))
  const textTokens = new Set(tokenize(text))
  let score = 0
  for (const token of queryTokens) {
    if (textTokens.has(token)) {
      score += 1
    }
  }
  return score / Math.max(queryTokens.size, 1)
}

async function evaluateSchemaMapping(dataset, schemaColumns) {
  const schemaByTarget = new Map(schemaColumns.columns.map((column) => [column.targetColumn, column]))
  const targets = dataset.targetColumns.map((targetColumn) => {
    const column = schemaByTarget.get(targetColumn)
    assert(column, `schema seed is missing target column ${targetColumn}`)
    return {
      id: targetColumn,
      text: asText([
        targetColumn,
        column.description,
        column.aliases ?? [],
        column.examples ?? [],
        dataset.domain,
        dataset.schemaType,
      ]),
    }
  })

  const started = performance.now()
  const embeddings = await embed([
    ...targets.map((item) => item.text),
    ...dataset.cases.map((item) => asText([item.sourceColumn, item.sheetName, item.sampleValues, item.neighborColumns])),
  ])
  const latencyMs = performance.now() - started
  const targetEmbeddings = targets.map((item, index) => ({ ...item, embedding: embeddings[index] }))
  const queryOffset = targets.length

  let positiveCases = 0
  let top1Hits = 0
  let top3Hits = 0
  let noCandidateCases = 0
  let noCandidateHits = 0
  let falsePositives = 0
  const failures = []

  for (let index = 0; index < dataset.cases.length; index += 1) {
    const item = dataset.cases[index]
    const queryText = asText([item.sourceColumn, item.sheetName, item.sampleValues, item.neighborColumns])
    const ranked = rankByVector(embeddings[queryOffset + index], targetEmbeddings)
      .map((candidate) => ({ ...candidate, score: candidate.score + (keywordScore(queryText, candidate.text) * schemaKeywordWeight) }))
      .sort((a, b) => b.score - a.score)
    const top = ranked[0]
    const predictedNoCandidate = !top || top.score < schemaScoreThreshold
    if (item.expectedTargetColumn === null) {
      noCandidateCases += 1
      if (predictedNoCandidate) {
        noCandidateHits += 1
      } else {
        falsePositives += 1
        failures.push({ id: item.id, reason: 'false_positive', predicted: top.id, score: top.score })
      }
      continue
    }

    positiveCases += 1
    const acceptable = new Set(item.acceptableTargetColumns)
    if (acceptable.has(ranked[0]?.id)) {
      top1Hits += 1
    } else {
      failures.push({ id: item.id, reason: 'top1_miss', expected: item.expectedTargetColumn, predicted: ranked[0]?.id, score: ranked[0]?.score })
    }
    if (ranked.slice(0, 3).some((candidate) => acceptable.has(candidate.id))) {
      top3Hits += 1
    }
  }

  return {
    cases: dataset.cases.length,
    positiveCases,
    noCandidateCases,
    top1Accuracy: ratio(top1Hits, positiveCases),
    top3Accuracy: ratio(top3Hits, positiveCases),
    noCandidatePrecision: ratio(noCandidateHits, noCandidateCases),
    falsePositiveRate: ratio(falsePositives, dataset.cases.length),
    latencyMs,
    gates: dataset.acceptanceGates,
    gatesPassed: {
      top1Accuracy: ratio(top1Hits, positiveCases) >= dataset.acceptanceGates.top1Accuracy,
      top3Accuracy: ratio(top3Hits, positiveCases) >= dataset.acceptanceGates.top3Accuracy,
      noCandidatePrecision: ratio(noCandidateHits, noCandidateCases) >= dataset.acceptanceGates.noCandidatePrecision,
      falsePositiveRate: ratio(falsePositives, dataset.cases.length) <= dataset.acceptanceGates.maxFalsePositiveRate,
    },
    failures: failures.slice(0, 20),
  }
}

async function evaluateDriveRag(dataset) {
  const documents = dataset.documents.map((item) => ({
    ...item,
    searchableText: asText([item.title, item.tags ?? [], item.text]),
  }))
  const started = performance.now()
  const embeddings = await embed([
    ...documents.map((item) => item.searchableText),
    ...dataset.queries.map((item) => item.query),
  ])
  const latencyMs = performance.now() - started
  const embeddedDocuments = documents.map((item, index) => ({ ...item, embedding: embeddings[index] }))
  const documentByID = new Map(embeddedDocuments.map((item) => [item.id, item]))
  const queryOffset = documents.length

  let semanticQueries = 0
  let semanticHits = 0
  let hybridQueries = 0
  let hybridHits = 0
  let forbiddenChecks = 0
  let forbiddenPasses = 0
  let citationChecks = 0
  let citationPasses = 0
  let answerFactChecks = 0
  let answerFactPasses = 0
  let noCitationAnswerViolations = 0
  const failures = []

  for (let index = 0; index < dataset.queries.length; index += 1) {
    const item = dataset.queries[index]
    const visibleCandidates = embeddedDocuments.filter((doc) => doc.visibility === 'viewable')
    const vectorRanked = rankByVector(embeddings[queryOffset + index], visibleCandidates)
    const ranked = applyDriveModeRanking(item.mode, item.query, vectorRanked)
    const returned = ranked.slice(0, topK)
    const returnedIDs = returned.map((doc) => doc.id)

    const hasExpected = item.expectedDocumentIds.length === 0 || item.expectedDocumentIds.some((id) => returnedIDs.includes(id))
    if (item.mode === 'semantic') {
      semanticQueries += 1
      if (hasExpected) {
        semanticHits += 1
      } else {
        failures.push({ id: item.id, reason: 'semantic_recall_miss', expectedDocumentIds: item.expectedDocumentIds, returnedIDs })
      }
    }
    if (item.mode === 'hybrid') {
      hybridQueries += 1
      if (hasExpected) {
        hybridHits += 1
      } else {
        failures.push({ id: item.id, reason: 'hybrid_recall_miss', expectedDocumentIds: item.expectedDocumentIds, returnedIDs })
      }
    }

    for (const forbiddenID of item.forbiddenDocumentIds) {
      const forbiddenDocument = documentByID.get(forbiddenID)
      if (forbiddenDocument?.visibility === 'viewable') {
        continue
      }
      forbiddenChecks += 1
      if (!returnedIDs.includes(forbiddenID)) {
        forbiddenPasses += 1
      } else {
        failures.push({ id: item.id, reason: 'forbidden_document_returned', forbiddenID })
      }
    }

    if (item.requiredCitations.length > 0) {
      citationChecks += 1
      if (item.requiredCitations.every((id) => returnedIDs.includes(id))) {
        citationPasses += 1
      } else {
        failures.push({ id: item.id, reason: 'citation_not_retrieved', requiredCitations: item.requiredCitations, returnedIDs })
      }
    }
    if (item.expectedAnswerFacts.length > 0) {
      answerFactChecks += 1
      const returnedText = returned.map((doc) => doc.searchableText).join('\n')
      const missingFacts = item.expectedAnswerFacts.filter((fact) => !returnedText.includes(fact))
      if (missingFacts.length === 0) {
        answerFactPasses += 1
      } else {
        failures.push({ id: item.id, reason: 'answer_fact_not_in_context', missingFacts, returnedIDs })
      }
    }
    if (item.expectedDocumentIds.length === 0 && item.expectedNoAnswerReason && item.forbiddenDocumentIds.some((id) => returnedIDs.includes(id))) {
      noCitationAnswerViolations += 1
      failures.push({ id: item.id, reason: 'no_answer_query_returned_documents', returnedIDs })
    }
  }

  return {
    documents: dataset.documents.length,
    queries: dataset.queries.length,
    semanticRecallAt5: ratio(semanticHits, semanticQueries),
    hybridRecallAt5: ratio(hybridHits, hybridQueries),
    forbiddenDocumentExclusion: ratio(forbiddenPasses, forbiddenChecks),
    citationCoverage: ratio(citationPasses, citationChecks),
    answerFactCoverage: ratio(answerFactPasses, answerFactChecks),
    noCitationAnswerRate: ratio(noCitationAnswerViolations, dataset.queries.length),
    latencyMs,
    gates: dataset.acceptanceGates,
    gatesPassed: {
      semanticRecallAt5: ratio(semanticHits, semanticQueries) >= dataset.acceptanceGates.semanticRecallAt5,
      hybridRecallAt5: ratio(hybridHits, hybridQueries) >= dataset.acceptanceGates.hybridRecallAt5,
      forbiddenDocumentExclusion: ratio(forbiddenPasses, forbiddenChecks) >= dataset.acceptanceGates.forbiddenDocumentExclusion,
      citationCoverage: ratio(citationPasses, citationChecks) >= dataset.acceptanceGates.citationCoverage,
      answerFactCoverage: ratio(answerFactPasses, answerFactChecks) >= dataset.acceptanceGates.answerFactCoverage,
      noCitationAnswerRate: ratio(noCitationAnswerViolations, dataset.queries.length) <= dataset.acceptanceGates.maxNoCitationAnswerRate,
      p95LatencyMsLocalModel: latencyMs <= dataset.acceptanceGates.p95LatencyMsLocalModel,
    },
    failures: failures.slice(0, 20),
  }
}

function applyDriveModeRanking(mode, query, vectorRanked) {
  if (mode === 'keyword') {
    return vectorRanked
      .map((candidate) => ({ ...candidate, score: keywordScore(query, candidate.searchableText) }))
      .filter((candidate) => candidate.score > 0)
      .sort((a, b) => b.score - a.score)
  }
  if (mode === 'hybrid') {
    return vectorRanked
      .map((candidate) => ({ ...candidate, score: candidate.score + keywordScore(query, candidate.searchableText) }))
      .filter((candidate) => candidate.score >= driveScoreThreshold)
      .sort((a, b) => b.score - a.score)
  }
  return vectorRanked.filter((candidate) => candidate.score >= driveScoreThreshold)
}

function ratio(numerator, denominator) {
  if (denominator === 0) {
    return 1
  }
  return Number((numerator / denominator).toFixed(4))
}

function allGatesPassed(result) {
  return Object.values(result.gatesPassed).every(Boolean)
}

const schemaMapping = readJSON(schemaMappingPath)
const schemaColumns = readJSON(schemaColumnsPath)
const driveRag = readJSON(driveRagPath)
const schemaMappingResult = await evaluateSchemaMapping(schemaMapping, schemaColumns)
const driveRagResult = await evaluateDriveRag(driveRag)
const output = {
  ok: allGatesPassed(schemaMappingResult) && allGatesPassed(driveRagResult),
  enforced: enforceGates,
  runtime,
  model,
  runtimeURL: runtimeURL || null,
  scoreThreshold,
  schemaScoreThreshold,
  driveScoreThreshold,
  schemaKeywordWeight,
  topK,
  schemaMapping: schemaMappingResult,
  driveRag: driveRagResult,
}

console.log(JSON.stringify(output, null, 2))

if (enforceGates && !output.ok) {
  process.exit(1)
}
