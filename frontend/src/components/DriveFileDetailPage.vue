<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  ClipboardCheck,
  Download,
  Edit3,
  Eye,
  FileText,
  Info,
  Lock,
  RefreshCw,
  Share2,
  ShieldCheck,
  Tags,
  Upload,
} from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { fetchDriveFileDataPipelineReviewItems, type DataPipelineReviewItemBody } from '../api/data-pipelines'
import { refreshDriveFileManifest } from '../api/drive'
import { toApiErrorMessage } from '../api/client'
import type {
  DriveActivityBody,
  DriveFileBody,
  DriveItemBody,
  MedallionCatalogBody,
  DriveOcrOutputBody,
  DrivePermissionsBody,
  DriveProductExtractionItemBody,
} from '../api/generated/types.gen'
import type { DriveOcrActionStatus } from '../utils/driveOcrStatus'
import { driveOcrToneFromStatus } from '../utils/driveOcrStatus'
import {
  driveItemContentType,
  driveItemIsCsv,
  driveItemKind,
  driveItemName,
  driveItemPublicId,
  driveItemUpdatedAt,
  formatDriveDate,
  formatDriveSize,
} from '../utils/driveItems'
import DriveFileTypeIcon from './DriveFileTypeIcon.vue'
import DriveOCRRunStatus from './DriveOCRRunStatus.vue'
import DriveOCRTextViewer from './DriveOCRTextViewer.vue'
import DriveProductExtractionStatus from './DriveProductExtractionStatus.vue'
import DriveProductExtractionTable from './DriveProductExtractionTable.vue'
import MedallionCatalogPanel from './MedallionCatalogPanel.vue'

const props = defineProps<{
  selectedItem: DriveItemBody | null
  permissions: DrivePermissionsBody | null
  ocrResult: DriveOcrOutputBody | null
  productExtractionItems: DriveProductExtractionItemBody[]
  medallionCatalog: MedallionCatalogBody | null
  medallionLoading: boolean
  ocrLoading: boolean
  ocrActionStatus: DriveOcrActionStatus
  ocrActionResourceId: string
  ocrErrorMessage: string
  productExtractionActionStatus: DriveOcrActionStatus
  productExtractionActionResourceId: string
  productExtractionErrorMessage: string
  busyResourceId: string
  activities: DriveActivityBody[]
}>()

const emit = defineEmits<{
  downloadFile: [file: DriveFileBody]
  renameItem: [item: DriveItemBody]
  overwriteFile: [file: DriveFileBody]
  editMetadataItem: [item: DriveItemBody]
  previewItem: [item: DriveItemBody]
  shareItem: [item: DriveItemBody]
  requestOcr: [filePublicId: string]
  requestProductExtraction: [filePublicId: string]
}>()

const { t } = useI18n()
const activeTab = ref<'details' | 'activity' | 'permissions' | 'ocr'>('details')
const manifestRefreshing = ref(false)
const manifestMessage = ref('')
const manifestError = ref('')
const pipelineReviewItems = ref<DataPipelineReviewItemBody[]>([])
const pipelineReviewLoading = ref(false)
const pipelineReviewError = ref('')

