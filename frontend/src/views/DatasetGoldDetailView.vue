<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Archive, Crown, RefreshCw, RotateCw } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { toApiErrorMessage, toApiErrorRequestId } from '../api/client'
import ConfirmActionDialog from '../components/ConfirmActionDialog.vue'
import MedallionCatalogPanel from '../components/MedallionCatalogPanel.vue'
import { useDatasetStore } from '../stores/datasets'
import { useRealtimeStore } from '../stores/realtime'
import { useTenantStore } from '../stores/tenants'

type GoldConfirmAction = 'refresh' | 'unpublish' | 'archive' | ''

interface GoldSchemaColumn {
  name: string
  type: string
  ordinal: number
}

const route = useRoute()
const datasetStore = useDatasetStore()
const realtimeStore = useRealtimeStore()
const tenantStore = useTenantStore()
const { d, n, t } = useI18n()

const actionErrorMessage = ref('')
const confirmAction = ref<GoldConfirmAction>('')
let refreshTimer: number | undefined

const goldPublicId = computed(() => {
  const raw = Array.isArray(route.params.goldPublicId)
    ? route.params.goldPublicId[0]
    : route.params.goldPublicId
  return raw ?? ''
})
const publication = computed(() => datasetStore.selectedGoldPublication)
const previewColumns = computed(() => datasetStore.goldPublicationPreview?.columns ?? [])
const previewRows = computed(() => datasetStore.goldPublicationPreview?.previewRows ?? [])
const hasActiveRun = computed(() => datasetStore.goldPublishRuns.some((item) => ['pending', 'processing'].includes(item.status)))
const canRefresh = computed(() => Boolean(publication.value && ['active', 'failed'].includes(publication.value.status) && !hasActiveRun.value))
const canUnpublish = computed(() => Boolean(publication.value && ['active', 'failed', 'pending'].includes(publication.value.status)))
const canArchive = computed(() => Boolean(publication.value && publication.value.status !== 'archived'))
const canPreview = computed(() => publication.value?.status === 'active')
const requestErrorMessage = computed(() => actionErrorMessage.value || datasetStore.goldErrorMessage)
const goldTableLabel = computed(() => {
  const item = publication.value
  return item ? `\`${item.goldDatabase}\`.\`${item.goldTable}\`` : '-'
})
const sourceWorkTableLabel = computed(() => {
  const item = publication.value
  if (!item?.sourceWorkTablePublicId) {
    return '-'
  }
  return item.sourceWorkTablePublicId
})
const schemaColumns = computed(() => {
  const items = publication.value?.schemaSummary?.items
  if (!Array.isArray(items)) {
    return [] as GoldSchemaColumn[]
  }
  return items
    .map(schemaColumnFromValue)
    .filter((item): item is GoldSchemaColumn => Boolean(item))
    .sort((a, b) => a.ordinal - b.ordinal)
})
const latestPublishDate = computed(() => {
  const item = publication.value
  return item?.latestPublishRun?.completedAt || item?.publishedAt
})
const activeTenantLabel = computed(() => (
  tenantStore.activeTenant
    ? `${tenantStore.activeTenant.displayName} / ${tenantStore.activeTenant.slug}`
    : t('datasets.noTenant')
))
const confirmTitle = computed(() => {
  if (confirmAction.value === 'refresh') {
    return t('datasets.refreshGold')
  }
  if (confirmAction.value === 'archive') {
    return t('datasets.archiveGold')
  }
  return t('datasets.unpublishGold')
})
const confirmMessage = computed(() => {
  if (confirmAction.value === 'refresh') {
    return t('datasets.refreshGoldMessage')
  }
  if (confirmAction.value === 'archive') {
    return t('datasets.archiveGoldMessage')
  }
  return t('datasets.unpublishGoldMessage')
})
const confirmLabel = computed(() => {
  if (confirmAction.value === 'refresh') {
    return t('datasets.refreshGold')
  }
  if (confirmAction.value === 'archive') {
    return t('datasets.archiveGold')
  }
  return t('datasets.unpublishGold')
})

onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
  refreshTimer = window.setInterval(async () => {
    if (!realtimeStore.connected && datasetStore.hasActiveGoldPublishRuns) {
      await refreshGoldDetail()
    }
  }, 4000)
})

onBeforeUnmount(() => {
  if (refreshTimer !== undefined) {
    window.clearInterval(refreshTimer)
  }
})

watch(
  () => [tenantStore.activeTenant?.slug, goldPublicId.value],
  async ([slug, publicId]) => {
    actionErrorMessage.value = ''
    confirmAction.value = ''
    datasetStore.selectedGoldPublication = null
    datasetStore.goldPublicationPreview = null
    datasetStore.goldPublishRuns = []
    datasetStore.goldMedallionCatalog = null
    if (!slug || !publicId) {
      return
    }
    try {
      await datasetStore.loadGoldPublication(publicId)
    } catch (error) {
      actionErrorMessage.value = formatActionError(error)
    }
  },
  { immediate: true },
)

