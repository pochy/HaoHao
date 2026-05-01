<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Database, Play, RefreshCw, Upload } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { toApiErrorMessage } from '../api/client'
import ConfirmActionDialog from '../components/ConfirmActionDialog.vue'
import { useDatasetStore } from '../stores/datasets'
import { useTenantStore } from '../stores/tenants'

const datasetStore = useDatasetStore()
const tenantStore = useTenantStore()
const { d, n, t } = useI18n()

const fileInput = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const datasetName = ref('')
const statement = ref('')
const actionErrorMessage = ref('')
const deleteTargetPublicId = ref('')
let refreshTimer: number | undefined

const selectedDataset = computed(() => datasetStore.selectedDataset)
const latestQuery = computed(() => datasetStore.latestQuery ?? datasetStore.queryJobs[0] ?? null)
const resultColumns = computed(() => latestQuery.value?.resultColumns ?? [])
const resultRows = computed(() => latestQuery.value?.resultRows ?? [])

const selectedTableName = computed(() => {
  const item = selectedDataset.value
  if (!item) {
    return ''
  }
  return `\`${item.rawDatabase}\`.\`${item.rawTable}\``
})

const activeTenantLabel = computed(() => (
  tenantStore.activeTenant
    ? `${tenantStore.activeTenant.displayName} / ${tenantStore.activeTenant.slug}`
    : t('datasets.noTenant')
))

onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
  refreshTimer = window.setInterval(async () => {
    if (datasetStore.hasActiveImports) {
      await datasetStore.load()
    }
  }, 4000)
})

onBeforeUnmount(() => {
  if (refreshTimer !== undefined) {
    window.clearInterval(refreshTimer)
  }
})

watch(
  () => tenantStore.activeTenant?.slug,
  async (slug) => {
    actionErrorMessage.value = ''
    statement.value = ''
    datasetStore.reset()
    if (slug) {
      await datasetStore.load()
      fillSampleQuery()
    }
  },
  { immediate: true },
)

watch(
  () => selectedDataset.value?.publicId,
  () => {
    if (!statement.value.trim()) {
      fillSampleQuery()
    }
  },
)

function formatDate(value?: string) {
  return value ? d(new Date(value), 'long') : '-'
}

function formatBytes(value: number) {
  if (value < 1024) {
    return `${value} B`
  }
  const units = ['KB', 'MB', 'GB', 'TB']
  let size = value / 1024
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index++
  }
  return `${new Intl.NumberFormat(undefined, { maximumFractionDigits: size >= 10 ? 1 : 2 }).format(size)} ${units[index]}`
}

function statusClass(status: string) {
  if (status === 'ready' || status === 'completed') {
    return 'success'
  }
  if (status === 'failed') {
    return 'danger'
  }
  return 'warning'
}

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0] ?? null
  selectedFile.value = file
  if (file && !datasetName.value.trim()) {
    datasetName.value = file.name.replace(/\.[^.]+$/, '')
  }
}

