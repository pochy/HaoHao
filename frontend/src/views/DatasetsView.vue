<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Database, FileText, Play, RefreshCw, Search } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { toApiErrorMessage, toApiErrorRequestId } from '../api/client'
import type { DatasetQueryJobBody } from '../api/generated/types.gen'
import ConfirmActionDialog from '../components/ConfirmActionDialog.vue'
import { useDatasetStore } from '../stores/datasets'
import { useTenantStore } from '../stores/tenants'

type DatasetTab = 'sql' | 'schema' | 'history'

const datasetStore = useDatasetStore()
const tenantStore = useTenantStore()
const { d, n, t } = useI18n()

const sourceSearch = ref('')
const datasetName = ref('')
const statement = ref('')
const actionErrorMessage = ref('')
const deleteTargetPublicId = ref('')
const activeDatasetTab = ref<DatasetTab>('sql')
let refreshTimer: number | undefined

const selectedDataset = computed(() => datasetStore.selectedDataset)
const selectedSourceFile = computed(() => datasetStore.selectedSourceFile)
const latestQuery = computed(() => datasetStore.latestQuery ?? datasetStore.queryJobs[0] ?? null)
const resultColumns = computed(() => latestQuery.value?.resultColumns ?? [])
const resultRows = computed(() => latestQuery.value?.resultRows ?? [])
const requestErrorMessage = computed(() => actionErrorMessage.value || datasetStore.errorMessage)

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