const file = computed(() => props.selectedItem?.file ?? null)
const title = computed(() => (props.selectedItem ? driveItemName(props.selectedItem) : t('routes.driveFile')))
const publicId = computed(() => (props.selectedItem ? driveItemPublicId(props.selectedItem) : ''))
const contentType = computed(() => (props.selectedItem ? driveItemContentType(props.selectedItem) : ''))
const previewUrl = computed(() => (file.value ? `/api/v1/drive/files/${encodeURIComponent(file.value.publicId)}/preview` : ''))
const previewKind = computed(() => {
  if (!file.value) {
    return 'none'
  }
  if (contentType.value.startsWith('image/')) {
    return 'image'
  }
  if (contentType.value.includes('pdf')) {
    return 'pdf'
  }
  if (props.selectedItem && driveItemIsCsv(props.selectedItem)) {
    return 'icon'
  }
  if (contentType.value.startsWith('text/') || /\.(md|txt|csv|json)$/i.test(file.value.originalFilename)) {
    return 'text'
  }
  return 'icon'
})
const ocrRun = computed(() => props.ocrResult?.run ?? null)
const ocrPages = computed(() => props.ocrResult?.pages ?? [])
const ocrRunStatusLabel = computed(() => (ocrRun.value ? t(`drive.ocrStatus.${ocrRun.value.status}`) : ''))
const ocrActionApplies = computed(() => Boolean(file.value && props.ocrActionResourceId === file.value.publicId))
const ocrFactStatusLabel = computed(() => {
  if (ocrActionApplies.value) {
    switch (props.ocrActionStatus) {
      case 'requesting':
        return t('drive.ocrStatus.requesting')
      case 'queued':
        return t('drive.ocrStatus.pending')
      case 'polling':
        return t('drive.ocrStatus.running')
      case 'succeeded':
        return t('drive.ocrStatus.completed')
      case 'failed':
        return t('drive.ocrStatus.failed')
    }
  }
  if (ocrRun.value) {
    return ocrRunStatusLabel.value
  }
  return props.ocrLoading ? t('drive.ocrStatus.loading') : t('drive.ocrStatus.notRun')
})
const ocrRunStatusClass = computed(() => {
  const tone = driveOcrToneFromStatus(ocrRun.value?.status)
  return tone === 'neutral' ? '' : tone
})
const directPermissions = computed(() => props.permissions?.direct ?? [])
const inheritedPermissions = computed(() => props.permissions?.inherited ?? [])
const selectedTags = computed(() => props.selectedItem?.tags ?? [])
const ocrText = computed(() => ocrPages.value.map((page) => page.rawText).filter(Boolean).join('\n\n'))
const ocrSummary = computed(() => {
  const text = ocrText.value.trim()
  if (!text) {
    return ''
  }
  return text.length > 900 ? `${text.slice(0, 900)}...` : text
})
const primaryExtraction = computed(() => props.productExtractionItems[0] ?? null)
const selectedDescription = computed(() => (
  file.value?.description
  || primaryExtraction.value?.description
  || ocrSummary.value
  || ''
))
const statusClass = computed(() => {
  if (file.value?.locked || file.value?.dlpBlocked || file.value?.status === 'blocked') {
    return 'danger'
  }
  if (file.value?.scanStatus && file.value.scanStatus !== 'clean') {
    return 'warning'
  }
  return 'success'
})
const productFacts = computed(() => {
  const item = primaryExtraction.value
  if (!item) {
    return []
  }
  return [
    { label: t('drive.brand'), value: item.brand || item.manufacturer || '' },
    { label: t('drive.model'), value: item.model || item.sku || '' },
    { label: t('drive.category'), value: item.category || item.itemType || '' },
    { label: t('drive.janCode'), value: item.janCode || '' },
  ].filter((entry) => entry.value)
})

watch(() => file.value?.publicId ?? '', () => {
  manifestMessage.value = ''
  manifestError.value = ''
  loadPipelineReviewItems()
}, { immediate: true })

function reviewReasonLabel(item: DataPipelineReviewItemBody) {
  const first = item.reason?.[0]
  if (!first) {
    return item.sourceFingerprint || item.queue
  }
  return String(first.message || first.reason || first.code || item.sourceFingerprint || item.queue)
}

async function loadPipelineReviewItems() {
  if (!file.value) {
    pipelineReviewItems.value = []
    pipelineReviewError.value = ''
    return
  }
  pipelineReviewLoading.value = true
  pipelineReviewError.value = ''
  try {
    pipelineReviewItems.value = await fetchDriveFileDataPipelineReviewItems(file.value.publicId, { limit: 5 })
  } catch (error) {
    pipelineReviewItems.value = []
    pipelineReviewError.value = toApiErrorMessage(error)
  } finally {
    pipelineReviewLoading.value = false
  }
}

async function refreshManifest() {
  if (!file.value || manifestRefreshing.value) {
    return
  }
  manifestRefreshing.value = true
  manifestMessage.value = ''
  manifestError.value = ''
  try {
    const manifest = await refreshDriveFileManifest(file.value.publicId)
    manifestMessage.value = t('drive.manifestRefreshSuccess', { type: manifest.documentType })
  } catch (error) {
    manifestError.value = toApiErrorMessage(error)
  } finally {
    manifestRefreshing.value = false
  }
}

</script>

