import { defineStore } from 'pinia'

import { isApiForbidden, toApiErrorMessage, toApiErrorStatus } from '../api/client'
import type { DatasetBody, DatasetQueryJobBody, DatasetSourceFileBody, DatasetWorkTableBody, DatasetWorkTableExportBody, DatasetWorkTablePreviewBody } from '../api/generated/types.gen'
import {
  createDatasetFromDriveFile,
  createDatasetQuery,
  createDatasetScopedQuery,
  deleteDatasetItem,
  dropWorkTable,
  fetchDataset,
  fetchDatasetLinkedWorkTables,
  fetchDatasetScopedQueryJobs,
  fetchDatasets,
  fetchDatasetSourceFiles,
  fetchDatasetWorkTable,
  fetchDatasetWorkTablePreview,
  fetchDatasetWorkTables,
  fetchManagedDatasetWorkTable,
  fetchManagedDatasetWorkTablePreview,
  fetchWorkTableExports,
  linkWorkTable,
  promoteWorkTable,
  registerWorkTable,
  renameWorkTable,
  requestWorkTableExport,
  truncateWorkTable,
} from '../api/datasets'
import type { DatasetWorkTableExportFormat } from '../api/datasets'

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
    workTableExports: [] as DatasetWorkTableExportBody[],
    workTablesLoading: false,
    workTablePreviewLoading: false,
    workTableActionLoading: false,
    workTableErrorMessage: '',
    queryJobs: [] as DatasetQueryJobBody[],
    latestQuery: null as DatasetQueryJobBody | null,
    errorMessage: '',
    importing: false,
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
        this.status = 'ready'
      } catch (error) {
        this.status = toApiErrorStatus(error) === 403 || isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
        throw error
      }
    },

    async refreshSelected() {
      if (!this.selectedPublicId) {
        return
      }
      try {
        const updated = await fetchDataset(this.selectedPublicId)
        this.items = this.items.map((item) => item.publicId === updated.publicId ? updated : item)
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
          return
        }
        await this.selectWorkTable(next)
      } catch (error) {
        this.workTables = []
        this.selectedWorkTable = null
        this.workTablePreview = null
        this.workTableErrorMessage = toApiErrorMessage(error)
      } finally {
        this.workTablesLoading = false
      }
    },

    async selectWorkTable(table: DatasetWorkTableBody) {
      const existing = this.workTables.find((item) => sameWorkTable(item, table)) ?? table
      this.selectedWorkTable = existing
      this.workTablePreview = null
      this.workTableExports = []
      this.workTablePreviewLoading = true
      this.workTableErrorMessage = ''
      try {
        if (existing.publicId && existing.managed && existing.status !== 'active') {
          const [detail, exports] = await Promise.all([
            fetchManagedDatasetWorkTable(existing.publicId),
            fetchWorkTableExports(existing.publicId),
          ])
          this.selectedWorkTable = detail
          this.workTableExports = exports
          return
        }
        const [detail, preview, exports] = existing.publicId && existing.managed
          ? await Promise.all([
              fetchManagedDatasetWorkTable(existing.publicId),
              fetchManagedDatasetWorkTablePreview(existing.publicId),
              fetchWorkTableExports(existing.publicId),
            ])
          : await Promise.all([
              fetchDatasetWorkTable(existing.database, existing.table),
              fetchDatasetWorkTablePreview(existing.database, existing.table),
              Promise.resolve([] as DatasetWorkTableExportBody[]),
            ])
        this.selectedWorkTable = detail
        this.workTablePreview = preview
        this.workTableExports = exports
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
      } finally {
        this.workTablePreviewLoading = false
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
      } catch (error) {
        this.workTableErrorMessage = toApiErrorMessage(error)
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
      this.workTableExports = []
      this.workTablesLoading = false
      this.workTablePreviewLoading = false
      this.workTableActionLoading = false
      this.workTableErrorMessage = ''
      this.queryJobs = []
      this.latestQuery = null
      this.errorMessage = ''
      this.importing = false
      this.executing = false
      this.deletingPublicId = ''
    },
  },
})

function sameWorkTable(a: DatasetWorkTableBody, b: DatasetWorkTableBody) {
  return a.database === b.database && a.table === b.table
}
