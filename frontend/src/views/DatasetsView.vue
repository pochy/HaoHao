<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Database, FileText, RefreshCw, Search } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { toApiErrorMessage, toApiErrorRequestId } from '../api/client'
import type { DatasetWorkTableExportFormat } from '../api/datasets'
import type { DatasetLineageGraphSaveBodyWritable, DatasetWorkTableExportScheduleCreateBodyWritable, DatasetWorkTableExportScheduleUpdateBodyWritable } from '../api/generated/types.gen'
import DatasetWorkTableBrowser from '../components/DatasetWorkTableBrowser.vue'
import { useDatasetStore } from '../stores/datasets'
import { useRealtimeStore } from '../stores/realtime'
import { useTenantStore } from '../stores/tenants'

type DatasetHomeTab = 'datasets' | 'workTables'

const datasetStore = useDatasetStore()
const realtimeStore = useRealtimeStore()
const tenantStore = useTenantStore()
const router = useRouter()
const { d, t } = useI18n()

const sourceSearch = ref('')
const datasetName = ref('')
const actionErrorMessage = ref('')
const activeDatasetHomeTab = ref<DatasetHomeTab>('datasets')
let refreshTimer: number | undefined

const selectedSourceFile = computed(() => datasetStore.selectedSourceFile)
const requestErrorMessage = computed(() => actionErrorMessage.value || datasetStore.errorMessage)
const canReviewLineage = computed(() => tenantStore.activeTenant?.roles?.includes('tenant_admin') ?? false)

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
    if (datasetStore.hasActiveImports && !realtimeStore.connected) {
      await datasetStore.load()
    }
    if (datasetStore.hasActiveWorkTableExports) {
      await datasetStore.refreshSelectedWorkTableExports()
      await datasetStore.refreshSelectedWorkTableExportSchedules()
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
    datasetName.value = ''
    datasetStore.reset()
    if (slug) {
      await datasetStore.load()
      await datasetStore.loadWorkTables()
    }
  },
  { immediate: true },
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
    const created = await datasetStore.importFromDriveFile(source.publicId, datasetName.value)
    datasetName.value = ''
    if (created) {
      await router.push({ name: 'dataset-detail', params: { datasetPublicId: created.publicId } })
    }
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function refreshDatasets() {
  actionErrorMessage.value = ''
  await datasetStore.load()
  await datasetStore.loadWorkTables()
}

