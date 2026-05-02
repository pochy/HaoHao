<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Play, RefreshCw, RotateCw, Table2 } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { toApiErrorMessage, toApiErrorRequestId } from '../api/client'
import type { DatasetLineageGraphSaveBodyWritable, DatasetQueryJobBody } from '../api/generated/types.gen'
import ConfirmActionDialog from '../components/ConfirmActionDialog.vue'
import LineageCompactGraph from '../components/LineageCompactGraph.vue'
import LineageFlowGraph from '../components/LineageFlowGraph.vue'
import LineageTimeline from '../components/LineageTimeline.vue'
import { useDatasetStore } from '../stores/datasets'
import { useRealtimeStore } from '../stores/realtime'
import { useTenantStore } from '../stores/tenants'

type DatasetTab = 'sql' | 'schema' | 'history'
type LineageGraphTab = 'flow' | 'compact'

const route = useRoute()
const router = useRouter()
const datasetStore = useDatasetStore()
const realtimeStore = useRealtimeStore()
const tenantStore = useTenantStore()
const { d, n, t } = useI18n()

const statement = ref('')
const actionErrorMessage = ref('')
const deleteTargetPublicId = ref('')
const syncConfirmOpen = ref(false)
const activeDatasetTab = ref<DatasetTab>('sql')
const activeLineageGraphTab = ref<LineageGraphTab>('flow')
const lineageEditMode = ref(false)
let refreshTimer: number | undefined

const datasetPublicId = computed(() => {
  const raw = Array.isArray(route.params.datasetPublicId)
    ? route.params.datasetPublicId[0]
    : route.params.datasetPublicId
  return raw ?? ''
})

const selectedDataset = computed(() => datasetStore.selectedDataset)
const latestQuery = computed(() => datasetStore.latestQuery ?? datasetStore.queryJobs[0] ?? null)
const latestSync = computed(() => selectedDataset.value?.latestSyncJob ?? datasetStore.syncJobs[0] ?? null)
const latestQueryFailed = computed(() => latestQuery.value?.status === 'failed')
const resultColumns = computed(() => latestQuery.value?.resultColumns ?? [])
const resultRows = computed(() => latestQuery.value?.resultRows ?? [])
const requestErrorMessage = computed(() => actionErrorMessage.value || datasetStore.errorMessage)
const isWorkTableDataset = computed(() => selectedDataset.value?.sourceKind === 'work_table')
const sourceWorkTableLabel = computed(() => {
  const item = selectedDataset.value
  if (!item?.sourceWorkTableDatabase || !item.sourceWorkTableTable) {
    return '-'
  }
  return `\`${item.sourceWorkTableDatabase}\`.\`${item.sourceWorkTableTable}\``
})
const canRequestSync = computed(() => (
  Boolean(selectedDataset.value?.publicId && isWorkTableDataset.value && selectedDataset.value?.sourceWorkTablePublicId)
    && !datasetStore.hasActiveDatasetSync
    && !datasetStore.syncing
))
const canReviewLineage = computed(() => tenantStore.activeTenant?.roles?.includes('tenant_admin') ?? false)
const activeLineageDraftSource = computed(() => datasetStore.selectedLineageChangeSet?.changeSet.sourceKind ?? 'manual')

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
    if (!realtimeStore.connected && selectedDataset.value && ['pending', 'importing'].includes(selectedDataset.value.status)) {
      await refreshDatasetDetail()
    }
    if (selectedDataset.value && datasetStore.hasActiveDatasetSync) {
      await refreshDatasetSyncState()
    }
  }, 4000)
})

onBeforeUnmount(() => {
  if (refreshTimer !== undefined) {
    window.clearInterval(refreshTimer)
  }
})

