<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref } from 'vue'
import { Info, Search, Trash2 } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import type { DataPipelinePreviewBody, DataPipelineReviewItemBody, DataPipelineRunBody, DataPipelineRunStepBody, DataPipelineScheduleBody } from '../api/data-pipelines'

const props = defineProps<{
  preview: DataPipelinePreviewBody | null
  runs: DataPipelineRunBody[]
  reviewItems: DataPipelineReviewItemBody[]
  schedules: DataPipelineScheduleBody[]
  loading?: boolean
  actionLoading?: boolean
  canPreview?: boolean
  draftRunPreview?: boolean
  previewDisabledReason?: string
}>()

const emit = defineEmits<{
  preview: []
  disableSchedule: [publicId: string]
}>()

type DataPipelinePanelTab = 'preview' | 'runs' | 'reviews' | 'schedules'
type PreviewCellDialog = {
  column: string
  value: string
}
type StepMetadataDialog = {
  title: string
  subtitle: string
  facts: Array<{ label: string, value: string }>
  value: string
}

const PREVIEW_CELL_TRUNCATE_LENGTH = 100
const activeTab = ref<DataPipelinePanelTab>('preview')
const previewCellDialog = ref<PreviewCellDialog | null>(null)
const previewCellDialogRef = ref<HTMLDialogElement | null>(null)
const stepMetadataDialog = ref<StepMetadataDialog | null>(null)
const stepMetadataDialogRef = ref<HTMLDialogElement | null>(null)
const { d, t } = useI18n()

onBeforeUnmount(() => {
  if (previewCellDialogRef.value?.open) {
    previewCellDialogRef.value.close()
  }
  if (stepMetadataDialogRef.value?.open) {
    stepMetadataDialogRef.value.close()
  }
})

function previewClick() {
  activeTab.value = 'preview'
  emit('preview')
}

function previewCellText(value: unknown) {
  if (value === null || value === undefined) {
    return '-'
  }

  if (typeof value === 'string') {
    return value
  }

  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') {
    return String(value)
  }

  try {
    return JSON.stringify(value) ?? String(value)
  } catch {
    return String(value)
  }
}

function previewCellCharacters(value: unknown) {
  return Array.from(previewCellText(value))
}

function isPreviewCellLong(value: unknown) {
  return previewCellCharacters(value).length >= PREVIEW_CELL_TRUNCATE_LENGTH
}

function previewCellDisplay(value: unknown) {
  const text = previewCellText(value)
  const characters = Array.from(text)
  if (characters.length < PREVIEW_CELL_TRUNCATE_LENGTH) {
    return text
  }

  return `${characters.slice(0, PREVIEW_CELL_TRUNCATE_LENGTH).join('')}...`
}

function previewCellDialogText(value: unknown) {
  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
      try {
        return JSON.stringify(JSON.parse(trimmed), null, 2)
      } catch {
        return value
      }
    }

    return value
  }

  if (value !== null && typeof value === 'object') {
    try {
      return JSON.stringify(value, null, 2) ?? previewCellText(value)
    } catch {
      return previewCellText(value)
    }
  }

  return previewCellText(value)
}

async function openPreviewCellDialog(column: string, value: unknown) {
  if (!isPreviewCellLong(value)) {
    return
  }

  previewCellDialog.value = {
    column,
    value: previewCellDialogText(value),
  }
  await nextTick()
  if (!previewCellDialogRef.value?.open) {
    previewCellDialogRef.value?.showModal()
  }
}

function closePreviewCellDialog() {
  if (previewCellDialogRef.value?.open) {
    previewCellDialogRef.value.close()
    return
  }

  previewCellDialog.value = null
}

function handlePreviewCellDialogClose() {
  previewCellDialog.value = null
}

function stringifyMetadata(value: unknown) {
  try {
    return JSON.stringify(value, null, 2) ?? previewCellText(value)
  } catch {
    return previewCellText(value)
  }
}

function metadataValue(value: unknown) {
  if (value === null || value === undefined || value === '') {
    return '-'
  }
  if (typeof value === 'number') {
    return Number.isInteger(value) ? String(value) : value.toFixed(4)
  }
  return previewCellText(value)
}

function metadataArray(metadata: Record<string, unknown>, key: string) {
  const value = metadata[key]
  return Array.isArray(value) ? value : []
}