async function searchSourceFiles() {
  actionErrorMessage.value = ''
  try {
    await datasetStore.refreshSourceFiles(sourceSearch.value)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function registerWorkTable(datasetPublicId: string) {
  actionErrorMessage.value = ''
  try {
    await datasetStore.registerSelectedWorkTable(datasetPublicId)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function linkWorkTable(datasetPublicId: string) {
  actionErrorMessage.value = ''
  try {
    await datasetStore.linkSelectedWorkTable(datasetPublicId)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function renameWorkTable(tableName: string) {
  actionErrorMessage.value = ''
  try {
    await datasetStore.renameSelectedWorkTable(tableName)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function truncateWorkTable() {
  actionErrorMessage.value = ''
  try {
    await datasetStore.truncateSelectedWorkTable()
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function dropWorkTable() {
  actionErrorMessage.value = ''
  try {
    await datasetStore.dropSelectedWorkTable()
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function promoteWorkTable(name: string) {
  actionErrorMessage.value = ''
  try {
    const dataset = await datasetStore.promoteSelectedWorkTable(name)
    if (dataset) {
      await router.push({ name: 'dataset-detail', params: { datasetPublicId: dataset.publicId } })
    }
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function requestWorkTableExport(format: DatasetWorkTableExportFormat) {
  actionErrorMessage.value = ''
  try {
    await datasetStore.requestSelectedWorkTableExport(format)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function createWorkTableExportSchedule(body: DatasetWorkTableExportScheduleCreateBodyWritable) {
  actionErrorMessage.value = ''
  try {
    await datasetStore.createSelectedWorkTableExportSchedule(body)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function updateWorkTableExportSchedule(schedulePublicId: string, body: DatasetWorkTableExportScheduleUpdateBodyWritable) {
  actionErrorMessage.value = ''
  try {
    await datasetStore.updateSelectedWorkTableExportSchedule(schedulePublicId, body)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function disableWorkTableExportSchedule(schedulePublicId: string) {
  actionErrorMessage.value = ''
  try {
    await datasetStore.disableSelectedWorkTableExportSchedule(schedulePublicId)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function setLineageLevel(level: 'table' | 'column' | 'both') {
  datasetStore.setLineageLevel(level)
  await datasetStore.loadSelectedWorkTableLineage()
}

async function toggleLineageSource(source: 'metadata' | 'parser' | 'manual') {
  datasetStore.toggleLineageSource(source)
  await datasetStore.loadSelectedWorkTableLineage()
}

async function saveWorkTableLineageDraft(body: DatasetLineageGraphSaveBodyWritable) {
  actionErrorMessage.value = ''
  try {
    await datasetStore.saveSelectedWorkTableLineageDraft(body)
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

    <main class="dataset-main">
      <div class="dataset-tabs dataset-home-tabs" role="tablist" :aria-label="t('datasets.pageTabs')">
        <button
          type="button"
          role="tab"
          :aria-selected="activeDatasetHomeTab === 'datasets'"
          :class="{ active: activeDatasetHomeTab === 'datasets' }"
          @click="activeDatasetHomeTab = 'datasets'"
        >
          {{ t('datasets.datasets') }}
          <span class="status-pill">{{ datasetStore.items.length }}</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeDatasetHomeTab === 'workTables'"
          :class="{ active: activeDatasetHomeTab === 'workTables' }"
          @click="activeDatasetHomeTab = 'workTables'"
        >
          {{ t('datasets.workTables') }}
          <span class="status-pill">{{ datasetStore.workTables.length }}</span>
        </button>
      </div>

      <div
        v-if="activeDatasetHomeTab === 'datasets'"
        class="dataset-tab-panel"
        role="tabpanel"
      >
        <section class="panel stack dataset-home-panel">
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
            <RouterLink
              v-for="item in datasetStore.items"
              :key="item.publicId"
              class="dataset-row"
              :to="{ name: 'dataset-detail', params: { datasetPublicId: item.publicId } }"
            >
              <Database :size="17" aria-hidden="true" />
              <span>
                <strong>{{ item.name }}</strong>
                <small>{{ item.rawTable }} · {{ formatBytes(item.byteSize) }}</small>
              </span>
              <span class="status-pill" :class="statusClass(item.status)">{{ item.status }}</span>
            </RouterLink>
          </div>

          <div v-else-if="datasetStore.status === 'empty'" class="empty-state">
            <p>{{ t('datasets.empty') }}</p>
          </div>
        </section>
      </div>

      <div
        v-else
        class="dataset-tab-panel"
        role="tabpanel"
      >
        <DatasetWorkTableBrowser
          :tables="datasetStore.workTables"
          :datasets="datasetStore.items"
          :selected-table="datasetStore.selectedWorkTable"
          :preview="datasetStore.workTablePreview"
          :exports="datasetStore.workTableExports"
          :schedules="datasetStore.workTableExportSchedules"
          :medallion-catalog="datasetStore.workTableMedallionCatalog"
          :lineage="datasetStore.workTableLineage"
          :lineage-loading="datasetStore.workTableLineageLoading"
          :lineage-level="datasetStore.lineageLevel"
          :lineage-sources="datasetStore.lineageSources"
          :lineage-action-loading="datasetStore.lineageActionLoading"
          :lineage-draft-source-kind="datasetStore.selectedLineageChangeSet?.changeSet.sourceKind ?? 'manual'"
          :has-lineage-draft="Boolean(datasetStore.selectedLineageChangeSet)"
          :can-publish-lineage="canReviewLineage"
          :loading="datasetStore.workTablesLoading"
          :preview-loading="datasetStore.workTablePreviewLoading"
          :medallion-loading="datasetStore.workTableMedallionLoading"
          :action-loading="datasetStore.workTableActionLoading"
          :error-message="datasetStore.workTableErrorMessage"
          @refresh="datasetStore.loadWorkTables"
          @select="datasetStore.selectWorkTable"
          @register="registerWorkTable"
          @link="linkWorkTable"
          @rename="renameWorkTable"
          @truncate="truncateWorkTable"
          @drop="dropWorkTable"
          @promote="promoteWorkTable"
          @export="requestWorkTableExport"
          @create-schedule="createWorkTableExportSchedule"
          @update-schedule="updateWorkTableExportSchedule"
          @disable-schedule="disableWorkTableExportSchedule"
          @refresh-lineage="datasetStore.loadSelectedWorkTableLineage"
          @set-lineage-level="setLineageLevel"
          @toggle-lineage-source="toggleLineageSource"
          @save-lineage-draft="saveWorkTableLineageDraft"
          @publish-lineage-draft="publishLineageDraft"
          @reject-lineage-draft="rejectLineageDraft"
        />
      </div>
    </main>
  </div>
</template>
