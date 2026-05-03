<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { CalendarPlus, GitBranch, Play, Plus, RefreshCw, Save, Send, Workflow } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import type { DataPipelineGraph, DataPipelineScheduleWriteBody, DataPipelineStepType } from '../api/data-pipelines'
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
const { t } = useI18n()

const builderRef = ref<InstanceType<typeof DataPipelineFlowBuilder> | null>(null)
const newName = ref(t('dataPipelines.defaultPipelineName'))
const newDescription = ref('')
const editName = ref('')
const editDescription = ref('')
const scheduleFrequency = ref<'daily' | 'weekly' | 'monthly'>('daily')
const scheduleTimezone = ref('Asia/Tokyo')
const scheduleRunTime = ref('03:00')
const scheduleWeekday = ref<number | null>(1)
const scheduleMonthDay = ref<number | null>(1)
let refreshTimer: number | undefined

const nodeCatalog: Array<{ type: DataPipelineStepType, labelKey: string }> = [
  { type: 'input', labelKey: 'dataPipelines.step.input' },
  { type: 'profile', labelKey: 'dataPipelines.step.profile' },
  { type: 'clean', labelKey: 'dataPipelines.step.clean' },
  { type: 'normalize', labelKey: 'dataPipelines.step.normalize' },
  { type: 'validate', labelKey: 'dataPipelines.step.validate' },
  { type: 'schema_mapping', labelKey: 'dataPipelines.step.schema_mapping' },
  { type: 'schema_completion', labelKey: 'dataPipelines.step.schema_completion' },
  { type: 'enrich_join', labelKey: 'dataPipelines.step.enrich_join' },
  { type: 'transform', labelKey: 'dataPipelines.step.transform' },
  { type: 'output', labelKey: 'dataPipelines.step.output' },
]
const knownStatusValues = new Set(['pending', 'processing', 'completed', 'failed', 'ready', 'created', 'active', 'disabled', 'published', 'archived'])

const selectedPipeline = computed(() => store.detail?.pipeline ?? store.selectedPipeline)
const activeTenantLabel = computed(() => (
  tenantStore.activeTenant
    ? `${tenantStore.activeTenant.displayName} / ${tenantStore.activeTenant.slug}`
    : t('dataPipelines.noActiveTenant')
))
const canPreview = computed(() => Boolean(selectedPipeline.value))
const previewDisabledReason = computed(() => {
  if (!selectedPipeline.value) return t('dataPipelines.createOrSelectFirst')
  return ''
})
const runDisabledReason = computed(() => (selectedPipeline.value ? '' : t('dataPipelines.createOrSelectFirst')))

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
})

watch(
  () => tenantStore.activeTenant?.slug,
  async (slug) => {
    store.reset()
    datasetStore.reset()
    if (slug) {
      await Promise.all([
        store.load(),
        datasetStore.load().catch(() => undefined),
        datasetStore.loadWorkTables().catch(() => undefined),
      ])
    }
  },
  { immediate: true },
)

watch(selectedPipeline, (pipeline) => {
  editName.value = pipeline?.name ?? ''
  editDescription.value = pipeline?.description ?? ''
}, { immediate: true })

function selectPipeline(publicId: string) {
  store.loadDetail(publicId).catch(() => undefined)
}

async function createPipeline() {
  await store.create(newName.value, newDescription.value)
  newName.value = t('dataPipelines.defaultPipelineName')
  newDescription.value = ''
}

function updateGraph(graph: DataPipelineGraph) {
  store.draftGraph = graph
}

function selectNode(nodeId: string) {
  store.selectedNodeId = nodeId
}

