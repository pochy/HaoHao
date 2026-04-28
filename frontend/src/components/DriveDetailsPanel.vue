<script setup lang="ts">
import { computed, ref } from 'vue'
import { FileText, Info, Lock, ShieldCheck, X } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import type { DriveActivityBody, DriveFolderBody, DriveItemBody, DriveOcrOutputBody, DrivePermissionsBody, DriveProductExtractionItemBody } from '../api/generated/types.gen'
import type { DriveOcrActionStatus } from '../utils/driveOcrStatus'
import {
  driveItemContentType,
  driveItemName,
  driveItemPublicId,
  driveItemUpdatedAt,
  formatDriveDate,
  formatDriveSize,
} from '../utils/driveItems'
import DriveOCRRunStatus from './DriveOCRRunStatus.vue'
import DriveOCRTextViewer from './DriveOCRTextViewer.vue'
import DriveProductExtractionStatus from './DriveProductExtractionStatus.vue'
import DriveProductExtractionTable from './DriveProductExtractionTable.vue'

const props = defineProps<{
  open: boolean
  selectedItem: DriveItemBody | null
  currentFolder: DriveFolderBody
  permissions: DrivePermissionsBody | null
  ocrResult: DriveOcrOutputBody | null
  productExtractionItems: DriveProductExtractionItemBody[]
  ocrLoading: boolean
  ocrActionStatus: DriveOcrActionStatus
  ocrActionResourceId: string
  ocrErrorMessage: string
  productExtractionActionStatus: DriveOcrActionStatus
  productExtractionActionResourceId: string
  productExtractionErrorMessage: string
  busyResourceId: string
  activities: DriveActivityBody[]
  itemCount: number
  fileCount: number
  folderCount: number
}>()

const emit = defineEmits<{
  close: []
  shareItem: [item: DriveItemBody]
  requestOcr: [filePublicId: string]
  requestProductExtraction: [filePublicId: string]
}>()

const { t } = useI18n()
const activeTab = ref<'details' | 'activity' | 'permissions' | 'ocr'>('details')

const title = computed(() => (
  props.selectedItem ? driveItemName(props.selectedItem) : props.currentFolder.publicId === 'root' ? t('drive.root') : props.currentFolder.name
))
const directCount = computed(() => props.permissions?.direct?.length ?? 0)
const inheritedCount = computed(() => props.permissions?.inherited?.length ?? 0)
const selectedDescription = computed(() => props.selectedItem?.file?.description ?? props.selectedItem?.folder?.description ?? '')
const selectedTags = computed(() => props.selectedItem?.tags ?? [])
const selectedFile = computed(() => props.selectedItem?.file ?? null)
const ocrRun = computed(() => props.ocrResult?.run ?? null)
const ocrPages = computed(() => props.ocrResult?.pages ?? [])
const ocrActionApplies = computed(() => Boolean(selectedFile.value && props.ocrActionResourceId === selectedFile.value.publicId))
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
    return t(`drive.ocrStatus.${ocrRun.value.status}`)
  }
  return props.ocrLoading ? t('drive.ocrStatus.loading') : t('drive.ocrStatus.notRun')
})
</script>

