<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { CalendarClock, Crown, Database, Download, FileDown, Link2, Pencil, RefreshCw, Table2, Trash2 } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { fetchManagedDatasetWorkTableSCD2History, workTableExportDownloadUrl } from '../api/datasets'
import type { DatasetLineageLevel, DatasetLineageSource, DatasetWorkTableExportFormat, DatasetWorkTableExportFrequency } from '../api/datasets'
import type { DatasetBody, DatasetGoldPublicationCreateBodyWritable, DatasetLineageBody, DatasetLineageGraphSaveBodyWritable, DatasetWorkTableBody, DatasetWorkTableExportBody, DatasetWorkTableExportScheduleBody, DatasetWorkTableExportScheduleCreateBodyWritable, DatasetWorkTableExportScheduleUpdateBodyWritable, DatasetWorkTablePreviewBody, DatasetWorkTableScd2HistoryBody, MedallionCatalogBody } from '../api/generated/types.gen'
import ConfirmActionDialog from './ConfirmActionDialog.vue'
import LineageCompactGraph from './LineageCompactGraph.vue'
import LineageFlowGraph from './LineageFlowGraph.vue'
import LineageTimeline from './LineageTimeline.vue'
import MedallionCatalogPanel from './MedallionCatalogPanel.vue'
import TextInputDialog from './TextInputDialog.vue'

type LineageGraphTab = 'flow' | 'compact'
type WorkTableDetailTab = 'overview' | 'lineage' | 'exports'
type SnapshotPreviewFilter = 'all' | 'current' | 'history'

const props = withDefaults(defineProps<{
  tables: DatasetWorkTableBody[]
  datasets: DatasetBody[]
  selectedTable: DatasetWorkTableBody | null
  preview: DatasetWorkTablePreviewBody | null
  exports: DatasetWorkTableExportBody[]
  schedules: DatasetWorkTableExportScheduleBody[]
  medallionCatalog: MedallionCatalogBody | null
  lineage: DatasetLineageBody | null
  lineageLoading: boolean
  lineageLevel: DatasetLineageLevel
  lineageSources: DatasetLineageSource[]
  lineageActionLoading: boolean
  lineageDraftSourceKind: 'parser' | 'manual'
  hasLineageDraft: boolean
  canPublishLineage: boolean
  loading: boolean
  previewLoading: boolean
  medallionLoading: boolean
  actionLoading: boolean
  errorMessage?: string
  title?: string
}>(), {
  errorMessage: '',
  title: '',
})

const emit = defineEmits<{
  refresh: []
  select: [table: DatasetWorkTableBody]
  register: [datasetPublicId: string]
  link: [datasetPublicId: string]
  rename: [table: string]
  truncate: []
  drop: []
  promote: [name: string]
  publishGold: [body: DatasetGoldPublicationCreateBodyWritable]
  export: [format: DatasetWorkTableExportFormat]
  createSchedule: [body: DatasetWorkTableExportScheduleCreateBodyWritable]
  updateSchedule: [schedulePublicId: string, body: DatasetWorkTableExportScheduleUpdateBodyWritable]
  disableSchedule: [schedulePublicId: string]
  refreshLineage: []
  setLineageLevel: [level: DatasetLineageLevel]
  toggleLineageSource: [source: DatasetLineageSource]
  saveLineageDraft: [body: DatasetLineageGraphSaveBodyWritable]
  publishLineageDraft: []
  rejectLineageDraft: []
}>()

const { d, n, t } = useI18n()

const previewColumns = computed(() => props.preview?.columns ?? [])
const previewRows = computed(() => props.preview?.previewRows ?? [])
const selectedColumns = computed(() => props.selectedTable?.columns ?? [])
const browserTitle = computed(() => props.title || t('datasets.workTables'))
const selectedDatasetPublicId = ref('')
const exportFormat = ref<DatasetWorkTableExportFormat>('csv')
const activeWorkTableDetailTab = ref<WorkTableDetailTab>('overview')
const activeLineageGraphTab = ref<LineageGraphTab>('flow')
const snapshotPreviewFilter = ref<SnapshotPreviewFilter>('all')
const snapshotHistoryKey = ref('')
const snapshotHistoryLoading = ref(false)
const snapshotHistoryError = ref('')
const snapshotHistory = ref<DatasetWorkTableScd2HistoryBody | null>(null)
const lineageEditMode = ref(false)
const editingSchedulePublicId = ref('')
const textDialog = ref<'rename' | 'promote' | 'gold' | ''>('')
const confirmDialog = ref<'truncate' | 'drop' | ''>('')
const defaultTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
const scheduleForm = reactive({
  format: 'csv' as DatasetWorkTableExportFormat,
  frequency: 'daily' as DatasetWorkTableExportFrequency,
  timezone: defaultTimezone,
  runTime: '03:00',
  weekday: 1,
  monthDay: 1,
  retentionDays: 7,
  enabled: true,
})

