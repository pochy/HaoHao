import { defineStore } from 'pinia'

import { isApiForbidden, toApiErrorMessage, toApiErrorStatus } from '../api/client'
import type { DatasetBody, DatasetGoldPublicationBody, DatasetGoldPublicationCreateBodyWritable, DatasetGoldPublicationPreviewBody, DatasetGoldPublishRunBody, DatasetLineageBody, DatasetLineageChangeSetBody, DatasetLineageChangeSetGraphBody, DatasetLineageGraphSaveBodyWritable, DatasetLineageParseRunBody, DatasetQueryJobBody, DatasetSourceFileBody, DatasetSyncJobBody, DatasetWorkTableBody, DatasetWorkTableExportBody, DatasetWorkTableExportScheduleBody, DatasetWorkTableExportScheduleCreateBodyWritable, DatasetWorkTableExportScheduleUpdateBodyWritable, DatasetWorkTablePreviewBody, MedallionCatalogBody } from '../api/generated/types.gen'
import {
  archiveGoldPublication,
  createLineageChangeSet,
  createGoldPublication,
  createWorkTableExportSchedule,
  createDatasetFromDriveFile,
  createDatasetQuery,
  createDatasetScopedQuery,
  disableWorkTableExportSchedule,
  deleteDatasetItem,
  dropWorkTable,
  fetchDataset,
  fetchDatasetLineage,
  fetchDatasetLineageChangeSets,
  fetchDatasetLinkedWorkTables,
  fetchDatasetQueryJobLineageParseRuns,
  fetchDatasetScopedQueryJobs,
  fetchDatasetSyncJobs,
  fetchDatasets,
  fetchDatasetSourceFiles,
  fetchDatasetWorkTableLineage,
  fetchDatasetWorkTable,
  fetchDatasetWorkTablePreview,
  fetchDatasetWorkTables,
  fetchGoldPublication,
  fetchGoldPublicationPreview,
  fetchGoldPublications,
  fetchGoldPublishRuns,
  fetchManagedDatasetWorkTable,
  fetchManagedDatasetWorkTablePreview,
  fetchWorkTableExportSchedules,
  fetchWorkTableExports,
  fetchLineageChangeSet,
  linkWorkTable,
  publishLineageChangeSet,
  promoteWorkTable,
  registerWorkTable,
  rejectLineageChangeSet,
  requestDatasetQueryJobLineageParse,
  requestDatasetSync,
  refreshGoldPublication,
  renameWorkTable,
  requestWorkTableExport,
  saveLineageChangeSetGraph,
  truncateWorkTable,
  unpublishGoldPublication,
  updateWorkTableExportSchedule,
} from '../api/datasets'
import { fetchMedallionResourceCatalog } from '../api/medallion'
import type { DatasetLineageLevel, DatasetLineageSource, DatasetWorkTableExportFormat } from '../api/datasets'

type DatasetStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'forbidden' | 'error'

