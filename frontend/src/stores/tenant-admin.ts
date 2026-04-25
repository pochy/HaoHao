import { defineStore } from 'pinia'

import { isApiForbidden, toApiErrorMessage } from '../api/client'
import type {
  TenantAdminMembershipRequestBody,
  TenantAdminTenantBody,
  TenantAdminTenantDetailBody,
  TenantAdminTenantRequestBody,
} from '../api/generated/types.gen'
import {
  createTenantFromForm,
  deactivateTenant,
  fetchTenantAdminTenant,
  fetchTenantAdminTenants,
  grantTenantRole,
  revokeTenantRole,
  updateTenantFromForm,
} from '../api/tenant-admin'

type TenantAdminStatus = 'idle' | 'loading' | 'ready' | 'forbidden' | 'error'

export const useTenantAdminStore = defineStore('tenantAdmin', {
  state: () => ({
    status: 'idle' as TenantAdminStatus,
    items: [] as TenantAdminTenantBody[],
    current: null as TenantAdminTenantDetailBody | null,
    errorMessage: '',
    saving: false,
  }),

  actions: {
    async loadList() {
      this.status = 'loading'
      this.errorMessage = ''

      try {
        this.items = await fetchTenantAdminTenants()
        this.status = 'ready'
      } catch (error) {
        this.items = []
        this.status = isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async loadOne(tenantSlug: string) {
      this.status = 'loading'
      this.errorMessage = ''

      try {
        this.current = await fetchTenantAdminTenant(tenantSlug)
        this.status = 'ready'
      } catch (error) {
        this.current = null
        this.status = isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async create(body: TenantAdminTenantRequestBody) {
      this.saving = true
      this.errorMessage = ''
      try {
        return await createTenantFromForm(body)
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

    async update(tenantSlug: string, body: TenantAdminTenantRequestBody) {
      this.saving = true
      this.errorMessage = ''
      try {
        const updated = await updateTenantFromForm(tenantSlug, body)
        if (this.current?.tenant.slug === tenantSlug) {
          this.current = {
            ...this.current,
            tenant: updated,
          }
        }
        this.items = this.items.map((item) => (
          item.slug === tenantSlug ? updated : item
        ))
        return updated
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

    async deactivate(tenantSlug: string) {
      this.saving = true
      this.errorMessage = ''
      try {
        await deactivateTenant(tenantSlug)
        this.items = this.items.map((item) => (
          item.slug === tenantSlug ? { ...item, active: false } : item
        ))
        if (this.current?.tenant.slug === tenantSlug) {
          this.current = {
            ...this.current,
            tenant: {
              ...this.current.tenant,
              active: false,
            },
          }
        }
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

    async grantRole(tenantSlug: string, body: TenantAdminMembershipRequestBody) {
      this.saving = true
      this.errorMessage = ''
      try {
        await grantTenantRole(tenantSlug, body)
        await this.loadOne(tenantSlug)
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

    async revokeRole(tenantSlug: string, userPublicId: string, roleCode: string) {
      this.saving = true
      this.errorMessage = ''
      try {
        await revokeTenantRole(tenantSlug, userPublicId, roleCode)
        await this.loadOne(tenantSlug)
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