function metadataFacts(step: DataPipelineRunStepBody) {
  const metadata = step.metadata ?? {}
  const facts: Array<{ label: string, value: string }> = [
    { label: 'inputRows', value: metadataValue(metadata.inputRows) },
    { label: 'outputRows', value: metadataValue(metadata.outputRows ?? step.rowCount) },
    { label: 'warningCount', value: metadataValue(metadata.warningCount) },
    { label: 'failedRows', value: metadataValue(metadata.failedRows) },
  ]
  const profile = metadataRecord(metadata, 'profile')
  if (profile) {
    facts.push({ label: 'profile.rows', value: metadataValue(profile.rowCount) })
    facts.push({ label: 'profile.columns', value: metadataValue(profile.columnCount) })
  }
  const validation = metadataRecord(metadata, 'validation')
  if (validation) {
    facts.push({ label: 'validation.rules', value: metadataValue(validation.ruleCount) })
    facts.push({ label: 'validation.errors', value: metadataValue(validation.errorCount) })
    facts.push({ label: 'validation.warnings', value: metadataValue(validation.warningCount) })
  }
  const quality = metadataRecord(metadata, 'quality')
  if (quality) {
    facts.push({ label: 'quality.rows', value: metadataValue(quality.rowCount) })
    facts.push({ label: 'quality.columns', value: metadataValue(quality.columnCount) })
  }
  const confidenceGate = metadataRecord(metadata, 'confidenceGate')
  if (confidenceGate) {
    facts.push({ label: 'confidenceGate.threshold', value: metadataValue(confidenceGate.threshold) })
    facts.push({ label: 'confidenceGate.passRows', value: metadataValue(confidenceGate.passRows) })
    facts.push({ label: 'confidenceGate.needsReviewRows', value: metadataValue(confidenceGate.needsReviewRows) })
  }
  if (metadata.quarantinedRows !== undefined || metadata.passedRows !== undefined) {
    facts.push({ label: 'quarantine.quarantinedRows', value: metadataValue(metadata.quarantinedRows) })
    facts.push({ label: 'quarantine.passedRows', value: metadataValue(metadata.passedRows) })
  }
  const queryStats = metadataRecord(metadata, 'queryStats')
  if (queryStats) {
    facts.push({ label: 'queryStats.queryId', value: metadataValue(queryStats.queryId) })
    facts.push({ label: 'queryStats.elapsedMs', value: metadataValue(queryStats.elapsedMs) })
    facts.push({ label: 'queryStats.readRows', value: metadataValue(queryStats.readRows) })
  }
  return facts
}

function hasStepMetadata(step: DataPipelineRunStepBody) {
  return Object.keys(step.metadata ?? {}).length > 0
}

async function openStepMetadataDialog(step: DataPipelineRunStepBody) {
  const metadata = step.metadata ?? {}
  stepMetadataDialog.value = {
    title: `${stepLabel(step.stepType)}: ${step.nodeId}`,
    subtitle: stepMetadataSummary(step),
    facts: metadataFacts(step),
    value: stringifyMetadata({
      ...metadata,
      warnings: metadataArray(metadata, 'warnings'),
    }),
  }
  await nextTick()
  if (!stepMetadataDialogRef.value?.open) {
    stepMetadataDialogRef.value?.showModal()
  }
}

function closeStepMetadataDialog() {
  if (stepMetadataDialogRef.value?.open) {
    stepMetadataDialogRef.value.close()
    return
  }
  stepMetadataDialog.value = null
}

function handleStepMetadataDialogClose() {
  stepMetadataDialog.value = null
}

function formatDate(value?: string | null) {
  return value ? d(new Date(value), 'long') : '-'
}

function statusClass(status: string) {
  if (['completed', 'ready', 'created'].includes(status)) return 'success'
  if (['failed', 'disabled'].includes(status)) return 'danger'
  return 'warning'
}

function statusLabel(status: string) {
  return knownStatusValues.has(status) ? t(`dataPipelines.statusValue.${status}`) : status
}

function triggerLabel(trigger: string) {
  return knownTriggerKinds.has(trigger) ? t(`dataPipelines.triggerKind.${trigger}`) : trigger
}

function runOutputs(run: DataPipelineRunBody) {
  return run.outputs?.length ? run.outputs : []
}

function runSteps(run: DataPipelineRunBody) {
  return run.steps?.length ? run.steps : []
}

function reviewReasonLabel(item: DataPipelineReviewItemBody) {
  const reason = item.reason?.[0]
  if (!reason) return '-'
  const column = previewCellText(reason.column)
  const value = previewCellText(reason.value)
  return value && value !== '-' ? `${column}: ${value}` : column
}

