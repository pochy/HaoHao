<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowLeft, History, MoreHorizontal, Play, RefreshCw, RotateCcw, RotateCw, Save, Send, Settings2, Workflow, X } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { isDataPipelineDraftRunPreviewGraph, sanitizeDataPipelineGraph, type DataPipelineGraph, type DataPipelineNodeWarningBody, type DataPipelineScheduleWriteBody, type DataPipelineStepType, type DataPipelineVersionBody } from '../api/data-pipelines'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import DataPipelineFlowBuilder from '../components/DataPipelineFlowBuilder.vue'
import DataPipelineInspector from '../components/DataPipelineInspector.vue'
import DataPipelinePreviewPanel from '../components/DataPipelinePreviewPanel.vue'
import { Button } from '../components/ui/button'
import { ButtonGroup } from '../components/ui/button-group'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from '../components/ui/dropdown-menu'
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '../components/ui/resizable'
import { useDataPipelineStore } from '../stores/data-pipelines'
import { useDatasetStore } from '../stores/datasets'
import { useTenantStore } from '../stores/tenants'

const store = useDataPipelineStore()
const datasetStore = useDatasetStore()
const tenantStore = useTenantStore()
const route = useRoute()
const { t } = useI18n()

const settingsDialogRef = ref<HTMLDialogElement | null>(null)
const versionHistoryDialogRef = ref<HTMLDialogElement | null>(null)
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
const selectedHistoryVersionPublicId = ref('')
const compactLayout = ref(false)
let compactLayoutMediaQuery: MediaQueryList | undefined
let refreshTimer: number | undefined

type GraphUpdateOptions = {
  transient?: boolean
  commit?: boolean
}
type GraphHistory = {
  past: DataPipelineGraph[]
  current: DataPipelineGraph
  future: DataPipelineGraph[]
}

const graphHistoryLimit = 100
const graphHistory = ref<GraphHistory>({
  past: [],
  current: sanitizeDataPipelineGraph(store.draftGraph),
  future: [],
})

const nodeCatalog: Array<{ type: DataPipelineStepType, labelKey: string }> = [
  { type: 'input', labelKey: 'dataPipelines.step.input' },
  { type: 'extract_text', labelKey: 'dataPipelines.step.extract_text' },
  { type: 'json_extract', labelKey: 'dataPipelines.step.json_extract' },
  { type: 'excel_extract', labelKey: 'dataPipelines.step.excel_extract' },
  { type: 'classify_document', labelKey: 'dataPipelines.step.classify_document' },
  { type: 'extract_fields', labelKey: 'dataPipelines.step.extract_fields' },
  { type: 'extract_table', labelKey: 'dataPipelines.step.extract_table' },
  { type: 'product_extraction', labelKey: 'dataPipelines.step.product_extraction' },
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
  { type: 'join', labelKey: 'dataPipelines.step.join' },
  { type: 'enrich_join', labelKey: 'dataPipelines.step.enrich_join' },
  { type: 'transform', labelKey: 'dataPipelines.step.transform' },
  { type: 'confidence_gate', labelKey: 'dataPipelines.step.confidence_gate' },
  { type: 'quarantine', labelKey: 'dataPipelines.step.quarantine' },
  { type: 'human_review', labelKey: 'dataPipelines.step.human_review' },
  { type: 'sample_compare', labelKey: 'dataPipelines.step.sample_compare' },
  { type: 'quality_report', labelKey: 'dataPipelines.step.quality_report' },
  { type: 'output', labelKey: 'dataPipelines.step.output' },
]

