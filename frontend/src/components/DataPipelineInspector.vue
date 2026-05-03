<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { AlertCircle, Check, MousePointerClick, Plus, Trash2, Zap } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { isDataPipelineAutoPreviewEnabled, type DataPipelineGraph, type DataPipelineStepType } from '../api/data-pipelines'
import type { DatasetBody, DatasetWorkTableBody } from '../api/generated/types.gen'

type ConfigRecord = Record<string, unknown>

const props = defineProps<{
  graph: DataPipelineGraph
  selectedNodeId: string
  datasets?: DatasetBody[]
  workTables?: DatasetWorkTableBody[]
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
const aggregateFunctions = ['count', 'sum', 'avg', 'min', 'max']
const fieldTypes = ['string', 'number', 'date', 'boolean', 'json']

const label = ref('')
const configText = ref('{}')
const configDraft = ref<ConfigRecord>({})
const parseError = ref('')
const uiError = ref('')
const activeConfigTab = ref<'ui' | 'json'>('ui')
let syncingLocalChange = false
const { t } = useI18n()

const selectedNode = computed(() => props.graph.nodes.find((node) => node.id === props.selectedNodeId) ?? null)
const stepType = computed(() => selectedNode.value?.data.stepType ?? '')
const autoPreviewEnabled = computed(() => isDataPipelineAutoPreviewEnabled(selectedNode.value?.data))
const previewModeTitle = computed(() => autoPreviewEnabled.value ? t('dataPipelines.autoPreview') : t('dataPipelines.manualPreviewReason'))
const previewModeIcon = computed(() => autoPreviewEnabled.value ? Zap : MousePointerClick)
const datasetOptions = computed(() => props.datasets ?? [])
const workTableOptions = computed(() => (props.workTables ?? []).filter((item) => Boolean(item.publicId)))
const primaryColumns = computed(() => sourceColumnsFromConfig(inputConfigForColumns(), 'sourceKind', 'datasetPublicId', 'workTablePublicId'))
const rightColumns = computed(() => sourceColumnsFromConfig(configDraft.value, 'rightSourceKind', 'rightDatasetPublicId', 'rightWorkTablePublicId'))
const knownColumns = computed(() => uniqueStrings([...primaryColumns.value, ...rightColumns.value]))

watch(selectedNode, (node) => {
  if (syncingLocalChange) {
    return
  }
  label.value = node?.data.label ?? ''
  configDraft.value = cloneConfig(node?.data.config ?? {})
  configText.value = JSON.stringify(configDraft.value, null, 2)
  parseError.value = ''
  uiError.value = ''
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
  setConfig({
    sourceKind: kind,
    datasetPublicId: kind === 'dataset' ? stringConfig('datasetPublicId') : '',
    workTablePublicId: kind === 'work_table' ? stringConfig('workTablePublicId') : '',
    filePublicIds: kind === 'drive_file' ? stringList(configDraft.value.filePublicIds) : [],
  })
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
  const publicId = stringValue(kind === 'work_table' ? config[workTableKey] : config[datasetKey])
  if (!publicId) {
    return []
  }
  if (kind === 'work_table') {
    return workTableOptions.value.find((item) => item.publicId === publicId)?.columns?.map((column) => column.columnName) ?? []
  }
  return datasetOptions.value.find((item) => item.publicId === publicId)?.columns?.map((column) => column.columnName) ?? []
}

function inputConfigForColumns() {
  if (stepType.value === 'input') {
    return configDraft.value
  }
  return props.graph.nodes.find((node) => node.data.stepType === 'input')?.data.config ?? {}
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
          <label v-else class="field">
            <span>{{ t('dataPipelines.filePublicIds') }}</span>
            <textarea :value="listToInput(configDraft.filePublicIds)" rows="4" :placeholder="t('dataPipelines.filePublicIdsPlaceholder')" @input="updateConfigList('filePublicIds', targetValue($event))" />
          </label>
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

        <template v-else-if="stepType === 'enrich_join'">
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.rightSourceKind') }}</span>
              <select :value="sourceKind(true)" @change="setSourceKind(targetValue($event), true)">
                <option v-for="kind in sourceKindOptions(true)" :key="kind.value" :value="kind.value">{{ t(kind.labelKey) }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.joinType') }}</span>
              <select :value="stringConfig('joinType') || 'left'" @change="updateConfigField('joinType', targetValue($event))">
                <option value="left">{{ optionLabel('dataPipelines.joinTypeValue.left', 'left') }}</option>
              </select>
            </label>
          </div>
          <label class="field">
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
          <div class="config-grid">
            <label class="field">
              <span>{{ t('dataPipelines.leftKeys') }}</span>
              <input :value="listToInput(configDraft.leftKeys)" list="data-pipeline-column-options" @input="updateConfigField('leftKeys', parseListInput(targetValue($event)))">
            </label>
            <label class="field">
              <span>{{ t('dataPipelines.rightKeys') }}</span>
              <input :value="listToInput(configDraft.rightKeys)" list="data-pipeline-right-column-options" @input="updateConfigField('rightKeys', parseListInput(targetValue($event)))">
            </label>
          </div>
          <label class="field">
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

        <template v-else-if="stepType === 'output'">
          <label class="field">
            <span>{{ t('dataPipelines.displayName') }}</span>
            <input :value="stringConfig('displayName')" @input="updateConfigField('displayName', targetValue($event))">
          </label>
          <label class="field">
            <span>{{ t('dataPipelines.tableName') }}</span>
            <input :value="stringConfig('tableName')" @input="updateConfigOptionalString('tableName', targetValue($event))">
          </label>
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