const exportFormatOptions: Array<{ value: DatasetWorkTableExportFormat, labelKey: string }> = [
  { value: 'csv', labelKey: 'datasets.exportFormatCsv' },
  { value: 'json', labelKey: 'datasets.exportFormatJsonLines' },
  { value: 'parquet', labelKey: 'datasets.exportFormatParquet' },
]
const scheduleFrequencyOptions: Array<{ value: DatasetWorkTableExportFrequency, labelKey: string }> = [
  { value: 'daily', labelKey: 'datasets.frequencyDaily' },
  { value: 'weekly', labelKey: 'datasets.frequencyWeekly' },
  { value: 'monthly', labelKey: 'datasets.frequencyMonthly' },
]
const weekdayOptions = [
  { value: 1, labelKey: 'datasets.weekdayMonday' },
  { value: 2, labelKey: 'datasets.weekdayTuesday' },
  { value: 3, labelKey: 'datasets.weekdayWednesday' },
  { value: 4, labelKey: 'datasets.weekdayThursday' },
  { value: 5, labelKey: 'datasets.weekdayFriday' },
  { value: 6, labelKey: 'datasets.weekdaySaturday' },
  { value: 7, labelKey: 'datasets.weekdaySunday' },
]
const monthDayOptions = Array.from({ length: 28 }, (_, index) => index + 1)
const snapshotRequiredColumns = ['valid_from', 'valid_to', 'is_current', 'change_hash']
const snapshotKeyCandidates = ['id', 'product_id', 'sku', 'file_public_id']
const selectedColumnNames = computed(() => selectedColumns.value.map((column) => column.columnName))
const selectedColumnNameSet = computed(() => new Set(selectedColumnNames.value))
const scd2Summary = computed(() => props.preview?.scd2Summary ?? null)
const hasSCD2Columns = computed(() => Boolean(scd2Summary.value?.detected) || snapshotRequiredColumns.every((column) => selectedColumnNameSet.value.has(column)))
const snapshotKeyColumn = computed(() => scd2Summary.value?.keyColumn || snapshotKeyCandidates.find((column) => selectedColumnNameSet.value.has(column)) || '')
const snapshotPreviewFilters: Array<{ value: SnapshotPreviewFilter, labelKey: string }> = [
  { value: 'all', labelKey: 'datasets.snapshotPreviewFilterAll' },
  { value: 'current', labelKey: 'datasets.snapshotPreviewFilterCurrent' },
  { value: 'history', labelKey: 'datasets.snapshotPreviewFilterHistory' },
]
const currentSnapshotPreviewRows = computed(() => previewRows.value.filter(isCurrentSnapshotRow))
const historySnapshotPreviewRows = computed(() => previewRows.value.filter((row) => !isCurrentSnapshotRow(row)))
const snapshotTotalRows = computed(() => scd2Summary.value?.totalRows ?? previewRows.value.length)
const snapshotCurrentRows = computed(() => scd2Summary.value?.currentRows ?? currentSnapshotPreviewRows.value.length)
const snapshotHistoryRows = computed(() => scd2Summary.value?.historyRows ?? historySnapshotPreviewRows.value.length)
const snapshotKeyCount = computed(() => scd2Summary.value?.keyCount ?? 0)
const snapshotHistoryColumns = computed(() => snapshotHistory.value?.columns ?? [])
const snapshotHistoryResultRows = computed(() => snapshotHistory.value?.historyRows ?? [])
const filteredPreviewRows = computed(() => {
  if (!hasSCD2Columns.value) {
    return previewRows.value
  }
  if (snapshotPreviewFilter.value === 'current') {
    return currentSnapshotPreviewRows.value
  }
  if (snapshotPreviewFilter.value === 'history') {
    return historySnapshotPreviewRows.value
  }
  return previewRows.value
})
const textDialogTitle = computed(() => {
  if (textDialog.value === 'rename') {
    return t('datasets.renameWorkTable')
  }
  if (textDialog.value === 'gold') {
    return t('datasets.publishGoldPublication')
  }
  return t('datasets.promoteWorkTable')
})
const textDialogLabel = computed(() => textDialog.value === 'rename' ? t('datasets.newTableName') : t('datasets.displayName'))
const textDialogInitialValue = computed(() => textDialog.value === 'rename'
  ? props.selectedTable?.table ?? ''
  : props.selectedTable?.displayName ?? props.selectedTable?.table ?? '')
const textDialogConfirmLabel = computed(() => {
  if (textDialog.value === 'rename') {
    return t('common.rename')
  }
  if (textDialog.value === 'gold') {
    return t('datasets.publishGold')
  }
  return t('datasets.promote')
})

watch(
  () => [props.selectedTable?.originDatasetPublicId, props.datasets.length] as const,
  () => {
    selectedDatasetPublicId.value = props.selectedTable?.originDatasetPublicId || props.datasets[0]?.publicId || ''
  },
  { immediate: true },
)