export const useDatasetStore = defineStore('datasets', {
  state: () => ({
    status: 'idle' as DatasetStatus,
    items: [] as DatasetBody[],
    selectedPublicId: '',
    sourceFiles: [] as DatasetSourceFileBody[],
    selectedSourceFilePublicId: '',
    workTables: [] as DatasetWorkTableBody[],
    linkedWorkTables: [] as DatasetWorkTableBody[],
    selectedWorkTable: null as DatasetWorkTableBody | null,
    workTablePreview: null as DatasetWorkTablePreviewBody | null,
    goldPublications: [] as DatasetGoldPublicationBody[],
    selectedGoldPublication: null as DatasetGoldPublicationBody | null,
    goldPublicationPreview: null as DatasetGoldPublicationPreviewBody | null,
    goldPublishRuns: [] as DatasetGoldPublishRunBody[],
    workTableExports: [] as DatasetWorkTableExportBody[],
    workTableExportSchedules: [] as DatasetWorkTableExportScheduleBody[],
    datasetMedallionCatalog: null as MedallionCatalogBody | null,
    workTableMedallionCatalog: null as MedallionCatalogBody | null,
    goldMedallionCatalog: null as MedallionCatalogBody | null,
    datasetLineage: null as DatasetLineageBody | null,
    workTableLineage: null as DatasetLineageBody | null,
    lineageLevel: 'table' as DatasetLineageLevel,
    lineageSources: ['metadata', 'parser', 'manual'] as DatasetLineageSource[],
    lineageChangeSets: [] as DatasetLineageChangeSetBody[],
    selectedLineageChangeSet: null as DatasetLineageChangeSetGraphBody | null,
    lineageParseRuns: [] as DatasetLineageParseRunBody[],
    lineageActionLoading: false,
    datasetLineageLoading: false,
    workTableLineageLoading: false,
    workTablesLoading: false,
    workTablePreviewLoading: false,
    goldPublicationsLoading: false,
    goldPublicationLoading: false,
    goldPreviewLoading: false,
    goldRunsLoading: false,
    datasetMedallionLoading: false,
    workTableMedallionLoading: false,
    goldMedallionLoading: false,
    workTableActionLoading: false,
    goldActionLoading: false,
    workTableErrorMessage: '',
    goldErrorMessage: '',
    goldPreviewErrorMessage: '',
    queryJobs: [] as DatasetQueryJobBody[],
    syncJobs: [] as DatasetSyncJobBody[],
    latestQuery: null as DatasetQueryJobBody | null,
    errorMessage: '',
    importing: false,
    syncing: false,
    executing: false,
    deletingPublicId: '',
  }),

  getters: {
    selectedDataset: (state) => (
      state.selectedPublicId
        ? state.items.find((item) => item.publicId === state.selectedPublicId) ?? null
        : state.items[0] ?? null
    ),
    selectedSourceFile: (state) => (
      state.sourceFiles.find((item) => item.publicId === state.selectedSourceFilePublicId) ?? state.sourceFiles[0] ?? null
    ),
    hasActiveImports: (state) => state.items.some((item) => ['pending', 'importing'].includes(item.status)),
    hasActiveWorkTableExports: (state) => state.workTableExports.some((item) => ['pending', 'processing'].includes(item.status)),
    hasActiveGoldPublishRuns: (state) => (
      state.goldPublishRuns.some((item) => ['pending', 'processing'].includes(item.status))
      || state.goldPublications.some((item) => item.status === 'pending' || ['pending', 'processing'].includes(item.latestPublishRun?.status ?? ''))
    ),
    hasActiveDatasetSync: (state) => {
      if (state.syncJobs.some((item) => ['pending', 'processing'].includes(item.status))) {
        return true
      }
      const selected = state.items.find((item) => item.publicId === state.selectedPublicId)
      return ['pending', 'processing'].includes(selected?.latestSyncJob?.status ?? '')
    },
    lineageFetchOptions: (state) => ({
      level: state.lineageLevel,
      sources: state.lineageSources,
      includeDraft: Boolean(state.selectedLineageChangeSet?.changeSet.publicId),
      changeSetPublicId: state.selectedLineageChangeSet?.changeSet.publicId,
    }),
  },

  actions: {
    async load() {
      this.status = 'loading'
      this.errorMessage = ''
      try {
        const [datasets, sourceFiles] = await Promise.all([
          fetchDatasets(),
          fetchDatasetSourceFiles(),
        ])
        this.items = datasets
        this.sourceFiles = sourceFiles
        if (!this.selectedSourceFilePublicId || !this.sourceFiles.some((item) => item.publicId === this.selectedSourceFilePublicId)) {
          this.selectedSourceFilePublicId = this.sourceFiles[0]?.publicId ?? ''
        }
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.items = []
        this.queryJobs = []
        this.syncJobs = []
        this.datasetLineage = null
        this.workTableLineage = null
        this.datasetMedallionCatalog = null
        this.workTableMedallionCatalog = null
        this.lineageChangeSets = []
        this.selectedLineageChangeSet = null
        this.lineageParseRuns = []
        this.sourceFiles = []
        this.status = toApiErrorStatus(error) === 403 || isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async loadDataset(datasetPublicId: string) {
      this.status = 'loading'
      this.errorMessage = ''
      this.selectedPublicId = datasetPublicId
      try {
        const item = await fetchDataset(datasetPublicId)
        this.items = [item, ...this.items.filter((existing) => existing.publicId !== item.publicId)]
        await this.loadDatasetMedallion(item.publicId).catch(() => undefined)
        this.status = 'ready'
      } catch (error) {
        this.status = toApiErrorStatus(error) === 403 || isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
        throw error
      }
    },

    async loadDatasetSyncJobs(datasetPublicId: string) {
      this.errorMessage = ''
      try {
        this.syncJobs = await fetchDatasetSyncJobs(datasetPublicId)
      } catch (error) {
        this.syncJobs = []
        this.errorMessage = toApiErrorMessage(error)
        throw error
      }
    },

    async loadDatasetLineage(datasetPublicId: string) {
      this.datasetLineageLoading = true
      this.errorMessage = ''
      try {
        this.datasetLineage = await fetchDatasetLineage(datasetPublicId, 'both', this.lineageFetchOptions)
      } catch (error) {
        this.datasetLineage = null
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.datasetLineageLoading = false
      }
    },

    async requestSelectedDatasetSync() {
      const publicId = this.selectedPublicId
      if (!publicId) {
        return null
      }
      this.syncing = true
      this.errorMessage = ''
      try {
        const item = await requestDatasetSync(publicId, { mode: 'full_refresh' })
        this.syncJobs = [item, ...this.syncJobs.filter((syncJob) => syncJob.publicId !== item.publicId)]
        const selected = this.items.find((dataset) => dataset.publicId === publicId)
        if (selected) {
          selected.latestSyncJob = item
        }
        return item
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.syncing = false
      }
    },

    applyDatasetSyncUpdate(update: Partial<DatasetSyncJobBody> & { publicId: string }) {
      const index = this.syncJobs.findIndex((item) => item.publicId === update.publicId)
      if (index >= 0) {
        this.syncJobs[index] = {
          ...this.syncJobs[index],
          ...update,
        }
      }
      const selected = this.items.find((item) => item.publicId === this.selectedPublicId)
      if (selected?.latestSyncJob?.publicId === update.publicId) {
        selected.latestSyncJob = {
          ...selected.latestSyncJob,
          ...update,
        }
      }
      return index >= 0 || selected?.latestSyncJob?.publicId === update.publicId
    },

    async refreshSelected() {
      if (!this.selectedPublicId) {
        return
      }
      try {
        const updated = await fetchDataset(this.selectedPublicId)
        this.items = this.items.map((item) => item.publicId === updated.publicId ? updated : item)
        await this.loadDatasetMedallion(updated.publicId).catch(() => undefined)
      } catch {
        await this.load()
      }
    },

    async refreshSourceFiles(query = '') {
      this.errorMessage = ''
      try {
        this.sourceFiles = await fetchDatasetSourceFiles(query)
        if (!this.selectedSourceFilePublicId || !this.sourceFiles.some((item) => item.publicId === this.selectedSourceFilePublicId)) {
          this.selectedSourceFilePublicId = this.sourceFiles[0]?.publicId ?? ''
        }
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      }
    },

    async loadWorkTables() {
      this.workTablesLoading = true
      this.workTableErrorMessage = ''
      try {
        const items = await fetchDatasetWorkTables()
        this.workTables = items
        const current = this.selectedWorkTable
        const next = current
          ? items.find((item) => sameWorkTable(item, current)) ?? items[0] ?? null
          : items[0] ?? null
        if (!next) {
          this.selectedWorkTable = null
          this.workTablePreview = null
          this.workTableExports = []
          this.workTableExportSchedules = []
          this.workTableLineage = null
          this.workTableMedallionCatalog = null
          return
        }
        await this.selectWorkTable(next)
      } catch (error) {
        this.workTables = []
        this.selectedWorkTable = null
        this.workTablePreview = null
        this.workTableExports = []
        this.workTableExportSchedules = []
        this.workTableLineage = null
        this.workTableMedallionCatalog = null
        this.workTableErrorMessage = toApiErrorMessage(error)
      } finally {
        this.workTablesLoading = false
      }
    },

    async loadGoldPublications() {
      this.goldPublicationsLoading = true
      this.goldErrorMessage = ''
      try {
        this.goldPublications = await fetchGoldPublications()
      } catch (error) {
        this.goldPublications = []
        this.goldErrorMessage = toApiErrorMessage(error)
      } finally {
        this.goldPublicationsLoading = false
      }
    },

    async loadGoldPublication(goldPublicId: string) {
      if (!goldPublicId) {
        this.selectedGoldPublication = null
        this.goldPublicationPreview = null
        this.goldPublishRuns = []
        this.goldMedallionCatalog = null
        return null
      }
      this.goldPublicationLoading = true
      this.goldErrorMessage = ''
      this.goldPreviewErrorMessage = ''
      try {
        const item = await fetchGoldPublication(goldPublicId)
        this.selectedGoldPublication = item
        this.goldPublications = upsertGoldPublication(this.goldPublications, item)
        await Promise.all([
          this.loadGoldPublicationPreview(goldPublicId).catch(() => undefined),
          this.loadGoldPublishRuns(goldPublicId).catch(() => undefined),
          this.loadGoldMedallion(goldPublicId).catch(() => undefined),
        ])
        return item
      } catch (error) {
        this.selectedGoldPublication = null
        this.goldPublicationPreview = null
        this.goldPublishRuns = []
        this.goldMedallionCatalog = null
        this.goldErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.goldPublicationLoading = false
      }
    },

    async loadGoldPublicationPreview(goldPublicId: string) {
      if (!goldPublicId) {
        this.goldPublicationPreview = null
        return null
      }
      this.goldPreviewLoading = true
      this.goldPreviewErrorMessage = ''
      try {
        const preview = await fetchGoldPublicationPreview(goldPublicId)
        this.goldPublicationPreview = preview
        return preview
      } catch (error) {
        this.goldPublicationPreview = null
        this.goldPreviewErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.goldPreviewLoading = false
      }
    },

    async loadGoldPublishRuns(goldPublicId: string) {
      if (!goldPublicId) {
        this.goldPublishRuns = []
        return []
      }
      this.goldRunsLoading = true
      try {
        this.goldPublishRuns = await fetchGoldPublishRuns(goldPublicId)
        return this.goldPublishRuns
      } catch (error) {
        this.goldPublishRuns = []
        this.goldErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.goldRunsLoading = false
      }
    },

    async selectWorkTable(table: DatasetWorkTableBody) {
      const existing = this.workTables.find((item) => sameWorkTable(item, table)) ?? table
      this.selectedWorkTable = existing
      this.workTablePreview = null
      this.workTableExports = []
      this.workTableExportSchedules = []
      this.workTableLineage = null
      this.workTableMedallionCatalog = null
      this.workTablePreviewLoading = true
      this.workTableErrorMessage = ''
      try {
        if (existing.publicId && existing.managed && existing.status !== 'active') {
          const [detail, exports, schedules] = await Promise.all([
            fetchManagedDatasetWorkTable(existing.publicId),
            fetchWorkTableExports(existing.publicId),
            fetchWorkTableExportSchedules(existing.publicId),
          ])
          this.selectedWorkTable = detail
          this.workTableExports = exports
          this.workTableExportSchedules = schedules
          if (detail.publicId) {
            await this.loadWorkTableMedallion(detail.publicId).catch(() => undefined)
          }
          await this.loadSelectedWorkTableLineage()
          return
        }
        const [detail, preview, exports, schedules] = existing.publicId && existing.managed
          ? await Promise.all([
              fetchManagedDatasetWorkTable(existing.publicId),
              fetchManagedDatasetWorkTablePreview(existing.publicId),
              fetchWorkTableExports(existing.publicId),
              fetchWorkTableExportSchedules(existing.publicId),
            ])
          : await Promise.all([
              fetchDatasetWorkTable(existing.database, existing.table),
              fetchDatasetWorkTablePreview(existing.database, existing.table),
              Promise.resolve([] as DatasetWorkTableExportBody[]),
              Promise.resolve([] as DatasetWorkTableExportScheduleBody[]),
            ])
        this.selectedWorkTable = detail
        this.workTablePreview = preview
        this.workTableExports = exports
        this.workTableExportSchedules = schedules
        if (detail.publicId && detail.managed) {
          await this.loadWorkTableMedallion(detail.publicId).catch(() => undefined)
        }
        if (detail.publicId && detail.managed) {
          await this.loadSelectedWorkTableLineage()
        }
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
      } finally {
        this.workTablePreviewLoading = false
      }
    },

    async loadDatasetMedallion(datasetPublicId: string) {
      if (!datasetPublicId) {
        this.datasetMedallionCatalog = null
        return
      }
      this.datasetMedallionLoading = true
      try {
        this.datasetMedallionCatalog = await fetchMedallionResourceCatalog('dataset', datasetPublicId)
      } catch {
        this.datasetMedallionCatalog = null
      } finally {
        this.datasetMedallionLoading = false
      }
    },

    async loadWorkTableMedallion(workTablePublicId: string) {
      if (!workTablePublicId) {
        this.workTableMedallionCatalog = null
        return
      }
      this.workTableMedallionLoading = true
      try {
        this.workTableMedallionCatalog = await fetchMedallionResourceCatalog('work_table', workTablePublicId)
      } catch {
        this.workTableMedallionCatalog = null
      } finally {
        this.workTableMedallionLoading = false
      }
    },

    async loadGoldMedallion(goldPublicId: string) {
      if (!goldPublicId) {
        this.goldMedallionCatalog = null
        return
      }
      this.goldMedallionLoading = true
      try {
        this.goldMedallionCatalog = await fetchMedallionResourceCatalog('gold_table', goldPublicId)
      } catch {
        this.goldMedallionCatalog = null
      } finally {
        this.goldMedallionLoading = false
      }
    },

    async loadLinkedWorkTables(datasetPublicId: string) {
      this.workTableErrorMessage = ''
      try {
        this.linkedWorkTables = await fetchDatasetLinkedWorkTables(datasetPublicId)
      } catch (error) {
        this.linkedWorkTables = []
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      }
    },

    async registerSelectedWorkTable(datasetPublicId = '') {
      const table = this.selectedWorkTable
      if (!table) {
        return null
      }
      this.workTableActionLoading = true
      this.workTableErrorMessage = ''
      try {
        const updated = await registerWorkTable({
          database: table.database,
          table: table.table,
          displayName: table.displayName || table.table,
          ...(datasetPublicId ? { datasetPublicId } : {}),
        })
        await this.loadWorkTables()
        this.selectedWorkTable = updated
        return updated
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.workTableActionLoading = false
      }
    },

    async linkSelectedWorkTable(datasetPublicId: string) {
      const publicId = this.selectedWorkTable?.publicId
      if (!publicId) {
        return null
      }
      this.workTableActionLoading = true
      this.workTableErrorMessage = ''
      try {
        const updated = await linkWorkTable(publicId, { datasetPublicId })
        await this.loadWorkTables()
        this.selectedWorkTable = updated
        return updated
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.workTableActionLoading = false
      }
    },

    async renameSelectedWorkTable(tableName: string) {
      const publicId = this.selectedWorkTable?.publicId
      if (!publicId) {
        return null
      }
      this.workTableActionLoading = true
      this.workTableErrorMessage = ''
      try {
        const updated = await renameWorkTable(publicId, { table: tableName.trim() })
        await this.loadWorkTables()
        await this.selectWorkTable(updated)
        return updated
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.workTableActionLoading = false
      }
    },

    async truncateSelectedWorkTable() {
      const publicId = this.selectedWorkTable?.publicId
      if (!publicId) {
        return null
      }
      this.workTableActionLoading = true
      this.workTableErrorMessage = ''
      try {
        const updated = await truncateWorkTable(publicId)
        await this.loadWorkTables()
        await this.selectWorkTable(updated)
        return updated
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.workTableActionLoading = false
      }
    },

    async dropSelectedWorkTable() {
      const publicId = this.selectedWorkTable?.publicId
      if (!publicId) {
        return
      }
      this.workTableActionLoading = true
      this.workTableErrorMessage = ''
      try {
        await dropWorkTable(publicId)
        await this.loadWorkTables()
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.workTableActionLoading = false
      }
    },

    async promoteSelectedWorkTable(name: string) {
      const publicId = this.selectedWorkTable?.publicId
      if (!publicId) {
        return null
      }
      this.workTableActionLoading = true
      this.workTableErrorMessage = ''
      try {
        const dataset = await promoteWorkTable(publicId, { ...(name.trim() ? { name: name.trim() } : {}) })
        this.items = [dataset, ...this.items.filter((item) => item.publicId !== dataset.publicId)]
        return dataset
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.workTableActionLoading = false
      }
    },

    async publishSelectedWorkTableToGold(body: DatasetGoldPublicationCreateBodyWritable) {
      const publicId = this.selectedWorkTable?.publicId
      if (!publicId) {
        return null
      }
      return this.publishWorkTableToGold(publicId, body)
    },

    async publishWorkTableToGold(publicId: string, body: DatasetGoldPublicationCreateBodyWritable) {
      if (!publicId) {
        return null
      }
      this.workTableActionLoading = true
      this.workTableErrorMessage = ''
      try {
        const item = await createGoldPublication(publicId, body)
        this.goldPublications = upsertGoldPublication(this.goldPublications, item)
        this.selectedGoldPublication = item
        await this.loadSelectedWorkTableLineage()
        return item
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.workTableActionLoading = false
      }
    },

    async requestSelectedWorkTableExport(format: DatasetWorkTableExportFormat = 'csv') {
      const publicId = this.selectedWorkTable?.publicId
      if (!publicId) {
        return null
      }
      this.workTableActionLoading = true
      this.workTableErrorMessage = ''
      try {
        const item = await requestWorkTableExport(publicId, format)
        this.workTableExports = [item, ...this.workTableExports.filter((exportItem) => exportItem.publicId !== item.publicId)]
        await this.loadSelectedWorkTableLineage()
        return item
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.workTableActionLoading = false
      }
    },

    async refreshSelectedWorkTableExports() {
      const publicId = this.selectedWorkTable?.publicId
      if (!publicId || !this.selectedWorkTable?.managed) {
        return
      }
      try {
        this.workTableExports = await fetchWorkTableExports(publicId)
        await this.loadSelectedWorkTableLineage()
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
      }
    },

    async refreshSelectedWorkTableExportSchedules() {
      const publicId = this.selectedWorkTable?.publicId
      if (!publicId || !this.selectedWorkTable?.managed) {
        return
      }
      try {
        this.workTableExportSchedules = await fetchWorkTableExportSchedules(publicId)
        await this.loadSelectedWorkTableLineage()
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
      }
    },

    async loadSelectedWorkTableLineage() {
      const publicId = this.selectedWorkTable?.publicId
      if (!publicId || !this.selectedWorkTable?.managed) {
        this.workTableLineage = null
        return
      }
      this.workTableLineageLoading = true
      try {
        this.workTableLineage = await fetchDatasetWorkTableLineage(publicId, 'both', this.lineageFetchOptions)
      } catch (error) {
        this.workTableLineage = null
        this.workTableErrorMessage = toApiErrorMessage(error)
      } finally {
        this.workTableLineageLoading = false
      }
    },

    setLineageLevel(level: DatasetLineageLevel) {
      this.lineageLevel = level
    },

    toggleLineageSource(source: DatasetLineageSource) {
      if (this.lineageSources.includes(source)) {
        this.lineageSources = this.lineageSources.filter((item) => item !== source)
      } else {
        this.lineageSources = [...this.lineageSources, source]
      }
      if (this.lineageSources.length === 0) {
        this.lineageSources = ['metadata']
      }
    },

    async reloadVisibleLineage() {
      if (this.selectedPublicId) {
        await this.loadDatasetLineage(this.selectedPublicId)
      }
      if (this.selectedWorkTable?.publicId && this.selectedWorkTable.managed) {
        await this.loadSelectedWorkTableLineage()
      }
    },

    async loadLineageChangeSets() {
      try {
        this.lineageChangeSets = await fetchDatasetLineageChangeSets('draft')
      } catch (error) {
        this.lineageChangeSets = []
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async selectLineageChangeSet(publicId: string) {
      if (!publicId) {
        this.selectedLineageChangeSet = null
        await this.reloadVisibleLineage()
        return null
      }
      this.lineageActionLoading = true
      try {
        const graph = await fetchLineageChangeSet(publicId)
        this.selectedLineageChangeSet = graph
        await this.reloadVisibleLineage()
        return graph
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.lineageActionLoading = false
      }
    },

    async saveDatasetLineageDraft(body: DatasetLineageGraphSaveBodyWritable) {
      const root = this.selectedDataset
      if (!root?.publicId) {
        return null
      }
      return this.saveLineageDraft('dataset', root.publicId, root.name, body)
    },

    async saveSelectedWorkTableLineageDraft(body: DatasetLineageGraphSaveBodyWritable) {
      const root = this.selectedWorkTable
      if (!root?.publicId) {
        return null
      }
      return this.saveLineageDraft('dataset_work_table', root.publicId, root.displayName || root.table, body)
    },

    async saveLineageDraft(rootResourceType: string, rootResourcePublicId: string, title: string, body: DatasetLineageGraphSaveBodyWritable) {
      this.lineageActionLoading = true
      this.errorMessage = ''
      this.workTableErrorMessage = ''
      try {
        let changeSet = this.selectedLineageChangeSet?.changeSet
        if (!changeSet || changeSet.status !== 'draft' || changeSet.rootResourceType !== rootResourceType || changeSet.rootResourcePublicId !== rootResourcePublicId) {
          changeSet = await createLineageChangeSet({
            sourceKind: 'manual',
            rootResourceType,
            rootResourcePublicId,
            title: `${title} lineage draft`,
          })
        }
        const graph = await saveLineageChangeSetGraph(changeSet.publicId, body)
        this.selectedLineageChangeSet = graph
        await this.loadLineageChangeSets()
        await this.reloadVisibleLineage()
        return graph
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.lineageActionLoading = false
      }
    },

    async publishSelectedLineageChangeSet() {
      const publicId = this.selectedLineageChangeSet?.changeSet.publicId
      if (!publicId) {
        return null
      }
      this.lineageActionLoading = true
      try {
        const item = await publishLineageChangeSet(publicId)
        this.selectedLineageChangeSet = null
        await this.loadLineageChangeSets()
        await this.reloadVisibleLineage()
        return item
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.lineageActionLoading = false
      }
    },

    async rejectSelectedLineageChangeSet() {
      const publicId = this.selectedLineageChangeSet?.changeSet.publicId
      if (!publicId) {
        return null
      }
      this.lineageActionLoading = true
      try {
        const item = await rejectLineageChangeSet(publicId)
        this.selectedLineageChangeSet = null
        await this.loadLineageChangeSets()
        await this.reloadVisibleLineage()
        return item
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.lineageActionLoading = false
      }
    },

    async parseLatestQueryLineage() {
      const publicId = this.latestQuery?.publicId
      if (!publicId) {
        return null
      }
      this.lineageActionLoading = true
      this.errorMessage = ''
      try {
        const graph = await requestDatasetQueryJobLineageParse(publicId)
        this.selectedLineageChangeSet = graph
        this.lineageParseRuns = await fetchDatasetQueryJobLineageParseRuns(publicId)
        await this.loadLineageChangeSets()
        await this.reloadVisibleLineage()
        return graph
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.lineageActionLoading = false
      }
    },

    async loadLatestQueryLineageParseRuns() {
      const publicId = this.latestQuery?.publicId
      if (!publicId) {
        this.lineageParseRuns = []
        return
      }
      try {
        this.lineageParseRuns = await fetchDatasetQueryJobLineageParseRuns(publicId)
      } catch (error) {
        this.lineageParseRuns = []
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async createSelectedWorkTableExportSchedule(body: DatasetWorkTableExportScheduleCreateBodyWritable) {
      const publicId = this.selectedWorkTable?.publicId
      if (!publicId) {
        return null
      }
      this.workTableActionLoading = true
      this.workTableErrorMessage = ''
      try {
        const item = await createWorkTableExportSchedule(publicId, body)
        this.workTableExportSchedules = [item, ...this.workTableExportSchedules.filter((schedule) => schedule.publicId !== item.publicId)]
        await this.loadSelectedWorkTableLineage()
        return item
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.workTableActionLoading = false
      }
    },

    async updateSelectedWorkTableExportSchedule(schedulePublicId: string, body: DatasetWorkTableExportScheduleUpdateBodyWritable) {
      this.workTableActionLoading = true
      this.workTableErrorMessage = ''
      try {
        const item = await updateWorkTableExportSchedule(schedulePublicId, body)
        this.workTableExportSchedules = [item, ...this.workTableExportSchedules.filter((schedule) => schedule.publicId !== item.publicId)]
        await this.loadSelectedWorkTableLineage()
        return item
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.workTableActionLoading = false
      }
    },

    async disableSelectedWorkTableExportSchedule(schedulePublicId: string) {
      this.workTableActionLoading = true
      this.workTableErrorMessage = ''
      try {
        const item = await disableWorkTableExportSchedule(schedulePublicId)
        this.workTableExportSchedules = [item, ...this.workTableExportSchedules.filter((schedule) => schedule.publicId !== item.publicId)]
        await this.loadSelectedWorkTableLineage()
        return item
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.workTableActionLoading = false
      }
    },

    applyWorkTableExportUpdate(update: Partial<DatasetWorkTableExportBody> & { publicId: string }) {
      const index = this.workTableExports.findIndex((item) => item.publicId === update.publicId)
      if (index < 0) {
        return false
      }
      this.workTableExports[index] = {
        ...this.workTableExports[index],
        ...update,
      }
      return true
    },

    async refreshSelectedGoldPublication() {
      const publicId = this.selectedGoldPublication?.publicId
      if (!publicId) {
        return null
      }
      this.goldActionLoading = true
      this.goldErrorMessage = ''
      try {
        const run = await refreshGoldPublication(publicId)
        this.goldPublishRuns = upsertGoldPublishRun(this.goldPublishRuns, run)
        if (this.selectedGoldPublication) {
          this.selectedGoldPublication = {
            ...this.selectedGoldPublication,
            latestPublishRun: run,
          }
          this.goldPublications = upsertGoldPublication(this.goldPublications, this.selectedGoldPublication)
        }
        return run
      } catch (error) {
        this.goldErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.goldActionLoading = false
      }
    },

    async unpublishSelectedGoldPublication() {
      const publicId = this.selectedGoldPublication?.publicId
      if (!publicId) {
        return null
      }
      this.goldActionLoading = true
      this.goldErrorMessage = ''
      try {
        const item = await unpublishGoldPublication(publicId)
        this.selectedGoldPublication = item
        this.goldPublicationPreview = null
        this.goldPublications = upsertGoldPublication(this.goldPublications, item)
        await this.loadGoldMedallion(publicId).catch(() => undefined)
        return item
      } catch (error) {
        this.goldErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.goldActionLoading = false
      }
    },

    async archiveSelectedGoldPublication() {
      const publicId = this.selectedGoldPublication?.publicId
      if (!publicId) {
        return null
      }
      this.goldActionLoading = true
      this.goldErrorMessage = ''
      try {
        const item = await archiveGoldPublication(publicId)
        this.selectedGoldPublication = item
        this.goldPublicationPreview = null
        this.goldPublications = this.goldPublications.filter((existing) => existing.publicId !== item.publicId)
        await this.loadGoldMedallion(publicId).catch(() => undefined)
        return item
      } catch (error) {
        this.goldErrorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.goldActionLoading = false
      }
    },

    applyGoldPublishRunUpdate(update: Partial<DatasetGoldPublishRunBody> & { publicId: string }) {
      const index = this.goldPublishRuns.findIndex((item) => item.publicId === update.publicId)
      if (index >= 0) {
        this.goldPublishRuns[index] = {
          ...this.goldPublishRuns[index],
          ...update,
        }
      }
      if (this.selectedGoldPublication?.latestPublishRun?.publicId === update.publicId) {
        this.selectedGoldPublication = {
          ...this.selectedGoldPublication,
          latestPublishRun: {
            ...this.selectedGoldPublication.latestPublishRun,
            ...update,
          },
        }
        this.goldPublications = upsertGoldPublication(this.goldPublications, this.selectedGoldPublication)
      }
      return index >= 0 || this.selectedGoldPublication?.latestPublishRun?.publicId === update.publicId
    },

    applyGoldPublicationUpdate(update: Partial<DatasetGoldPublicationBody> & { publicId: string }) {
      const index = this.goldPublications.findIndex((item) => item.publicId === update.publicId)
      if (index >= 0) {
        this.goldPublications[index] = {
          ...this.goldPublications[index],
          ...update,
        }
      }
      if (this.selectedGoldPublication?.publicId === update.publicId) {
        this.selectedGoldPublication = {
          ...this.selectedGoldPublication,
          ...update,
        }
      }
      return index >= 0 || this.selectedGoldPublication?.publicId === update.publicId
    },

    async importFromDriveFile(driveFilePublicId: string, name: string) {
      this.importing = true
      this.errorMessage = ''
      try {
        const created = await createDatasetFromDriveFile({
          driveFilePublicId,
          ...(name.trim() ? { name: name.trim() } : {}),
        })
        this.items = [created, ...this.items.filter((item) => item.publicId !== created.publicId)]
        this.selectedPublicId = created.publicId
        this.status = 'ready'
        return created
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.importing = false
      }
    },

    async remove(datasetPublicId: string) {
      this.deletingPublicId = datasetPublicId
      this.errorMessage = ''
      try {
        await deleteDatasetItem(datasetPublicId)
        this.items = this.items.filter((item) => item.publicId !== datasetPublicId)
        if (this.selectedPublicId === datasetPublicId) {
          this.selectedPublicId = this.items[0]?.publicId ?? ''
        }
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.deletingPublicId = ''
      }
    },

    async loadQueryJobs(datasetPublicId: string) {
      this.errorMessage = ''
      try {
        this.queryJobs = await fetchDatasetScopedQueryJobs(datasetPublicId)
        this.latestQuery = null
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      }
    },

    async run(statement: string) {
      this.executing = true
      this.errorMessage = ''
      try {
        const job = await createDatasetQuery({ statement })
        this.latestQuery = job
        this.queryJobs = [job, ...this.queryJobs.filter((item) => item.publicId !== job.publicId)].slice(0, 25)
        return job
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.executing = false
      }
    },

    async runForDataset(datasetPublicId: string, statement: string) {
      this.executing = true
      this.errorMessage = ''
      try {
        const job = await createDatasetScopedQuery(datasetPublicId, { statement })
        this.latestQuery = job
        this.queryJobs = [job, ...this.queryJobs.filter((item) => item.publicId !== job.publicId)].slice(0, 25)
        return job
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.executing = false
      }
    },

    reset() {
      this.status = 'idle'
      this.items = []
      this.selectedPublicId = ''
      this.sourceFiles = []
      this.selectedSourceFilePublicId = ''
      this.workTables = []
      this.linkedWorkTables = []
      this.selectedWorkTable = null
      this.workTablePreview = null
      this.goldPublications = []
      this.selectedGoldPublication = null
      this.goldPublicationPreview = null
      this.goldPublishRuns = []
      this.workTableExports = []
      this.workTableExportSchedules = []
      this.datasetMedallionCatalog = null
      this.workTableMedallionCatalog = null
      this.goldMedallionCatalog = null
      this.datasetLineage = null
      this.workTableLineage = null
      this.lineageLevel = 'table'
      this.lineageSources = ['metadata', 'parser', 'manual']
      this.lineageChangeSets = []
      this.selectedLineageChangeSet = null
      this.lineageParseRuns = []
      this.lineageActionLoading = false
      this.datasetLineageLoading = false
      this.workTableLineageLoading = false
      this.workTablesLoading = false
      this.workTablePreviewLoading = false
      this.goldPublicationsLoading = false
      this.goldPublicationLoading = false
      this.goldPreviewLoading = false
      this.goldRunsLoading = false
      this.datasetMedallionLoading = false
      this.workTableMedallionLoading = false
      this.goldMedallionLoading = false
      this.workTableActionLoading = false
      this.goldActionLoading = false
      this.workTableErrorMessage = ''
      this.goldErrorMessage = ''
      this.goldPreviewErrorMessage = ''
      this.queryJobs = []
      this.syncJobs = []
      this.latestQuery = null
      this.errorMessage = ''
      this.importing = false
      this.syncing = false
      this.executing = false
      this.deletingPublicId = ''
    },
  },
})

function sameWorkTable(a: DatasetWorkTableBody, b: DatasetWorkTableBody) {
  return a.database === b.database && a.table === b.table
}

function upsertGoldPublication(items: DatasetGoldPublicationBody[], item: DatasetGoldPublicationBody) {
  return [item, ...items.filter((existing) => existing.publicId !== item.publicId)]
}

function upsertGoldPublishRun(items: DatasetGoldPublishRunBody[], item: DatasetGoldPublishRunBody) {
  return [item, ...items.filter((existing) => existing.publicId !== item.publicId)].slice(0, 25)
}