const pipelinePublicId = computed(() => String(route.params.pipelinePublicId ?? ''))
const selectedPipeline = computed(() => store.detail?.pipeline ?? null)
const primarySchedule = computed(() => store.schedules.find((schedule) => schedule.enabled) ?? store.schedules[0] ?? null)
const pageTitle = computed(() => selectedPipeline.value?.name || t('dataPipelines.pipelineDetail'))
const latestVersionNumber = computed(() => store.latestVersion?.versionNumber ?? null)
const activeVersionNumber = computed(() => store.publishedVersion?.versionNumber ?? null)
const latestVersionIsActive = computed(() => Boolean(store.latestVersion?.publicId && store.latestVersion.publicId === store.publishedVersion?.publicId))
const activateButtonLabel = computed(() => {
  if (!store.latestVersion) return t('dataPipelines.publish')
  if (latestVersionIsActive.value) return t('dataPipelines.activated')
  if (store.publishedVersion) return t('dataPipelines.activateCurrentVersion')
  return t('dataPipelines.publish')
})
const activateDisabled = computed(() => store.actionLoading || !store.latestVersion || latestVersionIsActive.value)
const activateButtonTitle = computed(() => (latestVersionIsActive.value ? t('dataPipelines.alreadyActivatedTitle') : ''))
const activeVersionLabel = computed(() => (
  activeVersionNumber.value
    ? t('dataPipelines.versionValue', { version: activeVersionNumber.value })
    : t('dataPipelines.noActiveVersion')
))
const editingVersionLabel = computed(() => (
  latestVersionNumber.value
    ? t('dataPipelines.versionValue', { version: latestVersionNumber.value })
    : t('dataPipelines.unsavedVersion')
))
const versionHistoryItems = computed(() => store.detail?.versions ?? [])
const selectedHistoryVersion = computed(() => (
  versionHistoryItems.value.find((version) => version.publicId === selectedHistoryVersionPublicId.value)
  ?? versionHistoryItems.value[0]
  ?? null
))
const selectedHistoryStepTypes = computed(() => {
  const version = selectedHistoryVersion.value
  if (!version) return []
  return version.graph.nodes.map((node) => node.data.stepType)
})
const selectedHistoryValidationErrors = computed(() => selectedHistoryVersion.value?.validationSummary?.errors ?? [])
const canPreview = computed(() => Boolean(selectedPipeline.value))
const draftRunPreview = computed(() => isDataPipelineDraftRunPreviewGraph(store.draftGraph))
const previewDisabledReason = computed(() => {
  if (!selectedPipeline.value) return t('dataPipelines.createOrSelectFirst')
  return ''
})
const runDisabledReason = computed(() => (selectedPipeline.value ? '' : t('dataPipelines.createOrSelectFirst')))
const canUndoGraph = computed(() => graphHistory.value.past.length > 0)
const canRedoGraph = computed(() => graphHistory.value.future.length > 0)
const graphIssueItems = computed(() => (store.selectedValidation?.nodeWarnings ?? []).map((warning) => ({
  warning,
  nodeLabel: labelForGraphWarningNode(warning.nodeId),
  message: graphWarningMessage(warning),
})))
const autoPreviewDelayMs = 350
let autoPreviewTimer: number | undefined
const autoValidationDelayMs = 250
let autoValidationTimer: number | undefined