watch(
  () => [tenantStore.activeTenant?.slug, datasetPublicId.value],
  async ([slug, publicId]) => {
    actionErrorMessage.value = ''
    statement.value = ''
    datasetStore.latestQuery = null
    datasetStore.queryJobs = []
    datasetStore.syncJobs = []
    if (!slug || !publicId) {
      datasetStore.selectedPublicId = ''
      return
    }
    try {
      datasetStore.selectedPublicId = publicId
      await Promise.all([
        datasetStore.loadDataset(publicId),
        datasetStore.loadQueryJobs(publicId),
        datasetStore.loadLinkedWorkTables(publicId),
        datasetStore.loadDatasetSyncJobs(publicId),
        datasetStore.loadDatasetLineage(publicId),
        datasetStore.loadLineageChangeSets(),
      ])
      await datasetStore.loadLatestQueryLineageParseRuns()
      fillSampleQuery()
    } catch (error) {
      actionErrorMessage.value = formatActionError(error)
    }
  },
  { immediate: true },
)

function formatDate(value?: string) {
  return value ? d(new Date(value), 'long') : '-'
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

function fillSampleQuery() {
  if (!selectedTableName.value) {
    return
  }
  statement.value = `SELECT *\nFROM ${selectedTableName.value}\nLIMIT 100`
}

async function refreshDatasetDetail() {
  if (!datasetPublicId.value) {
    return
  }
  actionErrorMessage.value = ''
  try {
    await Promise.all([
      datasetStore.loadDataset(datasetPublicId.value),
      datasetStore.loadQueryJobs(datasetPublicId.value),
      datasetStore.loadLinkedWorkTables(datasetPublicId.value),
      datasetStore.loadDatasetSyncJobs(datasetPublicId.value),
      datasetStore.loadDatasetLineage(datasetPublicId.value),
      datasetStore.loadLineageChangeSets(),
    ])
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function refreshDatasetSyncState() {
  if (!datasetPublicId.value) {
    return
  }
  try {
    await Promise.all([
      datasetStore.refreshSelected(),
      datasetStore.loadDatasetSyncJobs(datasetPublicId.value),
      datasetStore.loadDatasetLineage(datasetPublicId.value),
    ])
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function refreshDatasetLineage() {
  if (!datasetPublicId.value) {
    return
  }
  actionErrorMessage.value = ''
  try {
    await datasetStore.loadDatasetLineage(datasetPublicId.value)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function changeLineageLevel(event: Event) {
  datasetStore.setLineageLevel((event.target as HTMLSelectElement).value as 'table' | 'column' | 'both')
  await refreshDatasetLineage()
}

async function toggleLineageSource(source: 'metadata' | 'parser' | 'manual') {
  datasetStore.toggleLineageSource(source)
  await refreshDatasetLineage()
}

async function selectLineageDraft(event: Event) {
  actionErrorMessage.value = ''
  try {
    await datasetStore.selectLineageChangeSet((event.target as HTMLSelectElement).value)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function saveDatasetLineageDraft(body: DatasetLineageGraphSaveBodyWritable) {
  actionErrorMessage.value = ''
  try {
    await datasetStore.saveDatasetLineageDraft(body)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function publishLineageDraft() {
  actionErrorMessage.value = ''
  try {
    await datasetStore.publishSelectedLineageChangeSet()
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function rejectLineageDraft() {
  actionErrorMessage.value = ''
  try {
    await datasetStore.rejectSelectedLineageChangeSet()
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function parseLatestQueryLineage() {
  actionErrorMessage.value = ''
  try {
    await datasetStore.parseLatestQueryLineage()
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function runQuery() {
  if (!statement.value.trim() || !datasetPublicId.value) {
    return
  }
  actionErrorMessage.value = ''
  try {
    await datasetStore.runForDataset(datasetPublicId.value, statement.value)
    await datasetStore.loadLinkedWorkTables(datasetPublicId.value)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

function requestDelete(publicId: string) {
  deleteTargetPublicId.value = publicId
}

function requestSync() {
  syncConfirmOpen.value = true
}

function selectQueryJob(job: DatasetQueryJobBody) {
  datasetStore.latestQuery = job
  datasetStore.loadLatestQueryLineageParseRuns()
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
    await router.push('/datasets')
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function confirmSync() {
  syncConfirmOpen.value = false
  actionErrorMessage.value = ''
  try {
    await datasetStore.requestSelectedDatasetSync()
    if (datasetPublicId.value) {
      await datasetStore.loadDatasetSyncJobs(datasetPublicId.value)
    }
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
        <h1>{{ selectedDataset?.name ?? t('datasets.detailTitle') }}</h1>
        <p>{{ activeTenantLabel }}</p>
      </div>
      <div class="page-header-actions">
        <RouterLink class="secondary-button link-button" to="/datasets">
          {{ t('datasets.backToDatasets') }}
        </RouterLink>
        <button class="secondary-button" :disabled="datasetStore.status === 'loading'" type="button" @click="refreshDatasetDetail">
          <RefreshCw :size="17" aria-hidden="true" />
          {{ datasetStore.status === 'loading' ? t('common.refreshing') : t('common.refresh') }}
        </button>
      </div>
    </header>

    <p v-if="tenantStore.status === 'empty'" class="warning-message">
      {{ t('datasets.noTenantMessage') }}
    </p>
    <p v-if="datasetStore.status === 'loading'">
      {{ t('datasets.loadingDetail') }}
    </p>
    <p v-if="requestErrorMessage" class="error-message">
      {{ requestErrorMessage }}
    </p>

    <main v-if="selectedDataset" class="dataset-main">
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
            <button class="secondary-button" type="button" @click="fillSampleQuery">
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

          <div v-if="latestQueryFailed" class="dataset-query-error-panel" role="alert">
            <span class="status-pill danger">{{ t('datasets.executionFailed') }}</span>
            <p>{{ latestQuery?.errorSummary || t('datasets.executionFailedFallback') }}</p>
          </div>
          <p v-else-if="latestQuery?.errorSummary" class="warning-message">
            {{ latestQuery.errorSummary }}
          </p>

          <div v-if="!latestQueryFailed && resultColumns.length > 0" class="admin-table dataset-result-table">
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

          <div v-else-if="!latestQueryFailed" class="empty-state">
            <p>{{ t('datasets.noResults') }}</p>
          </div>
        </section>

        <section class="panel stack">
          <div class="section-header">
            <div>
              <span class="status-pill">{{ t('datasets.workTables') }}</span>
              <h2>{{ t('datasets.linkedWorkTables') }}</h2>
            </div>
            <RouterLink class="secondary-button compact-button" to="/datasets">
              {{ t('datasets.manageWorkTables') }}
            </RouterLink>
          </div>

          <div v-if="datasetStore.linkedWorkTables.length > 0" class="dataset-list dataset-work-table-list compact-work-table-list">
            <RouterLink
              v-for="item in datasetStore.linkedWorkTables"
              :key="item.publicId || `${item.database}.${item.table}`"
              class="dataset-row"
              to="/datasets"
            >
              <Table2 :size="17" aria-hidden="true" />
              <span>
                <strong>{{ item.displayName || item.table }}</strong>
                <small>{{ item.database }} · {{ item.engine || '-' }}</small>
              </span>
              <span class="status-pill">{{ t('datasets.approxRows', { count: n(item.totalRows ?? 0) }) }}</span>
            </RouterLink>
          </div>

          <div v-else class="empty-state">
            <p>{{ t('datasets.noLinkedWorkTables') }}</p>
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
              <span class="status-pill">{{ t('datasets.importedTable') }}</span>
              <h2>{{ selectedDataset.name }}</h2>
              <span class="cell-subtle monospace-cell">{{ selectedTableName }}</span>
            </div>
            <button
              class="secondary-button danger-button"
              :disabled="datasetStore.deletingPublicId === selectedDataset.publicId"
              type="button"
              @click="requestDelete(selectedDataset.publicId)"
            >
              {{ datasetStore.deletingPublicId === selectedDataset.publicId ? t('common.deleting') : t('common.delete') }}
            </button>
          </div>

          <dl class="metadata-grid dataset-metadata-grid">
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

          <div v-if="selectedDataset.columns?.length" class="admin-table dataset-column-table">
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
              <span class="status-pill">{{ t('datasets.lineage') }}</span>
              <h2>{{ t('datasets.lineageGraph') }}</h2>
              <span class="cell-subtle">{{ t('datasets.lineageMetadataConfidence') }}</span>
            </div>
            <button class="secondary-button compact-button" :disabled="datasetStore.datasetLineageLoading" type="button" @click="refreshDatasetLineage">
              <RefreshCw :size="16" aria-hidden="true" />
              {{ datasetStore.datasetLineageLoading ? t('common.loading') : t('common.refresh') }}
            </button>
          </div>

          <div class="lineage-control-bar">
            <label class="field compact-field">
              <span class="field-label">{{ t('datasets.lineageLevel') }}</span>
              <select class="field-input" :value="datasetStore.lineageLevel" @change="changeLineageLevel">
                <option value="table">{{ t('datasets.lineageLevelTable') }}</option>
                <option value="column">{{ t('datasets.lineageLevelColumn') }}</option>
                <option value="both">{{ t('datasets.lineageLevelBoth') }}</option>
              </select>
            </label>
            <div class="lineage-source-controls" :aria-label="t('datasets.lineageSources')">
              <label v-for="source in ['metadata', 'parser', 'manual']" :key="source" class="lineage-source-option">
                <input
                  type="checkbox"
                  :checked="datasetStore.lineageSources.includes(source as 'metadata' | 'parser' | 'manual')"
                  @change="toggleLineageSource(source as 'metadata' | 'parser' | 'manual')"
                >
                <span>{{ t(`datasets.lineageSource.${source}`) }}</span>
              </label>
            </div>
            <label class="field compact-field">
              <span class="field-label">{{ t('datasets.lineageDraft') }}</span>
              <select class="field-input" :value="datasetStore.selectedLineageChangeSet?.changeSet.publicId ?? ''" @change="selectLineageDraft">
                <option value="">{{ t('datasets.lineagePublishedGraph') }}</option>
                <option v-for="draft in datasetStore.lineageChangeSets" :key="draft.publicId" :value="draft.publicId">
                  {{ draft.title }}
                </option>
              </select>
            </label>
            <button v-if="activeLineageGraphTab === 'flow'" class="secondary-button compact-button" type="button" :disabled="datasetStore.lineageActionLoading" @click="lineageEditMode = !lineageEditMode">
              {{ lineageEditMode ? t('datasets.lineageViewMode') : t('datasets.lineageEditMode') }}
            </button>
            <button class="secondary-button compact-button" type="button" :disabled="!latestQuery?.publicId || datasetStore.lineageActionLoading" @click="parseLatestQueryLineage">
              {{ t('datasets.lineageParseSql') }}
            </button>
          </div>

          <div v-if="datasetStore.lineageParseRuns.length > 0" class="lineage-review-strip">
            <span v-for="run in datasetStore.lineageParseRuns" :key="run.publicId" class="status-pill" :class="statusClass(run.status)">
              {{ t('datasets.lineageParseRun') }}: {{ run.status }} / {{ run.tableRefCount }} tables / {{ run.columnEdgeCount }} columns
            </span>
          </div>

          <p v-if="datasetStore.datasetLineageLoading">
            {{ t('datasets.loadingLineage') }}
          </p>
          <template v-else>
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
              :lineage="datasetStore.datasetLineage"
              :editable="lineageEditMode"
              :saving="datasetStore.lineageActionLoading"
              :draft-source-kind="activeLineageDraftSource"
              :has-draft="Boolean(datasetStore.selectedLineageChangeSet)"
              :can-publish="canReviewLineage"
              @save-draft="saveDatasetLineageDraft"
              @publish-draft="publishLineageDraft"
              @reject-draft="rejectLineageDraft"
            />
            <LineageCompactGraph v-else :lineage="datasetStore.datasetLineage" />
            <LineageTimeline :lineage="datasetStore.datasetLineage" />
          </template>
        </section>

        <section v-if="isWorkTableDataset" class="panel stack">
          <div class="section-header">
            <div>
              <span class="status-pill">{{ t('datasets.sync') }}</span>
              <h2>{{ t('datasets.sourceWorkTable') }}</h2>
              <span class="cell-subtle monospace-cell">{{ sourceWorkTableLabel }}</span>
            </div>
            <button
              class="secondary-button"
              :disabled="!canRequestSync"
              type="button"
              @click="requestSync"
            >
              <RotateCw :size="17" aria-hidden="true" />
              {{ datasetStore.syncing ? t('datasets.requestingSync') : t('datasets.requestSync') }}
            </button>
          </div>

          <dl class="metadata-grid dataset-metadata-grid">
            <div>
              <dt>{{ t('datasets.sourceWorkTable') }}</dt>
              <dd>{{ selectedDataset.sourceWorkTableName || selectedDataset.sourceWorkTableTable || '-' }}</dd>
            </div>
            <div>
              <dt>{{ t('common.status') }}</dt>
              <dd><span class="status-pill" :class="statusClass(selectedDataset.sourceWorkTableStatus || '')">{{ selectedDataset.sourceWorkTableStatus || '-' }}</span></dd>
            </div>
            <div>
              <dt>{{ t('datasets.lastSynced') }}</dt>
              <dd>{{ formatDate(latestSync?.completedAt || latestSync?.updatedAt) }}</dd>
            </div>
            <div>
              <dt>{{ t('datasets.lastSyncStatus') }}</dt>
              <dd><span class="status-pill" :class="statusClass(latestSync?.status || '')">{{ latestSync?.status || '-' }}</span></dd>
            </div>
          </dl>
        </section>

        <section v-if="isWorkTableDataset" class="panel stack">
          <div class="section-header">
            <div>
              <span class="status-pill">{{ t('datasets.syncHistory') }}</span>
              <h2>{{ t('datasets.syncHistory') }}</h2>
            </div>
          </div>
          <div v-if="datasetStore.syncJobs.length > 0" class="admin-table dataset-column-table">
            <table>
              <thead>
                <tr>
                  <th>{{ t('common.status') }}</th>
                  <th>{{ t('datasets.mode') }}</th>
                  <th>{{ t('datasets.rows') }}</th>
                  <th>{{ t('datasets.oldRawTable') }}</th>
                  <th>{{ t('datasets.newRawTable') }}</th>
                  <th>{{ t('datasets.updated') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="job in datasetStore.syncJobs" :key="job.publicId">
                  <td><span class="status-pill" :class="statusClass(job.status)">{{ job.status }}</span></td>
                  <td>{{ t('datasets.syncModeFullRefresh') }}</td>
                  <td class="tabular-cell">{{ n(job.rowCount) }}</td>
                  <td class="monospace-cell">{{ job.oldRawDatabase }}.{{ job.oldRawTable }}</td>
                  <td class="monospace-cell">{{ job.newRawDatabase }}.{{ job.newRawTable }}</td>
                  <td>
                    {{ formatDate(job.completedAt || job.updatedAt) }}
                    <span v-if="job.errorSummary" class="dataset-query-error-text">{{ job.errorSummary }}</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-else class="empty-state">
            <p>{{ t('datasets.noSyncHistory') }}</p>
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
              <span class="dataset-query-copy">
                <span class="monospace-cell">{{ job.statement }}</span>
                <span v-if="job.status === 'failed'" class="dataset-query-error-text">{{ job.errorSummary || t('datasets.executionFailedFallback') }}</span>
              </span>
              <span class="cell-subtle tabular-cell">{{ formatDate(job.createdAt) }} · {{ job.durationMs }} ms</span>
            </button>
          </div>
          <div v-else class="empty-state">
            <p>{{ t('datasets.noQueryHistory') }}</p>
          </div>
        </section>
      </div>
    </main>

    <div v-else-if="datasetStore.status === 'error'" class="empty-state">
      <p>{{ t('datasets.datasetNotFound') }}</p>
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
    <ConfirmActionDialog
      :open="syncConfirmOpen"
      :title="t('datasets.syncDataset')"
      :message="t('datasets.syncMessage')"
      :confirm-label="t('datasets.requestSync')"
      :cancel-label="t('common.back')"
      @cancel="syncConfirmOpen = false"
      @confirm="confirmSync"
    />
  </div>
</template>
