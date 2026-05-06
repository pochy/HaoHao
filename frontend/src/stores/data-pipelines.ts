import { defineStore } from 'pinia'

import { isApiForbidden, toApiErrorMessage, toApiErrorStatus } from '../api/client'
import {
  createDataPipeline,
  createDataPipelineRun,
  createDataPipelineSchedule,
  disableDataPipelineSchedule,
  fetchDataPipeline,
  fetchDataPipelineRuns,
  fetchDataPipelines,
  isDataPipelineAutoPreviewEnabled,
  isDataPipelineDraftRunPreviewGraph,
  previewDataPipelineDraft,
  publishDataPipelineVersion,
  sanitizeDataPipelineGraph,
  saveDataPipelineVersion,
  updateDataPipeline,
  updateDataPipelineSchedule,
  type DataPipelineBody,
  type DataPipelineDetailBody,
  type DataPipelineGraph,
  type DataPipelineListParams,
  type DataPipelinePreviewBody,
  type DataPipelineRunBody,
  type DataPipelineScheduleBody,
  type DataPipelineScheduleWriteBody,
  type DataPipelineVersionBody,
} from '../api/data-pipelines'
import { i18n } from '../i18n'

type DataPipelineStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'forbidden' | 'error'
type DataPipelinePreviewCacheEntry = {
  graphSignature: string
  result: DataPipelinePreviewBody
}
type PreviewSelectedOptions = {
  automatic?: boolean
  silent?: boolean
}

