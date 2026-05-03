<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowLeft, Play, RefreshCw, Save, Send, Settings2, Workflow } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { isDataPipelineDraftRunPreviewGraph, sanitizeDataPipelineGraph, type DataPipelineGraph, type DataPipelineScheduleWriteBody, type DataPipelineStepType } from '../api/data-pipelines'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import DataPipelineFlowBuilder from '../components/DataPipelineFlowBuilder.vue'
import DataPipelineInspector from '../components/DataPipelineInspector.vue'
import DataPipelinePreviewPanel from '../components/DataPipelinePreviewPanel.vue'
import { useDataPipelineStore } from '../stores/data-pipelines'
import { useDatasetStore } from '../stores/datasets'
import { useTenantStore } from '../stores/tenants'

const store = useDataPipelineStore()
const datasetStore = useDatasetStore()
const tenantStore = useTenantStore()
const route = useRoute()
const { t } = useI18n()

const settingsDialogRef = ref<HTMLDialogElement | null>(null)
const settingsNameInputRef = ref<HTMLInputElement | null>(null)
const editName = ref('')
const editDescription = ref('')
const settingsName = ref('')
const settingsDescription = ref('')
const settingsSchedulePublicId = ref('')
const settingsScheduleEnabled = ref(false)
const settingsScheduleFrequency = ref<'daily' | 'weekly' | 'monthly'>('daily')
const settingsScheduleTimezone = ref('Asia/Tokyo')
const settingsScheduleRunTime = ref('03:00')
const settingsScheduleWeekday = ref<number | null>(1)
const settingsScheduleMonthDay = ref<number | null>(1)
const settingsError = ref('')
let refreshTimer: number | undefined

const nodeCatalog: Array<{ type: DataPipelineStepType, labelKey: string }> = [
  { type: 'input', labelKey: 'dataPipelines.step.input' },
  { type: 'extract_text', labelKey: 'dataPipelines.step.extract_text' },
  { type: 'classify_document', labelKey: 'dataPipelines.step.classify_document' },
  { type: 'extract_fields', labelKey: 'dataPipelines.step.extract_fields' },
  { type: 'extract_table', labelKey: 'dataPipelines.step.extract_table' },
  { type: 'deduplicate', labelKey: 'dataPipelines.step.deduplicate' },
  { type: 'canonicalize', labelKey: 'dataPipelines.step.canonicalize' },
  { type: 'redact_pii', labelKey: 'dataPipelines.step.redact_pii' },
  { type: 'detect_language_encoding', labelKey: 'dataPipelines.step.detect_language_encoding' },
  { type: 'schema_inference', labelKey: 'dataPipelines.step.schema_inference' },
  { type: 'entity_resolution', labelKey: 'dataPipelines.step.entity_resolution' },
  { type: 'unit_conversion', labelKey: 'dataPipelines.step.unit_conversion' },
  { type: 'relationship_extraction', labelKey: 'dataPipelines.step.relationship_extraction' },
  { type: 'profile', labelKey: 'dataPipelines.step.profile' },
  { type: 'clean', labelKey: 'dataPipelines.step.clean' },
  { type: 'normalize', labelKey: 'dataPipelines.step.normalize' },
  { type: 'validate', labelKey: 'dataPipelines.step.validate' },
  { type: 'schema_mapping', labelKey: 'dataPipelines.step.schema_mapping' },
  { type: 'schema_completion', labelKey: 'dataPipelines.step.schema_completion' },
  { type: 'enrich_join', labelKey: 'dataPipelines.step.enrich_join' },
  { type: 'transform', labelKey: 'dataPipelines.step.transform' },
  { type: 'confidence_gate', labelKey: 'dataPipelines.step.confidence_gate' },
  { type: 'human_review', labelKey: 'dataPipelines.step.human_review' },
  { type: 'sample_compare', labelKey: 'dataPipelines.step.sample_compare' },
  { type: 'quality_report', labelKey: 'dataPipelines.step.quality_report' },
  { type: 'output', labelKey: 'dataPipelines.step.output' },
]

const pipelinePublicId = computed(() => String(route.params.pipelinePublicId ?? ''))
const selectedPipeline = computed(() => store.detail?.pipeline ?? null)
const primarySchedule = computed(() => store.schedules.find((schedule) => schedule.enabled) ?? store.schedules[0] ?? null)
const pageTitle = computed(() => selectedPipeline.value?.name || t('dataPipelines.pipelineDetail'))
const canPreview = computed(() => Boolean(selectedPipeline.value))
const draftRunPreview = computed(() => isDataPipelineDraftRunPreviewGraph(store.draftGraph))
const previewDisabledReason = computed(() => {
  if (!selectedPipeline.value) return t('dataPipelines.createOrSelectFirst')
  return ''
})
const runDisabledReason = computed(() => (selectedPipeline.value ? '' : t('dataPipelines.createOrSelectFirst')))
const autoPreviewDelayMs = 350
let autoPreviewTimer: number | undefined

onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
  refreshTimer = window.setInterval(async () => {
    if (store.hasActiveRuns) {
      await store.refreshRuns().catch(() => undefined)
    }
  }, 4000)
})

onBeforeUnmount(() => {
  if (refreshTimer !== undefined) {
    window.clearInterval(refreshTimer)
  }
  if (autoPreviewTimer !== undefined) {
    window.clearTimeout(autoPreviewTimer)
  }
  if (settingsDialogRef.value?.open) {
    settingsDialogRef.value.close()
  }
})

watch(
  [() => tenantStore.activeTenant?.slug, pipelinePublicId],
  async ([slug, publicId]) => {
    await loadRoutePipeline(slug, publicId)
  },
  { immediate: true },
)

watch(selectedPipeline, (pipeline) => {
  editName.value = pipeline?.name ?? ''
  editDescription.value = pipeline?.description ?? ''
}, { immediate: true })

watch(
  () => store.selectedAutoPreviewKey,
  (previewKey) => {
    if (autoPreviewTimer !== undefined) {
      window.clearTimeout(autoPreviewTimer)
      autoPreviewTimer = undefined
    }
    if (!previewKey || store.status !== 'ready') {
      return
    }
    autoPreviewTimer = window.setTimeout(() => {
      autoPreviewTimer = undefined
      if (store.selectedAutoPreviewKey === previewKey) {
        void store.autoPreviewSelected().catch(() => undefined)
      }
    }, autoPreviewDelayMs)
  },
  { flush: 'post' },
)

async function loadRoutePipeline(slug: string | undefined, publicId: string) {
  store.reset()
  datasetStore.reset()
  if (!slug || !publicId) {
    return
  }
  await Promise.all([
    store.load(false),
    datasetStore.load().catch(() => undefined),
    datasetStore.loadWorkTables().catch(() => undefined),
  ])
  if (store.status !== 'forbidden') {
    await store.loadDetail(publicId).catch(() => undefined)
  }
}

async function refreshDetail() {
  await loadRoutePipeline(tenantStore.activeTenant?.slug, pipelinePublicId.value)
}

function updateGraph(graph: DataPipelineGraph) {
  store.draftGraph = sanitizeDataPipelineGraph(graph)
}

function selectNode(nodeId: string) {
  store.selectedNodeId = nodeId
}

async function saveDraft() {
  if (selectedPipeline.value) {
    const name = editName.value.trim()
    const description = editDescription.value.trim()
    if (name && (name !== selectedPipeline.value.name || description !== (selectedPipeline.value.description ?? ''))) {
      await store.update(name, description)
    }
  }
  await store.saveDraft()
}

async function publishLatest() {
  await store.publishLatest()
}

async function previewSelected() {
  await store.previewSelected().catch(() => undefined)
}

async function runPublished() {
  await store.runPublished().catch(() => undefined)
}

async function disableSchedule(publicId: string) {
  await store.disableSchedule(publicId)
}

async function openSettings() {
  const pipeline = selectedPipeline.value
  if (!pipeline) {
    return
  }
  const schedule = primarySchedule.value
  settingsName.value = pipeline.name
  settingsDescription.value = pipeline.description ?? ''
  settingsSchedulePublicId.value = schedule?.publicId ?? ''
  settingsScheduleEnabled.value = schedule?.enabled ?? false
  settingsScheduleFrequency.value = schedule?.frequency ?? 'daily'
  settingsScheduleTimezone.value = schedule?.timezone || 'Asia/Tokyo'
  settingsScheduleRunTime.value = schedule?.runTime || '03:00'
  settingsScheduleWeekday.value = schedule?.weekday ?? 1
  settingsScheduleMonthDay.value = schedule?.monthDay ?? 1
  settingsError.value = ''
  await nextTick()
  if (!settingsDialogRef.value?.open) {
    settingsDialogRef.value?.showModal()
  }
  await nextTick()
  settingsNameInputRef.value?.focus()
}

function closeSettings() {
  if (settingsDialogRef.value?.open) {
    settingsDialogRef.value.close()
  }
}

function handleSettingsClose() {
  settingsError.value = ''
}

