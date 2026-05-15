<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { AlertCircle, Check, MousePointerClick, Plus, RefreshCw, Sparkles, Trash2, Zap } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { fetchDriveFile, fetchDriveFileManifest, refreshDriveFileManifest, type DriveDocumentManifestBody, type DriveDocumentSheetManifest } from '../api/drive'
import { fetchSchemaMappingCandidates, isDataPipelineAutoPreviewEnabled, recordSchemaMappingExample, type DataPipelineGraph, type DataPipelineGraphValidationBody, type DataPipelineNodeWarningBody, type DataPipelinePreviewBody, type DataPipelineStepType, type SchemaMappingCandidateBody, type SchemaMappingCandidateItem } from '../api/data-pipelines'
import { toApiErrorMessage } from '../api/client'
import type { DatasetBody, DatasetWorkTableBody, DriveFileBody } from '../api/generated/types.gen'
import { inferDataPipelineStepOutputColumns } from '../utils/data-pipeline-step-output-schema'
import DriveFilePickerDialog from './DriveFilePickerDialog.vue'

type ConfigRecord = Record<string, unknown>
type SelectedDriveFile = {
  publicId: string
  file: DriveFileBody | null
}

const props = defineProps<{
  graph: DataPipelineGraph
  selectedNodeId: string
  datasets?: DatasetBody[]
  workTables?: DatasetWorkTableBody[]
  preview?: DataPipelinePreviewBody | null
  validation?: DataPipelineGraphValidationBody | null
  pipelinePublicId?: string
  versionPublicId?: string
}>()

const emit = defineEmits<{
  'update:graph': [graph: DataPipelineGraph]
}>()

const sourceKinds = [
  { value: 'dataset', labelKey: 'dataPipelines.dataset' },
  { value: 'work_table', labelKey: 'dataPipelines.workTable' },
  { value: 'drive_file', labelKey: 'dataPipelines.driveFile' },
]

const cleanOperations = [
  { value: 'drop_null_rows', labelKey: 'dataPipelines.cleanOperation.drop_null_rows' },
  { value: 'fill_null', labelKey: 'dataPipelines.cleanOperation.fill_null' },
  { value: 'null_if', labelKey: 'dataPipelines.cleanOperation.null_if' },
  { value: 'clamp', labelKey: 'dataPipelines.cleanOperation.clamp' },
  { value: 'trim_control_chars', labelKey: 'dataPipelines.cleanOperation.trim_control_chars' },
  { value: 'dedupe', labelKey: 'dataPipelines.cleanOperation.dedupe' },
]

const normalizeOperations = [
  { value: 'trim', labelKey: 'dataPipelines.normalizeOperation.trim' },
  { value: 'lowercase', labelKey: 'dataPipelines.normalizeOperation.lowercase' },
  { value: 'uppercase', labelKey: 'dataPipelines.normalizeOperation.uppercase' },
  { value: 'normalize_spaces', labelKey: 'dataPipelines.normalizeOperation.normalize_spaces' },
  { value: 'remove_symbols', labelKey: 'dataPipelines.normalizeOperation.remove_symbols' },
  { value: 'cast_decimal', labelKey: 'dataPipelines.normalizeOperation.cast_decimal' },
  { value: 'round', labelKey: 'dataPipelines.normalizeOperation.round' },
  { value: 'scale', labelKey: 'dataPipelines.normalizeOperation.scale' },
  { value: 'parse_date', labelKey: 'dataPipelines.normalizeOperation.parse_date' },
  { value: 'to_date', labelKey: 'dataPipelines.normalizeOperation.to_date' },
  { value: 'map_values', labelKey: 'dataPipelines.normalizeOperation.map_values' },
]

const conditionOperators = ['required', '=', '!=', '>', '>=', '<', '<=', 'in', 'regex']
const castOptions = ['', 'string', 'int64', 'float64', 'decimal', 'date', 'datetime']
const completionMethods = ['literal', 'copy_column', 'coalesce', 'concat', 'case_when']
const transformOperations = ['select_columns', 'drop_columns', 'rename_columns', 'filter', 'sort', 'aggregate']
const joinTypes = ['inner', 'left', 'right', 'full', 'cross']
const joinStrictnesses = ['all', 'any']
const aggregateFunctions = ['count', 'sum', 'avg', 'min', 'max']
const fieldTypes = ['string', 'number', 'date', 'boolean', 'json']
const canonicalizeOperations = ['trim', 'lowercase', 'uppercase', 'normalize_spaces', 'remove_symbols', 'zenkaku_to_hankaku_basic']
const piiTypes = ['email', 'phone', 'postal_code', 'api_key_like', 'credit_card_like']
const driveInputModes = [
  { value: '', labelKey: 'dataPipelines.driveInputMode.files' },
  { value: 'spreadsheet', labelKey: 'dataPipelines.driveInputMode.spreadsheet' },
  { value: 'json', labelKey: 'dataPipelines.driveInputMode.json' },
]

const label = ref('')
const configText = ref('{}')
const configDraft = ref<ConfigRecord>({})
const parseError = ref('')
const uiError = ref('')
const activeConfigTab = ref<'ui' | 'json'>('ui')
const drivePickerOpen = ref(false)
const selectedDriveFiles = ref<SelectedDriveFile[]>([])
const selectedDriveFilesLoading = ref(false)
const driveManifest = ref<DriveDocumentManifestBody | null>(null)
const driveManifestLoading = ref(false)
const driveManifestError = ref('')
const schemaMappingCandidateLoading = ref(false)
const schemaMappingCandidateError = ref('')
const schemaMappingCandidateItems = ref<SchemaMappingCandidateItem[]>([])
let syncingLocalChange = false
const { t } = useI18n()

const selectedNode = computed(() => props.graph.nodes.find((node) => node.id === props.selectedNodeId) ?? null)
const selectedIncomingNodeIds = computed(() => props.graph.edges
  .filter((edge) => edge.target === props.selectedNodeId)
  .map((edge) => edge.source))
const stepType = computed(() => selectedNode.value?.data.stepType ?? '')
const autoPreviewEnabled = computed(() => isDataPipelineAutoPreviewEnabled(selectedNode.value?.data))
const previewModeTitle = computed(() => autoPreviewEnabled.value ? t('dataPipelines.autoPreview') : t('dataPipelines.manualPreviewReason'))
const previewModeIcon = computed(() => autoPreviewEnabled.value ? Zap : MousePointerClick)
const datasetOptions = computed(() => props.datasets ?? [])
const workTableOptions = computed(() => (props.workTables ?? []).filter((item) => Boolean(item.publicId)))
const driveFileIds = computed(() => stringList(configDraft.value.filePublicIds))
const primaryDriveFileId = computed(() => driveFileIds.value[0] ?? '')
const driveManifestSheets = computed(() => driveManifest.value?.documentType === 'excel' ? (driveManifest.value.manifest.sheets ?? []) : [])
const selectedManifestSheet = computed(() => {
  const sheetName = stringConfig('sheetName')
  const sheetIndex = numberConfig('sheetIndex')
  return driveManifestSheets.value.find((sheet) => sheet.name === sheetName)
    ?? driveManifestSheets.value.find((sheet) => sheet.index === sheetIndex)
    ?? null
})
const primaryColumns = computed(() => {
  if (stepType.value === 'input') {
    return sourceColumnsFromConfig(configDraft.value, 'sourceKind', 'datasetPublicId', 'workTablePublicId')
  }
  const inferred = columnsForNodeOutput(selectedIncomingNodeIds.value[0])
  if (inferred.length > 0) {
    return inferred
  }
  return sourceColumnsFromConfig(inputConfigForColumns(), 'sourceKind', 'datasetPublicId', 'workTablePublicId')
})
const rightColumns = computed(() => {
  if (stepType.value === 'join') {
    return columnsForNodeOutput(selectedIncomingNodeIds.value[1])
  }
  return sourceColumnsFromConfig(configDraft.value, 'rightSourceKind', 'rightDatasetPublicId', 'rightWorkTablePublicId')
})
const knownColumns = computed(() => uniqueStrings([...primaryColumns.value, ...rightColumns.value]))
const missingColumnWarnings = computed(() => selectedMissingColumnWarnings())
const crossJoinSelected = computed(() => stringConfig('joinType') === 'cross')
const effectiveJoinSelectColumns = computed(() => {
  const configured = stringList(configDraft.value.selectColumns)
  return configured.length > 0 ? configured : rightColumns.value
})
const outputTableNameError = computed(() => {
  if (stepType.value !== 'output') {
    return ''
  }
  const tableName = stringConfig('tableName').trim()
  if (!tableName) {
    return ''
  }
  if (!/^[A-Za-z_][A-Za-z0-9_]{0,127}$/.test(tableName)) {
    return t('dataPipelines.invalidOutputTableName')
  }
  const duplicate = props.graph.nodes.find((node) => node.id !== props.selectedNodeId
    && node.data.stepType === 'output'
    && String(node.data.config?.tableName ?? '').trim().toLowerCase() === tableName.toLowerCase())
  return duplicate ? t('dataPipelines.duplicateOutputTableName') : ''
})
const schemaMappingCandidateMap = computed(() => {
  const out = new Map<string, SchemaMappingCandidateBody[]>()
  for (const item of schemaMappingCandidateItems.value) {
    out.set(item.sourceColumn, item.candidates ?? [])
  }
  return out
})
const schemaMappingCandidateReady = computed(() => schemaMappingRequestColumns().length > 0)

watch(selectedNode, (node) => {
  if (syncingLocalChange) {
    return
  }
  label.value = node?.data.label ?? ''
  configDraft.value = cloneConfig(node?.data.config ?? {})
  configText.value = JSON.stringify(configDraft.value, null, 2)
  parseError.value = ''
  uiError.value = ''
  schemaMappingCandidateItems.value = []
  schemaMappingCandidateError.value = ''
}, { immediate: true })

watch(() => driveFileIds.value.join('\n'), () => {
  if (sourceKind() === 'drive_file') {
    void hydrateSelectedDriveFiles(driveFileIds.value)
  } else {
    selectedDriveFiles.value = []
  }
}, { immediate: true })

watch(() => `${primaryDriveFileId.value}:${driveInputMode()}`, () => {
  if (sourceKind() === 'drive_file' && driveInputMode() === 'spreadsheet' && primaryDriveFileId.value) {
    void loadDriveManifest(false)
  } else {
    driveManifest.value = null
    driveManifestError.value = ''
  }
}, { immediate: true })

function onJsonInput(event: Event) {
  const text = targetValue(event)
  configText.value = text
  try {
    const parsed = parseConfigJSON(text)
    configDraft.value = parsed
    parseError.value = ''
    emitDraftGraph(parsed)
  } catch (error) {
    parseError.value = error instanceof Error ? error.message : t('dataPipelines.invalidJson')
  }
}

function applyChanges() {
  const node = selectedNode.value
  if (!node) {
    return
  }
  let config: ConfigRecord
  try {
    config = parseConfigJSON(configText.value || '{}')
  } catch (error) {
    parseError.value = error instanceof Error ? error.message : t('dataPipelines.invalidJson')
    return
  }
  configDraft.value = config
  parseError.value = ''
  emitDraftGraph(config)
}

function emitDraftGraph(config: ConfigRecord = configDraft.value) {
  const node = selectedNode.value
  if (!node) {
    return
  }
  syncingLocalChange = true
  emit('update:graph', {
    ...props.graph,
    nodes: props.graph.nodes.map((item) => item.id === node.id ? {
      ...item,
      data: {
        ...item.data,
        label: label.value.trim() || labelForStep(item.data.stepType),
        config,
      },
    } : item),
  })
  queueMicrotask(() => {
    syncingLocalChange = false
  })
}

function parseConfigJSON(text: string): ConfigRecord {
  const parsed = JSON.parse(text || '{}')
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(t('dataPipelines.configMustBeObject'))
  }
  return parsed as ConfigRecord
}

function syncConfigText() {
  configText.value = JSON.stringify(configDraft.value, null, 2)
  parseError.value = ''
  uiError.value = ''
  emitDraftGraph()
}

function updateLabel(value: string) {
  label.value = value
  emitDraftGraph()
}

function setConfig(patch: ConfigRecord) {
  configDraft.value = {
    ...configDraft.value,
    ...patch,
  }
  syncConfigText()
}

function updateConfigField(key: string, value: unknown) {
  setConfig({ [key]: value })
}

function updateConfigOptionalString(key: string, value: string) {
  const next = { ...configDraft.value }
  const trimmed = value.trim()
  if (trimmed) {
    next[key] = trimmed
  } else {
    delete next[key]
  }
  configDraft.value = next
  syncConfigText()
}

function updateConfigList(key: string, value: string) {
  setConfig({ [key]: parseListInput(value) })
}

async function hydrateSelectedDriveFiles(ids: string[]) {
  const uniqueIds = uniqueStrings(ids)
  if (uniqueIds.length === 0) {
    selectedDriveFiles.value = []
    return
  }
  selectedDriveFilesLoading.value = true
  const existing = new Map(selectedDriveFiles.value.map((item) => [item.publicId, item]))
  try {
    selectedDriveFiles.value = await Promise.all(uniqueIds.map(async (publicId) => {
      const cached = existing.get(publicId)
      if (cached?.file) {
        return cached
      }
      try {
        return { publicId, file: await fetchDriveFile(publicId) }
      } catch {
        return { publicId, file: null }
      }
    }))
  } finally {
    selectedDriveFilesLoading.value = false
  }
}

function applyDriveFileSelection(filePublicIds: string[]) {
  drivePickerOpen.value = false
  setConfig({ filePublicIds: uniqueStrings(filePublicIds) })
}

function removeDriveFile(publicId: string) {
  setConfig({ filePublicIds: driveFileIds.value.filter((item) => item !== publicId) })
}

function selectedDriveFileName(item: SelectedDriveFile) {
  return item.file?.originalFilename ?? t('dataPipelines.unknownDriveFile')
}