export const useDataPipelineStore = defineStore('data-pipelines', {
  state: () => ({
    status: 'idle' as DataPipelineStatus,
    items: [] as DataPipelineBody[],
    nextCursor: '',
    selectedPublicId: '',
    detail: null as DataPipelineDetailBody | null,
    draftGraph: defaultDataPipelineGraph(),
    selectedNodeId: '',
    previewByNodeId: {} as Record<string, DataPipelinePreviewCacheEntry>,
    pendingPreviewKeys: {} as Record<string, true>,
    runs: [] as DataPipelineRunBody[],
    schedules: [] as DataPipelineScheduleBody[],
    actionLoading: false,
    previewLoading: false,
    previewLoadingNodeId: '',
    runsLoading: false,
    errorMessage: '',
    actionMessage: '',
  }),

  getters: {
    selectedPipeline: (state) => (
      state.selectedPublicId
        ? state.items.find((item) => item.publicId === state.selectedPublicId) ?? null
        : state.items[0] ?? null
    ),
    latestVersion: (state): DataPipelineVersionBody | null => state.detail?.versions?.[0] ?? null,
    publishedVersion: (state): DataPipelineVersionBody | null => state.detail?.publishedVersion ?? null,
    hasActiveRuns: (state) => state.runs.some((run) => ['pending', 'processing'].includes(run.status)),
    selectedPreview: (state): DataPipelinePreviewBody | null => {
      const entry = state.selectedNodeId ? state.previewByNodeId[state.selectedNodeId] : null
      if (!entry || entry.graphSignature !== graphPreviewSignature(state.draftGraph)) {
        return null
      }
      return entry.result
    },
    selectedPreviewLoading: (state): boolean => state.previewLoading && state.previewLoadingNodeId === state.selectedNodeId,
    selectedAutoPreviewKey: (state): string => {
      const node = state.draftGraph.nodes.find((item) => item.id === state.selectedNodeId)
      if (!state.selectedPublicId || !node) {
        return ''
      }
      if (!isDataPipelineDraftRunPreviewGraph(state.draftGraph) && !isDataPipelineAutoPreviewEnabled(node.data)) {
        return ''
      }
      return `${state.selectedPublicId}:${node.id}:${graphPreviewSignature(state.draftGraph)}`
    },
  },

  actions: {
    reset() {
      this.status = 'idle'
      this.items = []
      this.nextCursor = ''
      this.selectedPublicId = ''
      this.detail = null
      this.draftGraph = defaultDataPipelineGraph()
      this.selectedNodeId = ''
      this.previewByNodeId = {}
      this.pendingPreviewKeys = {}
      this.runs = []
      this.schedules = []
      this.errorMessage = ''
      this.actionMessage = ''
      this.actionLoading = false
      this.previewLoading = false
      this.previewLoadingNodeId = ''
      this.runsLoading = false
    },

    async load(loadFirstDetail = false, params: DataPipelineListParams = {}, append = false) {
      this.status = 'loading'
      this.errorMessage = ''
      try {
        const result = await fetchDataPipelines(params)
        this.items = append ? [...this.items, ...result.items] : result.items
        this.nextCursor = result.nextCursor
        if (this.selectedPublicId && !this.items.some((item) => item.publicId === this.selectedPublicId)) {
          this.selectedPublicId = ''
        }
        this.status = this.items.length > 0 ? 'ready' : 'empty'
        if (loadFirstDetail) {
          if (!this.selectedPublicId) {
            this.selectedPublicId = this.items[0]?.publicId ?? ''
          }
        }
        if (loadFirstDetail && this.selectedPublicId) {
          await this.loadDetail(this.selectedPublicId)
        } else if (!loadFirstDetail) {
          this.detail = null
        }
      } catch (error) {
        this.items = []
        this.nextCursor = ''
        this.detail = null
        this.status = toApiErrorStatus(error) === 403 || isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async loadDetail(publicId: string) {
      this.selectedPublicId = publicId
      this.errorMessage = ''
      const selectedNodeId = this.selectedNodeId
      try {
        this.detail = await fetchDataPipeline(publicId)
        this.runs = this.detail.runs ?? []
        this.schedules = this.detail.schedules ?? []
        this.draftGraph = cloneGraph(this.detail.versions?.[0]?.graph ?? defaultDataPipelineGraph())
        this.selectedNodeId = this.draftGraph.nodes.some((node) => node.id === selectedNodeId)
          ? selectedNodeId
          : this.draftGraph.nodes[0]?.id ?? ''
        this.previewByNodeId = {}
        this.pendingPreviewKeys = {}
        this.status = 'ready'
      } catch (error) {
        this.detail = null
        this.status = toApiErrorStatus(error) === 403 || isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
        throw error
      }
    },

    async create(name: string, description = '') {
      this.actionLoading = true
      this.errorMessage = ''
      try {
        const item = await createDataPipeline({ name, description })
        this.items = [item, ...this.items.filter((existing) => existing.publicId !== item.publicId)]
        this.selectedPublicId = item.publicId
        await this.loadDetail(item.publicId)
        return item
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionLoading = false
      }
    },

    async update(name: string, description = '') {
      if (!this.selectedPublicId) {
        return null
      }
      this.actionLoading = true
      this.errorMessage = ''
      try {
        const item = await updateDataPipeline(this.selectedPublicId, { name, description })
        this.items = this.items.map((existing) => existing.publicId === item.publicId ? item : existing)
        if (this.detail) {
          this.detail.pipeline = item
        }
        return item
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionLoading = false
      }
    },

    async saveDraft() {
      if (!this.selectedPublicId) {
        this.errorMessage = dataPipelineText('messages.selectOrCreateBeforeSaving')
        return null
      }
      this.actionLoading = true
      this.errorMessage = ''
      try {
        const version = await saveDataPipelineVersion(this.selectedPublicId, this.draftGraph)
        await this.loadDetail(this.selectedPublicId)
        this.actionMessage = dataPipelineText('messages.saved', { version: version.versionNumber })
        return version
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionLoading = false
      }
    },

    async publishLatest() {
      const version = this.latestVersion
      if (!version) {
        this.errorMessage = dataPipelineText('messages.saveBeforePublishing')
        return null
      }
      this.actionLoading = true
      this.errorMessage = ''
      try {
        const published = await publishDataPipelineVersion(version.publicId)
        if (this.selectedPublicId) {
          await this.loadDetail(this.selectedPublicId)
        }
        this.actionMessage = dataPipelineText('messages.published', { version: published.versionNumber })
        return published
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionLoading = false
      }
    },

    async previewSelected(options: PreviewSelectedOptions = {}) {
      if (!this.selectedPublicId) {
        this.selectedPublicId = this.detail?.pipeline.publicId ?? this.items[0]?.publicId ?? ''
      }
      if (!this.selectedPublicId) {
        this.errorMessage = dataPipelineText('messages.selectOrCreateBeforeAction')
        return null
      }
      const nodeId = this.selectedNodeId || this.defaultPreviewNodeId()
      if (!nodeId) {
        this.errorMessage = dataPipelineText('messages.selectNodeBeforePreviewing')
        return null
      }
      const node = this.draftGraph.nodes.find((item) => item.id === nodeId)
      if (!node) {
        return null
      }
      if (options.automatic && !isDataPipelineDraftRunPreviewGraph(this.draftGraph) && !isDataPipelineAutoPreviewEnabled(node.data)) {
        return null
      }
      this.selectedNodeId = nodeId
      const graphSignature = graphPreviewSignature(this.draftGraph)
      const cached = this.previewByNodeId[nodeId]
      if (cached?.graphSignature === graphSignature) {
        return cached.result
      }
      const pendingKey = `${nodeId}:${graphSignature}`
      if (this.pendingPreviewKeys[pendingKey]) {
        return null
      }
      this.pendingPreviewKeys = {
        ...this.pendingPreviewKeys,
        [pendingKey]: true,
      }
      this.previewLoading = true
      this.previewLoadingNodeId = nodeId
      if (!options.silent) {
        this.errorMessage = ''
        this.actionMessage = ''
      }
      try {
        const preview = await previewDataPipelineDraft(this.selectedPublicId, this.draftGraph, nodeId, 100)
        this.previewByNodeId = {
          ...this.previewByNodeId,
          [nodeId]: {
            graphSignature,
            result: preview,
          },
        }
        if (!options.silent) {
          this.actionMessage = dataPipelineText('messages.previewedDraft')
        }
        return preview
      } catch (error) {
        this.previewByNodeId = omitPreview(this.previewByNodeId, nodeId)
        if (!options.silent) {
          this.errorMessage = toApiErrorMessage(error)
        }
        throw error
      } finally {
        this.pendingPreviewKeys = omitPendingPreview(this.pendingPreviewKeys, pendingKey)
        if (this.previewLoadingNodeId === nodeId) {
          this.previewLoading = false
          this.previewLoadingNodeId = ''
        }
      }
    },

    async autoPreviewSelected() {
      return await this.previewSelected({ automatic: true, silent: true })
    },

    async runPublished() {
      this.actionLoading = true
      this.errorMessage = ''
      this.actionMessage = ''
      try {
        const version = await this.ensureDraftVersion()
        if (!version) {
          return null
        }
        let runVersion = version
        if (this.publishedVersion?.publicId !== version.publicId || version.status !== 'published') {
          runVersion = await publishDataPipelineVersion(version.publicId)
          if (this.selectedPublicId) {
            await this.loadDetail(this.selectedPublicId)
          }
        }
        const run = await createDataPipelineRun(runVersion.publicId)
        this.runs = [run, ...this.runs.filter((item) => item.publicId !== run.publicId)]
        this.actionMessage = dataPipelineText('messages.runRequested', { version: runVersion.versionNumber })
        return run
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionLoading = false
      }
    },

    async refreshRuns() {
      if (!this.selectedPublicId) {
        this.errorMessage = dataPipelineText('messages.selectOrCreateBeforeRefreshingRuns')
        return
      }
      this.runsLoading = true
      this.errorMessage = ''
      try {
        this.runs = await fetchDataPipelineRuns(this.selectedPublicId)
        this.actionMessage = dataPipelineText('messages.runsRefreshed')
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.runsLoading = false
      }
    },

    async ensureDraftVersion() {
      if (!this.selectedPublicId) {
        this.selectedPublicId = this.detail?.pipeline.publicId ?? this.items[0]?.publicId ?? ''
      }
      if (!this.selectedPublicId) {
        this.errorMessage = dataPipelineText('messages.selectOrCreateBeforeAction')
        return null
      }
      const latest = this.latestVersion
      if (latest && graphsEqual(latest.graph, this.draftGraph)) {
        return latest
      }
      const version = await saveDataPipelineVersion(this.selectedPublicId, this.draftGraph)
      await this.loadDetail(this.selectedPublicId)
      return version
    },

    defaultPreviewNodeId() {
      return this.draftGraph.nodes.find((node) => node.data.stepType === 'output')?.id
        ?? this.draftGraph.nodes[0]?.id
        ?? ''
    },

    async createSchedule(body: DataPipelineScheduleWriteBody) {
      this.actionLoading = true
      this.errorMessage = ''
      this.actionMessage = ''
      try {
        const version = await this.ensureDraftVersion()
        if (!version) {
          return null
        }
        if (this.publishedVersion?.publicId !== version.publicId || version.status !== 'published') {
          await publishDataPipelineVersion(version.publicId)
          if (this.selectedPublicId) {
            await this.loadDetail(this.selectedPublicId)
          }
        }
        const schedule = await createDataPipelineSchedule(this.selectedPublicId, body)
        this.schedules = [schedule, ...this.schedules.filter((item) => item.publicId !== schedule.publicId)]
        this.actionMessage = dataPipelineText('messages.scheduleAdded')
        return schedule
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionLoading = false
      }
    },

    async updateSchedule(publicId: string, body: DataPipelineScheduleWriteBody) {
      this.actionLoading = true
      this.errorMessage = ''
      try {
        const schedule = await updateDataPipelineSchedule(publicId, body)
        this.schedules = this.schedules.map((item) => item.publicId === schedule.publicId ? schedule : item)
        return schedule
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionLoading = false
      }
    },

    async disableSchedule(publicId: string) {
      this.actionLoading = true
      this.errorMessage = ''
      try {
        const schedule = await disableDataPipelineSchedule(publicId)
        this.schedules = this.schedules.map((item) => item.publicId === schedule.publicId ? schedule : item)
        return schedule
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionLoading = false
      }
    },
  },
})

export function defaultDataPipelineGraph(): DataPipelineGraph {
  return {
    nodes: [
      {
        id: 'input_1',
        type: 'pipelineStep',
        position: { x: 60, y: 120 },
        data: {
          label: 'Input',
          stepType: 'input',
          config: { sourceKind: 'dataset', datasetPublicId: '' },
        },
      },
      {
        id: 'output_1',
        type: 'pipelineStep',
        position: { x: 520, y: 120 },
        data: {
          label: 'Output',
          stepType: 'output',
          config: { displayName: dataPipelineText('defaultOutputDisplayName'), writeMode: 'replace', engine: 'MergeTree' },
        },
      },
    ],
    edges: [{ id: 'edge_input_output', source: 'input_1', target: 'output_1' }],
  }
}

function cloneGraph(graph: DataPipelineGraph): DataPipelineGraph {
  return sanitizeDataPipelineGraph(JSON.parse(JSON.stringify(graph)) as DataPipelineGraph)
}

function graphsEqual(a: DataPipelineGraph, b: DataPipelineGraph): boolean {
  return JSON.stringify(sanitizeDataPipelineGraph(a)) === JSON.stringify(sanitizeDataPipelineGraph(b))
}

function graphPreviewSignature(graph: DataPipelineGraph): string {
  const sanitized = sanitizeDataPipelineGraph(graph)
  return JSON.stringify({
    nodes: sanitized.nodes
      .map((node) => ({
        id: node.id,
        stepType: node.data.stepType,
        config: stableValue(node.data.config ?? {}),
      }))
      .sort((a, b) => a.id.localeCompare(b.id)),
    edges: sanitized.edges
      .map((edge) => ({
        source: edge.source,
        target: edge.target,
      }))
      .sort((a, b) => `${a.source}\u0000${a.target}`.localeCompare(`${b.source}\u0000${b.target}`)),
  })
}

function stableValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => stableValue(item))
  }
  if (value && typeof value === 'object') {
    const record = value as Record<string, unknown>
    return Object.keys(record)
      .sort((a, b) => a.localeCompare(b))
      .reduce<Record<string, unknown>>((acc, key) => {
        acc[key] = stableValue(record[key])
        return acc
      }, {})
  }
  return value
}

function omitPreview(cache: Record<string, DataPipelinePreviewCacheEntry>, nodeId: string): Record<string, DataPipelinePreviewCacheEntry> {
  const next = { ...cache }
  delete next[nodeId]
  return next
}

function omitPendingPreview(cache: Record<string, true>, key: string): Record<string, true> {
  const next = { ...cache }
  delete next[key]
  return next
}

function dataPipelineText(key: string, params?: Record<string, unknown>) {
  const translate = i18n.global.t as unknown as (path: string, values?: Record<string, unknown>) => string
  const path = `dataPipelines.${key}`
  return params ? translate(path, params) : translate(path)
}
