import { defineStore } from 'pinia'

import { isApiForbidden, toApiErrorMessage, toApiErrorStatus } from '../api/client'
import type { DatasetBody, DatasetQueryJobBody } from '../api/generated/types.gen'
import {
  createDatasetQuery,
  deleteDatasetItem,
  fetchDataset,
  fetchDatasetQueryJobs,
  fetchDatasets,
  uploadDatasetFile,
} from '../api/datasets'

type DatasetStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'forbidden' | 'error'

export const useDatasetStore = defineStore('datasets', {
  state: () => ({
    status: 'idle' as DatasetStatus,
    items: [] as DatasetBody[],
    selectedPublicId: '',
    queryJobs: [] as DatasetQueryJobBody[],
    latestQuery: null as DatasetQueryJobBody | null,
    errorMessage: '',
    uploading: false,
    executing: false,
    deletingPublicId: '',
  }),

  getters: {
    selectedDataset: (state) => (
      state.items.find((item) => item.publicId === state.selectedPublicId) ?? state.items[0] ?? null
    ),
    hasActiveImports: (state) => state.items.some((item) => ['pending', 'importing'].includes(item.status)),
  },

  actions: {
    async load() {
      this.status = 'loading'
      this.errorMessage = ''
      try {
        const [datasets, queryJobs] = await Promise.all([
          fetchDatasets(),
          fetchDatasetQueryJobs(),
        ])
        this.items = datasets
        this.queryJobs = queryJobs
        if (!this.selectedPublicId || !this.items.some((item) => item.publicId === this.selectedPublicId)) {
          this.selectedPublicId = this.items[0]?.publicId ?? ''
        }
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.items = []
        this.queryJobs = []
        this.status = toApiErrorStatus(error) === 403 || isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
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

    async upload(file: File, name: string) {
      this.uploading = true
      this.errorMessage = ''
      try {
        const created = await uploadDatasetFile(file, name)
        this.items = [created, ...this.items.filter((item) => item.publicId !== created.publicId)]
        this.selectedPublicId = created.publicId
        this.status = 'ready'
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.uploading = false
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

    reset() {
      this.status = 'idle'
      this.items = []
      this.selectedPublicId = ''
      this.queryJobs = []
      this.latestQuery = null
      this.errorMessage = ''
      this.uploading = false
      this.executing = false
      this.deletingPublicId = ''
    },
  },
})