onMounted(async () => {
  setupCompactLayoutListener()
  window.addEventListener('keydown', handleGraphHistoryShortcut)
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
  if (autoValidationTimer !== undefined) {
    window.clearTimeout(autoValidationTimer)
  }
  if (settingsDialogRef.value?.open) {
    settingsDialogRef.value.close()
  }
  if (versionHistoryDialogRef.value?.open) {
    versionHistoryDialogRef.value.close()
  }
  window.removeEventListener('keydown', handleGraphHistoryShortcut)
  teardownCompactLayoutListener()
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

watch(
  () => store.selectedValidationKey,
  (validationKey) => {
    if (autoValidationTimer !== undefined) {
      window.clearTimeout(autoValidationTimer)
      autoValidationTimer = undefined
    }
    if (!validationKey || store.status !== 'ready') {
      return
    }
    autoValidationTimer = window.setTimeout(() => {
      autoValidationTimer = undefined
      if (store.selectedValidationKey === validationKey) {
        void store.autoValidateDraft().catch(() => undefined)
      }
    }, autoValidationDelayMs)
  },
  { flush: 'post' },
)

async function loadRoutePipeline(slug: string | undefined, publicId: string) {
  store.reset()
  datasetStore.reset()
  resetGraphHistory(store.draftGraph)
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
    resetGraphHistory(store.draftGraph)
  }
}

async function refreshDetail() {
  await loadRoutePipeline(tenantStore.activeTenant?.slug, pipelinePublicId.value)
}

function updateGraph(graph: DataPipelineGraph, options: GraphUpdateOptions = {}) {
  const sanitized = sanitizeDataPipelineGraph(graph)
  if (options.transient) {
    store.draftGraph = cloneGraph(sanitized)
    normalizeSelectedNodeForGraph(sanitized)
    return
  }
  recordGraphChange(sanitized)
  store.draftGraph = cloneGraph(sanitized)
  normalizeSelectedNodeForGraph(sanitized)
}

function selectNode(nodeId: string) {
  store.selectedNodeId = nodeId
}

function selectIssueNode(nodeId: string) {
  if (store.draftGraph.nodes.some((node) => node.id === nodeId)) {
    store.selectedNodeId = nodeId
  }
}

function labelForGraphWarningNode(nodeId: string) {
  const node = store.draftGraph.nodes.find((item) => item.id === nodeId)
  if (!node) {
    return nodeId
  }
  const text = node.data.label?.trim()
  if (text && text !== englishLabelForStep(node.data.stepType)) {
    return text
  }
  return t(`dataPipelines.step.${node.data.stepType}`)
}

function englishLabelForStep(type: DataPipelineStepType) {
  return type
    .split('_')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

function graphWarningMessage(warning: DataPipelineNodeWarningBody) {
  if (warning.code === 'missing_right_upstream_columns') {
    return t('dataPipelines.missingRightUpstreamColumns', { columns: warning.columns.join(', ') })
  }
  if (warning.code === 'missing_upstream_columns') {
    return t('dataPipelines.missingUpstreamColumns', { columns: warning.columns.join(', ') })
  }
  return warning.message
}

function resetGraphHistory(graph: DataPipelineGraph) {
  const sanitized = sanitizeDataPipelineGraph(graph)
  graphHistory.value = {
    past: [],
    current: cloneGraph(sanitized),
    future: [],
  }
}

function recordGraphChange(graph: DataPipelineGraph) {
  const sanitized = sanitizeDataPipelineGraph(graph)
  if (graphsEqual(graphHistory.value.current, sanitized)) {
    return
  }
  graphHistory.value = {
    past: [...graphHistory.value.past, cloneGraph(graphHistory.value.current)].slice(-graphHistoryLimit),
    current: cloneGraph(sanitized),
    future: [],
  }
}

function undoGraph() {
  if (!canUndoGraph.value) {
    return
  }
  const previous = graphHistory.value.past[graphHistory.value.past.length - 1]
  graphHistory.value = {
    past: graphHistory.value.past.slice(0, -1),
    current: cloneGraph(previous),
    future: [cloneGraph(graphHistory.value.current), ...graphHistory.value.future],
  }
  applyHistoryGraph(previous)
}

function redoGraph() {
  if (!canRedoGraph.value) {
    return
  }
  const next = graphHistory.value.future[0]
  graphHistory.value = {
    past: [...graphHistory.value.past, cloneGraph(graphHistory.value.current)].slice(-graphHistoryLimit),
    current: cloneGraph(next),
    future: graphHistory.value.future.slice(1),
  }
  applyHistoryGraph(next)
}

function applyHistoryGraph(graph: DataPipelineGraph) {
  const sanitized = sanitizeDataPipelineGraph(graph)
  store.draftGraph = cloneGraph(sanitized)
  normalizeSelectedNodeForGraph(sanitized)
}

function normalizeSelectedNodeForGraph(graph: DataPipelineGraph) {
  if (graph.nodes.some((node) => node.id === store.selectedNodeId)) {
    return
  }
  store.selectedNodeId = graph.nodes[0]?.id ?? ''
}

function handleGraphHistoryShortcut(event: KeyboardEvent) {
  if (!selectedPipeline.value || graphHistoryShortcutShouldUseNativeUndo(event)) {
    return
  }
  const key = event.key.toLowerCase()
  const modifierPressed = event.metaKey || event.ctrlKey
  if (!modifierPressed || event.altKey) {
    return
  }
  if (key === 'z' && event.shiftKey) {
    event.preventDefault()
    redoGraph()
    return
  }
  if (key === 'z') {
    event.preventDefault()
    undoGraph()
    return
  }
  if (key === 'y' && event.ctrlKey && !event.metaKey && !event.shiftKey) {
    event.preventDefault()
    redoGraph()
  }
}

function graphHistoryShortcutShouldUseNativeUndo(event: KeyboardEvent) {
  const target = event.target
  if (document.querySelector('dialog[open]')) {
    return true
  }
  if (!(target instanceof HTMLElement)) {
    return false
  }
  if (target.isContentEditable) {
    return true
  }
  return ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}

function setupCompactLayoutListener() {
  compactLayoutMediaQuery = window.matchMedia('(max-width: 980px)')
  compactLayout.value = compactLayoutMediaQuery.matches
  compactLayoutMediaQuery.addEventListener('change', updateCompactLayout)
}

function teardownCompactLayoutListener() {
  compactLayoutMediaQuery?.removeEventListener('change', updateCompactLayout)
  compactLayoutMediaQuery = undefined
}

function updateCompactLayout(event: MediaQueryListEvent) {
  compactLayout.value = event.matches
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
  resetGraphHistory(store.draftGraph)
}

async function publishLatest() {
  await store.publishLatest()
  resetGraphHistory(store.draftGraph)
}

async function previewSelected() {
  await store.previewSelected().catch(() => undefined)
}

async function runPublished() {
  const run = await store.runPublished().catch(() => null)
  if (run) {
    resetGraphHistory(store.draftGraph)
  }
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

async function openVersionHistory() {
  selectedHistoryVersionPublicId.value = store.publishedVersion?.publicId ?? store.latestVersion?.publicId ?? versionHistoryItems.value[0]?.publicId ?? ''
  await nextTick()
  if (!versionHistoryDialogRef.value?.open) {
    versionHistoryDialogRef.value?.showModal()
  }
}

function closeVersionHistory() {
  if (versionHistoryDialogRef.value?.open) {
    versionHistoryDialogRef.value.close()
  }
}

function selectHistoryVersion(version: DataPipelineVersionBody) {
  selectedHistoryVersionPublicId.value = version.publicId
}

function versionHistoryBadgeKey(version: DataPipelineVersionBody) {
  if (version.publicId === store.publishedVersion?.publicId) return 'activeVersionBadge'
  if (version.publicId === store.latestVersion?.publicId) return 'latestDraftBadge'
  return 'pastVersionBadge'
}

function versionHistoryBadgeClass(version: DataPipelineVersionBody) {
  if (version.publicId === store.publishedVersion?.publicId) return 'success'
  if (version.publicId === store.latestVersion?.publicId) return 'warning'
  return ''
}

function formatDateTime(value?: string | null) {
  return value ? new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '-'
}

function cloneGraph(graph: DataPipelineGraph): DataPipelineGraph {
  return sanitizeDataPipelineGraph(JSON.parse(JSON.stringify(graph)) as DataPipelineGraph)
}

function graphsEqual(a: DataPipelineGraph, b: DataPipelineGraph) {
  return JSON.stringify(sanitizeDataPipelineGraph(a)) === JSON.stringify(sanitizeDataPipelineGraph(b))
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
  <section class="data-pipeline-page data-pipeline-detail-page">
    <header class="page-header">
      <div class="page-header-copy">
        <h1>{{ pageTitle }}</h1>
        <div v-if="selectedPipeline" class="data-pipeline-version-summary" aria-live="polite">
          <span class="status-pill" :class="{ success: activeVersionNumber }">
            {{ t('dataPipelines.activeVersion') }}: {{ activeVersionLabel }}
          </span>
          <span class="status-pill">
            {{ t('dataPipelines.editingVersion') }}: {{ editingVersionLabel }}
          </span>
        </div>
      </div>
      <div class="page-header-actions data-pipeline-header-actions">
        <ButtonGroup class="data-pipeline-edit-actions" :aria-label="t('dataPipelines.graphSummary')">
          <Button
            variant="secondary"
            size="icon"
            :disabled="!canUndoGraph"
            :title="`${t('common.undo')} (Ctrl/Cmd+Z)`"
            :aria-label="`${t('common.undo')} (Ctrl/Cmd+Z)`"
            @click="undoGraph"
          >
            <RotateCcw :size="16" stroke-width="1.9" aria-hidden="true" />
          </Button>
          <Button
            variant="secondary"
            size="icon"
            :disabled="!canRedoGraph"
            :title="`${t('common.redo')} (Ctrl/Cmd+Shift+Z)`"
            :aria-label="`${t('common.redo')} (Ctrl/Cmd+Shift+Z)`"
            @click="redoGraph"
          >
            <RotateCw :size="16" stroke-width="1.9" aria-hidden="true" />
          </Button>
        </ButtonGroup>

        <ButtonGroup class="data-pipeline-primary-actions" :aria-label="t('common.actions')">
          <Button variant="primary" size="sm" :disabled="store.actionLoading || !selectedPipeline || !editName.trim()" @click="saveDraft">
            <Save :size="16" stroke-width="1.9" aria-hidden="true" />
            {{ t('common.save') }}
          </Button>
          <Button variant="primary" size="sm" :disabled="store.actionLoading || !selectedPipeline" :title="runDisabledReason" @click="runPublished">
            <Play :size="16" stroke-width="1.9" aria-hidden="true" />
            {{ store.actionLoading ? t('dataPipelines.running') : t('dataPipelines.run') }}
          </Button>
        </ButtonGroup>

        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button
              variant="secondary"
              size="icon"
              :aria-label="t('common.actions')"
              :title="t('common.actions')"
            >
              <MoreHorizontal :size="18" stroke-width="1.9" aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuLabel>{{ t('common.actions') }}</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem as-child>
              <RouterLink class="ui-dropdown-menu-link" :to="{ name: 'data-pipelines' }">
                <ArrowLeft :size="16" stroke-width="1.9" aria-hidden="true" />
                {{ t('dataPipelines.backToPipelines') }}
              </RouterLink>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem :disabled="store.status === 'loading'" @select="refreshDetail">
              <RefreshCw :size="16" stroke-width="1.9" aria-hidden="true" />
              {{ t('common.refresh') }}
            </DropdownMenuItem>
            <DropdownMenuItem :disabled="store.actionLoading || !selectedPipeline" @select="openSettings">
              <Settings2 :size="16" stroke-width="1.9" aria-hidden="true" />
              {{ t('dataPipelines.settings') }}
            </DropdownMenuItem>
            <DropdownMenuItem :disabled="!selectedPipeline" @select="openVersionHistory">
              <History :size="16" stroke-width="1.9" aria-hidden="true" />
              {{ t('dataPipelines.changeHistory') }}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem :disabled="activateDisabled" :title="activateButtonTitle" @select="publishLatest">
              <Send :size="16" stroke-width="1.9" aria-hidden="true" />
              {{ activateButtonLabel }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
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
        <ResizablePanelGroup
          v-if="!compactLayout"
          auto-save-id="data-pipeline-detail-main-v2"
          direction="vertical"
          class="data-pipeline-main-resizable"
        >
          <ResizablePanel id="data-pipeline-editor-panel" :default-size="76" :min-size="20" class="data-pipeline-main-panel">
            <div class="data-pipeline-editor-stack">
              <div class="data-pipeline-editor-pane">
                <ResizablePanelGroup
                  auto-save-id="data-pipeline-detail-builder"
                  direction="horizontal"
                  class="data-pipeline-builder-resizable"
                >
                  <ResizablePanel id="data-pipeline-flow-panel" :default-size="74" :min-size="45" class="data-pipeline-builder-panel">
                    <DataPipelineFlowBuilder
                      :graph="store.draftGraph"
                      :node-catalog="nodeCatalog"
                      :selected-node-id="store.selectedNodeId"
                      :validation="store.selectedValidation"
                      @update:graph="updateGraph"
                      @select-node="selectNode"
                    />
                  </ResizablePanel>
                  <ResizableHandle with-handle />
                  <ResizablePanel id="data-pipeline-inspector-panel" :default-size="26" :min-size="18" :max-size="42" class="data-pipeline-builder-panel">
                    <DataPipelineInspector
                      :graph="store.draftGraph"
                      :selected-node-id="store.selectedNodeId"
                      :datasets="datasetStore.items"
                      :work-tables="datasetStore.workTables"
                      :preview="store.selectedPreview"
                      :validation="store.selectedValidation"
                      :pipeline-public-id="selectedPipeline.publicId"
                      @update:graph="updateGraph"
                    />
                  </ResizablePanel>
                </ResizablePanelGroup>
              </div>

              <div class="data-pipeline-feedback" aria-live="polite">
                <section v-if="graphIssueItems.length > 0" class="data-pipeline-graph-issues" :aria-label="t('dataPipelines.graphIssues')">
                  <div class="data-pipeline-graph-issues-header">
                    <strong>{{ t('dataPipelines.graphIssues') }}</strong>
                    <span>{{ t('dataPipelines.graphIssuesCount', { count: graphIssueItems.length }) }}</span>
                  </div>
                  <button
                    v-for="issue in graphIssueItems"
                    :key="`${issue.warning.nodeId}:${issue.warning.code}:${issue.warning.columns.join(',')}`"
                    class="data-pipeline-graph-issue"
                    type="button"
                    @click="selectIssueNode(issue.warning.nodeId)"
                  >
                    <span>{{ issue.nodeLabel }}</span>
                    <small>{{ issue.message }}</small>
                  </button>
                </section>
                <p v-if="store.errorMessage" class="form-error">{{ store.errorMessage }}</p>
                <p v-else-if="store.actionMessage" class="form-success">{{ store.actionMessage }}</p>
              </div>
            </div>
          </ResizablePanel>

          <ResizableHandle with-handle />

          <ResizablePanel id="data-pipeline-preview-panel" :default-size="24" :min-size="7" class="data-pipeline-main-panel">
            <DataPipelinePreviewPanel
              :preview="store.selectedPreview"
              :runs="store.runs"
              :review-items="store.reviewItems"
              :schedules="store.schedules"
              :loading="store.selectedPreviewLoading"
              :action-loading="store.actionLoading"
              :can-preview="canPreview"
              :draft-run-preview="draftRunPreview"
              :preview-disabled-reason="previewDisabledReason"
              @preview="previewSelected"
              @disable-schedule="disableSchedule"
            />
          </ResizablePanel>
        </ResizablePanelGroup>

        <div v-else class="data-pipeline-compact-stack">
          <div class="data-pipeline-editor-pane">
            <div class="data-pipeline-compact-builder-stack">
              <DataPipelineFlowBuilder
                :graph="store.draftGraph"
                :node-catalog="nodeCatalog"
                :selected-node-id="store.selectedNodeId"
                :validation="store.selectedValidation"
                @update:graph="updateGraph"
                @select-node="selectNode"
              />
              <DataPipelineInspector
                :graph="store.draftGraph"
                :selected-node-id="store.selectedNodeId"
                :datasets="datasetStore.items"
                :work-tables="datasetStore.workTables"
                :preview="store.selectedPreview"
                :validation="store.selectedValidation"
                :pipeline-public-id="selectedPipeline.publicId"
                @update:graph="updateGraph"
              />
            </div>
          </div>

          <div class="data-pipeline-feedback" aria-live="polite">
            <section v-if="graphIssueItems.length > 0" class="data-pipeline-graph-issues" :aria-label="t('dataPipelines.graphIssues')">
              <div class="data-pipeline-graph-issues-header">
                <strong>{{ t('dataPipelines.graphIssues') }}</strong>
                <span>{{ t('dataPipelines.graphIssuesCount', { count: graphIssueItems.length }) }}</span>
              </div>
              <button
                v-for="issue in graphIssueItems"
                :key="`${issue.warning.nodeId}:${issue.warning.code}:${issue.warning.columns.join(',')}`"
                class="data-pipeline-graph-issue"
                type="button"
                @click="selectIssueNode(issue.warning.nodeId)"
              >
                <span>{{ issue.nodeLabel }}</span>
                <small>{{ issue.message }}</small>
              </button>
            </section>
            <p v-if="store.errorMessage" class="form-error">{{ store.errorMessage }}</p>
            <p v-else-if="store.actionMessage" class="form-success">{{ store.actionMessage }}</p>
          </div>

          <DataPipelinePreviewPanel
            :preview="store.selectedPreview"
            :runs="store.runs"
            :review-items="store.reviewItems"
            :schedules="store.schedules"
            :loading="store.selectedPreviewLoading"
            :action-loading="store.actionLoading"
            :can-preview="canPreview"
            :draft-run-preview="draftRunPreview"
            :preview-disabled-reason="previewDisabledReason"
            @preview="previewSelected"
            @disable-schedule="disableSchedule"
          />
        </div>
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
        <header class="data-pipeline-settings-header">
          <div>
            <span class="status-pill">{{ t('dataPipelines.settings') }}</span>
            <h2>{{ t('dataPipelines.pipelineSettings') }}</h2>
          </div>
          <button class="secondary-button compact-button" type="button" :disabled="store.actionLoading" @click="closeSettings">
            <X :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('common.close') }}
          </button>
        </header>

        <div class="data-pipeline-settings-content">
          <div class="data-pipeline-settings-basic-grid">
            <label class="field">
              <span class="field-label">{{ t('dataPipelines.name') }}</span>
              <input ref="settingsNameInputRef" v-model="settingsName" class="field-input" autocomplete="off" :disabled="store.actionLoading" required>
            </label>
            <label class="field">
              <span class="field-label">{{ t('dataPipelines.description') }}</span>
              <textarea v-model="settingsDescription" class="field-input data-pipeline-settings-description" autocomplete="off" :disabled="store.actionLoading" rows="3" />
            </label>
          </div>

          <section class="data-pipeline-settings-section" :aria-label="t('dataPipelines.schedule')">
            <div class="data-pipeline-settings-section-header">
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

        <div class="data-pipeline-settings-actions data-pipeline-dialog-actions">
          <button class="secondary-button" type="button" :disabled="store.actionLoading" @click="closeSettings">
            {{ t('common.cancel') }}
          </button>
          <button class="primary-button" type="submit" :disabled="store.actionLoading || !settingsName.trim()">
            {{ t('common.apply') }}
          </button>
        </div>
      </form>
    </dialog>

    <dialog
      ref="versionHistoryDialogRef"
      class="confirm-dialog data-pipeline-version-history-dialog"
      @cancel.prevent="closeVersionHistory"
    >
      <div class="confirm-dialog-panel data-pipeline-version-history-panel">
        <header class="data-pipeline-settings-header">
          <div>
            <span class="status-pill">{{ t('dataPipelines.changeHistory') }}</span>
            <h2>{{ t('dataPipelines.versionHistory') }}</h2>
          </div>
          <button class="secondary-button compact-button" type="button" @click="closeVersionHistory">
            <X :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('common.close') }}
          </button>
        </header>

        <div v-if="versionHistoryItems.length === 0" class="empty-state data-pipeline-version-history-empty">
          <Workflow :size="28" stroke-width="1.8" aria-hidden="true" />
          <h2>{{ t('dataPipelines.noVersionHistory') }}</h2>
        </div>

        <div v-else class="data-pipeline-version-history-content">
          <div class="data-pipeline-version-history-list" role="listbox" :aria-label="t('dataPipelines.versionHistory')">
            <button
              v-for="version in versionHistoryItems"
              :key="version.publicId"
              class="data-pipeline-version-history-row"
              :class="{ selected: selectedHistoryVersion?.publicId === version.publicId }"
              type="button"
              role="option"
              :aria-selected="selectedHistoryVersion?.publicId === version.publicId"
              @click="selectHistoryVersion(version)"
            >
              <span class="data-pipeline-version-history-row-main">
                <strong>{{ t('dataPipelines.versionValue', { version: version.versionNumber }) }}</strong>
                <span class="status-pill" :class="versionHistoryBadgeClass(version)">
                  {{ t(`dataPipelines.${versionHistoryBadgeKey(version)}`) }}
                </span>
              </span>
              <span class="data-pipeline-version-history-meta">
                {{ t('dataPipelines.savedAt') }}: {{ formatDateTime(version.createdAt) }}
              </span>
              <span v-if="version.publishedAt" class="data-pipeline-version-history-meta">
                {{ t('dataPipelines.activatedAt') }}: {{ formatDateTime(version.publishedAt) }}
              </span>
              <span class="data-pipeline-version-history-meta">
                {{ t('dataPipelines.graphSummary') }}:
                {{ t('dataPipelines.nodesCount', { count: version.graph.nodes.length }) }},
                {{ t('dataPipelines.edgesCount', { count: version.graph.edges.length }) }}
              </span>
              <span class="status-pill" :class="version.validationSummary.valid ? 'success' : 'danger'">
                {{ version.validationSummary.valid ? t('dataPipelines.validationValid') : t('dataPipelines.validationInvalid') }}
              </span>
            </button>
          </div>

          <aside v-if="selectedHistoryVersion" class="data-pipeline-version-history-detail">
            <header>
              <span class="status-pill" :class="versionHistoryBadgeClass(selectedHistoryVersion)">
                {{ t(`dataPipelines.${versionHistoryBadgeKey(selectedHistoryVersion)}`) }}
              </span>
              <h3>{{ t('dataPipelines.versionValue', { version: selectedHistoryVersion.versionNumber }) }}</h3>
            </header>

            <dl class="data-pipeline-version-history-facts">
              <div>
                <dt>{{ t('dataPipelines.savedAt') }}</dt>
                <dd>{{ formatDateTime(selectedHistoryVersion.createdAt) }}</dd>
              </div>
              <div>
                <dt>{{ t('dataPipelines.activatedAt') }}</dt>
                <dd>{{ formatDateTime(selectedHistoryVersion.publishedAt) }}</dd>
              </div>
              <div>
                <dt>{{ t('dataPipelines.publicId') }}</dt>
                <dd>{{ selectedHistoryVersion.publicId }}</dd>
              </div>
            </dl>

            <section class="data-pipeline-version-history-section">
              <h4>{{ t('dataPipelines.graphSummary') }}</h4>
              <p>
                {{ t('dataPipelines.nodesCount', { count: selectedHistoryVersion.graph.nodes.length }) }} /
                {{ t('dataPipelines.edgesCount', { count: selectedHistoryVersion.graph.edges.length }) }}
              </p>
              <div class="data-pipeline-version-step-list">
                <span v-for="(stepType, index) in selectedHistoryStepTypes" :key="`${stepType}-${index}`" class="status-pill">
                  {{ stepType }}
                </span>
              </div>
            </section>

            <section class="data-pipeline-version-history-section">
              <h4>{{ t('dataPipelines.validationErrors') }}</h4>
              <ul v-if="selectedHistoryValidationErrors.length > 0">
                <li v-for="error in selectedHistoryValidationErrors" :key="error">{{ error }}</li>
              </ul>
              <p v-else>{{ t('dataPipelines.validationValid') }}</p>
            </section>
          </aside>
        </div>
      </div>
    </dialog>
  </section>
</template>
