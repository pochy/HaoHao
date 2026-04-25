import { defineStore } from 'pinia'

import { isApiForbidden, toApiErrorMessage } from '../api/client'
import {
  createMachineClientFromForm,
  disableMachineClient,
  fetchMachineClient,
  fetchMachineClients,
  updateMachineClientFromForm,
} from '../api/machine-clients'
import type {
  MachineClientBody,
  MachineClientRequestBody,
} from '../api/generated/types.gen'

type AdminStatus = 'idle' | 'loading' | 'ready' | 'forbidden' | 'error'

export const useMachineClientStore = defineStore('machineClients', {
  state: () => ({
    status: 'idle' as AdminStatus,
    items: [] as MachineClientBody[],
    current: null as MachineClientBody | null,
    errorMessage: '',
    saving: false,
  }),

  actions: {
    async loadList() {
      this.status = 'loading'
      this.errorMessage = ''

      try {
        this.items = await fetchMachineClients()
        this.status = 'ready'
      } catch (error) {
        this.items = []
        this.status = isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async loadOne(id: number) {
      this.status = 'loading'
      this.errorMessage = ''

      try {
        this.current = await fetchMachineClient(id)
        this.status = 'ready'
      } catch (error) {
        this.current = null
        this.status = isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async create(body: MachineClientRequestBody) {
      this.saving = true
      this.errorMessage = ''
      try {
        return await createMachineClientFromForm(body)
      } catch (error) {
        if (isApiForbidden(error)) {
          this.status = 'forbidden'
        }
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async update(id: number, body: MachineClientRequestBody) {
      this.saving = true
      this.errorMessage = ''
      try {
        this.current = await updateMachineClientFromForm(id, body)
        return this.current
      } catch (error) {
        if (isApiForbidden(error)) {
          this.status = 'forbidden'
        }
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async disable(id: number) {
      this.saving = true
      this.errorMessage = ''
      try {
        await disableMachineClient(id)
        if (this.current?.id === id) {
          this.current = { ...this.current, active: false }
        }
        this.items = this.items.map((item) => (
          item.id === id ? { ...item, active: false } : item
        ))
      } catch (error) {
        if (isApiForbidden(error)) {
          this.status = 'forbidden'
        }
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },
  },
})