function addNode(type: DataPipelineStepType) {
  builderRef.value?.addNode(type)
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

async function refreshRuns() {
  await store.refreshRuns().catch(() => undefined)
}

async function createSchedule(body: DataPipelineScheduleWriteBody) {
  await store.createSchedule(body)
}

async function createTopSchedule() {
  await createSchedule({
    frequency: scheduleFrequency.value,
    timezone: scheduleTimezone.value,
    runTime: scheduleRunTime.value,
    weekday: scheduleFrequency.value === 'weekly' ? scheduleWeekday.value : null,
    monthDay: scheduleFrequency.value === 'monthly' ? scheduleMonthDay.value : null,
    enabled: true,
  }).catch(() => undefined)
}

async function disableSchedule(publicId: string) {
  await store.disableSchedule(publicId)
}

function statusClass(status = '') {
  if (['published', 'completed', 'active'].includes(status)) return 'success'
  if (['failed', 'archived'].includes(status)) return 'danger'
  return 'warning'
}

function statusLabel(status = '') {
  return knownStatusValues.has(status) ? t(`dataPipelines.statusValue.${status}`) : status
}
</script>

<template>
  <section class="data-pipeline-page">
    <header class="page-header">
      <div class="page-header-copy">
        <span class="status-pill">
          <Workflow :size="14" stroke-width="1.9" aria-hidden="true" />
          {{ activeTenantLabel }}
        </span>
        <h1>{{ t('routes.dataPipelines') }}</h1>
      </div>
      <div class="page-header-actions">
        <button class="secondary-button" type="button" :disabled="store.status === 'loading'" @click="store.load()">
          <RefreshCw :size="16" stroke-width="1.9" aria-hidden="true" />
          {{ t('common.refresh') }}
        </button>
        <button class="primary-button" type="button" :disabled="store.actionLoading || !selectedPipeline || !editName.trim()" @click="saveDraft">
          <Save :size="16" stroke-width="1.9" aria-hidden="true" />
          {{ t('common.save') }}
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

    <div v-else class="data-pipeline-layout">
      <aside class="data-pipeline-sidebar">
        <section class="sidebar-panel-section">
          <h2>{{ t('dataPipelines.newPipeline') }}</h2>
          <label class="field">
            <span>{{ t('dataPipelines.name') }}</span>
            <input v-model="newName">
          </label>
          <label class="field">
            <span>{{ t('dataPipelines.description') }}</span>
            <textarea v-model="newDescription" rows="3" />
          </label>
          <button class="primary-button full-width" type="button" :disabled="store.actionLoading || !newName.trim()" @click="createPipeline">
            <Plus :size="16" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.create') }}
          </button>
        </section>

        <section class="sidebar-panel-section">
          <h2>{{ t('dataPipelines.pipelines') }}</h2>
          <button
            v-for="item in store.items"
            :key="item.publicId"
            type="button"
            class="dataset-row"
            :class="{ active: item.publicId === store.selectedPublicId }"
            @click="selectPipeline(item.publicId)"
          >
            <span>
              <strong>{{ item.name }}</strong>
              <small>{{ item.description || item.publicId }}</small>
            </span>
            <span class="status-pill" :class="statusClass(item.status)">{{ statusLabel(item.status) }}</span>
          </button>
          <p v-if="store.items.length === 0" class="muted-panel">{{ t('dataPipelines.noPipelines') }}</p>
        </section>

        <section class="sidebar-panel-section">
          <h2>{{ t('dataPipelines.palette') }}</h2>
          <div class="data-pipeline-palette">
            <button v-for="node in nodeCatalog" :key="node.type" type="button" @click="addNode(node.type)">
              <GitBranch :size="15" stroke-width="1.9" aria-hidden="true" />
              {{ t(node.labelKey) }}
            </button>
          </div>
        </section>

        <section class="sidebar-panel-section">
          <h2>{{ t('dataPipelines.sources') }}</h2>
          <div class="data-pipeline-source-list">
            <p>{{ t('dataPipelines.datasets') }}</p>
            <code v-for="item in datasetStore.items.slice(0, 6)" :key="item.publicId">{{ item.name }}: {{ item.publicId }}</code>
            <p>{{ t('dataPipelines.workTables') }}</p>
            <code v-for="item in datasetStore.workTables.slice(0, 6)" :key="item.publicId">{{ item.displayName }}: {{ item.publicId }}</code>
          </div>
        </section>
      </aside>

      <main class="data-pipeline-main">
        <section class="data-pipeline-action-strip" :aria-label="t('dataPipelines.runAndScheduleActions')">
          <div class="data-pipeline-toolbar-fields">
            <label class="field compact-field">
              <span>{{ t('dataPipelines.name') }}</span>
              <input v-model="editName" :disabled="!selectedPipeline" autocomplete="off">
            </label>
            <label class="field compact-field">
              <span>{{ t('dataPipelines.description') }}</span>
              <input v-model="editDescription" :disabled="!selectedPipeline" autocomplete="off">
            </label>
          </div>

          <div class="data-pipeline-action-row">
            <div class="data-pipeline-run-controls">
              <span class="status-pill">{{ t('dataPipelines.runs') }}</span>
              <button class="secondary-button" type="button" :disabled="store.runsLoading || store.actionLoading" @click="refreshRuns">
                <RefreshCw :size="16" stroke-width="1.9" aria-hidden="true" />
                {{ store.runsLoading ? t('dataPipelines.refreshing') : t('dataPipelines.refresh') }}
              </button>
              <button class="primary-button" type="button" :disabled="store.actionLoading" :title="runDisabledReason" @click="runPublished">
                <Play :size="16" stroke-width="1.9" aria-hidden="true" />
                {{ store.actionLoading ? t('dataPipelines.running') : t('dataPipelines.run') }}
              </button>
            </div>

            <form class="data-pipeline-schedule-form top" @submit.prevent="createTopSchedule">
              <span class="status-pill">{{ t('dataPipelines.schedule') }}</span>
              <label class="field compact-field">
                <span>{{ t('dataPipelines.frequency') }}</span>
                <select v-model="scheduleFrequency">
                  <option value="daily">{{ t('dataPipelines.daily') }}</option>
                  <option value="weekly">{{ t('dataPipelines.weekly') }}</option>
                  <option value="monthly">{{ t('dataPipelines.monthly') }}</option>
                </select>
              </label>
              <label class="field compact-field">
                <span>{{ t('dataPipelines.timezone') }}</span>
                <input v-model="scheduleTimezone">
              </label>
              <label class="field compact-field">
                <span>{{ t('dataPipelines.runTime') }}</span>
                <input v-model="scheduleRunTime" placeholder="03:00">
              </label>
              <label v-if="scheduleFrequency === 'weekly'" class="field compact-field">
                <span>{{ t('dataPipelines.weekday') }}</span>
                <input v-model.number="scheduleWeekday" type="number" min="1" max="7">
              </label>
              <label v-if="scheduleFrequency === 'monthly'" class="field compact-field">
                <span>{{ t('dataPipelines.monthDay') }}</span>
                <input v-model.number="scheduleMonthDay" type="number" min="1" max="28">
              </label>
              <button class="primary-button" type="submit" :disabled="store.actionLoading" :title="runDisabledReason">
                <CalendarPlus :size="16" stroke-width="1.9" aria-hidden="true" />
                {{ t('dataPipelines.addSchedule') }}
              </button>
            </form>
          </div>
        </section>

        <div v-if="selectedPipeline" class="data-pipeline-builder-grid">
          <DataPipelineFlowBuilder
            ref="builderRef"
            :graph="store.draftGraph"
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
        <div v-else class="empty-state">
          <Workflow :size="28" stroke-width="1.8" aria-hidden="true" />
          <h2>{{ t('dataPipelines.noPipelineSelected') }}</h2>
        </div>

        <p v-if="store.errorMessage" class="form-error">{{ store.errorMessage }}</p>
        <p v-else-if="store.actionMessage" class="form-success">{{ store.actionMessage }}</p>

        <DataPipelinePreviewPanel
          :preview="store.preview"
          :runs="store.runs"
          :schedules="store.schedules"
          :loading="store.previewLoading"
          :action-loading="store.actionLoading"
          :can-preview="canPreview"
          :preview-disabled-reason="previewDisabledReason"
          @preview="previewSelected"
          @disable-schedule="disableSchedule"
        />
      </main>
    </div>
  </section>
</template>
