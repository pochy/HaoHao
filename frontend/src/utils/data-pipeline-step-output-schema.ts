type ConfigRecord = Record<string, unknown>

export function inferDataPipelineStepOutputColumns(
  stepType: string,
  config: ConfigRecord,
  upstreamColumns: string[],
): string[] | null {
  switch (stepType) {
  case 'profile':
  case 'clean':
  case 'normalize':
  case 'validate':
  case 'output':
  case 'quarantine':
    return uniqueStrings(upstreamColumns)
  case 'extract_text':
    return ['file_public_id', 'ocr_run_public_id', 'page_number', 'text', 'confidence', 'layout_json', 'boxes_json']
  case 'json_extract':
    return inferJSONExtractColumns(config, upstreamColumns)
  case 'excel_extract':
    return inferExcelExtractColumns(config, upstreamColumns)
  case 'classify_document':
    return inferClassifyDocumentColumns(config, upstreamColumns)
  case 'extract_fields':
    return inferExtractFieldsColumns(config, upstreamColumns)
  case 'extract_table':
    return inferExtractTableColumns(upstreamColumns)
  case 'product_extraction':
    return inferProductExtractionColumns(config, upstreamColumns)
  case 'quality_report':
    return inferQualityReportColumns(upstreamColumns)
  case 'confidence_gate':
    return inferConfidenceGateColumns(config, upstreamColumns)
  case 'deduplicate':
    return inferDeduplicateColumns(config, upstreamColumns)
  case 'canonicalize':
    return inferCanonicalizeColumns(config, upstreamColumns)
  case 'redact_pii':
    return inferRedactPIIColumns(config, upstreamColumns)
  case 'detect_language_encoding':
    return inferDetectLanguageColumns(config, upstreamColumns)
  case 'schema_inference':
    return inferSchemaInferenceColumns(upstreamColumns)
  case 'entity_resolution':
    return inferEntityResolutionColumns(config, upstreamColumns)
  case 'unit_conversion':
    return inferUnitConversionColumns(config, upstreamColumns)
  case 'relationship_extraction':
    return inferRelationshipExtractionColumns(upstreamColumns)
  case 'schema_mapping':
    return inferSchemaMappingColumns(config, upstreamColumns)
  case 'schema_completion':
    return inferSchemaCompletionColumns(config, upstreamColumns)
  case 'human_review':
    return inferHumanReviewColumns(config, upstreamColumns)
  case 'sample_compare':
    return inferSampleCompareColumns(upstreamColumns)
  default:
    return null
  }
}

function inferClassifyDocumentColumns(config: ConfigRecord, upstreamColumns: string[]) {
  return uniqueStrings([
    ...upstreamColumns,
    stringValue(config.outputColumn).trim() || 'document_type',
    stringValue(config.confidenceColumn).trim() || 'document_type_confidence',
    'document_type_reason',
  ])
}

function inferExtractFieldsColumns(config: ConfigRecord, upstreamColumns: string[]) {
  const fieldColumns = arrayFromConfig(config, 'fields')
    .map((field) => stringField(field, 'name').trim())
    .filter(Boolean)
  return uniqueStrings([
    ...upstreamColumns,
    ...fieldColumns,
    'fields_json',
    'evidence_json',
    'field_confidence',
  ])
}

function inferExtractTableColumns(upstreamColumns: string[]) {
  return uniqueStrings([
    ...upstreamColumns,
    'table_id',
    'row_number',
    'row_json',
    'source_text',
    'table_column_count',
    'table_missing_cell_count',
    'table_confidence',
  ])
}

function inferProductExtractionColumns(config: ConfigRecord, upstreamColumns: string[]) {
  const productColumns = [
    'product_extraction_item_public_id',
    'product_item_type',
    'product_name',
    'product_brand',
    'product_manufacturer',
    'product_model',
    'product_sku',
    'product_jan_code',
    'product_category',
    'product_description',
    'product_price_json',
    'product_promotion_json',
    'product_availability_json',
    'product_source_text',
    'product_evidence_json',
    'product_attributes_json',
    'product_confidence',
    'product_extraction_status',
    'product_extraction_reason',
  ]
  const baseColumns = config.includeSourceColumns === false ? [] : upstreamColumns
  return uniqueStrings([...baseColumns, ...productColumns])
}

function inferQualityReportColumns(upstreamColumns: string[]) {
  return uniqueStrings([
    ...upstreamColumns,
    'quality_report_json',
    'missing_rate_json',
    'validation_summary_json',
  ])
}

function inferConfidenceGateColumns(config: ConfigRecord, upstreamColumns: string[]) {
  return uniqueStrings([
    ...upstreamColumns,
    'gate_score',
    stringValue(config.statusColumn).trim() || 'gate_status',
    'gate_reason',
  ])
}

function inferDeduplicateColumns(config: ConfigRecord, upstreamColumns: string[]) {
  return uniqueStrings([
    ...upstreamColumns,
    stringValue(config.groupColumn).trim() || 'duplicate_group_id',
    stringValue(config.statusColumn).trim() || 'duplicate_status',
    'survivor_flag',
    'match_reason',
  ])
}

function inferCanonicalizeColumns(config: ConfigRecord, upstreamColumns: string[]) {
  const outputColumns = arrayFromConfig(config, 'rules')
    .map((rule) => stringField(rule, 'outputColumn').trim() || stringField(rule, 'column').trim())
    .filter(Boolean)
  return uniqueStrings([...upstreamColumns, ...outputColumns, 'canonicalization_json'])
}