function selectedDriveFileSubtitle(item: SelectedDriveFile) {
  return item.file ? [item.file.contentType, item.publicId].filter(Boolean).join(' · ') : item.publicId
}

async function loadDriveManifest(refresh: boolean) {
  const publicId = primaryDriveFileId.value
  if (!publicId) {
    driveManifest.value = null
    driveManifestError.value = ''
    return
  }
  driveManifestLoading.value = true
  driveManifestError.value = ''
  try {
    driveManifest.value = refresh
      ? await refreshDriveFileManifest(publicId)
      : await fetchDriveFileManifest(publicId)
  } catch (error) {
    driveManifest.value = null
    driveManifestError.value = error instanceof Error ? error.message : t('dataPipelines.documentManifestLoadFailed')
  } finally {
    driveManifestLoading.value = false
  }
}

function selectManifestSheet(value: string) {
  const index = Number(value)
  const sheet = driveManifestSheets.value.find((item) => item.index === index)
  if (!sheet) {
    return
  }
  configDraft.value = {
    ...configDraft.value,
    sheetName: sheet.name,
    sheetIndex: sheet.index,
  }
  syncConfigText()
}

function sheetOptionLabel(sheet: DriveDocumentSheetManifest) {
  return [
    `${sheet.index + 1}. ${sheet.name}`,
    sheet.usedRange,
    sheet.rowCountHint ? t('dataPipelines.rowCountHint', { count: sheet.rowCountHint }) : '',
  ].filter(Boolean).join(' · ')
}

function selectedSheetValue() {
  const sheet = selectedManifestSheet.value
  if (sheet) {
    return String(sheet.index)
  }
  const sheetIndex = numberConfig('sheetIndex')
  return sheetIndex >= 0 ? String(sheetIndex) : ''
}

function updateConfigOptionalNumber(key: string, value: string) {
  const next = { ...configDraft.value }
  const trimmed = value.trim()
  if (trimmed) {
    next[key] = Number(trimmed)
  } else {
    delete next[key]
  }
  configDraft.value = next
  syncConfigText()
}

function driveInputMode() {
  const mode = stringConfig('inputMode') || stringConfig('format')
  if (['spreadsheet', 'excel', 'xls', 'xlsx'].includes(mode)) {
    return 'spreadsheet'
  }
  if (['json', 'application/json'].includes(mode)) {
    return 'json'
  }
  return ''
}

function setDriveInputMode(mode: string) {
  const next = { ...configDraft.value }
  if (mode === 'spreadsheet') {
    next.inputMode = 'spreadsheet'
    if (next.headerRow === undefined) {
      next.headerRow = 1
    }
    if (!Array.isArray(next.columns)) {
      next.columns = []
    }
    delete next.recordPath
    delete next.fields
    delete next.includeRawRecord
  } else if (mode === 'json') {
    next.inputMode = 'json'
    if (next.recordPath === undefined) {
      next.recordPath = '$'
    }
    if (next.maxRows === undefined) {
      next.maxRows = 100000
    }
    if (next.includeSourceMetadataColumns === undefined) {
      next.includeSourceMetadataColumns = true
    }
    if (!Array.isArray(next.fields)) {
      next.fields = []
    }
    delete next.format
    delete next.sheetName
    delete next.sheetIndex
    delete next.range
    delete next.headerRow
    delete next.columns
  } else {
    delete next.inputMode
    delete next.format
    delete next.sheetName
    delete next.sheetIndex
    delete next.range
    delete next.headerRow
    delete next.columns
    delete next.maxRows
    delete next.includeSourceMetadataColumns
    delete next.recordPath
    delete next.fields
    delete next.includeRawRecord
  }
  configDraft.value = next
  syncConfigText()
}

function joinSelectColumnChecked(column: string) {
  return effectiveJoinSelectColumns.value.includes(column)
}

function toggleJoinSelectColumn(column: string, checked: boolean) {
  const current = stringList(configDraft.value.selectColumns)
  const base = current.length > 0 ? current : rightColumns.value
  const next = checked
    ? uniqueStrings([...base, column])
    : base.filter((item) => item !== column)
  setConfig({ selectColumns: next })
}

function sourceKind(right = false) {
  return stringConfig(right ? 'rightSourceKind' : 'sourceKind') || 'dataset'
}

function sourceKindOptions(right = false) {
  return right ? sourceKinds.filter((kind) => kind.value !== 'drive_file') : sourceKinds
}

function sourcePublicId(right = false) {
  const kind = sourceKind(right)
  if (right) {
    return kind === 'work_table' ? stringConfig('rightWorkTablePublicId') : stringConfig('rightDatasetPublicId')
  }
  return kind === 'work_table' ? stringConfig('workTablePublicId') : stringConfig('datasetPublicId')
}

function setSourceKind(kind: string, right = false) {
  if (right) {
    setConfig({
      rightSourceKind: kind,
      rightDatasetPublicId: kind === 'dataset' ? stringConfig('rightDatasetPublicId') : '',
      rightWorkTablePublicId: kind === 'work_table' ? stringConfig('rightWorkTablePublicId') : '',
    })
    return
  }
  const next: ConfigRecord = {
    sourceKind: kind,
    datasetPublicId: kind === 'dataset' ? stringConfig('datasetPublicId') : '',
    workTablePublicId: kind === 'work_table' ? stringConfig('workTablePublicId') : '',
    filePublicIds: kind === 'drive_file' ? stringList(configDraft.value.filePublicIds) : [],
  }
  if (kind === 'drive_file') {
    next.inputMode = stringConfig('inputMode')
  }
  configDraft.value = {
    ...configDraft.value,
    ...next,
  }
  if (kind !== 'drive_file') {
    delete configDraft.value.inputMode
    delete configDraft.value.format
    delete configDraft.value.sheetName
    delete configDraft.value.sheetIndex
    delete configDraft.value.range
    delete configDraft.value.headerRow
    delete configDraft.value.columns
    delete configDraft.value.maxRows
    delete configDraft.value.includeSourceMetadataColumns
    delete configDraft.value.recordPath
    delete configDraft.value.fields
    delete configDraft.value.includeRawRecord
  }
  syncConfigText()
}

function setSourcePublicId(publicId: string, right = false) {
  const kind = sourceKind(right)
  if (right) {
    setConfig(kind === 'work_table'
      ? { rightWorkTablePublicId: publicId, rightDatasetPublicId: '' }
      : { rightDatasetPublicId: publicId, rightWorkTablePublicId: '' })
    return
  }
  setConfig(kind === 'work_table'
    ? { workTablePublicId: publicId, datasetPublicId: '' }
    : { datasetPublicId: publicId, workTablePublicId: '' })
}

function stringConfig(key: string) {
  return stringValue(configDraft.value[key])
}

function numberConfig(key: string) {
  const raw = configDraft.value[key]
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    return raw
  }
  const parsed = Number(stringValue(raw))
  return Number.isFinite(parsed) ? parsed : -1
}

function stringField(record: ConfigRecord, key: string) {
  return stringValue(record[key])
}

function boolField(record: ConfigRecord, key: string) {
  return Boolean(record[key])
}

function recordField(record: ConfigRecord, key: string) {
  return asRecord(record[key])
}

function arrayConfig(key: string): ConfigRecord[] {
  const raw = configDraft.value[key]
  if (!Array.isArray(raw)) {
    return []
  }
  return raw.map((item) => asRecord(item))
}

function setArray(key: string, items: ConfigRecord[]) {
  setConfig({ [key]: items })
}

function addArrayItem(key: string, item: ConfigRecord) {
  setArray(key, [...arrayConfig(key), item])
}

function replaceArrayItem(key: string, index: number, item: ConfigRecord) {
  const items = arrayConfig(key)
  items[index] = item
  setArray(key, items)
}

function updateArrayItem(key: string, index: number, patch: ConfigRecord) {
  replaceArrayItem(key, index, {
    ...arrayConfig(key)[index],
    ...patch,
  })
}

function removeArrayItem(key: string, index: number) {
  setArray(key, arrayConfig(key).filter((_, itemIndex) => itemIndex !== index))
}

function jsonFieldPath(field: ConfigRecord) {
  const path = stringField(field, 'path').trim()
  if (path) {
    return path
  }
  return stringList(field.pathSegments).join('.')
}

function updateJsonFieldPath(key: string, index: number, value: string) {
  const item = { ...arrayConfig(key)[index] }
  const trimmed = value.trim()
  delete item.pathSegments
  if (trimmed) {
    item.path = trimmed
  } else {
    delete item.path
  }
  replaceArrayItem(key, index, item)
}

function updateArrayListField(key: string, index: number, field: string, value: string) {
  updateArrayItem(key, index, { [field]: parseListInput(value) })
}

function updateDropNullColumns(index: number, value: string) {
  const item = { ...arrayConfig('rules')[index] }
  delete item.column
  item.columns = parseListInput(value)
  replaceArrayItem('rules', index, item)
}

function updateArrayScalarField(key: string, index: number, field: string, value: string) {
  updateArrayItem(key, index, { [field]: parseScalarInput(value) })
}

function updateConfigOptionalStringForArray(key: string, index: number, field: string, value: string) {
  const item = { ...arrayConfig(key)[index] }
  const trimmed = value.trim()
  if (trimmed) {
    item[field] = trimmed
  } else {
    delete item[field]
  }
  replaceArrayItem(key, index, item)
}

function updateArrayOptionalNumberField(key: string, index: number, field: string, value: string) {
  const item = { ...arrayConfig(key)[index] }
  const trimmed = value.trim()
  if (!trimmed) {
    delete item[field]
  } else {
    item[field] = Number(trimmed)
  }
  replaceArrayItem(key, index, item)
}

function updateNestedObjectField(key: string, index: number, nestedKey: string, patch: ConfigRecord) {
  const item = arrayConfig(key)[index]
  updateArrayItem(key, index, {
    [nestedKey]: {
      ...asRecord(item[nestedKey]),
      ...patch,
    },
  })
}

function updateNestedConditionValue(key: string, index: number, nestedKey: string, value: string) {
  const condition = asRecord(arrayConfig(key)[index]?.[nestedKey])
  updateNestedObjectField(key, index, nestedKey, {
    value: stringValue(condition.operator) === 'in' ? parseListInput(value) : parseScalarInput(value),
  })
}

function updateConditionValue(key: string, index: number, value: string) {
  const condition = arrayConfig(key)[index]
  updateArrayItem(key, index, {
    value: stringValue(condition.operator) === 'in' ? parseListInput(value) : parseScalarInput(value),
  })
}

function updateArrayObjectFromJson(key: string, index: number, field: string, value: string) {
  try {
    const parsed = JSON.parse(value || '{}')
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      throw new Error(t('dataPipelines.jsonFieldMustBeObject', { field }))
    }
    updateArrayItem(key, index, { [field]: parsed as ConfigRecord })
  } catch (error) {
    uiError.value = error instanceof Error ? error.message : t('dataPipelines.invalidJsonObject')
  }
}

function updateArrayArrayFromJson(key: string, index: number, field: string, value: string) {
  try {
    const parsed = JSON.parse(value || '[]')
    if (!Array.isArray(parsed)) {
      throw new Error(t('dataPipelines.jsonFieldMustBeArray', { field }))
    }
    updateArrayItem(key, index, { [field]: parsed })
  } catch (error) {
    uiError.value = error instanceof Error ? error.message : t('dataPipelines.invalidJsonArray')
  }
}

function updateConfigArrayFromJson(key: string, value: string) {
  try {
    const parsed = JSON.parse(value || '[]')
    if (!Array.isArray(parsed)) {
      throw new Error(t('dataPipelines.jsonFieldMustBeArray', { field: key }))
    }
    updateConfigField(key, parsed)
  } catch (error) {
    uiError.value = error instanceof Error ? error.message : t('dataPipelines.invalidJsonArray')
  }
}

function addStepRule() {
  switch (stepType.value) {
  case 'clean':
    addArrayItem('rules', { operation: 'drop_null_rows', columns: [] })
    break
  case 'normalize':
    addArrayItem('rules', { operation: 'trim', column: '' })
    break
  case 'validate':
    addArrayItem('rules', { column: '', operator: 'required' })
    break
  case 'schema_completion':
    addArrayItem('rules', { targetColumn: '', method: 'literal', value: '' })
    break
  }
}

function addMapping() {
  addArrayItem('mappings', { sourceColumn: '', targetColumn: '', cast: '', required: false })
}

function schemaMappingRequestColumns() {
  return arrayFromConfig(configDraft.value, 'mappings')
    .map((mapping, index) => ({
      sourceColumn: stringField(mapping, 'sourceColumn').trim(),
      neighborColumns: knownColumns.value.filter((column) => column !== stringField(mapping, 'sourceColumn').trim()).slice(0, 20),
      index,
    }))
    .filter((item) => item.sourceColumn)
}

async function loadSchemaMappingCandidates() {
  const columns = schemaMappingRequestColumns()
  if (columns.length === 0) {
    uiError.value = t('dataPipelines.schemaMappingCandidatesNeedSourceColumns')
    return
  }
  schemaMappingCandidateLoading.value = true
  schemaMappingCandidateError.value = ''
  try {
    const result = await fetchSchemaMappingCandidates({
      pipelinePublicId: props.pipelinePublicId || undefined,
      versionPublicId: props.versionPublicId || undefined,
      domain: stringConfig('domain') || undefined,
      schemaType: stringConfig('schemaType') || undefined,
      limit: 3,
      columns: columns.map((column) => ({
        sourceColumn: column.sourceColumn,
        sampleValues: schemaMappingSampleValues(column.sourceColumn, column.index),
        neighborColumns: column.neighborColumns,
      })),
    })
    schemaMappingCandidateItems.value = result.items ?? []
  } catch (error) {
    schemaMappingCandidateItems.value = []
    schemaMappingCandidateError.value = toApiErrorMessage(error)
  } finally {
    schemaMappingCandidateLoading.value = false
  }
}