<template>
  <section v-if="selectedItem && file" class="drive-file-detail-page">
    <div class="drive-file-detail-bar">
      <strong>{{ t('drive.fileDetails') }}</strong>
      <nav aria-label="Drive file breadcrumb">
        <RouterLink to="/drive">Drive</RouterLink>
        <span aria-hidden="true">/</span>
        <span>{{ t('drive.fileDetails') }}</span>
      </nav>
    </div>

    <article class="drive-file-detail-surface">
      <aside class="drive-file-detail-media">
        <div class="drive-file-detail-preview" :class="previewKind">
          <img v-if="previewKind === 'image'" :src="previewUrl" :alt="title">
          <iframe v-else-if="previewKind === 'pdf' || previewKind === 'text'" :src="previewUrl" :title="title" />
          <div v-else class="drive-file-detail-icon-preview">
            <DriveFileTypeIcon :kind="driveItemKind(selectedItem)" :size="52" />
            <span>{{ file.originalFilename.split('.').pop() || t('drive.file') }}</span>
          </div>
        </div>

        <div class="drive-file-detail-thumbs" aria-hidden="true">
          <span v-for="index in 4" :key="index">
            <DriveFileTypeIcon :kind="driveItemKind(selectedItem)" :size="22" />
          </span>
        </div>

        <div class="drive-file-detail-actions">
          <button class="secondary-button compact-button" type="button" @click="emit('previewItem', selectedItem)">
            <Eye :size="16" stroke-width="1.9" aria-hidden="true" />
            {{ t('drive.preview') }}
          </button>
          <button class="secondary-button compact-button" type="button" @click="emit('downloadFile', file)">
            <Download :size="16" stroke-width="1.9" aria-hidden="true" />
            {{ t('common.download') }}
          </button>
          <button class="secondary-button compact-button" type="button" :disabled="file.locked" @click="emit('renameItem', selectedItem)">
            <Edit3 :size="16" stroke-width="1.9" aria-hidden="true" />
            {{ t('drive.rename') }}
          </button>
          <button class="secondary-button compact-button" type="button" :disabled="file.locked" @click="emit('editMetadataItem', selectedItem)">
            <Tags :size="16" stroke-width="1.9" aria-hidden="true" />
            {{ t('drive.metadata') }}
          </button>
          <button class="secondary-button compact-button" type="button" :disabled="manifestRefreshing" @click="refreshManifest">
            <RefreshCw :size="16" stroke-width="1.9" aria-hidden="true" />
            {{ manifestRefreshing ? t('drive.manifestRefreshing') : t('drive.refreshManifest') }}
          </button>
          <button class="secondary-button compact-button" type="button" :disabled="file.locked" @click="emit('overwriteFile', file)">
            <Upload :size="16" stroke-width="1.9" aria-hidden="true" />
            {{ t('drive.replace') }}
          </button>
          <button class="primary-button compact-button" type="button" @click="emit('shareItem', selectedItem)">
            <Share2 :size="16" stroke-width="1.9" aria-hidden="true" />
            {{ t('drive.share') }}
          </button>
        </div>
      </aside>

      <div class="drive-file-detail-info">
        <div class="drive-file-detail-title-row">
          <span class="status-pill" :class="statusClass">{{ file.status }}</span>
          <span v-if="ocrRun" class="status-pill" :class="ocrRunStatusClass">{{ ocrRunStatusLabel }}</span>
        </div>

        <h1>{{ title }}</h1>

        <div class="drive-file-detail-tabs" role="tablist" :aria-label="t('drive.detailsSections')">
          <button type="button" :class="{ active: activeTab === 'details' }" @click="activeTab = 'details'">
            <Info :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('common.details') }}
          </button>
          <button type="button" :class="{ active: activeTab === 'activity' }" @click="activeTab = 'activity'">
            <Lock :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('drive.activity') }}
          </button>
          <button type="button" :class="{ active: activeTab === 'permissions' }" @click="activeTab = 'permissions'">
            <ShieldCheck :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('drive.permissions') }}
          </button>
          <button type="button" :class="{ active: activeTab === 'ocr' }" @click="activeTab = 'ocr'">
            <FileText :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('drive.ocr') }}
          </button>
        </div>

        <div v-if="activeTab === 'details'" class="drive-file-detail-tab-panel">
          <dl class="drive-file-detail-metrics">
            <div>
              <dt>{{ t('common.publicId') }}</dt>
              <dd class="monospace-cell">{{ publicId }}</dd>
            </div>
            <div>
              <dt>{{ t('drive.contentType') }}</dt>
              <dd>{{ contentType || '-' }}</dd>
            </div>
            <div>
              <dt>{{ t('drive.sort.size') }}</dt>
              <dd class="tabular-cell">{{ formatDriveSize(file.byteSize) }}</dd>
            </div>
            <div>
              <dt>{{ t('common.updated') }}</dt>
              <dd>{{ formatDriveDate(driveItemUpdatedAt(selectedItem)) }}</dd>
            </div>
            <div>
              <dt>{{ t('drive.scan') }}</dt>
              <dd>{{ file.scanStatus || '-' }}</dd>
            </div>
            <div v-if="selectedItem.source">
              <dt>{{ t('signals.source') }}</dt>
              <dd>{{ selectedItem.source }}</dd>
            </div>
          </dl>

          <p v-if="manifestMessage" class="drive-file-detail-notice success">{{ manifestMessage }}</p>
          <p v-if="manifestError" class="drive-file-detail-notice error">{{ manifestError }}</p>

          <div class="drive-file-detail-primary-metric">
            <span>{{ t('drive.sort.size') }}</span>
            <strong class="tabular-cell">{{ formatDriveSize(file.byteSize) }}</strong>
            <small v-if="ocrRun">{{ t('drive.ocrProgress', { processed: ocrRun.processedPageCount, total: ocrRun.pageCount }) }}</small>
          </div>

          <MedallionCatalogPanel
            :catalog="medallionCatalog"
            :loading="medallionLoading"
            :title="t('medallion.driveTitle')"
          />

          <section class="drive-file-detail-section">
            <h2>{{ primaryExtraction ? t('drive.productInfo') : t('drive.description') }}</h2>
            <p>{{ selectedDescription || t('drive.noDescription') }}</p>
            <ul v-if="productFacts.length > 0">
              <li v-for="fact in productFacts" :key="fact.label">
                <strong>{{ fact.label }}:</strong> {{ fact.value }}
              </li>
            </ul>
          </section>

          <section class="drive-file-detail-section">
            <h2>{{ t('drive.tags') }}</h2>
            <div v-if="selectedTags.length > 0" class="drive-tag-list">
              <span v-for="tag in selectedTags" :key="tag" class="status-pill">{{ tag }}</span>
            </div>
            <p v-else class="cell-subtle">{{ t('drive.noTags') }}</p>
          </section>

          <section class="drive-file-detail-section">
            <div class="drive-file-detail-section-heading">
              <h2>{{ t('drive.pipelineReviews') }}</h2>
              <button class="secondary-button compact-button" type="button" :disabled="pipelineReviewLoading" @click="loadPipelineReviewItems">
                <RefreshCw :size="15" stroke-width="1.9" aria-hidden="true" />
                {{ t('common.refresh') }}
              </button>
            </div>
            <p v-if="pipelineReviewLoading" class="cell-subtle">{{ t('common.loading') }}</p>
            <p v-else-if="pipelineReviewError" class="drive-file-detail-notice error">{{ pipelineReviewError }}</p>
            <p v-else-if="pipelineReviewItems.length === 0" class="cell-subtle">{{ t('drive.noPipelineReviews') }}</p>
            <div v-else class="drive-file-detail-review-list">
              <article v-for="item in pipelineReviewItems" :key="item.publicId">
                <div>
                  <strong>{{ item.pipelineName || t('routes.dataPipelineDetail') }}</strong>
                  <span>{{ item.nodeId }} / {{ item.queue }}</span>
                  <span>{{ reviewReasonLabel(item) }}</span>
                </div>
                <span class="status-pill" :class="item.status === 'open' ? 'warning' : 'success'">{{ item.status }}</span>
                <RouterLink
                  v-if="item.pipelinePublicId"
                  class="secondary-button compact-button link-button"
                  :to="`/data-pipelines/${item.pipelinePublicId}`"
                >
                  <ClipboardCheck :size="15" stroke-width="1.9" aria-hidden="true" />
                  {{ t('drive.openPipelineReview') }}
                </RouterLink>
              </article>
            </div>
          </section>
        </div>

        <section v-else-if="activeTab === 'activity'" class="drive-file-detail-bottom-panel">
          <h2>{{ t('drive.activity') }}</h2>
          <p v-if="activities.length === 0" class="cell-subtle">{{ t('drive.activityEmpty') }}</p>
          <div v-else class="drive-file-detail-activity-list">
            <article v-for="activity in activities" :key="activity.publicId">
              <strong>{{ activity.action }}</strong>
              <span>{{ activity.actorDisplayName || activity.actorUserPublicId || t('drive.systemActor') }}</span>
              <time :datetime="activity.createdAt">{{ formatDriveDate(activity.createdAt) }}</time>
            </article>
          </div>
        </section>

        <section v-else-if="activeTab === 'permissions'" class="drive-file-detail-tab-panel">
          <div class="drive-details-summary">
            <span>{{ t('drive.directCount', { count: directPermissions.length }) }}</span>
            <span>{{ t('drive.inheritedCount', { count: inheritedPermissions.length }) }}</span>
            <span>{{ selectedItem.shareRole || t('common.none') }}</span>
          </div>
          <p class="cell-subtle">{{ t('drive.permissionsHint') }}</p>
          <button class="secondary-button compact-button drive-file-detail-manage-button" type="button" @click="emit('shareItem', selectedItem)">
            <ShieldCheck :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('drive.manageAccess') }}
          </button>
          <div class="drive-file-detail-permission-list">
            <p v-if="directPermissions.length === 0 && inheritedPermissions.length === 0" class="cell-subtle">
              {{ t('drive.permissionsHint') }}
            </p>
            <article v-for="permission in directPermissions" :key="`direct-${permission.publicId || `${permission.kind}-${permission.subjectId}`}`">
              <strong>{{ permission.subjectType || permission.kind }}</strong>
              <span>{{ permission.role }} / {{ permission.source }}</span>
            </article>
            <article v-for="permission in inheritedPermissions" :key="`inherited-${permission.publicId || `${permission.kind}-${permission.inheritedFromId}`}`">
              <strong>{{ permission.subjectType || permission.kind }}</strong>
              <span>{{ permission.role }} / {{ permission.source }}</span>
            </article>
          </div>
        </section>

        <div v-else class="drive-file-detail-tab-panel">
          <div class="drive-file-detail-section-heading">
            <h2>{{ t('drive.ocr') }}</h2>
          </div>
          <DriveOCRRunStatus
            :run="ocrRun"
            :loading="ocrLoading"
            :file-public-id="file.publicId"
            :busy-resource-id="busyResourceId"
            :action-status="ocrActionStatus"
            :action-resource-id="ocrActionResourceId"
            :error-message="ocrErrorMessage"
            @request-ocr="emit('requestOcr', file.publicId)"
          />
          <DriveProductExtractionStatus
            :run="ocrRun"
            :items="productExtractionItems"
            :loading="ocrLoading"
            :file-public-id="file.publicId"
            :busy-resource-id="busyResourceId"
            :action-status="productExtractionActionStatus"
            :action-resource-id="productExtractionActionResourceId"
            :error-message="productExtractionErrorMessage"
            @request-extraction="emit('requestProductExtraction', file.publicId)"
          />
          <dl class="drive-file-detail-inline-facts">
            <div>
              <dt>{{ t('common.status') }}</dt>
              <dd>{{ ocrFactStatusLabel }}</dd>
            </div>
            <div>
              <dt>{{ t('drive.ocrEngine') }}</dt>
              <dd>{{ ocrRun?.engine || '-' }}</dd>
            </div>
            <div>
              <dt>{{ t('drive.ocrConfidence') }}</dt>
              <dd>{{ ocrRun?.averageConfidence !== undefined ? `${Math.round((ocrRun.averageConfidence ?? 0) * 100)}%` : '-' }}</dd>
            </div>
            <div>
              <dt>{{ t('drive.ocrCompletedAt') }}</dt>
              <dd>{{ ocrRun?.completedAt ? formatDriveDate(ocrRun.completedAt) : '-' }}</dd>
            </div>
          </dl>
          <section class="drive-file-detail-section">
            <h2>{{ t('drive.ocrText') }}</h2>
            <DriveOCRTextViewer :pages="ocrPages" :loading="ocrLoading" />
          </section>
          <section class="drive-file-detail-bottom-panel">
            <h2>{{ t('drive.productExtractions') }}</h2>
            <p v-if="productExtractionItems.length === 0" class="cell-subtle">{{ t('drive.noProductExtractions') }}</p>
            <DriveProductExtractionTable v-else :items="productExtractionItems" />
          </section>
        </div>
      </div>
    </article>
  </section>

  <section v-else class="data-card">
    <p class="cell-subtle">{{ t('common.loading') }}</p>
  </section>
</template>
