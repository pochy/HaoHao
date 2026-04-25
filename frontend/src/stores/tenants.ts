import { defineStore } from 'pinia'

import { toApiErrorMessage } from '../api/client'
import { fetchTenants, switchActiveTenant } from '../api/tenants'
import type { TenantBody } from '../api/generated/types.gen'

type TenantStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'error'

export const useTenantStore = defineStore('tenants', {
  state: () => ({
    status: 'idle' as TenantStatus,
    items: [] as TenantBody[],
    activeTenant: null as TenantBody | null,
    defaultTenant: null as TenantBody | null,
    errorMessage: '',
    switchingSlug: '',
  }),

  getters: {
    hasMultipleTenants: (state) => state.items.length > 1,
  },

  actions: {
    async load() {
      this.status = 'loading'
      this.errorMessage = ''

      try {
        const data = await fetchTenants()
        this.items = data.items ?? []
        this.activeTenant = data.activeTenant ?? this.items.find((item) => item.selected) ?? null
        this.defaultTenant = data.defaultTenant ?? this.items.find((item) => item.default) ?? null
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.items = []
        this.activeTenant = null
        this.defaultTenant = null
        this.status = 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async select(tenantSlug: string) {
      if (!tenantSlug || tenantSlug === this.activeTenant?.slug) {
        return
      }

      this.switchingSlug = tenantSlug
      this.errorMessage = ''

      try {
        const activeTenant = await switchActiveTenant(tenantSlug)
        this.activeTenant = activeTenant
        this.items = this.items.map((item) => ({
          ...item,
          selected: item.slug === activeTenant.slug,
        }))
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.switchingSlug = ''
      }
    },

    reset() {
      this.status = 'idle'
      this.items = []
      this.activeTenant = null
      this.defaultTenant = null
      this.errorMessage = ''
      this.switchingSlug = ''
    },
  },
})