function scheduleWriteBody(): DataPipelineScheduleWriteBody {
  return {
    frequency: settingsScheduleFrequency.value,
    timezone: settingsScheduleTimezone.value.trim() || 'Asia/Tokyo',
    runTime: settingsScheduleRunTime.value.trim() || '03:00',
    weekday: settingsScheduleFrequency.value === 'weekly' ? settingsScheduleWeekday.value : null,
    monthDay: settingsScheduleFrequency.value === 'monthly' ? settingsScheduleMonthDay.value : null,
    enabled: settingsScheduleEnabled.value,
  }
}

async function applySettings() {
  const pipeline = selectedPipeline.value
  if (!pipeline) {
    return
  }
  const name = settingsName.value.trim()
  const description = settingsDescription.value.trim()
  if (!name) {
    settingsError.value = t('dataPipelines.nameRequired')
    return
  }

  settingsError.value = ''
  try {
    if (name !== pipeline.name || description !== (pipeline.description ?? '')) {
      const updated = await store.update(name, description)
      editName.value = updated?.name ?? name
      editDescription.value = updated?.description ?? description
    }

    const body = scheduleWriteBody()
    const schedulePublicId = settingsSchedulePublicId.value || primarySchedule.value?.publicId || ''
    if (settingsScheduleEnabled.value) {
      if (schedulePublicId) {
        await store.updateSchedule(schedulePublicId, body)
      } else {
        await store.createSchedule(body)
      }
    } else if (schedulePublicId) {
      await store.updateSchedule(schedulePublicId, body)
    }

    store.actionMessage = t('dataPipelines.settingsApplied')
    closeSettings()
  } catch {
    settingsError.value = store.errorMessage || t('dataPipelines.settingsApplyFailed')
  }
}
</script>

