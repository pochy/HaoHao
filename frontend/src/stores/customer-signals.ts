import { defineStore } from 'pinia'

import { isApiForbidden, toApiErrorMessage, toApiErrorStatus } from '../api/client'
import {
  createCustomerSignalItem,
  deleteCustomerSignalItem,
  type CustomerSignalListParams,
  fetchCustomerSignal,
  fetchCustomerSignals,
  updateCustomerSignalItem,
} from '../api/customer-signals'
import {
  createCustomerSignalSavedFilterItem,
  deleteCustomerSignalSavedFilterItem,
  type CustomerSignalSavedFilterListParams,
  fetchCustomerSignalSavedFilters,
} from '../api/customer-signal-saved-filters'
import type {
  CreateCustomerSignalBodyWritable,
  CustomerSignalBody,
  CustomerSignalSavedFilterBody,
  UpdateCustomerSignalBodyWritable,
} from '../api/generated/types.gen'

type CustomerSignalStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'forbidden' | 'error'

export const useCustomerSignalStore = defineStore('customerSignals', {
  state: () => ({
    status: 'idle' as CustomerSignalStatus,
    items: [] as CustomerSignalBody[],
    savedFilters: [] as CustomerSignalSavedFilterBody[],
    query: '',
    filters: {
      status: '',
      priority: '',
      source: '',
    },
    signalLimit: 25,
    nextCursor: '',
    savedFilterStatus: 'idle' as CustomerSignalStatus,
    savedFiltersQuery: '',
    savedFiltersFilters: {
      status: '',
      priority: '',
      source: '',
    },
    savedFilterLimit: 25,
    savedFiltersNextCursor: '',
    current: null as CustomerSignalBody | null,
    errorMessage: '',
    savedFilterErrorMessage: '',
    creating: false,
    updating: false,
    loadingMore: false,
    loadingMoreSavedFilters: false,
    deletingPublicId: '',
  }),

  actions: {
    async load(params: CustomerSignalListParams = {}) {
      const loadingMore = Boolean(params.cursor)
      if (loadingMore) {
        this.loadingMore = true
      } else {
        this.status = 'loading'
      }
      this.errorMessage = ''

      try {
        const data = await fetchCustomerSignals({
          q: (params.q ?? this.query) || undefined,
          status: (params.status ?? this.filters.status) || undefined,
          priority: (params.priority ?? this.filters.priority) || undefined,
          source: (params.source ?? this.filters.source) || undefined,
          cursor: params.cursor,
          limit: params.limit ?? this.signalLimit,
        })
        this.items = params.cursor ? [...this.items, ...(data.items ?? [])] : data.items ?? []
        this.nextCursor = data.nextCursor ?? ''
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        if (!loadingMore) {
          this.items = []
          this.nextCursor = ''
        }
        this.status = toApiErrorStatus(error) === 403 || isApiForbidden(error)
          ? 'forbidden'
          : 'error'
        this.errorMessage = toApiErrorMessage(error)
      } finally {
        this.loadingMore = false
      }
    },

    async loadSavedFilters(params: CustomerSignalSavedFilterListParams = {}) {
      const loadingMore = Boolean(params.cursor)
      if (loadingMore) {
        this.loadingMoreSavedFilters = true
      } else {
        this.savedFilterStatus = 'loading'
      }
      this.savedFilterErrorMessage = ''

      try {
        const data = await fetchCustomerSignalSavedFilters({
          q: (params.q ?? this.savedFiltersQuery) || undefined,
          status: (params.status ?? this.savedFiltersFilters.status) || undefined,
          priority: (params.priority ?? this.savedFiltersFilters.priority) || undefined,
          source: (params.source ?? this.savedFiltersFilters.source) || undefined,
          cursor: params.cursor,
          limit: params.limit ?? this.savedFilterLimit,
        })
        this.savedFilters = loadingMore ? [...this.savedFilters, ...data.items] : data.items
        this.savedFiltersNextCursor = data.nextCursor
        this.savedFilterStatus = this.savedFilters.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        if (!loadingMore) {
          this.savedFilters = []
          this.savedFiltersNextCursor = ''
        }
        this.savedFilterStatus = toApiErrorStatus(error) === 403 || isApiForbidden(error)
          ? 'forbidden'
          : 'error'
        this.savedFilterErrorMessage = toApiErrorMessage(error)
      } finally {
        this.loadingMoreSavedFilters = false
      }
    },

    async saveCurrentFilter(name: string) {
      const created = await createCustomerSignalSavedFilterItem({
        name,
        query: this.query,
        filters: {
          status: this.filters.status,
          priority: this.filters.priority,
          source: this.filters.source,
        },
      })
      await this.loadSavedFilters()
      return created
    },

    async deleteSavedFilter(filterPublicId: string) {
      this.savedFilterErrorMessage = ''
      try {
        await deleteCustomerSignalSavedFilterItem(filterPublicId)
        this.savedFilters = this.savedFilters.filter((item) => item.publicId !== filterPublicId)
        this.savedFilterStatus = this.savedFilters.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.savedFilterErrorMessage = toApiErrorMessage(error)
        throw error
      }
    },

    async loadOne(signalPublicId: string) {
      this.status = 'loading'
      this.errorMessage = ''

      try {
        this.current = await fetchCustomerSignal(signalPublicId)
        this.status = 'ready'
      } catch (error) {
        this.current = null
        this.status = toApiErrorStatus(error) === 403 || isApiForbidden(error)
          ? 'forbidden'
          : 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async create(body: CreateCustomerSignalBodyWritable) {
      this.creating = true
      this.errorMessage = ''

      try {
        const created = await createCustomerSignalItem(body)
        this.items = [created, ...this.items]
        this.status = 'ready'
        return created
      } catch (error) {
        if (toApiErrorStatus(error) === 403 || isApiForbidden(error)) {
          this.status = 'forbidden'
        }
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.creating = false
      }
    },

    async update(signalPublicId: string, body: UpdateCustomerSignalBodyWritable) {
      this.updating = true
      this.errorMessage = ''

      try {
        const updated = await updateCustomerSignalItem(signalPublicId, body)
        this.items = this.items.map((item) => (
          item.publicId === signalPublicId ? updated : item
        ))
        if (this.current?.publicId === signalPublicId) {
          this.current = updated
        }
        this.status = this.items.length > 0 || this.current ? 'ready' : 'empty'
        return updated
      } catch (error) {
        if (toApiErrorStatus(error) === 403 || isApiForbidden(error)) {
          this.status = 'forbidden'
        }
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.updating = false
      }
    },

    async remove(signalPublicId: string) {
      this.deletingPublicId = signalPublicId
      this.errorMessage = ''

      try {
        await deleteCustomerSignalItem(signalPublicId)
        this.items = this.items.filter((item) => item.publicId !== signalPublicId)
        if (this.current?.publicId === signalPublicId) {
          this.current = null
        }
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        if (toApiErrorStatus(error) === 403 || isApiForbidden(error)) {
          this.status = 'forbidden'
        }
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.deletingPublicId = ''
      }
    },

    reset() {
      this.status = 'idle'
      this.items = []
      this.savedFilters = []
      this.query = ''
      this.filters = { status: '', priority: '', source: '' }
      this.signalLimit = 25
      this.nextCursor = ''
      this.savedFilterStatus = 'idle'
      this.savedFiltersQuery = ''
      this.savedFiltersFilters = { status: '', priority: '', source: '' }
      this.savedFilterLimit = 25
      this.savedFiltersNextCursor = ''
      this.current = null
      this.errorMessage = ''
      this.savedFilterErrorMessage = ''
      this.creating = false
      this.updating = false
      this.loadingMore = false
      this.loadingMoreSavedFilters = false
      this.deletingPublicId = ''
    },
  },
})
