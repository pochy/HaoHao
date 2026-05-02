<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Database, Download, FileDown, Link2, Pencil, RefreshCw, Table2, Trash2 } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { workTableExportDownloadUrl } from '../api/datasets'
import type { DatasetBody, DatasetWorkTableBody, DatasetWorkTableExportBody, DatasetWorkTablePreviewBody } from '../api/generated/types.gen'
import ConfirmActionDialog from './ConfirmActionDialog.vue'
import TextInputDialog from './TextInputDialog.vue'

const props = withDefaults(defineProps<{
  tables: DatasetWorkTableBody[]
  datasets: DatasetBody[]
  selectedTable: DatasetWorkTableBody | null
  preview: DatasetWorkTablePreviewBody | null
  exports: DatasetWorkTableExportBody[]
  loading: boolean
  previewLoading: boolean
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
  export: []
}>()

const { d, n, t } = useI18n()

const previewColumns = computed(() => props.preview?.columns ?? [])
const previewRows = computed(() => props.preview?.previewRows ?? [])
const selectedColumns = computed(() => props.selectedTable?.columns ?? [])
const browserTitle = computed(() => props.title || t('datasets.workTables'))
const selectedDatasetPublicId = ref('')
const textDialog = ref<'rename' | 'promote' | ''>('')
const confirmDialog = ref<'truncate' | 'drop' | ''>('')

watch(
  () => [props.selectedTable?.originDatasetPublicId, props.datasets.length] as const,
  () => {
    selectedDatasetPublicId.value = props.selectedTable?.originDatasetPublicId || props.datasets[0]?.publicId || ''
  },
  { immediate: true },
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

function confirmText(value: string) {
  textDialog.value = value as 'rename' | 'promote'
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
            <span class="status-pill">{{ t('datasets.preview') }}</span>
            <h3>{{ selectedTable.table }}</h3>
            <span class="cell-subtle monospace-cell">`{{ selectedTable.database }}`.`{{ selectedTable.table }}`</span>
          </div>
        </div>

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
            <button class="secondary-button compact-button" type="button" :disabled="actionLoading" @click="emit('export')">
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

        <div v-if="selectedColumns.length > 0" class="admin-table dataset-column-table">
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

        <div class="section-header compact-section-header">
          <div>
            <span class="status-pill">{{ t('datasets.preview') }}</span>
            <h3>{{ t('datasets.previewRows') }}</h3>
          </div>
        </div>

        <p v-if="previewLoading">
          {{ t('datasets.loadingPreview') }}
        </p>

        <div v-else-if="previewColumns.length > 0 && previewRows.length > 0" class="admin-table dataset-result-table dataset-work-table-preview-table">
          <table>
            <thead>
              <tr>
                <th v-for="column in previewColumns" :key="column">{{ column }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, rowIndex) in previewRows" :key="rowIndex">
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

        <div v-if="selectedTable.managed" class="section-header compact-section-header">
          <div>
            <span class="status-pill">{{ t('datasets.exports') }}</span>
            <h3>{{ t('datasets.csvExports') }}</h3>
          </div>
        </div>

        <div v-if="selectedTable.managed && exports.length > 0" class="admin-table dataset-work-table-export-table">
          <table>
            <thead>
              <tr>
                <th>{{ t('common.status') }}</th>
                <th>{{ t('datasets.format') }}</th>
                <th>{{ t('datasets.created') }}</th>
                <th>{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in exports" :key="item.publicId">
                <td><span class="status-pill" :class="statusClass(item.status)">{{ item.status }}</span></td>
                <td>{{ item.format }}</td>
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
      :title="textDialog === 'rename' ? t('datasets.renameWorkTable') : t('datasets.promoteWorkTable')"
      :label="textDialog === 'rename' ? t('datasets.newTableName') : t('datasets.datasetName')"
      :initial-value="textDialog === 'rename' ? selectedTable?.table ?? '' : selectedTable?.displayName ?? selectedTable?.table ?? ''"
      :confirm-label="textDialog === 'rename' ? t('common.rename') : t('datasets.promote')"
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