<template>
  <aside v-if="open" class="drive-details-panel" :aria-label="t('drive.detailsPanel')">
    <header class="drive-details-header">
      <div>
        <span class="status-pill">{{ t('common.details') }}</span>
        <h2>{{ title }}</h2>
      </div>
      <button class="icon-button" type="button" :aria-label="t('drive.closeDetailsPanel')" :title="t('drive.closeDetailsPanel')" @click="emit('close')">
        <X :size="17" stroke-width="1.9" aria-hidden="true" />
      </button>
    </header>

    <div class="drive-details-tabs" role="tablist" :aria-label="t('drive.detailsSections')">
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
      <button v-if="selectedFile" type="button" :class="{ active: activeTab === 'ocr' }" @click="activeTab = 'ocr'">
        <FileText :size="15" stroke-width="1.9" aria-hidden="true" />
        {{ t('drive.ocr') }}
      </button>
    </div>

    <div v-if="activeTab === 'details'" class="drive-details-section">
      <dl class="metadata-grid compact">
        <div>
          <dt>{{ t('common.publicId') }}</dt>
          <dd class="monospace-cell">{{ selectedItem ? driveItemPublicId(selectedItem) : currentFolder.publicId }}</dd>
        </div>
        <div>
          <dt>{{ t('drive.typeLabel') }}</dt>
          <dd>{{ selectedItem?.folder ? t('drive.folder') : selectedItem?.file ? t('drive.file') : t('drive.folderContext') }}</dd>
        </div>
        <div v-if="selectedItem?.file">
          <dt>{{ t('drive.sort.size') }}</dt>
          <dd>{{ formatDriveSize(selectedItem.file.byteSize) }}</dd>
        </div>
        <div v-if="selectedItem?.file">
          <dt>{{ t('drive.contentType') }}</dt>
          <dd>{{ driveItemContentType(selectedItem) || '-' }}</dd>
        </div>
        <div>
          <dt>{{ t('common.updated') }}</dt>
          <dd>{{ selectedItem ? formatDriveDate(driveItemUpdatedAt(selectedItem)) : formatDriveDate(currentFolder.updatedAt) }}</dd>
        </div>
        <div v-if="selectedItem?.file?.scanStatus">
          <dt>{{ t('drive.scan') }}</dt>
          <dd>{{ selectedItem.file.scanStatus }}</dd>
        </div>
        <div v-if="selectedItem?.ownerDisplayName || selectedItem?.ownerUserPublicId">
          <dt>{{ t('drive.owner') }}</dt>
          <dd>{{ selectedItem.ownerDisplayName || selectedItem.ownerUserPublicId }}</dd>
        </div>
        <div v-if="selectedItem?.source">
          <dt>{{ t('signals.source') }}</dt>
          <dd>{{ selectedItem.source }}</dd>
        </div>
        <div v-if="selectedItem?.shareRole">
          <dt>{{ t('drive.shareRole') }}</dt>
          <dd>{{ selectedItem.shareRole }}</dd>
        </div>
      </dl>

      <div v-if="selectedItem" class="drive-metadata-stack">
        <div>
          <h3>{{ t('drive.description') }}</h3>
          <p class="cell-subtle">{{ selectedDescription || t('drive.noDescription') }}</p>
        </div>
        <div>
          <h3>{{ t('drive.tags') }}</h3>
          <div v-if="selectedTags.length > 0" class="drive-tag-list">
            <span v-for="tag in selectedTags" :key="tag" class="status-pill">{{ tag }}</span>
          </div>
          <p v-else class="cell-subtle">{{ t('drive.noTags') }}</p>
        </div>
      </div>

      <div v-if="!selectedItem" class="drive-details-summary">
        <span>{{ t('drive.itemCount', { count: itemCount }) }}</span>
        <span>{{ t('drive.fileCount', { count: fileCount }) }}</span>
        <span>{{ t('drive.folderCount', { count: folderCount }) }}</span>
      </div>
      <button v-if="selectedItem" class="primary-button compact-button" type="button" @click="emit('shareItem', selectedItem)">
        {{ t('drive.shareItem') }}
      </button>
    </div>

    <div v-else-if="activeTab === 'activity'" class="drive-details-section">
      <div v-if="activities.length > 0" class="drive-activity-list">
        <article v-for="activity in activities" :key="activity.publicId" class="drive-activity-row">
          <strong>{{ activity.action }}</strong>
          <span>{{ activity.actorDisplayName || activity.actorUserPublicId || t('drive.systemActor') }}</span>
          <time :datetime="activity.createdAt">{{ formatDriveDate(activity.createdAt) }}</time>
        </article>
      </div>
      <p v-else class="cell-subtle">
        {{ t('drive.activityEmpty') }}
      </p>
    </div>

    <div v-else-if="activeTab === 'permissions'" class="drive-details-section">
      <div class="drive-details-summary">
        <span>{{ t('drive.directCount', { count: directCount }) }}</span>
        <span>{{ t('drive.inheritedCount', { count: inheritedCount }) }}</span>
      </div>
      <p class="cell-subtle">
        {{ t('drive.permissionsHint') }}
      </p>
      <button v-if="selectedItem" class="secondary-button compact-button" type="button" @click="emit('shareItem', selectedItem)">
        {{ t('drive.manageAccess') }}
      </button>
    </div>

    <div v-else class="drive-details-section">
      <div v-if="selectedFile" class="drive-metadata-stack">
        <DriveOCRRunStatus
          :run="ocrRun"
          :loading="ocrLoading"
          :file-public-id="selectedFile.publicId"
          :busy-resource-id="busyResourceId"
          :action-status="ocrActionStatus"
          :action-resource-id="ocrActionResourceId"
          :error-message="ocrErrorMessage"
          @request-ocr="emit('requestOcr', selectedFile.publicId)"
        />
        <DriveProductExtractionStatus
          :run="ocrRun"
          :items="productExtractionItems"
          :loading="ocrLoading"
          :file-public-id="selectedFile.publicId"
          :busy-resource-id="busyResourceId"
          :action-status="productExtractionActionStatus"
          :action-resource-id="productExtractionActionResourceId"
          :error-message="productExtractionErrorMessage"
          @request-extraction="emit('requestProductExtraction', selectedFile.publicId)"
        />
        <dl class="metadata-grid compact">
          <div>
            <dt>{{ t('common.status') }}</dt>
            <dd>{{ ocrFactStatusLabel }}</dd>
          </div>
          <div v-if="ocrRun">
            <dt>{{ t('drive.ocrEngine') }}</dt>
            <dd>{{ ocrRun.engine }}</dd>
          </div>
          <div v-if="ocrRun">
            <dt>{{ t('drive.ocrPages') }}</dt>
            <dd class="tabular-cell">{{ ocrRun.processedPageCount }} / {{ ocrRun.pageCount }}</dd>
          </div>
          <div v-if="ocrRun?.averageConfidence !== undefined">
            <dt>{{ t('drive.ocrConfidence') }}</dt>
            <dd class="tabular-cell">{{ Math.round((ocrRun.averageConfidence ?? 0) * 100) }}%</dd>
          </div>
          <div v-if="ocrRun?.completedAt">
            <dt>{{ t('drive.ocrCompletedAt') }}</dt>
            <dd>{{ formatDriveDate(ocrRun.completedAt) }}</dd>
          </div>
          <div v-if="ocrRun?.errorCode || ocrRun?.errorMessage">
            <dt>{{ t('drive.ocrReason') }}</dt>
            <dd>{{ ocrRun.errorMessage || ocrRun.errorCode }}</dd>
          </div>
        </dl>

        <div>
          <h3>{{ t('drive.ocrText') }}</h3>
          <DriveOCRTextViewer :pages="ocrPages" :loading="ocrLoading" />
        </div>

        <div>
          <h3>{{ t('drive.productExtractions') }}</h3>
          <p v-if="productExtractionItems.length === 0" class="cell-subtle">{{ t('drive.noProductExtractions') }}</p>
          <DriveProductExtractionTable v-else :items="productExtractionItems" />
        </div>
      </div>
    </div>
  </aside>
</template>