watch(
  () => props.selectedTable ? props.selectedTable.publicId || `${props.selectedTable.database}.${props.selectedTable.table}` : '',
  () => {
    activeWorkTableDetailTab.value = 'overview'
    snapshotPreviewFilter.value = 'all'
    snapshotHistoryKey.value = ''
    snapshotHistoryError.value = ''
    snapshotHistory.value = null
    resetScheduleForm()
  },
)

function sameWorkTable(item: DatasetWorkTableBody) {
  if (item.publicId && props.selectedTable?.publicId) {
    return item.publicId === props.selectedTable.publicId
  }
  return item.database === props.selectedTable?.database && item.table === props.selectedTable?.table
}

function formatDate(value?: string) {
  return value ? d(new Date(value), 'long') : '-'
}

function formatBytes(value?: number | null) {
  const bytes = value ?? 0
  if (bytes < 1024) {
    return `${bytes} B`
  }
  const units = ['KB', 'MB', 'GB', 'TB']
  let size = bytes / 1024
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index++
  }
  return `${new Intl.NumberFormat(undefined, { maximumFractionDigits: size >= 10 ? 1 : 2 }).format(size)} ${units[index]}`
}

function formatExportFormat(format: string) {
  switch (format) {
    case 'csv':
      return t('datasets.exportFormatCsv')
    case 'json':
      return t('datasets.exportFormatJson')
    case 'parquet':
      return t('datasets.exportFormatParquet')
    default:
      return format
  }
}

function formatScheduleFrequency(frequency: string) {
  switch (frequency) {
    case 'daily':
      return t('datasets.frequencyDaily')
    case 'weekly':
      return t('datasets.frequencyWeekly')
    case 'monthly':
      return t('datasets.frequencyMonthly')
    default:
      return frequency
  }
}

function formatExportSource(source?: string) {
  return source === 'scheduled' ? t('datasets.exportSourceScheduled') : t('datasets.exportSourceManual')
}

function statusClass(status: string) {
  if (status === 'active' || status === 'ready') {
    return 'success'
  }
  if (status === 'dropped' || status === 'failed') {
    return 'danger'
  }
  if (status === 'unmanaged') {
    return ''
  }
  return 'warning'
}

function isCurrentSnapshotRow(row: Record<string, unknown>) {
  const value = row.is_current
  if (typeof value === 'boolean') {
    return value
  }
  if (typeof value === 'number') {
    return value === 1
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    return normalized === '1' || normalized === 'true' || normalized === 't'
  }
  return false
}

async function loadSnapshotHistory() {
  const workTablePublicId = props.selectedTable?.publicId
  const key = snapshotHistoryKey.value.trim()
  if (!workTablePublicId || !key) {
    snapshotHistoryError.value = t('datasets.snapshotHistoryKeyRequired')
    return
  }
  snapshotHistoryLoading.value = true
  snapshotHistoryError.value = ''
  try {
    snapshotHistory.value = await fetchManagedDatasetWorkTableSCD2History(workTablePublicId, key)
  } catch (error) {
    snapshotHistory.value = null
    snapshotHistoryError.value = error instanceof Error ? error.message : t('datasets.snapshotHistoryLoadFailed')
  } finally {
    snapshotHistoryLoading.value = false
  }
}

function confirmText(value: string) {
  textDialog.value = value as 'rename' | 'promote' | 'gold'
}

function closeTextDialog() {
  textDialog.value = ''
}

function submitTextDialog(value: string) {
  const mode = textDialog.value
  closeTextDialog()
  if (mode === 'rename') {
    emit('rename', value)
  } else if (mode === 'promote') {
    emit('promote', value)
  } else if (mode === 'gold') {
    const displayName = value.trim()
    emit('publishGold', displayName ? { displayName } : {})
  }
}

function runConfirmAction() {
  const action = confirmDialog.value
  confirmDialog.value = ''
  if (action === 'truncate') {
    emit('truncate')
  } else if (action === 'drop') {
    emit('drop')
  }
}

function resetScheduleForm() {
  editingSchedulePublicId.value = ''
  scheduleForm.format = 'csv'
  scheduleForm.frequency = 'daily'
  scheduleForm.timezone = defaultTimezone
  scheduleForm.runTime = '03:00'
  scheduleForm.weekday = 1
  scheduleForm.monthDay = 1
  scheduleForm.retentionDays = 7
  scheduleForm.enabled = true
}

function editSchedule(item: DatasetWorkTableExportScheduleBody) {
  editingSchedulePublicId.value = item.publicId
  scheduleForm.format = item.format as DatasetWorkTableExportFormat
  scheduleForm.frequency = item.frequency as DatasetWorkTableExportFrequency
  scheduleForm.timezone = item.timezone || defaultTimezone
  scheduleForm.runTime = item.runTime || '03:00'
  scheduleForm.weekday = item.weekday ?? 1
  scheduleForm.monthDay = item.monthDay ?? 1
  scheduleForm.retentionDays = item.retentionDays || 7
  scheduleForm.enabled = item.enabled
}