function inferRedactPIIColumns(config: ConfigRecord, upstreamColumns: string[]) {
  const suffix = stringValue(config.outputSuffix).trim() || '_redacted'
  const targetColumns = stringList(config.columns)
  const redactedColumns = targetColumns.map((column) => `${column}${suffix}`)
  return uniqueStrings([
    ...upstreamColumns,
    ...redactedColumns,
    'pii_detected',
    'pii_types_json',
  ])
}

function inferDetectLanguageColumns(config: ConfigRecord, upstreamColumns: string[]) {
  return uniqueStrings([
    ...upstreamColumns,
    stringValue(config.languageColumn).trim() || 'language',
    'encoding',
    stringValue(config.outputTextColumn).trim() || 'normalized_text',
    stringValue(config.mojibakeScoreColumn).trim() || 'mojibake_score',
    'fixes_applied_json',
  ])
}

function inferSchemaInferenceColumns(upstreamColumns: string[]) {
  return uniqueStrings([
    ...upstreamColumns,
    'schema_inference_json',
    'schema_field_count',
    'schema_confidence',
  ])
}

function inferEntityResolutionColumns(config: ConfigRecord, upstreamColumns: string[]) {
  const column = stringValue(config.column).trim() || 'vendor'
  const prefix = stringValue(config.outputPrefix).trim() || column
  return uniqueStrings([
    ...upstreamColumns,
    `${prefix}_entity_id`,
    `${prefix}_match_score`,
    `${prefix}_match_method`,
    `${prefix}_candidates_json`,
  ])
}

function inferUnitConversionColumns(config: ConfigRecord, upstreamColumns: string[]) {
  const outputColumns = arrayFromConfig(config, 'rules').flatMap((rule) => {
    const valueColumn = stringField(rule, 'valueColumn').trim()
    if (!valueColumn) {
      return []
    }
    return [
      stringField(rule, 'outputValueColumn').trim() || `${valueColumn}_normalized`,
      stringField(rule, 'outputUnitColumn').trim() || `${valueColumn}_unit`,
    ]
  })
  return uniqueStrings([...upstreamColumns, ...outputColumns, 'conversion_context_json'])
}

function inferRelationshipExtractionColumns(upstreamColumns: string[]) {
  return uniqueStrings([...upstreamColumns, 'relationships_json', 'relationship_count'])
}

function inferSchemaMappingColumns(config: ConfigRecord, upstreamColumns: string[]) {
  const columns = arrayFromConfig(config, 'mappings')
    .map((mapping) => stringField(mapping, 'targetColumn').trim())
    .filter(Boolean)
  if (columns.length === 0) {
    return upstreamColumns
  }
  const baseColumns = config.includeSourceColumns === true ? upstreamColumns : []
  return uniqueStrings([...baseColumns, ...columns])
}

function inferHumanReviewColumns(config: ConfigRecord, upstreamColumns: string[]) {
  return uniqueStrings([
    ...upstreamColumns,
    stringValue(config.statusColumn).trim() || 'review_status',
    stringValue(config.queueColumn).trim() || 'review_queue',
    'review_reason_json',
  ])
}

function inferSampleCompareColumns(upstreamColumns: string[]) {
  return uniqueStrings([...upstreamColumns, 'diff_json', 'changed_fields', 'changed_field_count'])
}

function inferJSONExtractColumns(config: ConfigRecord, upstreamColumns: string[]) {
  const extractedColumns = arrayFromConfig(config, 'fields')
    .map((field) => stringField(field, 'column').trim())
    .filter(Boolean)
  const baseColumns = config.includeSourceColumns === false ? [] : upstreamColumns
  const metadataColumns = ['json_row_number', 'json_record_path']
  const rawRecordColumns = config.includeRawRecord === true ? ['raw_record_json'] : []
  return uniqueStrings([
    ...baseColumns,
    ...metadataColumns,
    ...extractedColumns,
    ...rawRecordColumns,
  ])
}

function inferExcelExtractColumns(config: ConfigRecord, upstreamColumns: string[]) {
  const baseColumns = config.includeSourceColumns === false ? [] : upstreamColumns
  const metadataColumns = config.includeSourceMetadataColumns === false
    ? []
    : ['file_public_id', 'file_name', 'mime_type', 'file_revision', 'sheet_name', 'sheet_index', 'row_number']
  return uniqueStrings([
    ...baseColumns,
    ...metadataColumns,
    ...stringList(config.columns),
  ])
}

function inferSchemaCompletionColumns(config: ConfigRecord, upstreamColumns: string[]) {
  const columns = arrayFromConfig(config, 'rules')
    .map((rule) => stringField(rule, 'targetColumn').trim())
    .filter(Boolean)
  return uniqueStrings([...upstreamColumns, ...columns])
}

function arrayFromConfig(config: ConfigRecord, key: string): ConfigRecord[] {
  const raw = config[key]
  if (!Array.isArray(raw)) {
    return []
  }
  return raw.map((item) => asRecord(item))
}

function asRecord(value: unknown): ConfigRecord {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return {}
  }
  return { ...(value as ConfigRecord) }
}

function stringField(record: ConfigRecord, key: string) {
  return stringValue(record[key])
}

function stringValue(value: unknown) {
  if (value === null || value === undefined) {
    return ''
  }
  return String(value)
}

function stringList(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.map((item) => stringValue(item).trim()).filter(Boolean)
  }
  const text = stringValue(value).trim()
  return text ? [text] : []
}

function uniqueStrings(values: string[]) {
  return Array.from(new Set(values.filter(Boolean)))
}