function stepLabel(stepType: string) {
  const key = `dataPipelines.step.${stepType}`
  const translated = t(key)
  return translated === key ? stepType : translated
}

function metadataNumber(metadata: Record<string, unknown>, key: string) {
  const value = metadata[key]
  return typeof value === 'number' ? value : null
}

function metadataRecord(metadata: Record<string, unknown>, key: string) {
  const value = metadata[key]
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : null
}

function stepMetadataSummary(step: DataPipelineRunStepBody) {
  const metadata = step.metadata ?? {}
  const parts: string[] = []
  const warningCount = metadataNumber(metadata, 'warningCount')
  const failedRows = metadataNumber(metadata, 'failedRows')
  if (warningCount && warningCount > 0) {
    parts.push(t('dataPipelines.warningCount', { count: warningCount }))
  }
  if (failedRows && failedRows > 0) {
    parts.push(t('dataPipelines.failedRowsCount', { count: failedRows }))
  }
  const profile = metadataRecord(metadata, 'profile')
  if (profile) {
    parts.push(t('dataPipelines.profileSummary', {
      rows: profile.rowCount ?? metadata.outputRows ?? step.rowCount,
      columns: profile.columnCount ?? '-',
    }))
  }
  const validation = metadataRecord(metadata, 'validation')
  if (validation) {
    parts.push(t('dataPipelines.validationSummary', {
      failed: validation.failedRows ?? 0,
      errors: validation.errorCount ?? 0,
      warnings: validation.warningCount ?? 0,
    }))
  }
  const confidenceGate = metadataRecord(metadata, 'confidenceGate')
  if (confidenceGate) {
    parts.push(t('dataPipelines.confidenceGateSummary', {
      pass: confidenceGate.passRows ?? 0,
      review: confidenceGate.needsReviewRows ?? 0,
    }))
  }
  const quality = metadataRecord(metadata, 'quality')
  if (quality) {
    parts.push(t('dataPipelines.qualitySummary', {
      rows: quality.rowCount ?? metadata.outputRows ?? step.rowCount,
      columns: quality.columnCount ?? '-',
    }))
  }
  const quarantinedRows = metadataNumber(metadata, 'quarantinedRows')
  const passedRows = metadataNumber(metadata, 'passedRows')
  if (quarantinedRows !== null || passedRows !== null) {
    parts.push(t('dataPipelines.quarantineSummary', {
      quarantined: quarantinedRows ?? 0,
      passed: passedRows ?? 0,
    }))
  }
  return parts.length ? parts.join(' · ') : '-'
}

function scheduleFrequencyLabel(frequency: string) {
  switch (frequency) {
  case 'daily':
    return t('dataPipelines.daily')
  case 'weekly':
    return t('dataPipelines.weekly')
  case 'monthly':
    return t('dataPipelines.monthly')
  default:
    return frequency
  }
}

const knownStatusValues = new Set(['pending', 'processing', 'completed', 'failed', 'ready', 'created', 'active', 'disabled', 'published', 'archived', 'open', 'approved', 'rejected', 'needs_changes', 'closed'])
const knownTriggerKinds = new Set(['manual', 'scheduled'])
</script>

