<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref } from 'vue'
import { Search, Trash2 } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import type { DataPipelinePreviewBody, DataPipelineRunBody, DataPipelineScheduleBody } from '../api/data-pipelines'

const props = defineProps<{
  preview: DataPipelinePreviewBody | null
  runs: DataPipelineRunBody[]
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

type DataPipelinePanelTab = 'preview' | 'runs' | 'schedules'
type PreviewCellDialog = {
  column: string
  value: string
}

const PREVIEW_CELL_TRUNCATE_LENGTH = 100
const activeTab = ref<DataPipelinePanelTab>('preview')
const previewCellDialog = ref<PreviewCellDialog | null>(null)
const previewCellDialogRef = ref<HTMLDialogElement | null>(null)
const { d, t } = useI18n()

onBeforeUnmount(() => {
  if (previewCellDialogRef.value?.open) {
    previewCellDialogRef.value.close()
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

const knownStatusValues = new Set(['pending', 'processing', 'completed', 'failed', 'ready', 'created', 'active', 'disabled', 'published', 'archived'])
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
              <th>{{ t('dataPipelines.trigger') }}</th>
              <th>{{ t('dataPipelines.rows') }}</th>
              <th>{{ t('dataPipelines.created') }}</th>
              <th>{{ t('dataPipelines.error') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="run in runs" :key="run.publicId">
              <td><span class="status-pill" :class="statusClass(run.status)">{{ statusLabel(run.status) }}</span></td>
              <td>{{ triggerLabel(run.triggerKind) }}</td>
              <td>{{ run.rowCount }}</td>
              <td>{{ formatDate(run.createdAt) }}</td>
              <td>{{ run.errorSummary || '-' }}</td>
            </tr>
            <tr v-if="runs.length === 0">
              <td colspan="5">{{ t('dataPipelines.noRuns') }}</td>
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
  </section>
</template>