function schemaMappingCandidatesFor(sourceColumn: string) {
  return schemaMappingCandidateMap.value.get(sourceColumn.trim()) ?? []
}

function schemaMappingSampleValues(sourceColumn: string, mappingIndex: number) {
  const preview = props.preview
  if (!preview?.previewRows?.length) {
    return []
  }
  const mapping = arrayFromConfig(configDraft.value, 'mappings')[mappingIndex] ?? {}
  const targetColumn = stringField(mapping, 'targetColumn').trim()
  const candidateColumns = uniqueStrings([sourceColumn.trim(), targetColumn])
  const out: string[] = []
  for (const row of preview.previewRows) {
    for (const column of candidateColumns) {
      if (!column || !(column in row)) {
        continue
      }
      const value = previewSampleValue(row[column])
      if (value && !out.includes(value)) {
        out.push(value)
      }
      break
    }
    if (out.length >= 5) {
      break
    }
  }
  return out
}

function previewSampleValue(value: unknown) {
  if (value === null || value === undefined) {
    return ''
  }
  if (typeof value === 'object') {
    return JSON.stringify(value).slice(0, 160)
  }
  return String(value).trim().slice(0, 160)
}

async function applySchemaMappingCandidate(index: number, candidate: SchemaMappingCandidateBody) {
  const mapping = arrayFromConfig(configDraft.value, 'mappings')[index] ?? {}
  const sourceColumn = stringField(mapping, 'sourceColumn').trim()
  const neighborColumns = knownColumns.value.filter((column) => column !== sourceColumn).slice(0, 20)
  updateArrayItem('mappings', index, { targetColumn: candidate.targetColumn })
  if (!props.pipelinePublicId || !sourceColumn) {
    return
  }
  try {
    await recordSchemaMappingExample({
      pipelinePublicId: props.pipelinePublicId,
      versionPublicId: props.versionPublicId || undefined,
      schemaColumnPublicId: candidate.schemaColumnPublicId,
      sourceColumn,
      sampleValues: schemaMappingSampleValues(sourceColumn, index),
      neighborColumns,
      decision: 'accepted',
    })
  } catch (error) {
    schemaMappingCandidateError.value = toApiErrorMessage(error)
  }
}

function schemaMappingScoreLabel(score: number) {
  if (!Number.isFinite(score)) {
    return '0.00'
  }
  return score.toFixed(2)
}

function addTransformCondition() {
  addArrayItem('conditions', { column: '', operator: 'required' })
}

function addSort() {
  addArrayItem('sorts', { column: '', direction: 'ASC' })
}

function addAggregation() {
  addArrayItem('aggregations', { function: 'count', column: '', alias: '' })
}

function objectEntries(key: string) {
  return Object.entries(asRecord(configDraft.value[key])).map(([source, target]) => ({
    source,
    target: stringValue(target),
  }))
}

function addObjectEntry(key: string) {
  const object = asRecord(configDraft.value[key])
  let source = 'source_column'
  let suffix = 1
  while (Object.prototype.hasOwnProperty.call(object, source)) {
    suffix += 1
    source = `source_column_${suffix}`
  }
  setConfig({ [key]: { ...object, [source]: 'target_column' } })
}

function updateObjectEntrySource(key: string, index: number, value: string) {
  const entries = objectEntries(key)
  const current = entries[index]
  if (!current) {
    return
  }
  const object = asRecord(configDraft.value[key])
  delete object[current.source]
  if (value.trim()) {
    object[value.trim()] = current.target
  }
  setConfig({ [key]: object })
}

function updateObjectEntryTarget(key: string, index: number, value: string) {
  const entries = objectEntries(key)
  const current = entries[index]
  if (!current) {
    return
  }
  setConfig({
    [key]: {
      ...asRecord(configDraft.value[key]),
      [current.source]: value.trim(),
    },
  })
}

function removeObjectEntry(key: string, index: number) {
  const entries = objectEntries(key)
  const current = entries[index]
  if (!current) {
    return
  }
  const object = asRecord(configDraft.value[key])
  delete object[current.source]
  setConfig({ [key]: object })
}

function sourceColumnsFromConfig(config: ConfigRecord, kindKey: string, datasetKey: string, workTableKey: string) {
  const kind = stringValue(config[kindKey]) || 'dataset'
  if (kind === 'drive_file' && spreadsheetInputModeFromConfig(config)) {
    const sourceMetadataColumns = config.includeSourceMetadataColumns === false
      ? []
      : ['file_public_id', 'file_name', 'mime_type', 'file_revision', 'sheet_name', 'sheet_index', 'row_number']
    return uniqueStrings([
      ...sourceMetadataColumns,
      ...stringList(config.columns),
    ])
  }
  if (kind === 'drive_file' && jsonInputModeFromConfig(config)) {
    const sourceMetadataColumns = config.includeSourceMetadataColumns === false
      ? []
      : ['file_public_id', 'file_name', 'mime_type', 'file_revision', 'row_number', 'record_path']
    const jsonColumns = arrayFromConfig(config, 'fields')
      .map((field) => stringField(field, 'column').trim())
      .filter(Boolean)
    const rawRecordColumns = config.includeRawRecord === true ? ['raw_record_json'] : []
    return uniqueStrings([
      ...sourceMetadataColumns,
      ...jsonColumns,
      ...rawRecordColumns,
    ])
  }
  if (kind === 'drive_file') {
    return ['file_public_id', 'file_name', 'mime_type', 'file_revision']
  }
  const publicId = stringValue(kind === 'work_table' ? config[workTableKey] : config[datasetKey])
  if (!publicId) {
    return []
  }
  if (kind === 'work_table') {
    return workTableOptions.value.find((item) => item.publicId === publicId)?.columns?.map((column) => column.columnName) ?? []
  }
  return datasetOptions.value.find((item) => item.publicId === publicId)?.columns?.map((column) => column.columnName) ?? []
}

function spreadsheetInputModeFromConfig(config: ConfigRecord) {
  const mode = stringValue(config.inputMode || config.format)
  return ['spreadsheet', 'excel', 'xls', 'xlsx'].includes(mode)
}

function jsonInputModeFromConfig(config: ConfigRecord) {
  const mode = stringValue(config.inputMode || config.format)
  return ['json', 'application/json'].includes(mode)
}

function columnsForNodeOutput(nodeId?: string, visited = new Set<string>()): string[] {
  if (!nodeId || visited.has(nodeId)) {
    return []
  }
  visited.add(nodeId)
  const node = props.graph.nodes.find((item) => item.id === nodeId)
  if (!node) {
    return []
  }
  const validationSchemaColumns = props.validation?.outputSchemas?.find((schema) => schema.nodeId === nodeId)?.columns
  if (validationSchemaColumns && validationSchemaColumns.length > 0) {
    return validationSchemaColumns
  }
  const backendSchemaColumns = props.preview?.outputSchemas?.find((schema) => schema.nodeId === nodeId)?.columns
  if (backendSchemaColumns && backendSchemaColumns.length > 0) {
    return backendSchemaColumns
  }
  const config = node.id === props.selectedNodeId ? configDraft.value : asRecord(node.data.config)
  const upstreamIds = incomingNodeIds(node.id)
  const upstreamColumns = () => firstAvailableUpstreamColumns(upstreamIds, visited)
  switch (node.data.stepType) {
  case 'input':
    return sourceColumnsFromConfig(config, 'sourceKind', 'datasetPublicId', 'workTablePublicId')
  case 'transform':
    return inferTransformColumns(config, upstreamColumns())
  case 'join':
    return inferJoinColumns(
      config,
      columnsForNodeOutput(upstreamIds[0], new Set(visited)),
      columnsForNodeOutput(upstreamIds[1], new Set(visited)),
    )
  case 'enrich_join':
    return inferJoinColumns(
      config,
      upstreamColumns(),
      sourceColumnsFromConfig(config, 'rightSourceKind', 'rightDatasetPublicId', 'rightWorkTablePublicId'),
    )
  default:
    return inferDataPipelineStepOutputColumns(node.data.stepType, config, upstreamColumns()) ?? upstreamColumns()
  }
}

function selectedMissingColumnWarnings() {
  if (!selectedNode.value || stepType.value === 'input') {
    return []
  }
  if (props.validation) {
    return props.validation.nodeWarnings
      .filter((warning) => warning.nodeId === selectedNode.value?.id)
      .map(validationWarningMessage)
  }
  const warnings: string[] = []
  const primaryMissing = missingColumns(configuredPrimaryColumnRefs(stepType.value, configDraft.value), primaryColumns.value)
  if (primaryMissing.length > 0) {
    warnings.push(t('dataPipelines.missingUpstreamColumns', { columns: primaryMissing.join(', ') }))
  }
  if (stepType.value === 'join') {
    const rightMissing = missingColumns(stringList(configDraft.value.rightKeys), rightColumns.value)
    if (rightMissing.length > 0) {
      warnings.push(t('dataPipelines.missingRightUpstreamColumns', { columns: rightMissing.join(', ') }))
    }
  }
  return warnings
}

function validationWarningMessage(warning: DataPipelineNodeWarningBody) {
  if (warning.code === 'missing_right_upstream_columns') {
    return t('dataPipelines.missingRightUpstreamColumns', { columns: warning.columns.join(', ') })
  }
  if (warning.code === 'missing_upstream_columns') {
    return t('dataPipelines.missingUpstreamColumns', { columns: warning.columns.join(', ') })
  }
  return warning.message
}

function configuredPrimaryColumnRefs(type: string, config: ConfigRecord) {
  switch (type) {
  case 'json_extract':
    return [stringValue(config.sourceColumn)]
  case 'excel_extract':
    return [stringValue(config.sourceFileColumn)]
  case 'clean':
    return arrayFromConfig(config, 'rules').flatMap(cleanRuleColumnRefs)
  case 'normalize':
    return arrayFromConfig(config, 'rules').map((rule) => stringField(rule, 'column'))
  case 'validate':
    return arrayFromConfig(config, 'rules').map((rule) => stringField(rule, 'column'))
  case 'schema_mapping':
    return arrayFromConfig(config, 'mappings').map((mapping) => stringField(mapping, 'sourceColumn'))
  case 'schema_completion':
    return arrayFromConfig(config, 'rules').flatMap((rule) => [
      stringField(rule, 'sourceColumn'),
      ...stringList(rule.sourceColumns),
    ])
  case 'join':
    return stringList(config.leftKeys)
  case 'transform':
    return transformColumnRefs(config)
  case 'schema_inference':
  case 'quality_report':
  case 'deduplicate':
  case 'redact_pii':
  case 'union':
    return stringList(config.columns)
  case 'quarantine':
    return [stringValue(config.statusColumn)]
  case 'route_by_condition':
    return arrayFromConfig(config, 'rules').map((rule) => stringField(rule, 'column'))
  case 'partition_filter':
    return [
      stringValue(config.dateColumn || config.column),
      stringValue(config.partitionKey),
    ]
  case 'watermark_filter':
    return [stringValue(config.watermarkColumn || config.column)]
  case 'canonicalize':
    return arrayFromConfig(config, 'rules').map((rule) => stringField(rule, 'column'))
  case 'classify_document':
  case 'relationship_extraction':
    return [stringValue(config.textColumn)]
  case 'detect_language_encoding':
    return [stringValue(config.textColumn)]
  case 'unit_conversion':
    return arrayFromConfig(config, 'rules').flatMap((rule) => [
      stringField(rule, 'valueColumn'),
      stringField(rule, 'unitColumn'),
    ])
  case 'sample_compare':
    return arrayFromConfig(config, 'pairs').flatMap((pair) => [
      stringField(pair, 'beforeColumn'),
      stringField(pair, 'afterColumn'),
    ])
  case 'output':
    return arrayFromConfig(config, 'columns').map((column) => stringField(column, 'sourceColumn') || stringField(column, 'column'))
  default:
    return []
  }
}

function cleanRuleColumnRefs(rule: ConfigRecord) {
  const operation = stringField(rule, 'operation')
  if (operation === 'dedupe') {
    return [...stringList(rule.keys), stringField(rule, 'orderBy')]
  }
  if (operation === 'drop_null_rows') {
    return stringList(rule.columns || rule.column)
  }
  return [stringField(rule, 'column')]
}

function transformColumnRefs(config: ConfigRecord) {
  const operation = stringValue(config.operation || config.type) || 'select_columns'
  switch (operation) {
  case 'select_columns':
  case 'drop_columns':
    return stringList(config.columns)
  case 'rename_columns':
    return Object.keys(asRecord(config.renames))
  case 'filter':
    return arrayFromConfig(config, 'conditions').map((condition) => stringField(condition, 'column'))
  case 'sort':
    return arrayFromConfig(config, 'sorts').map((sort) => stringField(sort, 'column'))
  case 'aggregate':
    return [
      ...stringList(config.groupBy),
      ...arrayFromConfig(config, 'aggregations').map((aggregation) => stringField(aggregation, 'column')),
    ]
  default:
    return []
  }
}

function missingColumns(columns: string[], availableColumns: string[]) {
  const available = new Set(availableColumns)
  return uniqueStrings(columns.map((column) => column.trim()).filter(Boolean))
    .filter((column) => !available.has(column))
}

function incomingNodeIds(nodeId: string) {
  return props.graph.edges
    .filter((edge) => edge.target === nodeId)
    .map((edge) => edge.source)
}

function firstAvailableUpstreamColumns(upstreamIds: string[], visited: Set<string>) {
  for (const upstreamId of upstreamIds) {
    const columns = columnsForNodeOutput(upstreamId, new Set(visited))
    if (columns.length > 0) {
      return columns
    }
  }
  return []
}

function inferTransformColumns(config: ConfigRecord, upstreamColumns: string[]) {
  const operation = stringValue(config.operation || config.type) || 'select_columns'
  switch (operation) {
  case 'select_columns': {
    const columns = stringList(config.columns)
    return columns.length > 0 ? uniqueStrings(columns) : upstreamColumns
  }
  case 'drop_columns': {
    const drops = new Set(stringList(config.columns))
    return upstreamColumns.filter((column) => !drops.has(column))
  }
  case 'rename_columns': {
    const renames = asRecord(config.renames)
    return upstreamColumns.map((column) => stringValue(renames[column]).trim() || column)
  }
  case 'aggregate':
    return inferAggregateColumns(config)
  default:
    return upstreamColumns
  }
}

