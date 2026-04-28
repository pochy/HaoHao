<script setup lang="ts">
import { computed } from 'vue'
import { Boxes } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import type { DriveOcrRunBody, DriveProductExtractionItemBody } from '../api/generated/types.gen'
import { isDriveOcrActiveStatus, type DriveOcrActionStatus, type DriveOcrTone } from '../utils/driveOcrStatus'

type ProductExtractionStatusKey = 'loading' | 'notRun' | 'waitingForOcr' | 'running' | 'completed' | 'failed' | 'skipped' | 'blocked'

const props = defineProps<{
  run: DriveOcrRunBody | null
  items: DriveProductExtractionItemBody[]
  loading: boolean
  filePublicId: string
  busyResourceId: string
  actionStatus: DriveOcrActionStatus
  actionResourceId: string
  errorMessage: string
}>()

const emit = defineEmits<{
  requestExtraction: []
}>()

const { t } = useI18n()

const runStatus = computed(() => props.run?.status ?? '')
const actionApplies = computed(() => props.actionResourceId === props.filePublicId)
const actionActive = computed(() => actionApplies.value && ['requesting', 'polling'].includes(props.actionStatus))
const itemCount = computed(() => props.items.length)
const statusKey = computed<ProductExtractionStatusKey>(() => {
  if (actionActive.value) {
    return 'running'
  }
  if (actionApplies.value && props.actionStatus === 'failed') {
    return 'failed'
  }
  if (!props.run) {
    return props.loading ? 'loading' : 'notRun'
  }
  if (props.run.status === 'skipped') {
    return 'skipped'
  }
  if (props.run.status === 'failed') {
    return props.run.errorCode === 'product_extraction_failed' ? 'failed' : 'blocked'
  }
  if (isDriveOcrActiveStatus(runStatus.value)) {
    return 'waitingForOcr'
  }
  if (actionApplies.value && props.actionStatus === 'polling' && props.run.status === 'completed' && itemCount.value === 0) {
    return 'running'
  }
  if (props.run.status === 'completed') {
    return 'completed'
  }
  return 'notRun'
})
const statusTone = computed<DriveOcrTone>(() => {
  switch (statusKey.value) {
    case 'completed':
      return 'success'
    case 'failed':
    case 'blocked':
      return 'danger'
    case 'waitingForOcr':
    case 'running':
    case 'loading':
    case 'skipped':
      return 'warning'
    default:
      return 'neutral'
  }
})
const statusPillClass = computed(() => (statusTone.value === 'neutral' ? '' : statusTone.value))
const statusLabel = computed(() => t(`drive.productExtractionStatus.${statusKey.value}`))
const statusDetail = computed(() => t(`drive.productExtractionStatusDetail.${statusKey.value}`, { count: itemCount.value }))
const errorMessage = computed(() => {
  if (actionApplies.value && props.errorMessage) {
    return props.errorMessage
  }
  if (statusKey.value !== 'failed' || !props.run) {
    return ''
  }
  return props.run.errorMessage || props.run.errorCode || ''
})
const requestDisabled = computed(() => (
  props.loading
  || props.run?.status !== 'completed'
  || Boolean(props.filePublicId && props.busyResourceId === props.filePublicId)
  || actionActive.value
))
const requestLabel = computed(() => (
  actionActive.value ? t('drive.productExtractionRunning') : t('drive.productExtractionRun')
))
</script>

<template>
  <section :class="['drive-ocr-run-status', statusTone]" aria-live="polite">
    <div class="drive-ocr-run-status-copy">
      <strong>{{ t('drive.productExtractions') }}</strong>
      <span :class="['status-pill', statusPillClass]">{{ statusLabel }}</span>
      <p>{{ statusDetail }}</p>
      <span v-if="run" class="cell-subtle">
        <Boxes :size="14" stroke-width="1.9" aria-hidden="true" />
        {{ t('drive.productExtractionEngine', { extractor: run.structuredExtractor || '-' }) }}
      </span>
      <p v-if="errorMessage" class="error-message">{{ errorMessage }}</p>
    </div>
    <button
      class="secondary-button compact-button"
      type="button"
      :disabled="requestDisabled"
      :aria-busy="actionActive"
      @click="emit('requestExtraction')"
    >
      <Boxes :size="15" stroke-width="1.9" aria-hidden="true" />
      {{ requestLabel }}
    </button>
  </section>
</template>
