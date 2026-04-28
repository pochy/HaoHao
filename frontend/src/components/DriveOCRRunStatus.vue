<script setup lang="ts">
import { computed } from 'vue'
import { RefreshCw } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import type { DriveOcrRunBody } from '../api/generated/types.gen'
import {
  driveOcrToneFromStatus,
  isDriveOcrActiveStatus,
  type DriveOcrActionStatus,
} from '../utils/driveOcrStatus'

const props = defineProps<{
  run: DriveOcrRunBody | null
  loading: boolean
  filePublicId: string
  busyResourceId: string
  actionStatus: DriveOcrActionStatus
  actionResourceId: string
  errorMessage: string
}>()

const emit = defineEmits<{
  requestOcr: []
}>()

const { t } = useI18n()

const runStatus = computed(() => props.run?.status ?? '')
const actionApplies = computed(() => props.actionResourceId === props.filePublicId)
const actionActive = computed(() => (
  actionApplies.value && ['requesting', 'queued', 'polling'].includes(props.actionStatus)
))
const requestInFlight = computed(() => actionApplies.value && props.actionStatus === 'requesting')
const statusKey = computed(() => {
  if (requestInFlight.value) {
    return 'requesting'
  }
  if (actionApplies.value && props.actionStatus === 'queued' && !runStatus.value) {
    return 'pending'
  }
  if (actionApplies.value && props.actionStatus === 'polling' && (!runStatus.value || isDriveOcrActiveStatus(runStatus.value))) {
    return 'running'
  }
  if (actionApplies.value && props.actionStatus === 'succeeded' && !runStatus.value) {
    return 'completed'
  }
  if (actionApplies.value && props.actionStatus === 'failed' && !runStatus.value) {
    return 'failed'
  }
  if (runStatus.value) {
    return runStatus.value
  }
  if (props.loading) {
    return 'loading'
  }
  return 'notRun'
})
const statusTone = computed(() => {
  if (statusKey.value === 'requesting' || statusKey.value === 'loading') {
    return 'warning'
  }
  return driveOcrToneFromStatus(statusKey.value)
})
const statusPillClass = computed(() => (statusTone.value === 'neutral' ? '' : statusTone.value))
const statusLabel = computed(() => t(`drive.ocrStatus.${statusKey.value}`))
const statusDetail = computed(() => t(`drive.ocrStatusDetail.${statusKey.value}`))
const progressLabel = computed(() => {
  if (!props.run) {
    return ''
  }
  return t('drive.ocrProgress', {
    processed: props.run.processedPageCount,
    total: props.run.pageCount,
  })
})
const scopedErrorMessage = computed(() => (actionApplies.value ? props.errorMessage : ''))
const requestDisabled = computed(() => (
  props.loading
  || Boolean(props.filePublicId && props.busyResourceId === props.filePublicId)
  || actionActive.value
  || isDriveOcrActiveStatus(runStatus.value)
))
const requestLabel = computed(() => {
  if (requestInFlight.value) {
    return t('drive.ocrRequesting')
  }
  if (statusKey.value === 'pending') {
    return t('drive.ocrQueued')
  }
  if (statusKey.value === 'running') {
    return t('drive.ocrRunning')
  }
  return t('drive.ocrRerun')
})
</script>

<template>
  <section :class="['drive-ocr-run-status', statusTone]" aria-live="polite">
    <div class="drive-ocr-run-status-copy">
      <strong>{{ t('drive.ocr') }}</strong>
      <span :class="['status-pill', statusPillClass]">{{ statusLabel }}</span>
      <p>{{ statusDetail }}</p>
      <span v-if="progressLabel" class="cell-subtle tabular-cell">{{ progressLabel }}</span>
      <p v-if="scopedErrorMessage" class="error-message">{{ scopedErrorMessage }}</p>
    </div>
    <button
      class="secondary-button compact-button"
      type="button"
      :disabled="requestDisabled"
      :aria-busy="actionActive || loading"
      @click="emit('requestOcr')"
    >
      <RefreshCw :size="15" stroke-width="1.9" aria-hidden="true" />
      {{ requestLabel }}
    </button>
  </section>
</template>
