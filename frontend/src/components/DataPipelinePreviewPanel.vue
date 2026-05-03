<script setup lang="ts">
import { ref } from 'vue'
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

const activeTab = ref<DataPipelinePanelTab>('preview')
const { d, t } = useI18n()

function previewClick() {
  activeTab.value = 'preview'
  emit('preview')
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

    <div v-if="activeTab === 'preview'" class="data-pipeline-panel-section" role="tabpanel">
      <header class="panel-header compact">
        <h2>{{ t('dataPipelines.preview') }}</h2>
        <button class="secondary-button" type="button" :disabled="loading || !canPreview" :title="props.previewDisabledReason" @click="previewClick">
          <Search :size="16" stroke-width="1.9" aria-hidden="true" />
          {{ loading ? t(draftRunPreview ? 'dataPipelines.draftRunPreviewing' : 'dataPipelines.previewing') : t(draftRunPreview ? 'dataPipelines.draftRunPreview' : 'dataPipelines.preview') }}
        </button>
      </header>
      <p v-if="!canPreview && previewDisabledReason" class="muted-panel">{{ previewDisabledReason }}</p>
      <p v-else-if="draftRunPreview" class="muted-panel">{{ t('dataPipelines.draftRunPreviewNotice') }}</p>
      <div v-if="preview" class="dataset-work-table-preview-table data-pipeline-preview-table">
        <table>
          <thead>
            <tr>
              <th v-for="column in preview.columns" :key="column">{{ column }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, index) in preview.previewRows" :key="index">
              <td v-for="column in preview.columns" :key="column">{{ row[column] ?? '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else class="muted-panel">{{ t('dataPipelines.noPreviewResult') }}</p>
    </div>

    <div v-else-if="activeTab === 'runs'" class="data-pipeline-panel-section" role="tabpanel">
      <header class="panel-header compact">
        <h2>{{ t('dataPipelines.runs') }}</h2>
      </header>
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
      <header class="panel-header compact">
        <h2>{{ t('dataPipelines.schedules') }}</h2>
      </header>
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
  </section>
</template>