watch(
  () => selectedSourceFile.value?.publicId,
  () => {
    if (!datasetName.value.trim()) {
      fillDatasetNameFromSource()
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

function sourceDatasetName(filename = '') {
  return filename.replace(/\.[^.]+$/, '').trim()
}

function fillDatasetNameFromSource() {
  const name = sourceDatasetName(selectedSourceFile.value?.originalFilename ?? '')
  if (name) {
    datasetName.value = name
  }
}

function selectSourceFile(publicId: string) {
  datasetStore.selectedSourceFilePublicId = publicId
  datasetName.value = sourceDatasetName(selectedSourceFile.value?.originalFilename ?? '')
}

async function importDataset() {
  const source = selectedSourceFile.value
  if (!source) {
    return
  }
  actionErrorMessage.value = ''
  try {
    await datasetStore.importFromDriveFile(source.publicId, datasetName.value)
    datasetName.value = ''
    fillSampleQuery()
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function refreshDatasets() {
  actionErrorMessage.value = ''
  await datasetStore.load()
}

async function searchSourceFiles() {
  actionErrorMessage.value = ''
  try {
    await datasetStore.refreshSourceFiles(sourceSearch.value)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
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
    actionErrorMessage.value = formatActionError(error)
  }
}

function requestDelete(publicId: string) {
  deleteTargetPublicId.value = publicId
}

function selectQueryJob(job: DatasetQueryJobBody) {
  datasetStore.latestQuery = job
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
    actionErrorMessage.value = formatActionError(error)
  }
}

function formatActionError(error: unknown) {
  const message = toApiErrorMessage(error)
  const requestId = toApiErrorRequestId(error)
  return requestId ? `${message} (request: ${requestId})` : message
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
    <p v-if="requestErrorMessage" class="error-message">
      {{ requestErrorMessage }}
    </p>

    <div class="dataset-layout">
      <section class="panel stack dataset-sidebar-panel">
        <form class="dataset-source-form" @submit.prevent="importDataset">
          <div class="section-header compact-section-header">
            <div>
              <h2>{{ t('datasets.driveSources') }}</h2>
              <span class="cell-subtle">{{ datasetStore.sourceFiles.length }} {{ t('common.files') }}</span>
            </div>
            <RouterLink class="secondary-button compact-button" to="/drive">
              <FileText :size="16" aria-hidden="true" />
              Drive
            </RouterLink>
          </div>

          <div class="dataset-source-search">
            <input
              v-model="sourceSearch"
              class="field-input"
              maxlength="120"
              autocomplete="off"
              :placeholder="t('datasets.sourceSearchPlaceholder')"
              @keydown.enter.prevent="searchSourceFiles"
            >
            <button class="icon-button" type="button" :aria-label="t('common.search')" :title="t('common.search')" @click="searchSourceFiles">
              <Search :size="17" aria-hidden="true" />
            </button>
          </div>

          <div v-if="datasetStore.sourceFiles.length > 0" class="dataset-list dataset-source-list">
            <button
              v-for="file in datasetStore.sourceFiles"
              :key="file.publicId"
              class="dataset-row"
              :class="{ active: file.publicId === selectedSourceFile?.publicId }"
              type="button"
              @click="selectSourceFile(file.publicId)"
            >
              <FileText :size="17" aria-hidden="true" />
              <span>
                <strong>{{ file.originalFilename }}</strong>
                <small>{{ formatBytes(file.byteSize) }} · {{ formatDate(file.updatedAt) }}</small>
              </span>
            </button>
          </div>

          <div v-else class="empty-state">
            <p>{{ t('datasets.noDriveSources') }}</p>
          </div>

          <label class="field">
            <span class="field-label">{{ t('datasets.datasetName') }}</span>
            <input v-model="datasetName" class="field-input" maxlength="160" autocomplete="off">
          </label>
          <button class="primary-button" :disabled="!selectedSourceFile || datasetStore.importing" type="submit">
            <Database :size="17" aria-hidden="true" />
            {{ datasetStore.importing ? t('datasets.importing') : t('datasets.importFromDrive') }}
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
        <div class="dataset-tabs" role="tablist" :aria-label="t('datasets.tabs')">
          <button
            id="dataset-tab-sql"
            type="button"
            role="tab"
            :aria-selected="activeDatasetTab === 'sql'"
            aria-controls="dataset-panel-sql"
            :class="{ active: activeDatasetTab === 'sql' }"
            @click="activeDatasetTab = 'sql'"
          >
            {{ t('datasets.sql') }}
          </button>
          <button
            id="dataset-tab-schema"
            type="button"
            role="tab"
            :aria-selected="activeDatasetTab === 'schema'"
            aria-controls="dataset-panel-schema"
            :class="{ active: activeDatasetTab === 'schema' }"
            @click="activeDatasetTab = 'schema'"
          >
            {{ t('datasets.schema') }}
          </button>
          <button
            id="dataset-tab-history"
            type="button"
            role="tab"
            :aria-selected="activeDatasetTab === 'history'"
            aria-controls="dataset-panel-history"
            :class="{ active: activeDatasetTab === 'history' }"
            @click="activeDatasetTab = 'history'"
          >
            {{ t('datasets.history') }}
          </button>
        </div>

        <div
          v-if="activeDatasetTab === 'sql'"
          id="dataset-panel-sql"
          class="dataset-tab-panel"
          role="tabpanel"
          aria-labelledby="dataset-tab-sql"
        >
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

            <p v-if="requestErrorMessage" class="error-message dataset-sql-error">
              {{ requestErrorMessage }}
            </p>
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
        </div>

        <div
          v-else-if="activeDatasetTab === 'schema'"
          id="dataset-panel-schema"
          class="dataset-tab-panel"
          role="tabpanel"
          aria-labelledby="dataset-tab-schema"
        >
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
        </div>

        <div
          v-else
          id="dataset-panel-history"
          class="dataset-tab-panel"
          role="tabpanel"
          aria-labelledby="dataset-tab-history"
        >
          <section class="panel stack">
            <div class="section-header">
              <div>
                <span class="status-pill">{{ t('datasets.history') }}</span>
                <h2>{{ t('datasets.recentQueries') }}</h2>
              </div>
            </div>
            <div v-if="datasetStore.queryJobs.length > 0" class="list-stack">
              <button
                v-for="job in datasetStore.queryJobs"
                :key="job.publicId"
                class="dataset-query-row"
                type="button"
                @click="selectQueryJob(job)"
              >
                <span class="status-pill" :class="statusClass(job.status)">{{ job.status }}</span>
                <span class="monospace-cell">{{ job.statement }}</span>
                <span class="cell-subtle tabular-cell">{{ formatDate(job.createdAt) }} · {{ job.durationMs }} ms</span>
              </button>
            </div>
            <div v-else class="empty-state">
              <p>{{ t('datasets.noQueryHistory') }}</p>
            </div>
          </section>
        </div>
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