function scheduleBody(): DatasetWorkTableExportScheduleCreateBodyWritable {
  return {
    format: scheduleForm.format,
    frequency: scheduleForm.frequency,
    timezone: scheduleForm.timezone.trim() || 'UTC',
    runTime: scheduleForm.runTime,
    retentionDays: Number(scheduleForm.retentionDays) || 7,
    ...(scheduleForm.frequency === 'weekly' ? { weekday: Number(scheduleForm.weekday) || 1 } : {}),
    ...(scheduleForm.frequency === 'monthly' ? { monthDay: Number(scheduleForm.monthDay) || 1 } : {}),
  }
}

function submitSchedule() {
  const body = scheduleBody()
  if (editingSchedulePublicId.value) {
    emit('updateSchedule', editingSchedulePublicId.value, { ...body, enabled: scheduleForm.enabled })
    return
  }
  emit('createSchedule', body)
}

function changeLineageLevel(event: Event) {
  emit('setLineageLevel', (event.target as HTMLSelectElement).value as DatasetLineageLevel)
}
</script>

<template>
  <section class="panel stack dataset-work-table-browser">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('datasets.tenantWorkTables') }}</span>
        <h2>{{ browserTitle }}</h2>
        <span class="cell-subtle">{{ tables.length }} {{ t('datasets.tables') }}</span>
      </div>
      <button class="secondary-button" :disabled="loading" type="button" @click="emit('refresh')">
        <RefreshCw :size="17" aria-hidden="true" />
        {{ loading ? t('common.refreshing') : t('common.refresh') }}
      </button>
    </div>

    <p v-if="errorMessage" class="error-message">
      {{ errorMessage }}
    </p>

    <p v-if="loading">
      {{ t('datasets.loadingWorkTables') }}
    </p>

    <div v-else-if="tables.length > 0" class="dataset-work-table-layout">
      <div class="dataset-list dataset-work-table-list">
        <button
          v-for="item in tables"
          :key="item.publicId || `${item.database}.${item.table}`"
          class="dataset-row"
          :class="{ active: sameWorkTable(item) }"
          type="button"
          @click="emit('select', item)"
        >
          <Table2 :size="17" aria-hidden="true" />
          <span>
            <strong>{{ item.displayName || item.table }}</strong>
            <small>
              {{ item.database }} · {{ item.engine || '-' }}
              <template v-if="item.originDatasetName"> · {{ item.originDatasetName }}</template>
            </small>
          </span>
          <span class="status-pill" :class="statusClass(item.status)">{{ item.managed ? item.status : t('datasets.unmanaged') }}</span>
          <span class="status-pill">{{ t('datasets.approxRows', { count: n(item.totalRows ?? 0) }) }}</span>
        </button>
      </div>

      <div v-if="selectedTable" class="dataset-work-table-detail">
        <div class="section-header compact-section-header">
          <div>
            <span class="status-pill">{{ t('datasets.workTables') }}</span>
            <h3>{{ selectedTable.table }}</h3>
            <span class="cell-subtle monospace-cell">`{{ selectedTable.database }}`.`{{ selectedTable.table }}`</span>
          </div>
        </div>

        <div class="dataset-tabs dataset-work-table-tabs" role="tablist" :aria-label="t('datasets.workTableSections')">
          <button
            type="button"
            role="tab"
            :aria-selected="activeWorkTableDetailTab === 'overview'"
            :class="{ active: activeWorkTableDetailTab === 'overview' }"
            @click="activeWorkTableDetailTab = 'overview'"
          >
            {{ t('datasets.overview') }}
          </button>
          <button
            v-if="selectedTable.managed"
            type="button"
            role="tab"
            :aria-selected="activeWorkTableDetailTab === 'lineage'"
            :class="{ active: activeWorkTableDetailTab === 'lineage' }"
            @click="activeWorkTableDetailTab = 'lineage'"
          >
            {{ t('datasets.lineage') }}
          </button>
          <button
            v-if="selectedTable.managed"
            type="button"
            role="tab"
            :aria-selected="activeWorkTableDetailTab === 'exports'"
            :class="{ active: activeWorkTableDetailTab === 'exports' }"
            @click="activeWorkTableDetailTab = 'exports'"
          >
            {{ t('datasets.exports') }}
          </button>
        </div>

        <div v-if="activeWorkTableDetailTab === 'overview'" class="dataset-work-table-tab-panel">
        <div class="dataset-work-table-actions">
          <template v-if="!selectedTable.managed">
            <button class="primary-button compact-button" type="button" :disabled="actionLoading" @click="emit('register', selectedDatasetPublicId)">
              <Database :size="16" aria-hidden="true" />
              {{ t('datasets.register') }}
            </button>
          </template>
          <template v-else-if="selectedTable.status === 'active'">
            <button class="secondary-button compact-button" type="button" :disabled="actionLoading" @click="confirmText('rename')">
              <Pencil :size="16" aria-hidden="true" />
              {{ t('common.rename') }}
            </button>
            <button class="secondary-button compact-button" type="button" :disabled="actionLoading" @click="confirmText('promote')">
              <Database :size="16" aria-hidden="true" />
              {{ t('datasets.promote') }}
            </button>
            <button class="primary-button compact-button" type="button" :disabled="actionLoading" @click="confirmText('gold')">
              <Crown :size="16" aria-hidden="true" />
              {{ t('datasets.publishGold') }}
            </button>
            <select v-model="exportFormat" class="field-input dataset-export-format-select" :disabled="actionLoading" :aria-label="t('datasets.exportFormat')">
              <option v-for="option in exportFormatOptions" :key="option.value" :value="option.value">
                {{ t(option.labelKey) }}
              </option>
            </select>
            <button class="secondary-button compact-button" type="button" :disabled="actionLoading" @click="emit('export', exportFormat)">
              <FileDown :size="16" aria-hidden="true" />
              {{ t('datasets.requestExport') }}
            </button>
            <button class="secondary-button danger-button compact-button" type="button" :disabled="actionLoading" @click="confirmDialog = 'truncate'">
              {{ t('datasets.truncate') }}
            </button>
            <button class="secondary-button danger-button compact-button" type="button" :disabled="actionLoading" @click="confirmDialog = 'drop'">
              <Trash2 :size="16" aria-hidden="true" />
              {{ t('datasets.drop') }}
            </button>
          </template>
        </div>

        <div v-if="selectedTable.managed && selectedTable.status === 'active'" class="dataset-work-table-link-row">
          <label class="field compact-field">
            <span class="field-label">{{ t('datasets.originDataset') }}</span>
            <select v-model="selectedDatasetPublicId" class="field-input">
              <option value="">{{ t('common.none') }}</option>
              <option v-for="dataset in datasets" :key="dataset.publicId" :value="dataset.publicId">
                {{ dataset.name }}
              </option>
            </select>
          </label>
          <button class="secondary-button compact-button" type="button" :disabled="actionLoading || !selectedDatasetPublicId" @click="emit('link', selectedDatasetPublicId)">
            <Link2 :size="16" aria-hidden="true" />
            {{ t('datasets.link') }}
          </button>
        </div>

        <dl class="metadata-grid dataset-metadata-grid dataset-work-table-metadata">
          <div>
            <dt>{{ t('common.status') }}</dt>
            <dd><span class="status-pill" :class="statusClass(selectedTable.status)">{{ selectedTable.managed ? selectedTable.status : t('datasets.unmanaged') }}</span></dd>
          </div>
          <div>
            <dt>{{ t('datasets.engine') }}</dt>
            <dd>{{ selectedTable.engine || '-' }}</dd>
          </div>
          <div>
            <dt>{{ t('datasets.approxRowsLabel') }}</dt>
            <dd class="tabular-cell">{{ n(selectedTable.totalRows ?? 0) }}</dd>
          </div>
          <div>
            <dt>{{ t('datasets.totalBytes') }}</dt>
            <dd class="tabular-cell">{{ formatBytes(selectedTable.totalBytes) }}</dd>
          </div>
          <div>
            <dt>{{ t('datasets.created') }}</dt>
            <dd>{{ formatDate(selectedTable.createdAt) }}</dd>
          </div>
        </dl>

        <MedallionCatalogPanel
          v-if="selectedTable.managed"
          :catalog="medallionCatalog"
          :loading="medallionLoading"
          :title="t('medallion.workTableTitle')"
        />
        </div>

        <div v-if="selectedTable.managed && activeWorkTableDetailTab === 'lineage'" class="section-header compact-section-header">
          <div>
            <span class="status-pill">{{ t('datasets.lineage') }}</span>
            <h3>{{ t('datasets.lineageGraph') }}</h3>
            <span class="cell-subtle">{{ t('datasets.lineageMetadataConfidence') }}</span>
          </div>
          <button class="secondary-button compact-button" :disabled="lineageLoading" type="button" @click="emit('refreshLineage')">
            <RefreshCw :size="16" aria-hidden="true" />
            {{ lineageLoading ? t('common.loading') : t('common.refresh') }}
          </button>
        </div>

        <div v-if="selectedTable.managed && activeWorkTableDetailTab === 'lineage'" class="lineage-control-bar">
          <label class="field compact-field">
            <span class="field-label">{{ t('datasets.lineageLevel') }}</span>
            <select class="field-input" :value="lineageLevel" @change="changeLineageLevel">
              <option value="table">{{ t('datasets.lineageLevelTable') }}</option>
              <option value="column">{{ t('datasets.lineageLevelColumn') }}</option>
              <option value="both">{{ t('datasets.lineageLevelBoth') }}</option>
            </select>
          </label>
          <div class="lineage-source-controls" :aria-label="t('datasets.lineageSources')">
            <label v-for="source in ['metadata', 'parser', 'manual']" :key="source" class="lineage-source-option">
              <input
                type="checkbox"
                :checked="lineageSources.includes(source as DatasetLineageSource)"
                @change="emit('toggleLineageSource', source as DatasetLineageSource)"
              >
              <span>{{ t(`datasets.lineageSource.${source}`) }}</span>
            </label>
          </div>
          <button v-if="activeLineageGraphTab === 'flow'" class="secondary-button compact-button" type="button" :disabled="lineageActionLoading" @click="lineageEditMode = !lineageEditMode">
            {{ lineageEditMode ? t('datasets.lineageViewMode') : t('datasets.lineageEditMode') }}
          </button>
        </div>

        <p v-if="selectedTable.managed && activeWorkTableDetailTab === 'lineage' && lineageLoading">
          {{ t('datasets.loadingLineage') }}
        </p>
        <template v-else-if="selectedTable.managed && activeWorkTableDetailTab === 'lineage'">
          <div class="lineage-view-tabs" role="tablist" :aria-label="t('datasets.lineageGraphView')">
            <button
              type="button"
              role="tab"
              :aria-selected="activeLineageGraphTab === 'flow'"
              :class="{ active: activeLineageGraphTab === 'flow' }"
              @click="activeLineageGraphTab = 'flow'"
            >
              {{ t('datasets.lineageFlowView') }}
            </button>
            <button
              type="button"
              role="tab"
              :aria-selected="activeLineageGraphTab === 'compact'"
              :class="{ active: activeLineageGraphTab === 'compact' }"
              @click="activeLineageGraphTab = 'compact'"
            >
              {{ t('datasets.lineageCompactView') }}
            </button>
          </div>
          <LineageFlowGraph
            v-if="activeLineageGraphTab === 'flow'"
            :lineage="lineage"
            :editable="lineageEditMode"
            :saving="lineageActionLoading"
            :draft-source-kind="lineageDraftSourceKind"
            :has-draft="hasLineageDraft"
            :can-publish="canPublishLineage"
            @save-draft="emit('saveLineageDraft', $event)"
            @publish-draft="emit('publishLineageDraft')"
            @reject-draft="emit('rejectLineageDraft')"
          />
          <LineageCompactGraph v-else :lineage="lineage" />
          <LineageTimeline :lineage="lineage" />
        </template>

        <div v-if="activeWorkTableDetailTab === 'overview' && selectedColumns.length > 0" class="admin-table dataset-column-table">
          <table>
            <thead>
              <tr>
                <th>{{ t('datasets.column') }}</th>
                <th>{{ t('datasets.type') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="column in selectedColumns" :key="column.columnName">
                <td class="monospace-cell">{{ column.columnName }}</td>
                <td class="monospace-cell">{{ column.clickHouseType }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-if="activeWorkTableDetailTab === 'overview'" class="section-header compact-section-header">
          <div>
            <span class="status-pill">{{ t('datasets.preview') }}</span>
            <h3>{{ t('datasets.previewRows') }}</h3>
          </div>
        </div>

        <p v-if="activeWorkTableDetailTab === 'overview' && previewLoading">
          {{ t('datasets.loadingPreview') }}
        </p>

        <template v-else-if="activeWorkTableDetailTab === 'overview'">
          <div v-if="hasSCD2Columns" class="snapshot-preview-panel">
            <div class="snapshot-preview-panel-header">
              <div>
                <span class="status-pill success">{{ t('datasets.snapshotSCD2Detected') }}</span>
                <h3>{{ t('datasets.snapshotPreviewTitle') }}</h3>
                <p>{{ scd2Summary ? t('datasets.snapshotSummaryDescription') : t('datasets.snapshotPreviewDescription') }}</p>
              </div>
              <div class="lineage-view-tabs snapshot-preview-tabs" role="tablist" :aria-label="t('datasets.snapshotPreviewFilter')">
                <button
                  v-for="option in snapshotPreviewFilters"
                  :key="option.value"
                  type="button"
                  role="tab"
                  :aria-selected="snapshotPreviewFilter === option.value"
                  :class="{ active: snapshotPreviewFilter === option.value }"
                  @click="snapshotPreviewFilter = option.value"
                >
                  {{ t(option.labelKey) }}
                </button>
              </div>
            </div>
            <div class="snapshot-preview-metrics">
              <div>
                <span>{{ scd2Summary ? t('datasets.snapshotTotalRows') : t('datasets.snapshotPreviewRowsLoaded') }}</span>
                <strong>{{ n(snapshotTotalRows) }}</strong>
              </div>
              <div>
                <span>{{ t('datasets.snapshotPreviewCurrentRows') }}</span>
                <strong>{{ n(snapshotCurrentRows) }}</strong>
              </div>
              <div>
                <span>{{ t('datasets.snapshotPreviewHistoryRows') }}</span>
                <strong>{{ n(snapshotHistoryRows) }}</strong>
              </div>
              <div>
                <span>{{ scd2Summary ? t('datasets.snapshotKeyCount') : t('datasets.snapshotPreviewKeyColumn') }}</span>
                <strong>{{ scd2Summary ? n(snapshotKeyCount) : snapshotKeyColumn || '-' }}</strong>
              </div>
            </div>
            <form v-if="snapshotKeyColumn" class="snapshot-history-form" @submit.prevent="loadSnapshotHistory">
              <label class="field compact-field">
                <span class="field-label">{{ t('datasets.snapshotHistoryKeyLabel', { column: snapshotKeyColumn }) }}</span>
                <input v-model="snapshotHistoryKey" class="field-input" type="text" :placeholder="t('datasets.snapshotHistoryKeyPlaceholder')">
              </label>
              <button class="secondary-button compact-button" type="submit" :disabled="snapshotHistoryLoading">
                {{ snapshotHistoryLoading ? t('common.loading') : t('datasets.snapshotHistoryLoad') }}
              </button>
            </form>
            <p v-if="snapshotHistoryError" class="error-message compact-message">{{ snapshotHistoryError }}</p>
            <div v-if="snapshotHistory && snapshotHistoryColumns.length > 0" class="snapshot-history-result">
              <div class="section-header compact-section-header">
                <div>
                  <span class="status-pill">{{ t('datasets.snapshotHistory') }}</span>
                  <h3>{{ t('datasets.snapshotHistoryTitle', { key: snapshotHistory.keyValue }) }}</h3>
                </div>
              </div>
              <div v-if="snapshotHistoryResultRows.length > 0" class="admin-table dataset-result-table dataset-work-table-preview-table">
                <table>
                  <thead>
                    <tr>
                      <th v-for="column in snapshotHistoryColumns" :key="column">{{ column }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(row, rowIndex) in snapshotHistoryResultRows" :key="rowIndex">
                      <td v-for="column in snapshotHistoryColumns" :key="column" class="monospace-cell">
                        {{ row[column] ?? 'NULL' }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div v-else class="empty-state compact-empty-state">
                <p>{{ t('datasets.snapshotHistoryNoRows') }}</p>
              </div>
            </div>
          </div>

          <div v-if="previewColumns.length > 0 && filteredPreviewRows.length > 0" class="admin-table dataset-result-table dataset-work-table-preview-table">
            <table>
              <thead>
                <tr>
                  <th v-for="column in previewColumns" :key="column">{{ column }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, rowIndex) in filteredPreviewRows" :key="rowIndex">
                  <td v-for="column in previewColumns" :key="column" class="monospace-cell">
                    {{ row[column] ?? 'NULL' }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-else class="empty-state">
            <p>{{ t('datasets.noPreviewRows') }}</p>
          </div>
        </template>

        <div v-if="selectedTable.managed && activeWorkTableDetailTab === 'exports' && selectedTable.status === 'active'" class="section-header compact-section-header">
          <div>
            <span class="status-pill">{{ t('datasets.scheduledExports') }}</span>
            <h3>{{ t('datasets.exportSchedules') }}</h3>
          </div>
        </div>

        <div v-if="selectedTable.managed && activeWorkTableDetailTab === 'exports' && selectedTable.status === 'active'" class="dataset-export-schedule-form">
          <label class="field compact-field">
            <span class="field-label">{{ t('datasets.format') }}</span>
            <select v-model="scheduleForm.format" class="field-input">
              <option v-for="option in exportFormatOptions" :key="option.value" :value="option.value">
                {{ t(option.labelKey) }}
              </option>
            </select>
          </label>
          <label class="field compact-field">
            <span class="field-label">{{ t('datasets.frequency') }}</span>
            <select v-model="scheduleForm.frequency" class="field-input">
              <option v-for="option in scheduleFrequencyOptions" :key="option.value" :value="option.value">
                {{ t(option.labelKey) }}
              </option>
            </select>
          </label>
          <label v-if="scheduleForm.frequency === 'weekly'" class="field compact-field">
            <span class="field-label">{{ t('datasets.weekday') }}</span>
            <select v-model.number="scheduleForm.weekday" class="field-input">
              <option v-for="option in weekdayOptions" :key="option.value" :value="option.value">
                {{ t(option.labelKey) }}
              </option>
            </select>
          </label>
          <label v-if="scheduleForm.frequency === 'monthly'" class="field compact-field">
            <span class="field-label">{{ t('datasets.monthDay') }}</span>
            <select v-model.number="scheduleForm.monthDay" class="field-input">
              <option v-for="day in monthDayOptions" :key="day" :value="day">
                {{ day }}
              </option>
            </select>
          </label>
          <label class="field compact-field">
            <span class="field-label">{{ t('datasets.runTime') }}</span>
            <input v-model="scheduleForm.runTime" class="field-input" type="time">
          </label>
          <label class="field compact-field">
            <span class="field-label">{{ t('datasets.timezone') }}</span>
            <input v-model="scheduleForm.timezone" class="field-input" type="text">
          </label>
          <label class="field compact-field">
            <span class="field-label">{{ t('datasets.retentionDays') }}</span>
            <input v-model.number="scheduleForm.retentionDays" class="field-input" type="number" min="1" max="365">
          </label>
          <label v-if="editingSchedulePublicId" class="toggle-inline">
            <input v-model="scheduleForm.enabled" type="checkbox">
            {{ t('common.enabled') }}
          </label>
          <button class="secondary-button compact-button" type="button" :disabled="actionLoading" @click="submitSchedule">
            <CalendarClock :size="16" aria-hidden="true" />
            {{ editingSchedulePublicId ? t('datasets.updateSchedule') : t('datasets.createSchedule') }}
          </button>
          <button v-if="editingSchedulePublicId" class="secondary-button compact-button" type="button" :disabled="actionLoading" @click="resetScheduleForm">
            {{ t('common.cancel') }}
          </button>
        </div>

        <div v-if="selectedTable.managed && activeWorkTableDetailTab === 'exports' && schedules.length > 0" class="admin-table dataset-work-table-schedule-table">
          <table>
            <thead>
              <tr>
                <th>{{ t('common.status') }}</th>
                <th>{{ t('datasets.format') }}</th>
                <th>{{ t('datasets.frequency') }}</th>
                <th>{{ t('datasets.nextRun') }}</th>
                <th>{{ t('datasets.lastRun') }}</th>
                <th>{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in schedules" :key="item.publicId">
                <td><span class="status-pill" :class="item.enabled ? 'success' : ''">{{ item.enabled ? t('common.enabled') : t('common.disabled') }}</span></td>
                <td>{{ formatExportFormat(item.format) }}</td>
                <td>{{ formatScheduleFrequency(item.frequency) }} · {{ item.runTime }} · {{ item.timezone }}</td>
                <td>{{ formatDate(item.nextRunAt) }}</td>
                <td>
                  <span>{{ item.lastStatus || '-' }}</span>
                  <small v-if="item.lastErrorSummary" class="cell-subtle"> · {{ item.lastErrorSummary }}</small>
                </td>
                <td>
                  <button class="secondary-button compact-button" type="button" :disabled="actionLoading" @click="editSchedule(item)">
                    <Pencil :size="16" aria-hidden="true" />
                    {{ t('common.edit') }}
                  </button>
                  <button v-if="item.enabled" class="secondary-button compact-button" type="button" :disabled="actionLoading" @click="emit('disableSchedule', item.publicId)">
                    {{ t('datasets.disableSchedule') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-if="selectedTable.managed && activeWorkTableDetailTab === 'exports'" class="section-header compact-section-header">
          <div>
            <span class="status-pill">{{ t('datasets.exports') }}</span>
            <h3>{{ t('datasets.exportHistory') }}</h3>
          </div>
        </div>

        <div v-if="selectedTable.managed && activeWorkTableDetailTab === 'exports' && exports.length > 0" class="admin-table dataset-work-table-export-table">
          <table>
            <thead>
              <tr>
                <th>{{ t('common.status') }}</th>
                <th>{{ t('datasets.format') }}</th>
                <th>{{ t('datasets.source') }}</th>
                <th>{{ t('datasets.created') }}</th>
                <th>{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in exports" :key="item.publicId">
                <td><span class="status-pill" :class="statusClass(item.status)">{{ item.status }}</span></td>
                <td>{{ formatExportFormat(item.format) }}</td>
                <td>{{ formatExportSource(item.source) }}</td>
                <td>{{ formatDate(item.createdAt) }}</td>
                <td>
                  <a
                    v-if="item.status === 'ready'"
                    class="secondary-button compact-button inline-action-link"
                    :href="workTableExportDownloadUrl(item.publicId)"
                  >
                    <Download :size="16" aria-hidden="true" />
                    {{ t('common.download') }}
                  </a>
                  <span v-else class="cell-subtle">{{ item.errorSummary || '-' }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-else class="empty-state">
        <p>{{ t('datasets.selectWorkTable') }}</p>
      </div>
    </div>

    <div v-else class="empty-state">
      <p>{{ t('datasets.noWorkTables') }}</p>
    </div>

    <TextInputDialog
      :open="textDialog !== ''"
      :title="textDialogTitle"
      :label="textDialogLabel"
      :message="textDialog === 'gold' ? t('datasets.publishGoldMessage') : ''"
      :initial-value="textDialogInitialValue"
      :confirm-label="textDialogConfirmLabel"
      :cancel-label="t('common.back')"
      @cancel="closeTextDialog"
      @confirm="submitTextDialog"
    />

    <ConfirmActionDialog
      :open="confirmDialog !== ''"
      :title="confirmDialog === 'truncate' ? t('datasets.truncateWorkTable') : t('datasets.dropWorkTable')"
      :message="confirmDialog === 'truncate' ? t('datasets.truncateMessage') : t('datasets.dropMessage')"
      :confirm-label="confirmDialog === 'truncate' ? t('datasets.truncate') : t('datasets.drop')"
      :cancel-label="t('common.back')"
      @cancel="confirmDialog = ''"
      @confirm="runConfirmAction"
    />
  </section>
</template>