function schemaColumnFromValue(value: unknown): GoldSchemaColumn | null {
  if (!value || typeof value !== 'object') {
    return null
  }
  const raw = value as Record<string, unknown>
  if (typeof raw.name !== 'string' || typeof raw.type !== 'string') {
    return null
  }
  return {
    name: raw.name,
    type: raw.type,
    ordinal: typeof raw.ordinal === 'number' ? raw.ordinal : 0,
  }
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
  if (status === 'active' || status === 'completed') {
    return 'success'
  }
  if (status === 'failed') {
    return 'danger'
  }
  if (status === 'unpublished' || status === 'archived') {
    return ''
  }
  return 'warning'
}

async function refreshGoldDetail() {
  if (!goldPublicId.value) {
    return
  }
  actionErrorMessage.value = ''
  try {
    await datasetStore.loadGoldPublication(goldPublicId.value)
    await datasetStore.loadGoldPublications().catch(() => undefined)
  } catch (error) {
    actionErrorMessage.value = formatActionError(error)
  }
}

async function runConfirmAction() {
  const action = confirmAction.value
  confirmAction.value = ''
  if (!goldPublicId.value || !action) {
    return
  }
  actionErrorMessage.value = ''
  try {
    if (action === 'refresh') {
      await datasetStore.refreshSelectedGoldPublication()
      await datasetStore.loadGoldPublishRuns(goldPublicId.value).catch(() => undefined)
    } else if (action === 'unpublish') {
      await datasetStore.unpublishSelectedGoldPublication()
      await datasetStore.loadGoldPublishRuns(goldPublicId.value).catch(() => undefined)
    } else if (action === 'archive') {
      await datasetStore.archiveSelectedGoldPublication()
      await datasetStore.loadGoldPublishRuns(goldPublicId.value).catch(() => undefined)
    }
    await datasetStore.loadGoldPublications().catch(() => undefined)
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
        <span class="status-pill">{{ t('datasets.goldPublication') }}</span>
        <h1>{{ publication?.displayName ?? t('datasets.goldDetailTitle') }}</h1>
        <p>{{ activeTenantLabel }}</p>
      </div>
      <div class="page-header-actions">
        <RouterLink class="secondary-button link-button" to="/datasets">
          {{ t('datasets.backToDatasets') }}
        </RouterLink>
        <button class="secondary-button" :disabled="datasetStore.goldPublicationLoading" type="button" @click="refreshGoldDetail">
          <RefreshCw :size="17" aria-hidden="true" />
          {{ datasetStore.goldPublicationLoading ? t('common.refreshing') : t('common.refresh') }}
        </button>
      </div>
    </header>

    <p v-if="tenantStore.status === 'empty'" class="warning-message">
      {{ t('datasets.noTenantMessage') }}
    </p>
    <p v-if="datasetStore.goldPublicationLoading">
      {{ t('datasets.loadingGoldPublication') }}
    </p>
    <p v-if="requestErrorMessage" class="error-message">
      {{ requestErrorMessage }}
    </p>

    <main v-if="publication" class="dataset-main">
      <section class="panel stack">
        <div class="section-header">
          <div>
            <span class="status-pill" :class="statusClass(publication.status)">{{ publication.status }}</span>
            <h2>{{ t('datasets.goldOverview') }}</h2>
            <span class="cell-subtle monospace-cell">{{ goldTableLabel }}</span>
          </div>
          <div class="page-header-actions">
            <button class="primary-button compact-button" type="button" :disabled="datasetStore.goldActionLoading || !canRefresh" @click="confirmAction = 'refresh'">
              <RotateCw :size="16" aria-hidden="true" />
              {{ t('datasets.refreshGold') }}
            </button>
            <button class="secondary-button compact-button" type="button" :disabled="datasetStore.goldActionLoading || !canUnpublish" @click="confirmAction = 'unpublish'">
              {{ t('datasets.unpublishGold') }}
            </button>
            <button class="secondary-button danger-button compact-button" type="button" :disabled="datasetStore.goldActionLoading || !canArchive" @click="confirmAction = 'archive'">
              <Archive :size="16" aria-hidden="true" />
              {{ t('datasets.archiveGold') }}
            </button>
          </div>
        </div>

        <dl class="metadata-grid dataset-metadata-grid">
          <div>
            <dt>{{ t('common.publicId') }}</dt>
            <dd class="monospace-cell">{{ publication.publicId }}</dd>
          </div>
          <div>
            <dt>{{ t('datasets.goldSqlTable') }}</dt>
            <dd class="monospace-cell">{{ goldTableLabel }}</dd>
          </div>
          <div>
            <dt>{{ t('datasets.sourceWorkTable') }}</dt>
            <dd class="monospace-cell">{{ sourceWorkTableLabel }}</dd>
          </div>
          <div>
            <dt>{{ t('datasets.refreshPolicy') }}</dt>
            <dd>{{ publication.refreshPolicy }}</dd>
          </div>
          <div>
            <dt>{{ t('datasets.rows') }}</dt>
            <dd class="tabular-cell">{{ n(publication.rowCount) }}</dd>
          </div>
          <div>
            <dt>{{ t('datasets.totalBytes') }}</dt>
            <dd class="tabular-cell">{{ formatBytes(publication.totalBytes) }}</dd>
          </div>
          <div>
            <dt>{{ t('datasets.lastPublish') }}</dt>
            <dd>{{ formatDate(latestPublishDate) }}</dd>
          </div>
          <div>
            <dt>{{ t('common.updated') }}</dt>
            <dd>{{ formatDate(publication.updatedAt) }}</dd>
          </div>
        </dl>

        <MedallionCatalogPanel
          :catalog="datasetStore.goldMedallionCatalog"
          :loading="datasetStore.goldMedallionLoading"
          :title="t('medallion.goldTitle')"
        />
      </section>

      <section class="panel stack">
        <div class="section-header">
          <div>
            <span class="status-pill">{{ t('datasets.schema') }}</span>
            <h2>{{ t('datasets.goldSchema') }}</h2>
          </div>
        </div>

        <div v-if="schemaColumns.length > 0" class="admin-table dataset-column-table">
          <table>
            <thead>
              <tr>
                <th>{{ t('datasets.column') }}</th>
                <th>{{ t('datasets.type') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="column in schemaColumns" :key="column.name">
                <td class="monospace-cell">{{ column.name }}</td>
                <td class="monospace-cell">{{ column.type }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else class="empty-state">
          <p>{{ t('datasets.noSchemaSummary') }}</p>
        </div>
      </section>

      <section class="panel stack">
        <div class="section-header">
          <div>
            <span class="status-pill">{{ t('datasets.preview') }}</span>
            <h2>{{ t('datasets.goldPreview') }}</h2>
          </div>
        </div>

        <p v-if="datasetStore.goldPreviewLoading">
          {{ t('datasets.loadingPreview') }}
        </p>
        <p v-else-if="datasetStore.goldPreviewErrorMessage && canPreview" class="error-message">
          {{ datasetStore.goldPreviewErrorMessage }}
        </p>

        <div v-else-if="previewColumns.length > 0 && previewRows.length > 0" class="admin-table dataset-result-table">
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
          <p>{{ canPreview ? t('datasets.noPreviewRows') : t('datasets.goldPreviewUnavailable') }}</p>
        </div>
      </section>

      <section class="panel stack">
        <div class="section-header">
          <div>
            <span class="status-pill">{{ t('datasets.publishHistory') }}</span>
            <h2>{{ t('datasets.goldPublishRuns') }}</h2>
          </div>
        </div>

        <p v-if="datasetStore.goldRunsLoading">
          {{ t('common.loading') }}
        </p>

        <div v-else-if="datasetStore.goldPublishRuns.length > 0" class="admin-table dataset-column-table">
          <table>
            <thead>
              <tr>
                <th>{{ t('common.status') }}</th>
                <th>{{ t('datasets.goldSqlTable') }}</th>
                <th>{{ t('datasets.rows') }}</th>
                <th>{{ t('datasets.started') }}</th>
                <th>{{ t('datasets.completed') }}</th>
                <th>{{ t('datasets.errorSummary') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="run in datasetStore.goldPublishRuns" :key="run.publicId">
                <td><span class="status-pill" :class="statusClass(run.status)">{{ run.status }}</span></td>
                <td class="monospace-cell">`{{ run.goldDatabase }}`.`{{ run.goldTable }}`</td>
                <td class="tabular-cell">{{ n(run.rowCount) }}</td>
                <td>{{ formatDate(run.startedAt || run.createdAt) }}</td>
                <td>{{ formatDate(run.completedAt) }}</td>
                <td>{{ run.errorSummary || '-' }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else class="empty-state">
          <p>{{ t('datasets.noGoldPublishRuns') }}</p>
        </div>
      </section>
    </main>

    <div v-else-if="!datasetStore.goldPublicationLoading" class="empty-state">
      <Crown :size="18" aria-hidden="true" />
      <p>{{ t('datasets.goldPublicationNotFound') }}</p>
    </div>

    <ConfirmActionDialog
      :open="confirmAction !== ''"
      :title="confirmTitle"
      :message="confirmMessage"
      :confirm-label="confirmLabel"
      :cancel-label="t('common.back')"
      @cancel="confirmAction = ''"
      @confirm="runConfirmAction"
    />
  </div>
</template>