<template>
  <section class="data-pipeline-page">
    <header class="page-header">
      <div class="page-header-copy">
        <h1>{{ pageTitle }}</h1>
      </div>
      <div class="page-header-actions">
        <RouterLink class="secondary-button link-button" :to="{ name: 'data-pipelines' }">
          <ArrowLeft :size="16" stroke-width="1.9" aria-hidden="true" />
          {{ t('dataPipelines.backToPipelines') }}
        </RouterLink>
        <button class="secondary-button" type="button" :disabled="store.status === 'loading'" @click="refreshDetail">
          <RefreshCw :size="16" stroke-width="1.9" aria-hidden="true" />
          {{ t('common.refresh') }}
        </button>
        <button class="primary-button" type="button" :disabled="store.actionLoading || !selectedPipeline || !editName.trim()" @click="saveDraft">
          <Save :size="16" stroke-width="1.9" aria-hidden="true" />
          {{ t('common.save') }}
        </button>
        <button class="primary-button" type="button" :disabled="store.actionLoading || !selectedPipeline" :title="runDisabledReason" @click="runPublished">
          <Play :size="16" stroke-width="1.9" aria-hidden="true" />
          {{ store.actionLoading ? t('dataPipelines.running') : t('dataPipelines.run') }}
        </button>
        <button class="secondary-button" type="button" :disabled="store.actionLoading || !selectedPipeline" @click="openSettings">
          <Settings2 :size="16" stroke-width="1.9" aria-hidden="true" />
          {{ t('dataPipelines.settings') }}
        </button>
        <button class="secondary-button" type="button" :disabled="store.actionLoading || !store.latestVersion" @click="publishLatest">
          <Send :size="16" stroke-width="1.9" aria-hidden="true" />
          {{ t('dataPipelines.publish') }}
        </button>
      </div>
    </header>

    <AdminAccessDenied
      v-if="store.status === 'forbidden'"
      :title="t('dataPipelines.accessRequiredTitle')"
      role-label="data_pipeline_user"
      :message="t('dataPipelines.accessRequiredMessage')"
    />

    <div v-else-if="selectedPipeline" class="data-pipeline-detail-layout">
      <main class="data-pipeline-main">
        <div class="data-pipeline-builder-grid">
          <DataPipelineFlowBuilder
            :graph="store.draftGraph"
            :node-catalog="nodeCatalog"
            :selected-node-id="store.selectedNodeId"
            @update:graph="updateGraph"
            @select-node="selectNode"
          />
          <DataPipelineInspector
            :graph="store.draftGraph"
            :selected-node-id="store.selectedNodeId"
            :datasets="datasetStore.items"
            :work-tables="datasetStore.workTables"
            @update:graph="updateGraph"
          />
        </div>

        <p v-if="store.errorMessage" class="form-error">{{ store.errorMessage }}</p>
        <p v-else-if="store.actionMessage" class="form-success">{{ store.actionMessage }}</p>

        <DataPipelinePreviewPanel
          :preview="store.selectedPreview"
          :runs="store.runs"
          :schedules="store.schedules"
          :loading="store.selectedPreviewLoading"
          :action-loading="store.actionLoading"
          :can-preview="canPreview"
          :draft-run-preview="draftRunPreview"
          :preview-disabled-reason="previewDisabledReason"
          @preview="previewSelected"
          @disable-schedule="disableSchedule"
        />
      </main>
    </div>

    <div v-else class="empty-state">
      <Workflow :size="28" stroke-width="1.8" aria-hidden="true" />
      <h2>{{ store.status === 'loading' ? t('common.loading') : t('dataPipelines.noPipelineSelected') }}</h2>
      <p v-if="store.errorMessage" class="form-error">{{ store.errorMessage }}</p>
      <RouterLink class="primary-button link-button" :to="{ name: 'data-pipelines' }">
        {{ t('dataPipelines.backToPipelines') }}
      </RouterLink>
    </div>

    <dialog
      ref="settingsDialogRef"
      class="confirm-dialog data-pipeline-settings-dialog"
      @close="handleSettingsClose"
      @cancel.prevent="closeSettings"
    >
      <form class="confirm-dialog-panel data-pipeline-settings-panel" @submit.prevent="applySettings">
        <div class="stack">
          <span class="status-pill">{{ t('dataPipelines.settings') }}</span>
          <h2>{{ t('dataPipelines.pipelineSettings') }}</h2>

          <div class="data-pipeline-settings-grid">
            <label class="field">
              <span class="field-label">{{ t('dataPipelines.name') }}</span>
              <input ref="settingsNameInputRef" v-model="settingsName" class="field-input" autocomplete="off" :disabled="store.actionLoading" required>
            </label>
            <label class="field">
              <span class="field-label">{{ t('dataPipelines.description') }}</span>
              <input v-model="settingsDescription" class="field-input" autocomplete="off" :disabled="store.actionLoading">
            </label>
          </div>

          <section class="data-pipeline-settings-section" :aria-label="t('dataPipelines.schedule')">
            <div class="panel-header compact">
              <h3>{{ t('dataPipelines.schedule') }}</h3>
              <label class="data-pipeline-toggle">
                <input v-model="settingsScheduleEnabled" type="checkbox" :disabled="store.actionLoading">
                <span>{{ t('common.enabled') }}</span>
              </label>
            </div>

            <div class="data-pipeline-settings-grid">
              <label class="field">
                <span class="field-label">{{ t('dataPipelines.frequency') }}</span>
                <select v-model="settingsScheduleFrequency" class="field-input" :disabled="store.actionLoading || !settingsScheduleEnabled">
                  <option value="daily">{{ t('dataPipelines.daily') }}</option>
                  <option value="weekly">{{ t('dataPipelines.weekly') }}</option>
                  <option value="monthly">{{ t('dataPipelines.monthly') }}</option>
                </select>
              </label>
              <label class="field">
                <span class="field-label">{{ t('dataPipelines.timezone') }}</span>
                <input v-model="settingsScheduleTimezone" class="field-input" autocomplete="off" :disabled="store.actionLoading || !settingsScheduleEnabled">
              </label>
              <label class="field">
                <span class="field-label">{{ t('dataPipelines.runTime') }}</span>
                <input v-model="settingsScheduleRunTime" class="field-input" autocomplete="off" placeholder="03:00" :disabled="store.actionLoading || !settingsScheduleEnabled">
              </label>
              <label v-if="settingsScheduleFrequency === 'weekly'" class="field">
                <span class="field-label">{{ t('dataPipelines.weekday') }}</span>
                <input v-model.number="settingsScheduleWeekday" class="field-input" type="number" min="1" max="7" :disabled="store.actionLoading || !settingsScheduleEnabled">
              </label>
              <label v-if="settingsScheduleFrequency === 'monthly'" class="field">
                <span class="field-label">{{ t('dataPipelines.monthDay') }}</span>
                <input v-model.number="settingsScheduleMonthDay" class="field-input" type="number" min="1" max="28" :disabled="store.actionLoading || !settingsScheduleEnabled">
              </label>
            </div>
          </section>

          <p v-if="settingsError" class="form-error">{{ settingsError }}</p>
        </div>

        <div class="action-row data-pipeline-dialog-actions">
          <button class="secondary-button" type="button" :disabled="store.actionLoading" @click="closeSettings">
            {{ t('common.cancel') }}
          </button>
          <button class="primary-button" type="submit" :disabled="store.actionLoading || !settingsName.trim()">
            {{ t('common.apply') }}
          </button>
        </div>
      </form>
    </dialog>
  </section>
</template>