function inferAggregateColumns(config: ConfigRecord) {
  const columns = [...stringList(config.groupBy)]
  for (const aggregation of arrayFromConfig(config, 'aggregations')) {
    const fn = stringField(aggregation, 'function').toLowerCase() || 'count'
    const column = stringField(aggregation, 'column')
    const alias = stringField(aggregation, 'alias') || (column ? `${fn}_${column}` : fn)
    if (alias) {
      columns.push(alias)
    }
  }
  return columns.length > 0 ? uniqueStrings(columns) : ['count']
}

function inferJoinColumns(config: ConfigRecord, leftColumns: string[], rightColumns: string[]) {
  const selectedRightColumns = stringList(config.selectColumns)
  const rightSelection = selectedRightColumns.length > 0 ? selectedRightColumns : rightColumns
  const columns = [...leftColumns]
  for (const column of rightSelection) {
    columns.push(columns.includes(column) ? `${column}_right` : column)
  }
  return uniqueStrings(columns)
}

function inputConfigForColumns() {
  if (stepType.value === 'input') {
    return configDraft.value
  }
  return graphInputConfigForColumns(selectedIncomingNodeIds.value[0])
    ?? props.graph.nodes.find((node) => node.data.stepType === 'input')?.data.config
    ?? {}
}

function graphInputConfigForColumns(nodeId?: string) {
  const node = props.graph.nodes.find((item) => item.id === nodeId)
  if (node?.data.stepType === 'input') {
    return node.data.config ?? {}
  }
  return null
}

function arrayFromConfig(config: ConfigRecord, key: string): ConfigRecord[] {
  const raw = config[key]
  if (!Array.isArray(raw)) {
    return []
  }
  return raw.map((item) => asRecord(item))
}

function sourceOptions(kind: string) {
  return kind === 'work_table' ? workTableOptions.value : datasetOptions.value
}

function sourceOptionValue(item: DatasetBody | DatasetWorkTableBody) {
  return item.publicId ?? ''
}

function sourceOptionLabel(item: DatasetBody | DatasetWorkTableBody) {
  if ('displayName' in item) {
    return `${item.displayName} / ${item.table}`
  }
  return `${item.name} / ${item.rawTable}`
}

function sourceSelectLabel(right = false) {
  return sourceKind(right) === 'work_table' ? t('dataPipelines.workTable') : t('dataPipelines.dataset')
}

function parseListInput(value: string) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function listToInput(value: unknown) {
  return stringList(value).join(', ')
}

function parseScalarInput(value: string): unknown {
  const trimmed = value.trim()
  if (trimmed === '') {
    return ''
  }
  if (trimmed === 'true') {
    return true
  }
  if (trimmed === 'false') {
    return false
  }
  if (trimmed === 'null') {
    return null
  }
  if (/^-?\d+(\.\d+)?$/.test(trimmed)) {
    return Number(trimmed)
  }
  return value
}

function scalarToInput(value: unknown) {
  if (Array.isArray(value)) {
    return value.map((item) => stringValue(item)).join(', ')
  }
  if (value === null || value === undefined) {
    return ''
  }
  if (typeof value === 'object') {
    return JSON.stringify(value)
  }
  return String(value)
}

function jsonForField(value: unknown, fallback: unknown = {}) {
  return JSON.stringify(value ?? fallback, null, 2)
}

function numberField(record: ConfigRecord, key: string) {
  const value = record[key]
  return typeof value === 'number' ? String(value) : stringValue(value)
}

function targetValue(event: Event) {
  return (event.target as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement).value
}

function targetChecked(event: Event) {
  return (event.target as HTMLInputElement).checked
}

function asRecord(value: unknown): ConfigRecord {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return {}
  }
  return { ...(value as ConfigRecord) }
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

function cloneConfig(config: ConfigRecord): ConfigRecord {
  return JSON.parse(JSON.stringify(config)) as ConfigRecord
}

function stepLabel(type: DataPipelineStepType | string) {
  const key = `dataPipelines.step.${type}`
  const value = t(key)
  return value === key ? labelForStep(type) : value
}

function optionLabel(key: string, fallback?: string) {
  const value = t(key)
  const parts = key.split('.')
  return value === key ? fallback ?? parts[parts.length - 1] ?? key : value
}

function conditionOperatorLabel(operator: string) {
  if (operator === 'required' || operator === 'in' || operator === 'regex') {
    return t(`dataPipelines.conditionOperator.${operator}`)
  }
  return operator
}

function transformOperationLabel(operation: string) {
  return t(`dataPipelines.transformOperation.${operation}`)
}

function completionMethodLabel(method: string) {
  return t(`dataPipelines.completionMethod.${method}`)
}

function labelForStep(type: DataPipelineStepType | string) {
  return type
    .split('_')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}
</script>

