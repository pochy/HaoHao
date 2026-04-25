import { defineStore } from 'pinia'

import { isApiForbidden, toApiErrorMessage, toApiErrorStatus } from '../api/client'
import {
  createCustomerSignalItem,
  deleteCustomerSignalItem,
  fetchCustomerSignal,
  fetchCustomerSignals,
  updateCustomerSignalItem,
} from '../api/customer-signals'
import type {
  CreateCustomerSignalBodyWritable,
  CustomerSignalBody,
  UpdateCustomerSignalBodyWritable,
} from '../api/generated/types.gen'

type CustomerSignalStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'forbidden' | 'error'

export const useCustomerSignalStore = defineStore('customerSignals', {
  state: () => ({
    status: 'idle' as CustomerSignalStatus,
    items: [] as CustomerSignalBody[],
    current: null as CustomerSignalBody | null,
    errorMessage: '',
    creating: false,
    updating: false,
    deletingPublicId: '',
  }),

  actions: {
    async load() {
      this.status = 'loading'
      this.errorMessage = ''

      try {
        this.items = await fetchCustomerSignals()
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.items = []
        this.status = toApiErrorStatus(error) === 403 || isApiForbidden(error)
          ? 'forbidden'
          : 'error'
        this.errorMessage = toApiErrorMessage(error)
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
      this.current = null
      this.errorMessage = ''
      this.creating = false
      this.updating = false
      this.deletingPublicId = ''
    },
  },
})
