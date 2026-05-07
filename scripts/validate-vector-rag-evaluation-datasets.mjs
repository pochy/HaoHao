import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const schemaMappingPath = resolve('samples/evaluation/schema-mapping-invoice-ja.json')
const driveRagPath = resolve('samples/evaluation/drive-rag-retrieval-ja.json')

function readJSON(path) {
  try {
    return JSON.parse(readFileSync(path, 'utf8'))
  } catch (error) {
    throw new Error(`${path}: ${error.message}`)
  }
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

function assertString(value, path) {
  assert(typeof value === 'string' && value.trim().length > 0, `${path} must be a non-empty string`)
}

function assertArray(value, path) {
  assert(Array.isArray(value), `${path} must be an array`)
}

function assertNumber(value, path) {
  assert(typeof value === 'number' && Number.isFinite(value), `${path} must be a finite number`)
}

function findDuplicates(values) {
  const seen = new Set()
  const duplicates = new Set()
  for (const value of values) {
    if (seen.has(value)) {
      duplicates.add(value)
    }
    seen.add(value)
  }
  return [...duplicates]
}

function validateGateRange(gates, fields, prefix) {
  for (const field of fields) {
    assertNumber(gates[field], `${prefix}.acceptanceGates.${field}`)
    assert(gates[field] >= 0 && gates[field] <= 1, `${prefix}.acceptanceGates.${field} must be between 0 and 1`)
  }
}

function validateSchemaMapping(dataset) {
  assertString(dataset.dataset, 'schemaMapping.dataset')
  assertString(dataset.domain, 'schemaMapping.domain')
  assertString(dataset.schemaType, 'schemaMapping.schemaType')
  assertString(dataset.language, 'schemaMapping.language')
  assert(dataset.acceptanceGates && typeof dataset.acceptanceGates === 'object', 'schemaMapping.acceptanceGates is required')
  assertArray(dataset.targetColumns, 'schemaMapping.targetColumns')
  assertArray(dataset.cases, 'schemaMapping.cases')

  const targetColumns = new Set(dataset.targetColumns)
  assert(targetColumns.size === dataset.targetColumns.length, 'schemaMapping.targetColumns must not contain duplicates')
  validateGateRange(dataset.acceptanceGates, ['top1Accuracy', 'top3Accuracy', 'noCandidatePrecision', 'maxFalsePositiveRate'], 'schemaMapping')
  assertNumber(dataset.acceptanceGates.minimumCasesBeforeBroadRollout, 'schemaMapping.acceptanceGates.minimumCasesBeforeBroadRollout')
  assertNumber(dataset.acceptanceGates.starterCases, 'schemaMapping.acceptanceGates.starterCases')
  assert(dataset.cases.length >= dataset.acceptanceGates.starterCases, 'schemaMapping.cases must satisfy starterCases')

  const duplicateIDs = findDuplicates(dataset.cases.map((item) => item.id))
  assert(duplicateIDs.length === 0, `schemaMapping.cases duplicate ids: ${duplicateIDs.join(', ')}`)

  let noCandidateCases = 0
  for (const [index, item] of dataset.cases.entries()) {
    const prefix = `schemaMapping.cases[${index}]`
    assertString(item.id, `${prefix}.id`)
    assertString(item.sourceColumn, `${prefix}.sourceColumn`)
    assertArray(item.sampleValues, `${prefix}.sampleValues`)
    assertArray(item.neighborColumns, `${prefix}.neighborColumns`)
    assertArray(item.acceptableTargetColumns, `${prefix}.acceptableTargetColumns`)

    if (item.expectedTargetColumn === null) {
      noCandidateCases += 1
      assert(item.acceptableTargetColumns.length === 0, `${prefix}.acceptableTargetColumns must be empty when expectedTargetColumn is null`)
      continue
    }

    assertString(item.expectedTargetColumn, `${prefix}.expectedTargetColumn`)
    assert(targetColumns.has(item.expectedTargetColumn), `${prefix}.expectedTargetColumn is not in targetColumns`)
    assert(item.acceptableTargetColumns.includes(item.expectedTargetColumn), `${prefix}.acceptableTargetColumns must include expectedTargetColumn`)
    for (const target of item.acceptableTargetColumns) {
      assert(targetColumns.has(target), `${prefix}.acceptableTargetColumns contains unknown target ${target}`)
    }
  }
  assert(noCandidateCases > 0, 'schemaMapping.cases must include at least one no-candidate case')

  return {
    cases: dataset.cases.length,
    noCandidateCases,
    targetColumns: dataset.targetColumns.length,
  }
}

function validateDriveRag(dataset) {
  assertString(dataset.dataset, 'driveRag.dataset')
  assertString(dataset.language, 'driveRag.language')
  assert(dataset.acceptanceGates && typeof dataset.acceptanceGates === 'object', 'driveRag.acceptanceGates is required')
  assertArray(dataset.documents, 'driveRag.documents')
  assertArray(dataset.queries, 'driveRag.queries')

  validateGateRange(dataset.acceptanceGates, ['semanticRecallAt5', 'hybridRecallAt5', 'forbiddenDocumentExclusion', 'citationCoverage', 'answerFactCoverage', 'maxNoCitationAnswerRate'], 'driveRag')
  assertNumber(dataset.acceptanceGates.minimumQueriesBeforeBroadRollout, 'driveRag.acceptanceGates.minimumQueriesBeforeBroadRollout')
  assertNumber(dataset.acceptanceGates.starterQueries, 'driveRag.acceptanceGates.starterQueries')
  assertNumber(dataset.acceptanceGates.p95LatencyMsLocalModel, 'driveRag.acceptanceGates.p95LatencyMsLocalModel')
  assert(dataset.queries.length >= dataset.acceptanceGates.starterQueries, 'driveRag.queries must satisfy starterQueries')

  const duplicateDocumentIDs = findDuplicates(dataset.documents.map((item) => item.id))
  assert(duplicateDocumentIDs.length === 0, `driveRag.documents duplicate ids: ${duplicateDocumentIDs.join(', ')}`)
  const documentIDs = new Set(dataset.documents.map((item) => item.id))
  for (const [index, item] of dataset.documents.entries()) {
    const prefix = `driveRag.documents[${index}]`
    assertString(item.id, `${prefix}.id`)
    assertString(item.resourceKind, `${prefix}.resourceKind`)
    assertString(item.title, `${prefix}.title`)
    assert(['viewable', 'forbidden', 'dlp_blocked'].includes(item.visibility), `${prefix}.visibility is invalid`)
    assertArray(item.tags, `${prefix}.tags`)
    assertString(item.text, `${prefix}.text`)
  }

  const duplicateQueryIDs = findDuplicates(dataset.queries.map((item) => item.id))
  assert(duplicateQueryIDs.length === 0, `driveRag.queries duplicate ids: ${duplicateQueryIDs.join(', ')}`)
  let deniedQueries = 0
  for (const [index, item] of dataset.queries.entries()) {
    const prefix = `driveRag.queries[${index}]`
    assertString(item.id, `${prefix}.id`)
    assertString(item.query, `${prefix}.query`)
    assert(['keyword', 'semantic', 'hybrid'].includes(item.mode), `${prefix}.mode is invalid`)
    assertArray(item.expectedDocumentIds, `${prefix}.expectedDocumentIds`)
    assertArray(item.forbiddenDocumentIds, `${prefix}.forbiddenDocumentIds`)
    assertArray(item.expectedAnswerFacts, `${prefix}.expectedAnswerFacts`)
    assertArray(item.requiredCitations, `${prefix}.requiredCitations`)

    for (const field of ['expectedDocumentIds', 'forbiddenDocumentIds', 'requiredCitations']) {
      for (const id of item[field]) {
        assert(documentIDs.has(id), `${prefix}.${field} contains unknown document id ${id}`)
      }
    }
    for (const [factIndex, fact] of item.expectedAnswerFacts.entries()) {
      assertString(fact, `${prefix}.expectedAnswerFacts[${factIndex}]`)
    }
    for (const id of item.requiredCitations) {
      assert(item.expectedDocumentIds.includes(id), `${prefix}.requiredCitations must be expected documents`)
    }
    if (item.expectedDocumentIds.length === 0) {
      deniedQueries += 1
      assertString(item.expectedNoAnswerReason, `${prefix}.expectedNoAnswerReason`)
      assert(item.requiredCitations.length === 0, `${prefix}.requiredCitations must be empty when no answer is expected`)
    }
  }
  assert(deniedQueries > 0, 'driveRag.queries must include at least one denied/no-answer query')

  return {
    documents: dataset.documents.length,
    queries: dataset.queries.length,
    deniedQueries,
  }
}

const schemaMappingSummary = validateSchemaMapping(readJSON(schemaMappingPath))
const driveRagSummary = validateDriveRag(readJSON(driveRagPath))

console.log(JSON.stringify({
  ok: true,
  schemaMapping: schemaMappingSummary,
  driveRag: driveRagSummary,
}, null, 2))