<template>
  <section class="data-pipeline-bottom-panel">
    <div class="data-pipeline-panel-toolbar">
      <div class="data-pipeline-panel-tabs" role="tablist" :aria-label="t('dataPipelines.lowerPanel')">
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'preview'"
          :class="{ active: activeTab === 'preview' }"
          @click="activeTab = 'preview'"
        >
          {{ t('dataPipelines.preview') }}
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'runs'"
          :class="{ active: activeTab === 'runs' }"
          @click="activeTab = 'runs'"
        >
          {{ t('dataPipelines.runs') }}
          <span>{{ runs.length }}</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'reviews'"
          :class="{ active: activeTab === 'reviews' }"
          @click="activeTab = 'reviews'"
        >
          {{ t('dataPipelines.reviewItems') }}
          <span>{{ reviewItems.length }}</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'schedules'"
          :class="{ active: activeTab === 'schedules' }"
          @click="activeTab = 'schedules'"
        >
          {{ t('dataPipelines.schedules') }}
          <span>{{ schedules.length }}</span>
        </button>
      </div>

      <p v-if="activeTab === 'preview' && !canPreview && previewDisabledReason" class="muted-panel data-pipeline-panel-notice">
        {{ previewDisabledReason }}
      </p>
      <p v-else-if="activeTab === 'preview' && draftRunPreview" class="muted-panel data-pipeline-panel-notice">
        {{ t('dataPipelines.draftRunPreviewNotice') }}
      </p>
      <span v-else class="data-pipeline-panel-notice" aria-hidden="true"></span>

      <button
        v-if="activeTab === 'preview'"
        class="secondary-button data-pipeline-preview-action"
        type="button"
        :disabled="loading || !canPreview"
        :title="props.previewDisabledReason"
        @click="previewClick"
      >
        <Search :size="16" stroke-width="1.9" aria-hidden="true" />
        {{ loading ? t(draftRunPreview ? 'dataPipelines.draftRunPreviewing' : 'dataPipelines.previewing') : t(draftRunPreview ? 'dataPipelines.draftRunPreview' : 'dataPipelines.preview') }}
      </button>
    </div>

    <div v-if="activeTab === 'preview'" class="data-pipeline-panel-section" role="tabpanel">
      <div v-if="preview" class="dataset-work-table-preview-table data-pipeline-preview-table">
        <table>
          <thead>
            <tr>
              <th v-for="column in preview.columns" :key="column">{{ column }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, index) in preview.previewRows" :key="index">
              <td
                v-for="column in preview.columns"
                :key="column"
                :class="{ 'data-pipeline-preview-cell-truncated': isPreviewCellLong(row[column]) }"
                :tabindex="isPreviewCellLong(row[column]) ? 0 : undefined"
                :role="isPreviewCellLong(row[column]) ? 'button' : undefined"
                :aria-label="isPreviewCellLong(row[column]) ? t('dataPipelines.openPreviewCellValue', { column }) : undefined"
                @dblclick="openPreviewCellDialog(column, row[column])"
                @keydown.enter="openPreviewCellDialog(column, row[column])"
                @keydown.space.prevent="openPreviewCellDialog(column, row[column])"
              >
                <span :class="{ 'data-pipeline-preview-cell-text': isPreviewCellLong(row[column]) }">
                  {{ previewCellDisplay(row[column]) }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else class="muted-panel">{{ t('dataPipelines.noPreviewResult') }}</p>
    </div>

    <div v-else-if="activeTab === 'runs'" class="data-pipeline-panel-section" role="tabpanel">
      <div class="compact-table">
        <table>
          <thead>
            <tr>
              <th>{{ t('dataPipelines.status') }}</th>
              <th>{{ t('dataPipelines.node') }}</th>
              <th>{{ t('dataPipelines.trigger') }}</th>
              <th>{{ t('dataPipelines.rows') }}</th>
              <th>{{ t('dataPipelines.created') }}</th>
              <th>{{ t('dataPipelines.error') }}</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="run in runs" :key="run.publicId">
              <tr>
                <td><span class="status-pill" :class="statusClass(run.status)">{{ statusLabel(run.status) }}</span></td>
                <td>{{ t('dataPipelines.run') }}</td>
                <td>{{ triggerLabel(run.triggerKind) }}</td>
                <td>{{ run.rowCount }}</td>
                <td>{{ formatDate(run.createdAt) }}</td>
                <td>{{ run.errorSummary || '-' }}</td>
              </tr>
              <tr v-for="output in runOutputs(run)" :key="`${run.publicId}-${output.nodeId}`">
                <td><span class="status-pill" :class="statusClass(output.status)">{{ statusLabel(output.status) }}</span></td>
                <td>{{ t('dataPipelines.output') }}: {{ output.nodeId }}</td>
                <td>{{ output.outputWorkTableId ?? '-' }}</td>
                <td>{{ output.rowCount }}</td>
                <td>-</td>
                <td>{{ output.errorSummary || '-' }}</td>
              </tr>
              <tr v-for="step in runSteps(run)" :key="`${run.publicId}-step-${step.nodeId}`">
                <td><span class="status-pill" :class="statusClass(step.status)">{{ statusLabel(step.status) }}</span></td>
                <td>{{ stepLabel(step.stepType) }}: {{ step.nodeId }}</td>
                <td class="cell-subtle">{{ stepMetadataSummary(step) }}</td>
                <td>{{ step.rowCount }}</td>
                <td>{{ formatDate(step.completedAt || step.updatedAt) }}</td>
                <td>
                  <span>{{ step.errorSummary || '-' }}</span>
                  <button
                    v-if="hasStepMetadata(step)"
                    class="icon-button"
                    type="button"
                    :aria-label="t('dataPipelines.stepMetadataDetails')"
                    @click="openStepMetadataDialog(step)"
                  >
                    <Info :size="15" stroke-width="1.9" aria-hidden="true" />
                  </button>
                </td>
              </tr>
            </template>
            <tr v-if="runs.length === 0">
              <td colspan="6">{{ t('dataPipelines.noRuns') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-else-if="activeTab === 'reviews'" class="data-pipeline-panel-section" role="tabpanel">
      <div class="compact-table">
        <table>
          <thead>
            <tr>
              <th>{{ t('dataPipelines.status') }}</th>
              <th>{{ t('dataPipelines.node') }}</th>
              <th>{{ t('dataPipelines.queue') }}</th>
              <th>{{ t('dataPipelines.reason') }}</th>
              <th>{{ t('dataPipelines.created') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in reviewItems" :key="item.publicId">
              <td><span class="status-pill" :class="statusClass(item.status)">{{ statusLabel(item.status) }}</span></td>
              <td>{{ item.nodeId }}</td>
              <td>{{ item.queue }}</td>
              <td>{{ reviewReasonLabel(item) }}</td>
              <td>{{ formatDate(item.createdAt) }}</td>
            </tr>
            <tr v-if="reviewItems.length === 0">
              <td colspan="5">{{ t('dataPipelines.noReviewItems') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-else class="data-pipeline-panel-section" role="tabpanel">
      <div class="compact-table">
        <table>
          <thead>
            <tr>
              <th>{{ t('dataPipelines.status') }}</th>
              <th>{{ t('dataPipelines.frequency') }}</th>
              <th>{{ t('dataPipelines.next') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="schedule in schedules" :key="schedule.publicId">
              <td><span class="status-pill" :class="schedule.enabled ? 'success' : 'danger'">{{ schedule.enabled ? t('common.enabled') : t('common.disabled') }}</span></td>
              <td>{{ scheduleFrequencyLabel(schedule.frequency) }} {{ schedule.runTime }}</td>
              <td>{{ formatDate(schedule.nextRunAt) }}</td>
              <td>
                <button class="icon-button" type="button" :aria-label="t('dataPipelines.disableSchedule', { publicId: schedule.publicId })" :disabled="!schedule.enabled || actionLoading" @click="$emit('disableSchedule', schedule.publicId)">
                  <Trash2 :size="16" stroke-width="1.9" aria-hidden="true" />
                </button>
              </td>
            </tr>
            <tr v-if="schedules.length === 0">
              <td colspan="4">{{ t('dataPipelines.noSchedules') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <dialog
      ref="previewCellDialogRef"
      class="confirm-dialog data-pipeline-cell-dialog"
      @close="handlePreviewCellDialogClose"
      @cancel.prevent="closePreviewCellDialog"
    >
      <div class="confirm-dialog-panel data-pipeline-cell-dialog-panel">
        <div class="stack">
          <span class="status-pill">{{ t('dataPipelines.preview') }}</span>
          <h2>{{ t('dataPipelines.previewCellValue') }}</h2>
          <p class="cell-subtle">{{ previewCellDialog?.column }}</p>
        </div>

        <pre class="data-pipeline-cell-dialog-value">{{ previewCellDialog?.value }}</pre>

        <div class="action-row">
          <button class="secondary-button" type="button" autofocus @click="closePreviewCellDialog">
            {{ t('common.close') }}
          </button>
        </div>
      </div>
    </dialog>

    <dialog
      ref="stepMetadataDialogRef"
      class="confirm-dialog data-pipeline-cell-dialog"
      @close="handleStepMetadataDialogClose"
      @cancel.prevent="closeStepMetadataDialog"
    >
      <div class="confirm-dialog-panel data-pipeline-cell-dialog-panel">
        <div class="stack">
          <span class="status-pill">{{ t('dataPipelines.stepMetadata') }}</span>
          <h2>{{ stepMetadataDialog?.title }}</h2>
          <p class="cell-subtle">{{ stepMetadataDialog?.subtitle }}</p>
        </div>

        <dl class="metadata-grid compact">
          <div v-for="fact in stepMetadataDialog?.facts ?? []" :key="fact.label">
            <dt>{{ fact.label }}</dt>
            <dd>{{ fact.value }}</dd>
          </div>
        </dl>

        <pre class="data-pipeline-cell-dialog-value">{{ stepMetadataDialog?.value }}</pre>

        <div class="action-row">
          <button class="secondary-button" type="button" autofocus @click="closeStepMetadataDialog">
            {{ t('common.close') }}
          </button>
        </div>
      </div>
    </dialog>
  </section>
</template>