<template>
  <aside class="data-pipeline-inspector" :aria-label="t('dataPipelines.inspector')">
    <header class="panel-header compact data-pipeline-inspector-header">
      <div class="data-pipeline-inspector-title">
        <h2>{{ t('dataPipelines.inspector') }}</h2>
        <div v-if="selectedNode" class="data-pipeline-inspector-badges">
          <span class="status-pill">{{ stepLabel(stepType) }}</span>
          <span
            class="data-pipeline-preview-mode-pill"
            :class="{ manual: !autoPreviewEnabled }"
            :title="previewModeTitle"
            :aria-label="previewModeTitle"
            role="img"
          >
            <component :is="previewModeIcon" :size="14" stroke-width="2.1" aria-hidden="true" />
          </span>
        </div>
      </div>
    </header>

    <div v-if="selectedNode" class="inspector-fields">
      <label class="field inspector-label-field">
        <span class="field-label">{{ t('dataPipelines.label') }}</span>
        <input :value="label" :placeholder="t('dataPipelines.stepLabelPlaceholder')" @input="updateLabel(targetValue($event))">
      </label>

      <datalist id="data-pipeline-column-options">
        <option v-for="column in knownColumns" :key="column" :value="column" />
      </datalist>
      <datalist id="data-pipeline-right-column-options">
        <option v-for="column in rightColumns" :key="column" :value="column" />
      </datalist>

      <div class="data-pipeline-inspector-tabs" role="tablist" :aria-label="t('dataPipelines.inspectorConfigEditor')">
        <button
          type="button"
          role="tab"
          :aria-selected="activeConfigTab === 'ui'"
          :class="{ active: activeConfigTab === 'ui' }"
          @click="activeConfigTab = 'ui'"
        >
          {{ t('dataPipelines.configUi') }}
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeConfigTab === 'json'"
          :class="{ active: activeConfigTab === 'json' }"
          @click="activeConfigTab = 'json'"
        >
          {{ t('dataPipelines.configJson') }}
        </button>
      </div>

      <section v-if="activeConfigTab === 'ui'" class="config-form-section" role="tabpanel">
        <header class="config-section-header">
          <h3>{{ t('dataPipelines.configUi') }}</h3>
          <span class="status-pill">{{ stepLabel(stepType) }}</span>
        </header>
        <div v-if="missingColumnWarnings.length > 0" class="config-warning-list" role="status">
          <p v-for="warning in missingColumnWarnings" :key="warning" class="warning-message">
            <AlertCircle :size="14" stroke-width="1.9" aria-hidden="true" />
            {{ warning }}
          </p>
        </div>

        <template v-if="stepType === 'input'">
          <label class="field">
            <span>{{ t('dataPipelines.sourceKind') }}</span>
            <select :value="sourceKind()" @change="setSourceKind(targetValue($event))">
              <option v-for="kind in sourceKindOptions()" :key="kind.value" :value="kind.value">{{ t(kind.labelKey) }}</option>
            </select>
          </label>
          <label v-if="sourceKind() !== 'drive_file'" class="field">
            <span>{{ sourceSelectLabel() }}</span>
            <select :value="sourcePublicId()" @change="setSourcePublicId(targetValue($event))">
              <option value="">{{ t('dataPipelines.selectSource') }}</option>
              <option
                v-for="item in sourceOptions(sourceKind())"
                :key="sourceOptionValue(item)"
                :value="sourceOptionValue(item)"
              >
                {{ sourceOptionLabel(item) }}
              </option>
            </select>
          </label>
          <template v-else>
            <div class="data-pipeline-drive-file-selector">
              <div class="config-section-header">
                <h3>{{ t('dataPipelines.selectedDriveFiles') }}</h3>
                <button class="secondary-button compact-button" type="button" @click="drivePickerOpen = true">
                  <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
                  {{ t('dataPipelines.chooseDriveFiles') }}
                </button>
              </div>
              <p v-if="selectedDriveFilesLoading" class="cell-subtle">{{ t('common.loading') }}</p>
              <p v-else-if="selectedDriveFiles.length === 0" class="cell-subtle">{{ t('dataPipelines.noSelectedDriveFiles') }}</p>
              <div v-else class="data-pipeline-selected-drive-files">
                <div v-for="item in selectedDriveFiles" :key="item.publicId" class="data-pipeline-selected-drive-file">
                  <span>
                    <strong>{{ selectedDriveFileName(item) }}</strong>
                    <small>{{ selectedDriveFileSubtitle(item) }}</small>
                  </span>
                  <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeDriveFile', { name: selectedDriveFileName(item) })" @click="removeDriveFile(item.publicId)">
                    <Trash2 :size="14" stroke-width="1.9" aria-hidden="true" />
                  </button>
                </div>
              </div>
            </div>
            <label class="field">
              <span>{{ t('dataPipelines.driveInputModeLabel') }}</span>
              <select :value="driveInputMode()" @change="setDriveInputMode(targetValue($event))">
                <option v-for="mode in driveInputModes" :key="mode.value" :value="mode.value">{{ t(mode.labelKey) }}</option>
              </select>
            </label>
            <div v-if="driveInputMode() === 'spreadsheet'" class="config-grid">
              <div class="data-pipeline-manifest-section">
                <div class="config-section-header">
                  <h3>{{ t('dataPipelines.documentManifest') }}</h3>
                  <button class="secondary-button compact-button" type="button" :disabled="!primaryDriveFileId || driveManifestLoading" @click="loadDriveManifest(true)">
                    <RefreshCw :size="14" stroke-width="1.9" aria-hidden="true" />
                    {{ t('dataPipelines.refreshManifest') }}
                  </button>
                </div>
                <p v-if="driveManifestLoading" class="cell-subtle">{{ t('common.loading') }}</p>
                <p v-else-if="driveManifestError" class="error-message">{{ driveManifestError }}</p>
                <p v-else-if="!primaryDriveFileId" class="cell-subtle">{{ t('dataPipelines.selectDriveFileForManifest') }}</p>
                <p v-else-if="driveManifest && driveManifest.documentType !== 'excel'" class="cell-subtle">
                  {{ t('dataPipelines.unsupportedSpreadsheetManifest') }}
                </p>
              </div>
              <label class="field">
                <span>{{ t('dataPipelines.sheetName') }}</span>
                <select :value="selectedSheetValue()" :disabled="driveManifestSheets.length === 0" @change="selectManifestSheet(targetValue($event))">
                  <option value="">{{ t('dataPipelines.selectSheet') }}</option>
                  <option v-for="sheet in driveManifestSheets" :key="sheet.index" :value="String(sheet.index)">
                    {{ sheetOptionLabel(sheet) }}
                  </option>
                </select>
              </label>
              <div v-if="selectedManifestSheet" class="data-pipeline-sheet-hint">
                <span v-if="selectedManifestSheet.usedRange">{{ selectedManifestSheet.usedRange }}</span>
                <span v-if="selectedManifestSheet.headerPreview?.length">{{ t('dataPipelines.headerPreview') }}: {{ selectedManifestSheet.headerPreview.join(', ') }}</span>
              </div>
              <p v-else-if="stringConfig('sheetName')" class="cell-subtle">
                {{ t('dataPipelines.sheetFromJsonOnly', { name: stringConfig('sheetName') }) }}
              </p>
              <label class="field">
                <span>{{ t('dataPipelines.range') }}</span>
                <input :value="stringConfig('range')" :placeholder="selectedManifestSheet?.usedRange || 'A1:H1200'" @input="updateConfigOptionalString('range', targetValue($event))">
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.headerRow') }}</span>
                <input :value="numberField(configDraft, 'headerRow')" type="number" min="0" step="1" :placeholder="t('dataPipelines.headerRowPlaceholder')" @input="updateConfigOptionalNumber('headerRow', targetValue($event))">
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.maxRows') }}</span>
                <input :value="numberField(configDraft, 'maxRows')" type="number" min="1" step="1" placeholder="100000" @input="updateConfigOptionalNumber('maxRows', targetValue($event))">
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.columns') }}</span>
                <textarea :value="listToInput(configDraft.columns)" rows="3" :placeholder="t('dataPipelines.spreadsheetColumnsPlaceholder')" @input="updateConfigList('columns', targetValue($event))" />
              </label>
              <label class="checkbox-field">
                <input :checked="configDraft.includeSourceMetadataColumns !== false" type="checkbox" @change="updateConfigField('includeSourceMetadataColumns', targetChecked($event))">
                <span>{{ t('dataPipelines.includeSourceMetadataColumns') }}</span>
              </label>
            </div>
            <template v-else-if="driveInputMode() === 'json'">
              <div class="config-grid">
                <label class="field">
                  <span>{{ t('dataPipelines.jsonRecordPath') }}</span>
                  <input :value="stringConfig('recordPath') || '$'" placeholder="$" @input="updateConfigOptionalString('recordPath', targetValue($event))">
                </label>
                <label class="field">
                  <span>{{ t('dataPipelines.maxRows') }}</span>
                  <input :value="numberField(configDraft, 'maxRows')" type="number" min="1" step="1" placeholder="100000" @input="updateConfigOptionalNumber('maxRows', targetValue($event))">
                </label>
                <label class="checkbox-field">
                  <input :checked="configDraft.includeSourceMetadataColumns !== false" type="checkbox" @change="updateConfigField('includeSourceMetadataColumns', targetChecked($event))">
                  <span>{{ t('dataPipelines.includeSourceMetadataColumns') }}</span>
                </label>
                <label class="checkbox-field">
                  <input :checked="boolField(configDraft, 'includeRawRecord')" type="checkbox" @change="updateConfigField('includeRawRecord', targetChecked($event))">
                  <span>{{ t('dataPipelines.includeRawRecordJson') }}</span>
                </label>
              </div>
              <div class="config-section-header">
                <h3>{{ t('dataPipelines.jsonFields') }}</h3>
                <button class="secondary-button compact-button" type="button" @click="addArrayItem('fields', { column: '', path: '' })">
                  <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
                  {{ t('dataPipelines.addJsonField') }}
                </button>
              </div>
              <p v-if="arrayConfig('fields').length === 0" class="cell-subtle">{{ t('dataPipelines.noConfigFields') }}</p>
              <div v-for="(field, index) in arrayConfig('fields')" :key="index" class="config-rule">
                <div class="config-rule-header">
                  <strong>{{ stringField(field, 'column') || `${t('dataPipelines.jsonField')} ${index + 1}` }}</strong>
                  <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeJsonField', { index: index + 1 })" @click="removeArrayItem('fields', index)">
                    <Trash2 :size="14" stroke-width="1.9" aria-hidden="true" />
                  </button>
                </div>
                <div class="config-grid two-column">
                  <label class="field">
                    <span>{{ t('dataPipelines.column') }}</span>
                    <input :value="stringField(field, 'column')" placeholder="name_en" @input="updateArrayItem('fields', index, { column: targetValue($event) })">
                  </label>
                  <label class="field">
                    <span>{{ t('dataPipelines.jsonPath') }}</span>
                    <input :value="jsonFieldPath(field)" placeholder="name.english" @input="updateJsonFieldPath('fields', index, targetValue($event))">
                  </label>
                  <label class="field">
                    <span>{{ t('dataPipelines.joinArrayWith') }}</span>
                    <input :value="stringField(field, 'join')" placeholder="|" @input="updateConfigOptionalStringForArray('fields', index, 'join', targetValue($event))">
                  </label>
                  <label class="field">
                    <span>{{ t('dataPipelines.defaultValue') }}</span>
                    <input :value="stringField(field, 'default')" @input="updateConfigOptionalStringForArray('fields', index, 'default', targetValue($event))">
                  </label>
                </div>
              </div>
            </template>
            <DriveFilePickerDialog
              :open="drivePickerOpen"
              :selected-file-ids="driveFileIds"
              :multiple="false"
              :spreadsheet-mode="driveInputMode() === 'spreadsheet'"
              @close="drivePickerOpen = false"
              @apply="applyDriveFileSelection"
            />
          </template>
        </template>

        <template v-else-if="stepType === 'json_extract'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.sourceColumn') }}</span>
              <input :value="stringConfig('sourceColumn')" list="data-pipeline-column-options" placeholder="raw_record_json" @input="updateConfigOptionalString('sourceColumn', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.jsonRecordPath') }}</span>
              <input :value="stringConfig('recordPath') || '$'" placeholder="$" @input="updateConfigOptionalString('recordPath', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.maxRows') }}</span>
              <input :value="numberField(configDraft, 'maxRows')" type="number" min="1" step="1" placeholder="100000" @input="updateConfigOptionalNumber('maxRows', targetValue($event))">
            </label>
            <label class="checkbox-field">
              <input :checked="configDraft.includeSourceColumns !== false" type="checkbox" @change="updateConfigField('includeSourceColumns', targetChecked($event))">
              <span>{{ t('dataPipelines.includeSourceColumns') }}</span>
            </label>
            <label class="checkbox-field">
              <input :checked="boolField(configDraft, 'includeRawRecord')" type="checkbox" @change="updateConfigField('includeRawRecord', targetChecked($event))">
              <span>{{ t('dataPipelines.includeRawRecordJson') }}</span>
            </label>
          </div>
          <div class="config-section-header">
            <h3>{{ t('dataPipelines.jsonFields') }}</h3>
            <button class="secondary-button compact-button" type="button" @click="addArrayItem('fields', { column: '', path: '' })">
              <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
              {{ t('dataPipelines.addJsonField') }}
            </button>
          </div>
          <p v-if="arrayConfig('fields').length === 0" class="cell-subtle">{{ t('dataPipelines.noConfigFields') }}</p>
          <div v-for="(field, index) in arrayConfig('fields')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <strong>{{ stringField(field, 'column') || `${t('dataPipelines.jsonField')} ${index + 1}` }}</strong>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeJsonField', { index: index + 1 })" @click="removeArrayItem('fields', index)">
                <Trash2 :size="14" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>
            <div class="config-grid two-column">
              <label class="field">
                <span>{{ t('dataPipelines.column') }}</span>
                <input :value="stringField(field, 'column')" placeholder="name_en" @input="updateArrayItem('fields', index, { column: targetValue($event) })">
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.jsonPath') }}</span>
                <input :value="jsonFieldPath(field)" placeholder="name.english" @input="updateJsonFieldPath('fields', index, targetValue($event))">
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.joinArrayWith') }}</span>
                <input :value="stringField(field, 'join')" placeholder="|" @input="updateConfigOptionalStringForArray('fields', index, 'join', targetValue($event))">
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.defaultValue') }}</span>
                <input :value="stringField(field, 'default')" @input="updateConfigOptionalStringForArray('fields', index, 'default', targetValue($event))">
              </label>
            </div>
          </div>
        </template>

        <template v-else-if="stepType === 'excel_extract'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.sourceFileColumn') }}</span>
              <input :value="stringConfig('sourceFileColumn')" list="data-pipeline-column-options" placeholder="file_public_id" @input="updateConfigOptionalString('sourceFileColumn', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.sheetName') }}</span>
              <input :value="stringConfig('sheetName')" :placeholder="t('dataPipelines.firstSheet')" @input="updateConfigOptionalString('sheetName', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.sheetIndex') }}</span>
              <input :value="numberField(configDraft, 'sheetIndex')" type="number" min="0" step="1" placeholder="0" @input="updateConfigOptionalNumber('sheetIndex', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.range') }}</span>
              <input :value="stringConfig('range')" placeholder="A1:H1200" @input="updateConfigOptionalString('range', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.headerRow') }}</span>
              <input :value="numberField(configDraft, 'headerRow')" type="number" min="0" step="1" :placeholder="t('dataPipelines.headerRowPlaceholder')" @input="updateConfigOptionalNumber('headerRow', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.maxRows') }}</span>
              <input :value="numberField(configDraft, 'maxRows')" type="number" min="1" step="1" placeholder="100000" @input="updateConfigOptionalNumber('maxRows', targetValue($event))">
            </label>
            <label class="field config-grid-full">
              <span>{{ t('dataPipelines.columns') }}</span>
              <textarea :value="listToInput(configDraft.columns)" rows="3" :placeholder="t('dataPipelines.spreadsheetColumnsPlaceholder')" @input="updateConfigList('columns', targetValue($event))" />
            </label>
            <label class="checkbox-field">
              <input :checked="configDraft.includeSourceColumns !== false" type="checkbox" @change="updateConfigField('includeSourceColumns', targetChecked($event))">
              <span>{{ t('dataPipelines.includeSourceColumns') }}</span>
            </label>
            <label class="checkbox-field">
              <input :checked="configDraft.includeSourceMetadataColumns !== false" type="checkbox" @change="updateConfigField('includeSourceMetadataColumns', targetChecked($event))">
              <span>{{ t('dataPipelines.includeSourceMetadataColumns') }}</span>
            </label>
          </div>
        </template>

        <template v-else-if="stepType === 'clean'">
          <div v-for="(rule, index) in arrayConfig('rules')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <label class="field">
                <span>{{ t('dataPipelines.operation') }}</span>
                <select :value="stringField(rule, 'operation')" @change="updateArrayItem('rules', index, { operation: targetValue($event) })">
                  <option v-for="operation in cleanOperations" :key="operation.value" :value="operation.value">{{ t(operation.labelKey) }}</option>
                </select>
              </label>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeCleanRule', { index: index + 1 })" @click="removeArrayItem('rules', index)">
                <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>

            <label v-if="stringField(rule, 'operation') === 'drop_null_rows'" class="field">
              <span>{{ t('dataPipelines.columns') }}</span>
              <input :value="listToInput(rule.columns || rule.column)" list="data-pipeline-column-options" @input="updateDropNullColumns(index, targetValue($event))">
            </label>

            <template v-else-if="stringField(rule, 'operation') === 'dedupe'">
              <label class="field">
                <span>{{ t('dataPipelines.keys') }}</span>
                <input :value="listToInput(rule.keys)" list="data-pipeline-column-options" @input="updateArrayListField('rules', index, 'keys', targetValue($event))">
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.orderBy') }}</span>
                <input :value="stringField(rule, 'orderBy')" list="data-pipeline-column-options" @input="updateArrayItem('rules', index, { orderBy: targetValue($event) })">
              </label>
            </template>

            <template v-else>
              <label class="field">
                <span>{{ t('dataPipelines.column') }}</span>
                <input :value="stringField(rule, 'column')" list="data-pipeline-column-options" @input="updateArrayItem('rules', index, { column: targetValue($event) })">
              </label>
              <label v-if="stringField(rule, 'operation') === 'fill_null'" class="field">
                <span>{{ t('dataPipelines.value') }}</span>
                <input :value="scalarToInput(rule.value)" @input="updateArrayScalarField('rules', index, 'value', targetValue($event))">
              </label>
              <div v-if="stringField(rule, 'operation') === 'null_if'" class="config-grid">
                <label class="field">
                  <span>{{ t('dataPipelines.operator') }}</span>
                  <select
                    :value="stringField(recordField(rule, 'condition'), 'operator')"
                    @change="updateNestedObjectField('rules', index, 'condition', { operator: targetValue($event) })"
                  >
                    <option v-for="operator in conditionOperators" :key="operator" :value="operator">{{ conditionOperatorLabel(operator) }}</option>
                  </select>
                </label>
                <label class="field">
                  <span>{{ t('dataPipelines.value') }}</span>
                  <input
                    :value="scalarToInput(recordField(rule, 'condition').value)"
                    @input="updateNestedConditionValue('rules', index, 'condition', targetValue($event))"
                  >
                </label>
              </div>
              <div v-if="stringField(rule, 'operation') === 'clamp'" class="config-grid">
                <label class="field">
                  <span>{{ t('dataPipelines.min') }}</span>
                  <input :value="numberField(rule, 'min')" inputmode="decimal" @input="updateArrayOptionalNumberField('rules', index, 'min', targetValue($event))">
                </label>
                <label class="field">
                  <span>{{ t('dataPipelines.max') }}</span>
                  <input :value="numberField(rule, 'max')" inputmode="decimal" @input="updateArrayOptionalNumberField('rules', index, 'max', targetValue($event))">
                </label>
              </div>
            </template>
          </div>
          <button class="secondary-button compact-button" type="button" @click="addStepRule">
            <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.addRule') }}
          </button>
        </template>

        <template v-else-if="stepType === 'normalize'">
          <div v-for="(rule, index) in arrayConfig('rules')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <label class="field">
                <span>{{ t('dataPipelines.operation') }}</span>
                <select :value="stringField(rule, 'operation')" @change="updateArrayItem('rules', index, { operation: targetValue($event) })">
                  <option v-for="operation in normalizeOperations" :key="operation.value" :value="operation.value">{{ t(operation.labelKey) }}</option>
                </select>
              </label>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeNormalizeRule', { index: index + 1 })" @click="removeArrayItem('rules', index)">
                <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>
            <label class="field">
              <span>{{ t('dataPipelines.column') }}</span>
              <input :value="stringField(rule, 'column')" list="data-pipeline-column-options" @input="updateArrayItem('rules', index, { column: targetValue($event) })">
            </label>
            <label v-if="stringField(rule, 'operation') === 'cast_decimal'" class="field">
              <span>{{ t('dataPipelines.scale') }}</span>
              <input :value="numberField(rule, 'scale')" inputmode="numeric" @input="updateArrayOptionalNumberField('rules', index, 'scale', targetValue($event))">
            </label>
            <label v-if="stringField(rule, 'operation') === 'round'" class="field">
              <span>{{ t('dataPipelines.precision') }}</span>
              <input :value="numberField(rule, 'precision')" inputmode="numeric" @input="updateArrayOptionalNumberField('rules', index, 'precision', targetValue($event))">
            </label>
            <label v-if="stringField(rule, 'operation') === 'scale'" class="field">
              <span>{{ t('dataPipelines.factor') }}</span>
              <input :value="numberField(rule, 'factor')" inputmode="decimal" @input="updateArrayOptionalNumberField('rules', index, 'factor', targetValue($event))">
            </label>
            <label v-if="stringField(rule, 'operation') === 'map_values'" class="field">
              <span>{{ t('dataPipelines.valuesJson') }}</span>
              <textarea :value="jsonForField(rule.values)" rows="4" spellcheck="false" @input="updateArrayObjectFromJson('rules', index, 'values', targetValue($event))" />
            </label>
          </div>
          <button class="secondary-button compact-button" type="button" @click="addStepRule">
            <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.addRule') }}
          </button>
        </template>

        <template v-else-if="stepType === 'validate'">
          <div v-for="(rule, index) in arrayConfig('rules')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <label class="field">
                <span>{{ t('dataPipelines.column') }}</span>
                <input :value="stringField(rule, 'column')" list="data-pipeline-column-options" @input="updateArrayItem('rules', index, { column: targetValue($event) })">
              </label>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeValidationRule', { index: index + 1 })" @click="removeArrayItem('rules', index)">
                <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>
            <div class="config-grid">
              <label class="field">
                <span>{{ t('dataPipelines.operator') }}</span>
                <select :value="stringField(rule, 'operator')" @change="updateArrayItem('rules', index, { operator: targetValue($event) })">
                  <option v-for="operator in conditionOperators" :key="operator" :value="operator">{{ conditionOperatorLabel(operator) }}</option>
                </select>
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.value') }}</span>
                <input :value="scalarToInput(rule.value)" @input="updateConditionValue('rules', index, targetValue($event))">
              </label>
            </div>
          </div>
          <button class="secondary-button compact-button" type="button" @click="addStepRule">
            <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.addRule') }}
          </button>
        </template>

        <template v-else-if="stepType === 'schema_mapping'">
          <div class="config-section-header">
            <h3>{{ t('dataPipelines.schemaMappingCandidates') }}</h3>
            <button class="secondary-button compact-button" type="button" :disabled="schemaMappingCandidateLoading || !schemaMappingCandidateReady" @click="loadSchemaMappingCandidates">
              <Sparkles :size="15" stroke-width="1.9" aria-hidden="true" />
              {{ schemaMappingCandidateLoading ? t('common.loading') : t('dataPipelines.getSchemaMappingCandidates') }}
            </button>
          </div>
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.domain') }}</span>
              <input :value="stringConfig('domain')" placeholder="invoice" @input="updateConfigOptionalString('domain', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.schemaType') }}</span>
              <input :value="stringConfig('schemaType')" placeholder="ap_invoice" @input="updateConfigOptionalString('schemaType', targetValue($event))">
            </label>
          </div>
          <p v-if="schemaMappingCandidateError" class="inline-error">
            <AlertCircle :size="14" stroke-width="1.9" aria-hidden="true" />
            {{ schemaMappingCandidateError }}
          </p>
          <div v-for="(mapping, index) in arrayConfig('mappings')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <label class="field">
                <span>{{ t('dataPipelines.sourceColumn') }}</span>
                <input :value="stringField(mapping, 'sourceColumn')" list="data-pipeline-column-options" @input="updateArrayItem('mappings', index, { sourceColumn: targetValue($event) })">
              </label>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeMapping', { index: index + 1 })" @click="removeArrayItem('mappings', index)">
                <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>
            <label class="field">
              <span>{{ t('dataPipelines.targetColumn') }}</span>
              <input :value="stringField(mapping, 'targetColumn')" @input="updateArrayItem('mappings', index, { targetColumn: targetValue($event) })">
            </label>
            <div class="config-grid">
              <label class="field">
                <span>{{ t('dataPipelines.cast') }}</span>
                <select :value="stringField(mapping, 'cast')" @change="updateArrayItem('mappings', index, { cast: targetValue($event) })">
                  <option v-for="cast in castOptions" :key="cast" :value="cast">{{ cast || t('common.none') }}</option>
                </select>
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.defaultValue') }}</span>
                <input :value="scalarToInput(mapping.defaultValue)" @input="updateArrayScalarField('mappings', index, 'defaultValue', targetValue($event))">
              </label>
            </div>
            <label class="checkbox-field">
              <input :checked="boolField(mapping, 'required')" type="checkbox" @change="updateArrayItem('mappings', index, { required: targetChecked($event) })">
              <span>{{ t('dataPipelines.required') }}</span>
            </label>
            <div v-if="schemaMappingCandidatesFor(stringField(mapping, 'sourceColumn')).length > 0" class="schema-mapping-candidates">
              <span class="cell-subtle">{{ t('dataPipelines.schemaMappingCandidates') }}</span>
              <button
                v-for="candidate in schemaMappingCandidatesFor(stringField(mapping, 'sourceColumn'))"
                :key="candidate.schemaColumnPublicId"
                class="schema-mapping-candidate-button"
                type="button"
                @click="applySchemaMappingCandidate(index, candidate)"
              >
                <span>
                  <strong>{{ candidate.targetColumn }}</strong>
                  <small>{{ candidate.matchMethod }} · {{ schemaMappingScoreLabel(candidate.score) }} · {{ candidate.reason }}</small>
                </span>
                <span v-if="candidate.acceptedEvidence || candidate.rejectedEvidence" class="status-pill">
                  {{ candidate.acceptedEvidence }}/{{ candidate.rejectedEvidence }}
                </span>
              </button>
            </div>
          </div>
          <button class="secondary-button compact-button" type="button" @click="addMapping">
            <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.addMapping') }}
          </button>
        </template>

        <template v-else-if="stepType === 'schema_completion'">
          <div v-for="(rule, index) in arrayConfig('rules')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <label class="field">
                <span>{{ t('dataPipelines.targetColumn') }}</span>
                <input :value="stringField(rule, 'targetColumn')" @input="updateArrayItem('rules', index, { targetColumn: targetValue($event) })">
              </label>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeCompletionRule', { index: index + 1 })" @click="removeArrayItem('rules', index)">
                <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>
            <label class="field">
              <span>{{ t('dataPipelines.method') }}</span>
              <select :value="stringField(rule, 'method') || 'literal'" @change="updateArrayItem('rules', index, { method: targetValue($event) })">
                <option v-for="method in completionMethods" :key="method" :value="method">{{ completionMethodLabel(method) }}</option>
              </select>
            </label>
            <label v-if="['literal'].includes(stringField(rule, 'method') || 'literal')" class="field">
              <span>{{ t('dataPipelines.value') }}</span>
              <input :value="scalarToInput(rule.value)" @input="updateArrayScalarField('rules', index, 'value', targetValue($event))">
            </label>
            <label v-if="stringField(rule, 'method') === 'copy_column'" class="field">
              <span>{{ t('dataPipelines.sourceColumn') }}</span>
              <input :value="stringField(rule, 'sourceColumn')" list="data-pipeline-column-options" @input="updateArrayItem('rules', index, { sourceColumn: targetValue($event) })">
            </label>
            <label v-if="['coalesce', 'concat'].includes(stringField(rule, 'method'))" class="field">
              <span>{{ t('dataPipelines.sourceColumns') }}</span>
              <input :value="listToInput(rule.sourceColumns)" list="data-pipeline-column-options" @input="updateArrayListField('rules', index, 'sourceColumns', targetValue($event))">
            </label>
            <label v-if="['coalesce', 'case_when'].includes(stringField(rule, 'method'))" class="field">
              <span>{{ t('dataPipelines.defaultValue') }}</span>
              <input :value="scalarToInput(rule.defaultValue)" @input="updateArrayScalarField('rules', index, 'defaultValue', targetValue($event))">
            </label>
          </div>
          <button class="secondary-button compact-button" type="button" @click="addStepRule">
            <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.addRule') }}
          </button>
        </template>

        <template v-else-if="stepType === 'join' || stepType === 'enrich_join'">
          <div class="config-grid">
            <label v-if="stepType === 'enrich_join'" class="field">
              <span>{{ t('dataPipelines.rightSourceKind') }}</span>
              <select :value="sourceKind(true)" @change="setSourceKind(targetValue($event), true)">
                <option v-for="kind in sourceKindOptions(true)" :key="kind.value" :value="kind.value">{{ t(kind.labelKey) }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.joinType') }}</span>
              <select :value="stringConfig('joinType') || 'left'" @change="updateConfigField('joinType', targetValue($event))">
                <option v-for="joinType in joinTypes" :key="joinType" :value="joinType">{{ optionLabel(`dataPipelines.joinTypeValue.${joinType}`, joinType) }}</option>
              </select>
            </label>
            <label v-if="!crossJoinSelected" class="field">
              <span>{{ t('dataPipelines.joinStrictness') }}</span>
              <select :value="stringConfig('joinStrictness') || 'all'" @change="updateConfigField('joinStrictness', targetValue($event))">
                <option v-for="strictness in joinStrictnesses" :key="strictness" :value="strictness">{{ optionLabel(`dataPipelines.joinStrictnessValue.${strictness}`, strictness) }}</option>
              </select>
            </label>
          </div>
          <label v-if="stepType === 'enrich_join'" class="field">
            <span>{{ sourceSelectLabel(true) }}</span>
            <select :value="sourcePublicId(true)" @change="setSourcePublicId(targetValue($event), true)">
              <option value="">{{ t('dataPipelines.selectSource') }}</option>
              <option
                v-for="item in sourceOptions(sourceKind(true))"
                :key="sourceOptionValue(item)"
                :value="sourceOptionValue(item)"
              >
                {{ sourceOptionLabel(item) }}
              </option>
            </select>
          </label>
          <div v-if="!crossJoinSelected" class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.leftKeys') }}</span>
              <input :value="listToInput(configDraft.leftKeys)" list="data-pipeline-column-options" @input="updateConfigField('leftKeys', parseListInput(targetValue($event)))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.rightKeys') }}</span>
              <input :value="listToInput(configDraft.rightKeys)" list="data-pipeline-right-column-options" @input="updateConfigField('rightKeys', parseListInput(targetValue($event)))">
            </label>
          </div>
          <div v-if="rightColumns.length > 0" class="config-rule">
            <span>{{ t('dataPipelines.selectColumns') }}</span>
            <label v-for="column in rightColumns" :key="column" class="checkbox-field">
              <input :checked="joinSelectColumnChecked(column)" type="checkbox" @change="toggleJoinSelectColumn(column, targetChecked($event))">
              <span>{{ column }}</span>
            </label>
          </div>
          <label v-else class="field">
            <span>{{ t('dataPipelines.selectColumns') }}</span>
            <input :value="listToInput(configDraft.selectColumns)" list="data-pipeline-right-column-options" @input="updateConfigField('selectColumns', parseListInput(targetValue($event)))">
          </label>
        </template>

        <template v-else-if="stepType === 'transform'">
          <label class="field">
            <span>{{ t('dataPipelines.operation') }}</span>
            <select :value="stringConfig('operation') || 'select_columns'" @change="updateConfigField('operation', targetValue($event))">
              <option v-for="operation in transformOperations" :key="operation" :value="operation">{{ transformOperationLabel(operation) }}</option>
            </select>
          </label>

          <label v-if="['select_columns', 'drop_columns'].includes(stringConfig('operation') || 'select_columns')" class="field">
            <span>{{ t('dataPipelines.columns') }}</span>
            <input :value="listToInput(configDraft.columns)" list="data-pipeline-column-options" @input="updateConfigField('columns', parseListInput(targetValue($event)))">
          </label>

          <template v-else-if="stringConfig('operation') === 'rename_columns'">
            <div v-for="(entry, index) in objectEntries('renames')" :key="`${entry.source}-${index}`" class="config-rule">
              <div class="config-grid with-remove">
                <label class="field">
                  <span>{{ t('dataPipelines.sourceColumn') }}</span>
                  <input :value="entry.source" list="data-pipeline-column-options" @input="updateObjectEntrySource('renames', index, targetValue($event))">
                </label>
                <label class="field">
                  <span>{{ t('dataPipelines.targetColumn') }}</span>
                  <input :value="entry.target" @input="updateObjectEntryTarget('renames', index, targetValue($event))">
                </label>
                <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeRename', { index: index + 1 })" @click="removeObjectEntry('renames', index)">
                  <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
                </button>
              </div>
            </div>
            <button class="secondary-button compact-button" type="button" @click="addObjectEntry('renames')">
              <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
              {{ t('dataPipelines.addRename') }}
            </button>
          </template>

          <template v-else-if="stringConfig('operation') === 'filter'">
            <div v-for="(condition, index) in arrayConfig('conditions')" :key="index" class="config-rule">
              <div class="config-rule-header">
                <label class="field">
                  <span>{{ t('dataPipelines.column') }}</span>
                  <input :value="stringField(condition, 'column')" list="data-pipeline-column-options" @input="updateArrayItem('conditions', index, { column: targetValue($event) })">
                </label>
                <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeFilterCondition', { index: index + 1 })" @click="removeArrayItem('conditions', index)">
                  <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
                </button>
              </div>
              <div class="config-grid">
                <label class="field">
                  <span>{{ t('dataPipelines.operator') }}</span>
                  <select :value="stringField(condition, 'operator')" @change="updateArrayItem('conditions', index, { operator: targetValue($event) })">
                    <option v-for="operator in conditionOperators" :key="operator" :value="operator">{{ conditionOperatorLabel(operator) }}</option>
                  </select>
                </label>
                <label class="field">
                  <span>{{ t('dataPipelines.value') }}</span>
                  <input :value="scalarToInput(condition.value)" @input="updateConditionValue('conditions', index, targetValue($event))">
                </label>
              </div>
            </div>
            <button class="secondary-button compact-button" type="button" @click="addTransformCondition">
              <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
              {{ t('dataPipelines.addCondition') }}
            </button>
          </template>

          <template v-else-if="stringConfig('operation') === 'sort'">
            <div v-for="(sort, index) in arrayConfig('sorts')" :key="index" class="config-rule">
              <div class="config-grid with-remove">
                <label class="field">
                  <span>{{ t('dataPipelines.column') }}</span>
                  <input :value="stringField(sort, 'column')" list="data-pipeline-column-options" @input="updateArrayItem('sorts', index, { column: targetValue($event) })">
                </label>
                <label class="field">
                  <span>{{ t('dataPipelines.direction') }}</span>
                  <select :value="stringField(sort, 'direction') || 'ASC'" @change="updateArrayItem('sorts', index, { direction: targetValue($event) })">
                    <option value="ASC">{{ optionLabel('dataPipelines.sortDirection.ASC', 'ASC') }}</option>
                    <option value="DESC">{{ optionLabel('dataPipelines.sortDirection.DESC', 'DESC') }}</option>
                  </select>
                </label>
                <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeSort', { index: index + 1 })" @click="removeArrayItem('sorts', index)">
                  <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
                </button>
              </div>
            </div>
            <button class="secondary-button compact-button" type="button" @click="addSort">
              <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
              {{ t('dataPipelines.addSort') }}
            </button>
          </template>

          <template v-else-if="stringConfig('operation') === 'aggregate'">
            <label class="field">
              <span>{{ t('dataPipelines.groupBy') }}</span>
              <input :value="listToInput(configDraft.groupBy)" list="data-pipeline-column-options" @input="updateConfigField('groupBy', parseListInput(targetValue($event)))">
            </label>
            <div v-for="(aggregation, index) in arrayConfig('aggregations')" :key="index" class="config-rule">
              <div class="config-rule-header">
                <label class="field">
                  <span>{{ t('dataPipelines.function') }}</span>
                  <select :value="stringField(aggregation, 'function') || 'count'" @change="updateArrayItem('aggregations', index, { function: targetValue($event) })">
                    <option v-for="fn in aggregateFunctions" :key="fn" :value="fn">{{ optionLabel(`dataPipelines.aggregateFunction.${fn}`, fn) }}</option>
                  </select>
                </label>
                <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeAggregation', { index: index + 1 })" @click="removeArrayItem('aggregations', index)">
                  <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
                </button>
              </div>
              <div class="config-grid">
                <label class="field">
                  <span>{{ t('dataPipelines.column') }}</span>
                  <input :value="stringField(aggregation, 'column')" list="data-pipeline-column-options" @input="updateArrayItem('aggregations', index, { column: targetValue($event) })">
                </label>
                <label class="field">
                  <span>{{ t('dataPipelines.alias') }}</span>
                  <input :value="stringField(aggregation, 'alias')" @input="updateArrayItem('aggregations', index, { alias: targetValue($event) })">
                </label>
              </div>
            </div>
            <button class="secondary-button compact-button" type="button" @click="addAggregation">
              <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
              {{ t('dataPipelines.addAggregation') }}
            </button>
          </template>
        </template>

        <template v-else-if="stepType === 'extract_text'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.chunkMode') }}</span>
              <select :value="stringConfig('chunkMode') || 'page'" @change="updateConfigField('chunkMode', targetValue($event))">
                <option value="page">{{ t('dataPipelines.chunkModePage') }}</option>
                <option value="full_text">{{ t('dataPipelines.chunkModeFullText') }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.ocrEngine') }}</span>
              <input :value="stringConfig('ocrEngine')" placeholder="tesseract" @input="updateConfigOptionalString('ocrEngine', targetValue($event))">
            </label>
          </div>
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.languages') }}</span>
              <input :value="listToInput(configDraft.languages)" placeholder="jpn, eng" @input="updateConfigList('languages', targetValue($event))">
            </label>
            <label class="data-pipeline-toggle">
              <input :checked="Boolean(configDraft.includeBoxes)" type="checkbox" @change="updateConfigField('includeBoxes', targetChecked($event))">
              <span>{{ t('dataPipelines.includeBoxes') }}</span>
            </label>
          </div>
          <label class="field">
            <span>{{ t('dataPipelines.orderBy') }}</span>
            <input :value="stringList(configDraft.orderBy).join(', ')" @input="updateConfigList('orderBy', targetValue($event))">
          </label>
        </template>

        <template v-else-if="stepType === 'classify_document'">
          <div v-for="(docClass, index) in arrayConfig('classes')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <label class="field">
                <span>{{ t('dataPipelines.classLabel') }}</span>
                <input :value="stringField(docClass, 'label')" @input="updateArrayItem('classes', index, { label: targetValue($event) })">
              </label>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeClass', { index: index + 1 })" @click="removeArrayItem('classes', index)">
                <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>
            <div class="config-grid">
              <label class="field">
                <span>{{ t('dataPipelines.keywords') }}</span>
                <input :value="listToInput(docClass.keywords)" @input="updateArrayListField('classes', index, 'keywords', targetValue($event))">
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.regexes') }}</span>
                <input :value="listToInput(docClass.regexes)" @input="updateArrayListField('classes', index, 'regexes', targetValue($event))">
              </label>
            </div>
          </div>
          <button class="secondary-button compact-button" type="button" @click="addArrayItem('classes', { label: '', keywords: [], regexes: [] })">
            <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.addClass') }}
          </button>
        </template>

        <template v-else-if="stepType === 'extract_fields'">
          <div v-for="(field, index) in arrayConfig('fields')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <label class="field">
                <span>{{ t('dataPipelines.fieldName') }}</span>
                <input :value="stringField(field, 'name')" @input="updateArrayItem('fields', index, { name: targetValue($event) })">
              </label>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeField', { index: index + 1 })" @click="removeArrayItem('fields', index)">
                <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>
            <div class="config-grid">
              <label class="field">
                <span>{{ t('dataPipelines.type') }}</span>
                <select :value="stringField(field, 'type') || 'string'" @change="updateArrayItem('fields', index, { type: targetValue($event) })">
                  <option v-for="fieldType in fieldTypes" :key="fieldType" :value="fieldType">{{ fieldType }}</option>
                </select>
              </label>
              <label class="data-pipeline-toggle">
                <input :checked="boolField(field, 'required')" type="checkbox" @change="updateArrayItem('fields', index, { required: targetChecked($event) })">
                <span>{{ t('dataPipelines.required') }}</span>
              </label>
            </div>
            <label class="field">
              <span>{{ t('dataPipelines.patterns') }}</span>
              <input :value="listToInput(field.patterns)" @input="updateArrayListField('fields', index, 'patterns', targetValue($event))">
            </label>
          </div>
          <button class="secondary-button compact-button" type="button" @click="addArrayItem('fields', { name: '', type: 'string', required: false, patterns: [] })">
            <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.addField') }}
          </button>
        </template>

        <template v-else-if="stepType === 'extract_table'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.source') }}</span>
              <select :value="stringConfig('source') || 'text_delimited'" @change="updateConfigField('source', targetValue($event))">
                <option value="text_delimited">{{ t('dataPipelines.textDelimited') }}</option>
                <option value="ocr_layout">{{ t('dataPipelines.ocrLayout') }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.delimiter') }}</span>
              <input :value="stringConfig('delimiter') || ','" @input="updateConfigField('delimiter', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.expectedColumnCount') }}</span>
              <input :value="stringConfig('expectedColumnCount')" inputmode="numeric" placeholder="4" @input="updateConfigOptionalNumber('expectedColumnCount', targetValue($event))">
            </label>
          </div>
        </template>

        <template v-else-if="stepType === 'confidence_gate'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.threshold') }}</span>
              <input :value="numberField(configDraft, 'threshold') || '0.8'" type="number" min="0" max="1" step="0.01" @input="updateConfigField('threshold', Number(targetValue($event)))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.mode') }}</span>
              <select :value="stringConfig('mode') || 'annotate'" @change="updateConfigField('mode', targetValue($event))">
                <option value="annotate">{{ t('dataPipelines.annotate') }}</option>
                <option value="filter_pass">{{ t('dataPipelines.filterPass') }}</option>
              </select>
            </label>
          </div>
          <label class="field">
            <span>{{ t('dataPipelines.scoreColumns') }}</span>
            <input :value="listToInput(configDraft.scoreColumns)" placeholder="confidence, field_confidence, document_confidence" @input="updateConfigList('scoreColumns', targetValue($event))">
          </label>
        </template>

        <template v-else-if="stepType === 'quarantine'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.statusColumn') }}</span>
              <input :value="stringConfig('statusColumn') || 'gate_status'" list="data-pipeline-column-options" @input="updateConfigField('statusColumn', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.outputMode') }}</span>
              <select :value="stringConfig('outputMode') || 'quarantine_only'" @change="updateConfigField('outputMode', targetValue($event))">
                <option value="quarantine_only">{{ t('dataPipelines.quarantineOnly') }}</option>
                <option value="pass_only">{{ t('dataPipelines.passOnly') }}</option>
              </select>
            </label>
          </div>
          <label class="field">
            <span>{{ t('dataPipelines.matchValues') }}</span>
            <input :value="listToInput(configDraft.matchValues)" placeholder="needs_review" @input="updateConfigList('matchValues', targetValue($event))">
          </label>
        </template>

        <template v-else-if="stepType === 'deduplicate'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.keyColumns') }}</span>
              <input :value="listToInput(configDraft.keyColumns)" list="data-pipeline-column-options" @input="updateConfigList('keyColumns', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.mode') }}</span>
              <select :value="stringConfig('mode') || 'annotate'" @change="updateConfigField('mode', targetValue($event))">
                <option value="annotate">{{ t('dataPipelines.annotate') }}</option>
                <option value="keep_first">{{ t('dataPipelines.keepFirst') }}</option>
              </select>
            </label>
          </div>
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.statusColumn') }}</span>
              <input :value="stringConfig('statusColumn')" @input="updateConfigField('statusColumn', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.groupColumn') }}</span>
              <input :value="stringConfig('groupColumn')" @input="updateConfigField('groupColumn', targetValue($event))">
            </label>
          </div>
        </template>

        <template v-else-if="stepType === 'canonicalize'">
          <div v-for="(rule, index) in arrayConfig('rules')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <label class="field">
                <span>{{ t('dataPipelines.column') }}</span>
                <input :value="stringField(rule, 'column')" list="data-pipeline-column-options" @input="updateArrayItem('rules', index, { column: targetValue($event) })">
              </label>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeRule', { index: index + 1 })" @click="removeArrayItem('rules', index)">
                <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>
            <label class="field">
              <span>{{ t('dataPipelines.outputColumn') }}</span>
              <input :value="stringField(rule, 'outputColumn')" @input="updateArrayItem('rules', index, { outputColumn: targetValue($event) })">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.operations') }}</span>
              <input :value="listToInput(rule.operations)" :placeholder="canonicalizeOperations.join(', ')" @input="updateArrayListField('rules', index, 'operations', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.mappingsJson') }}</span>
              <textarea :value="jsonForField(rule.mappings)" rows="4" spellcheck="false" @input="updateArrayObjectFromJson('rules', index, 'mappings', targetValue($event))" />
            </label>
          </div>
          <button class="secondary-button compact-button" type="button" @click="addArrayItem('rules', { column: '', outputColumn: '', operations: ['trim', 'normalize_spaces'], mappings: {} })">
            <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.addRule') }}
          </button>
        </template>

        <template v-else-if="stepType === 'redact_pii'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.columns') }}</span>
              <input :value="listToInput(configDraft.columns)" list="data-pipeline-column-options" @input="updateConfigList('columns', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.piiTypes') }}</span>
              <input :value="listToInput(configDraft.types)" :placeholder="piiTypes.join(', ')" @input="updateConfigList('types', targetValue($event))">
            </label>
          </div>
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.mode') }}</span>
              <select :value="stringConfig('mode') || 'mask'" @change="updateConfigField('mode', targetValue($event))">
                <option value="mask">{{ t('dataPipelines.mask') }}</option>
                <option value="remove">{{ t('dataPipelines.remove') }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.outputSuffix') }}</span>
              <input :value="stringConfig('outputSuffix') || '_redacted'" @input="updateConfigField('outputSuffix', targetValue($event))">
            </label>
          </div>
        </template>

        <template v-else-if="stepType === 'detect_language_encoding'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.textColumn') }}</span>
              <input :value="stringConfig('textColumn') || 'text'" list="data-pipeline-column-options" @input="updateConfigField('textColumn', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.outputTextColumn') }}</span>
              <input :value="stringConfig('outputTextColumn') || 'normalized_text'" @input="updateConfigField('outputTextColumn', targetValue($event))">
            </label>
          </div>
        </template>

        <template v-else-if="stepType === 'schema_inference'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.columns') }}</span>
              <input :value="listToInput(configDraft.columns)" list="data-pipeline-column-options" @input="updateConfigList('columns', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.sampleLimit') }}</span>
              <input :value="numberField(configDraft, 'sampleLimit') || '1000'" type="number" min="1" step="1" @input="updateConfigField('sampleLimit', Number(targetValue($event)))">
            </label>
          </div>
        </template>

        <template v-else-if="stepType === 'entity_resolution'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.column') }}</span>
              <input :value="stringConfig('column')" list="data-pipeline-column-options" @input="updateConfigField('column', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.outputPrefix') }}</span>
              <input :value="stringConfig('outputPrefix')" @input="updateConfigField('outputPrefix', targetValue($event))">
            </label>
          </div>
          <label class="field">
            <span>{{ t('dataPipelines.dictionaryJson') }}</span>
            <textarea :value="jsonForField(configDraft.dictionary, [])" rows="5" spellcheck="false" @input="updateConfigArrayFromJson('dictionary', targetValue($event))" />
          </label>
        </template>

        <template v-else-if="stepType === 'unit_conversion'">
          <div v-for="(rule, index) in arrayConfig('rules')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <label class="field">
                <span>{{ t('dataPipelines.valueColumn') }}</span>
                <input :value="stringField(rule, 'valueColumn')" list="data-pipeline-column-options" @input="updateArrayItem('rules', index, { valueColumn: targetValue($event) })">
              </label>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeRule', { index: index + 1 })" @click="removeArrayItem('rules', index)">
                <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>
            <div class="config-grid">
              <label class="field">
                <span>{{ t('dataPipelines.unitColumn') }}</span>
                <input :value="stringField(rule, 'unitColumn')" list="data-pipeline-column-options" @input="updateArrayItem('rules', index, { unitColumn: targetValue($event) })">
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.outputUnit') }}</span>
                <input :value="stringField(rule, 'outputUnit')" @input="updateArrayItem('rules', index, { outputUnit: targetValue($event) })">
              </label>
            </div>
            <label class="field">
              <span>{{ t('dataPipelines.conversionsJson') }}</span>
              <textarea :value="jsonForField(rule.conversions, [])" rows="4" spellcheck="false" @input="updateArrayArrayFromJson('rules', index, 'conversions', targetValue($event))" />
            </label>
          </div>
          <button class="secondary-button compact-button" type="button" @click="addArrayItem('rules', { valueColumn: '', unitColumn: '', outputUnit: '', conversions: [] })">
            <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.addRule') }}
          </button>
        </template>

        <template v-else-if="stepType === 'relationship_extraction'">
          <label class="field">
            <span>{{ t('dataPipelines.textColumn') }}</span>
            <input :value="stringConfig('textColumn') || 'text'" list="data-pipeline-column-options" @input="updateConfigField('textColumn', targetValue($event))">
          </label>
          <div v-for="(pattern, index) in arrayConfig('patterns')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <label class="field">
                <span>{{ t('dataPipelines.relationType') }}</span>
                <input :value="stringField(pattern, 'relationType')" @input="updateArrayItem('patterns', index, { relationType: targetValue($event) })">
              </label>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeRule', { index: index + 1 })" @click="removeArrayItem('patterns', index)">
                <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>
            <label class="field">
              <span>{{ t('dataPipelines.pattern') }}</span>
              <input :value="stringField(pattern, 'pattern')" @input="updateArrayItem('patterns', index, { pattern: targetValue($event) })">
            </label>
          </div>
          <button class="secondary-button compact-button" type="button" @click="addArrayItem('patterns', { relationType: 'related_to', pattern: '' })">
            <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.addRule') }}
          </button>
        </template>

        <template v-else-if="stepType === 'human_review'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.reasonColumns') }}</span>
              <input :value="listToInput(configDraft.reasonColumns)" list="data-pipeline-column-options" @input="updateConfigList('reasonColumns', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.mode') }}</span>
              <select :value="stringConfig('mode') || 'annotate'" @change="updateConfigField('mode', targetValue($event))">
                <option value="annotate">{{ t('dataPipelines.annotate') }}</option>
                <option value="filter_review">{{ t('dataPipelines.filterReview') }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.queue') }}</span>
              <input :value="stringConfig('queue') || 'default'" @input="updateConfigField('queue', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.reviewItemLimit') }}</span>
              <input :value="stringConfig('reviewItemLimit') || '1000'" inputmode="numeric" @input="updateConfigOptionalNumber('reviewItemLimit', targetValue($event))">
            </label>
            <label class="data-pipeline-toggle">
              <input :checked="Boolean(configDraft.createReviewItems)" type="checkbox" @change="updateConfigField('createReviewItems', targetChecked($event))">
              <span>{{ t('dataPipelines.createReviewItems') }}</span>
            </label>
          </div>
        </template>

        <template v-else-if="stepType === 'route_by_condition'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.routeColumn') }}</span>
              <input :value="stringConfig('routeColumn') || 'route_key'" @input="updateConfigField('routeColumn', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.defaultRoute') }}</span>
              <input :value="stringConfig('defaultRoute') || 'default'" @input="updateConfigField('defaultRoute', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.mode') }}</span>
              <select :value="stringConfig('mode') || 'annotate'" @change="updateConfigField('mode', targetValue($event))">
                <option value="annotate">{{ t('dataPipelines.annotate') }}</option>
                <option value="filter_route">{{ t('dataPipelines.filterRoute') }}</option>
              </select>
            </label>
            <label v-if="stringConfig('mode') === 'filter_route'" class="field">
              <span>{{ t('dataPipelines.selectedRoute') }}</span>
              <input :value="stringConfig('route')" @input="updateConfigField('route', targetValue($event))">
            </label>
          </div>
          <div v-for="(rule, index) in arrayConfig('rules')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <label class="field">
                <span>{{ t('dataPipelines.route') }}</span>
                <input :value="stringField(rule, 'route')" @input="updateArrayItem('rules', index, { route: targetValue($event) })">
              </label>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeRule', { index: index + 1 })" @click="removeArrayItem('rules', index)">
                <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>
            <div class="config-grid">
              <label class="field">
                <span>{{ t('dataPipelines.column') }}</span>
                <input :value="stringField(rule, 'column')" list="data-pipeline-column-options" @input="updateArrayItem('rules', index, { column: targetValue($event) })">
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.operator') }}</span>
                <select :value="stringField(rule, 'operator') || '='" @change="updateArrayItem('rules', index, { operator: targetValue($event) })">
                  <option v-for="operator in conditionOperators" :key="operator" :value="operator">{{ conditionOperatorLabel(operator) }}</option>
                </select>
              </label>
            </div>
            <label v-if="stringField(rule, 'operator') !== 'required'" class="field">
              <span>{{ t('dataPipelines.value') }}</span>
              <input :value="listToInput(rule.value)" @input="updateConditionValue('rules', index, targetValue($event))">
            </label>
          </div>
          <button class="secondary-button compact-button" type="button" @click="addArrayItem('rules', { column: '', operator: '=', value: '', route: '' })">
            <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.addRule') }}
          </button>
        </template>

        <template v-else-if="stepType === 'union'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.columns') }}</span>
              <input :value="listToInput(configDraft.columns)" list="data-pipeline-column-options" @input="updateConfigList('columns', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.sourceLabelColumn') }}</span>
              <input :value="stringConfig('sourceLabelColumn')" @input="updateConfigOptionalString('sourceLabelColumn', targetValue($event))">
            </label>
          </div>
        </template>

        <template v-else-if="stepType === 'partition_filter'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.dateColumn') }}</span>
              <input :value="stringConfig('dateColumn') || 'updated_at'" list="data-pipeline-column-options" @input="updateConfigField('dateColumn', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.valueType') }}</span>
              <select :value="stringConfig('valueType') || 'datetime'" @change="updateConfigField('valueType', targetValue($event))">
                <option value="datetime">{{ t('dataPipelines.datetime') }}</option>
                <option value="number">{{ t('dataPipelines.number') }}</option>
                <option value="string">{{ t('dataPipelines.string') }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.start') }}</span>
              <input :value="stringConfig('start')" @input="updateConfigOptionalString('start', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.end') }}</span>
              <input :value="stringConfig('end')" @input="updateConfigOptionalString('end', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.partitionKey') }}</span>
              <input :value="stringConfig('partitionKey')" list="data-pipeline-column-options" @input="updateConfigOptionalString('partitionKey', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.partitionValue') }}</span>
              <input :value="stringConfig('partitionValue')" @input="updateConfigOptionalString('partitionValue', targetValue($event))">
            </label>
            <label class="data-pipeline-toggle">
              <input :checked="Boolean(configDraft.includeEnd)" type="checkbox" @change="updateConfigField('includeEnd', targetChecked($event))">
              <span>{{ t('dataPipelines.includeEnd') }}</span>
            </label>
          </div>
        </template>

        <template v-else-if="stepType === 'watermark_filter'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.watermarkColumn') }}</span>
              <input :value="stringConfig('column') || 'updated_at'" list="data-pipeline-column-options" @input="updateConfigField('column', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.watermarkSource') }}</span>
              <select :value="stringConfig('watermarkSource') || 'fixed'" @change="updateConfigField('watermarkSource', targetValue($event))">
                <option value="fixed">{{ t('dataPipelines.fixedWatermark') }}</option>
                <option value="previous_success">{{ t('dataPipelines.previousSuccessWatermark') }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.watermarkValue') }}</span>
              <input :value="stringConfig('watermarkValue')" @input="updateConfigOptionalString('watermarkValue', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.valueType') }}</span>
              <select :value="stringConfig('valueType') || 'datetime'" @change="updateConfigField('valueType', targetValue($event))">
                <option value="datetime">{{ t('dataPipelines.datetime') }}</option>
                <option value="number">{{ t('dataPipelines.number') }}</option>
                <option value="string">{{ t('dataPipelines.string') }}</option>
              </select>
            </label>
            <label class="data-pipeline-toggle">
              <input :checked="Boolean(configDraft.inclusive)" type="checkbox" @change="updateConfigField('inclusive', targetChecked($event))">
              <span>{{ t('dataPipelines.inclusive') }}</span>
            </label>
          </div>
        </template>

        <template v-else-if="stepType === 'sample_compare'">
          <div v-for="(pair, index) in arrayConfig('pairs')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <label class="field">
                <span>{{ t('dataPipelines.fieldName') }}</span>
                <input :value="stringField(pair, 'field')" @input="updateArrayItem('pairs', index, { field: targetValue($event) })">
              </label>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeRule', { index: index + 1 })" @click="removeArrayItem('pairs', index)">
                <Trash2 :size="15" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>
            <div class="config-grid">
              <label class="field">
                <span>{{ t('dataPipelines.beforeColumn') }}</span>
                <input :value="stringField(pair, 'beforeColumn')" list="data-pipeline-column-options" @input="updateArrayItem('pairs', index, { beforeColumn: targetValue($event) })">
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.afterColumn') }}</span>
                <input :value="stringField(pair, 'afterColumn')" list="data-pipeline-column-options" @input="updateArrayItem('pairs', index, { afterColumn: targetValue($event) })">
              </label>
            </div>
          </div>
          <button class="secondary-button compact-button" type="button" @click="addArrayItem('pairs', { field: '', beforeColumn: '', afterColumn: '' })">
            <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.addRule') }}
          </button>
        </template>

        <template v-else-if="stepType === 'quality_report'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.columns') }}</span>
              <input :value="listToInput(configDraft.columns)" list="data-pipeline-column-options" @input="updateConfigList('columns', targetValue($event))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.outputMode') }}</span>
              <select :value="stringConfig('outputMode') || 'row_summary'" @change="updateConfigField('outputMode', targetValue($event))">
                <option value="row_summary">{{ t('dataPipelines.rowSummary') }}</option>
                <option value="dataset_summary">{{ t('dataPipelines.datasetSummary') }}</option>
              </select>
            </label>
          </div>
        </template>

        <template v-else-if="stepType === 'output'">
          <label class="field">
            <span>{{ t('dataPipelines.displayName') }}</span>
            <input :value="stringConfig('displayName')" @input="updateConfigField('displayName', targetValue($event))">
          </label>
          <label class="field">
            <span>{{ t('dataPipelines.tableName') }}</span>
            <input :value="stringConfig('tableName')" @input="updateConfigOptionalString('tableName', targetValue($event))">
          </label>
          <p v-if="outputTableNameError" class="inline-error">
            <AlertCircle :size="14" stroke-width="1.9" aria-hidden="true" />
            {{ outputTableNameError }}
          </p>
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.writeMode') }}</span>
              <select :value="stringConfig('writeMode') || 'replace'" @change="updateConfigField('writeMode', targetValue($event))">
                <option value="replace">{{ optionLabel('dataPipelines.writeModeValue.replace', 'replace') }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.engine') }}</span>
              <select :value="stringConfig('engine') || 'MergeTree'" @change="updateConfigField('engine', targetValue($event))">
                <option value="MergeTree">MergeTree</option>
              </select>
            </label>
          </div>
          <label class="field">
            <span>{{ t('dataPipelines.orderBy') }}</span>
            <input :value="stringList(configDraft.orderBy).join(', ')" list="data-pipeline-column-options" @input="updateConfigList('orderBy', targetValue($event))">
          </label>
          <div class="config-section-header">
            <h3>{{ t('dataPipelines.outputColumns') }}</h3>
            <button class="secondary-button compact-button" type="button" @click="addArrayItem('columns', { sourceColumn: '', name: '', type: 'string' })">
              <Plus :size="15" stroke-width="1.9" aria-hidden="true" />
              {{ t('dataPipelines.addOutputColumn') }}
            </button>
          </div>
          <p v-if="arrayConfig('columns').length === 0" class="cell-subtle">{{ t('dataPipelines.outputColumnsPassThrough') }}</p>
          <div v-for="(column, index) in arrayConfig('columns')" :key="index" class="config-rule">
            <div class="config-rule-header">
              <strong>{{ stringField(column, 'name') || stringField(column, 'sourceColumn') || `${t('dataPipelines.outputColumn')} ${index + 1}` }}</strong>
              <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeOutputColumn', { index: index + 1 })" @click="removeArrayItem('columns', index)">
                <Trash2 :size="14" stroke-width="1.9" aria-hidden="true" />
              </button>
            </div>
            <div class="config-grid two-column">
              <label class="field">
                <span>{{ t('dataPipelines.sourceColumn') }}</span>
                <input :value="stringField(column, 'sourceColumn')" list="data-pipeline-column-options" @input="updateArrayItem('columns', index, { sourceColumn: targetValue($event) })">
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.outputColumn') }}</span>
                <input :value="stringField(column, 'name')" @input="updateArrayItem('columns', index, { name: targetValue($event) })">
              </label>
              <label class="field">
                <span>{{ t('dataPipelines.type') }}</span>
                <select :value="stringField(column, 'type') || 'string'" @change="updateArrayItem('columns', index, { type: targetValue($event) })">
                  <option value="string">{{ t('dataPipelines.string') }}</option>
                  <option value="int64">Int64</option>
                  <option value="float64">Float64</option>
                  <option value="bool">Bool</option>
                  <option value="date">Date</option>
                  <option value="datetime">{{ t('dataPipelines.datetime') }}</option>
                </select>
              </label>
            </div>
          </div>
        </template>

        <p v-else class="muted-panel">{{ t('dataPipelines.noConfigFields') }}</p>
      </section>

      <p v-if="activeConfigTab === 'ui' && uiError" class="inline-error">
        <AlertCircle :size="14" stroke-width="1.9" aria-hidden="true" />
        {{ uiError }}
      </p>

      <div v-if="activeConfigTab === 'json'" class="config-json-panel" role="tabpanel">
        <label class="field config-json-field">
          <span>{{ t('dataPipelines.configJson') }}</span>
          <textarea :value="configText" spellcheck="false" rows="14" @input="onJsonInput" />
        </label>
        <p v-if="parseError" class="inline-error">
          <AlertCircle :size="14" stroke-width="1.9" aria-hidden="true" />
          {{ parseError }}
        </p>
        <button class="primary-button" type="button" @click="applyChanges">
          <Check :size="16" stroke-width="1.9" aria-hidden="true" />
        {{ t('common.apply') }}
        </button>
      </div>
    </div>

    <p v-else class="muted-panel">{{ t('dataPipelines.noNodeSelected') }}</p>
  </aside>
</template>