async function uploadDataset() {
  if (!selectedFile.value) {
    return
  }
  actionErrorMessage.value = ''
  try {
    await datasetStore.upload(selectedFile.value, datasetName.value)
    selectedFile.value = null
    datasetName.value = ''
    if (fileInput.value) {
      fileInput.value.value = ''
    }
    fillSampleQuery()
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function refreshDatasets() {
  actionErrorMessage.value = ''
  await datasetStore.load()
}

function selectDataset(publicId: string) {
  datasetStore.selectedPublicId = publicId
  fillSampleQuery()
}

function fillSampleQuery() {
  if (!selectedTableName.value) {
    return
  }
  statement.value = `SELECT *\nFROM ${selectedTableName.value}\nLIMIT 100`
}

async function runQuery() {
  if (!statement.value.trim()) {
    return
  }
  actionErrorMessage.value = ''
  try {
    await datasetStore.run(statement.value)
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

function requestDelete(publicId: string) {
  deleteTargetPublicId.value = publicId
}

async function confirmDelete() {
  const publicId = deleteTargetPublicId.value
  deleteTargetPublicId.value = ''
  if (!publicId) {
    return
  }
  actionErrorMessage.value = ''
  try {
    await datasetStore.remove(publicId)
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}
</script>

<template>
  <div class="dataset-workspace">
    <header class="page-header">
      <div class="page-header-copy">
        <span class="status-pill">{{ t('datasets.badge') }}</span>
        <h1>{{ t('datasets.title') }}</h1>
        <p>{{ activeTenantLabel }}</p>
      </div>
      <div class="page-header-actions">
        <button class="secondary-button" :disabled="datasetStore.status === 'loading'" type="button" @click="refreshDatasets">
          <RefreshCw :size="17" aria-hidden="true" />
          {{ datasetStore.status === 'loading' ? t('common.refreshing') : t('common.refresh') }}
        </button>
      </div>
    </header>

    <p v-if="tenantStore.status === 'empty'" class="warning-message">
      {{ t('datasets.noTenantMessage') }}
    </p>
    <p v-if="actionErrorMessage || datasetStore.errorMessage" class="error-message">
      {{ actionErrorMessage || datasetStore.errorMessage }}
    </p>

    <div class="dataset-layout">
      <section class="panel stack dataset-sidebar-panel">
        <form class="dataset-upload-form" @submit.prevent="uploadDataset">
          <label class="field">
            <span class="field-label">{{ t('datasets.csvFile') }}</span>
            <input ref="fileInput" class="field-input" accept=".csv,text/csv" type="file" @change="onFileChange">
          </label>
          <label class="field">
            <span class="field-label">{{ t('datasets.datasetName') }}</span>
            <input v-model="datasetName" class="field-input" maxlength="160" autocomplete="off">
          </label>
          <button class="primary-button" :disabled="!selectedFile || datasetStore.uploading" type="submit">
            <Upload :size="17" aria-hidden="true" />
            {{ datasetStore.uploading ? t('common.uploading') : t('common.upload') }}
          </button>
        </form>

        <div class="section-header">
          <div>
            <h2>{{ t('datasets.datasets') }}</h2>
            <span class="cell-subtle">{{ datasetStore.items.length }} {{ t('common.files') }}</span>
          </div>
        </div>

        <div v-if="datasetStore.items.length > 0" class="dataset-list">
          <button
            v-for="item in datasetStore.items"
            :key="item.publicId"
            class="dataset-row"
            :class="{ active: item.publicId === selectedDataset?.publicId }"
            type="button"
            @click="selectDataset(item.publicId)"
          >
            <Database :size="17" aria-hidden="true" />
            <span>
              <strong>{{ item.name }}</strong>
              <small>{{ item.rawTable }} · {{ formatBytes(item.byteSize) }}</small>
            </span>
            <span class="status-pill" :class="statusClass(item.status)">{{ item.status }}</span>
          </button>
        </div>

        <div v-else-if="datasetStore.status === 'empty'" class="empty-state">
          <p>{{ t('datasets.empty') }}</p>
        </div>
      </section>

      <main class="dataset-main">
        <section class="panel stack">
          <div class="section-header">
            <div>
              <span class="status-pill">{{ t('datasets.schema') }}</span>
              <h2>{{ selectedDataset?.name ?? t('datasets.noDataset') }}</h2>
              <span v-if="selectedDataset" class="cell-subtle monospace-cell">{{ selectedTableName }}</span>
            </div>
            <button
              v-if="selectedDataset"
              class="secondary-button danger-button"
              :disabled="datasetStore.deletingPublicId === selectedDataset.publicId"
              type="button"
              @click="requestDelete(selectedDataset.publicId)"
            >
              {{ datasetStore.deletingPublicId === selectedDataset.publicId ? t('common.deleting') : t('common.delete') }}
            </button>
          </div>

          <dl v-if="selectedDataset" class="metadata-grid dataset-metadata-grid">
            <div>
              <dt>{{ t('common.status') }}</dt>
              <dd><span class="status-pill" :class="statusClass(selectedDataset.status)">{{ selectedDataset.status }}</span></dd>
            </div>
            <div>
              <dt>{{ t('datasets.rows') }}</dt>
              <dd class="tabular-cell">{{ n(selectedDataset.rowCount) }}</dd>
            </div>
            <div>
              <dt>{{ t('datasets.imported') }}</dt>
              <dd>{{ formatDate(selectedDataset.importedAt) }}</dd>
            </div>
            <div>
              <dt>{{ t('datasets.invalidRows') }}</dt>
              <dd class="tabular-cell">{{ n(selectedDataset.importJob?.invalidRows ?? 0) }}</dd>
            </div>
          </dl>

          <div v-if="selectedDataset?.columns?.length" class="admin-table dataset-column-table">
            <table>
              <thead>
                <tr>
                  <th>{{ t('datasets.column') }}</th>
                  <th>{{ t('datasets.original') }}</th>
                  <th>{{ t('datasets.type') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="column in selectedDataset.columns" :key="column.columnName">
                  <td class="monospace-cell">{{ column.columnName }}</td>
                  <td>{{ column.originalName || '-' }}</td>
                  <td class="monospace-cell">{{ column.clickHouseType }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="panel stack">
          <div class="section-header">
            <div>
              <span class="status-pill">{{ t('datasets.sql') }}</span>
              <h2>{{ t('datasets.editor') }}</h2>
            </div>
            <button class="secondary-button" :disabled="!selectedDataset" type="button" @click="fillSampleQuery">
              {{ t('datasets.sample') }}
            </button>
          </div>

          <textarea
            v-model="statement"
            class="field-input textarea-input dataset-sql-input"
            spellcheck="false"
            :placeholder="t('datasets.sqlPlaceholder')"
          />

          <div class="action-row">
            <button class="primary-button" :disabled="datasetStore.executing || statement.trim() === ''" type="button" @click="runQuery">
              <Play :size="17" aria-hidden="true" />
              {{ datasetStore.executing ? t('datasets.running') : t('datasets.run') }}
            </button>
          </div>
        </section>

        <section class="panel stack">
          <div class="section-header">
            <div>
              <span class="status-pill">{{ t('datasets.results') }}</span>
              <h2>{{ latestQuery?.status ?? t('common.none') }}</h2>
              <span v-if="latestQuery" class="cell-subtle tabular-cell">{{ latestQuery.durationMs }} ms · {{ latestQuery.rowCount }} rows</span>
            </div>
          </div>

          <p v-if="latestQuery?.errorSummary" class="error-message">
            {{ latestQuery.errorSummary }}
          </p>

          <div v-if="resultColumns.length > 0" class="admin-table dataset-result-table">
            <table>
              <thead>
                <tr>
                  <th v-for="column in resultColumns" :key="column">{{ column }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, rowIndex) in resultRows" :key="rowIndex">
                  <td v-for="column in resultColumns" :key="column" class="monospace-cell">
                    {{ row[column] ?? 'NULL' }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-else class="empty-state">
            <p>{{ t('datasets.noResults') }}</p>
          </div>
        </section>

        <section v-if="datasetStore.queryJobs.length > 0" class="panel stack">
          <div class="section-header">
            <div>
              <span class="status-pill">{{ t('datasets.history') }}</span>
              <h2>{{ t('datasets.recentQueries') }}</h2>
            </div>
          </div>
          <div class="list-stack">
            <button
              v-for="job in datasetStore.queryJobs"
              :key="job.publicId"
              class="dataset-query-row"
              type="button"
              @click="datasetStore.latestQuery = job"
            >
              <span class="status-pill" :class="statusClass(job.status)">{{ job.status }}</span>
              <span class="monospace-cell">{{ job.statement }}</span>
              <span class="cell-subtle tabular-cell">{{ formatDate(job.createdAt) }} · {{ job.durationMs }} ms</span>
            </button>
          </div>
        </section>
      </main>
    </div>

    <ConfirmActionDialog
      :open="deleteTargetPublicId !== ''"
      :title="t('datasets.deleteTitle')"
      :message="t('datasets.deleteMessage')"
      :confirm-label="t('common.delete')"
      :cancel-label="t('common.back')"
      @cancel="deleteTargetPublicId = ''"
      @confirm="confirmDelete"
    />
  </div>
</template>
