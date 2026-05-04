import { defineStore } from 'pinia'

import { toApiErrorMessage } from '../api/client'
import {
  fetchTenantEntitlements,
  updateTenantEntitlementItems,
} from '../api/entitlements'
import {
  createCustomerSignalImportItem,
  fetchCustomerSignalImports,
} from '../api/customer-signal-imports'
import {
  createTenantDataExportItem,
  fetchTenantDataExports,
} from '../api/tenant-data-exports'
import {
  createWebhookItem,
  fetchWebhooks,
} from '../api/webhooks'
import {
  createTenantInvitationItem,
  fetchTenantInvitations,
  provisionTenantInvitationIdentityItem,
  revokeTenantInvitationItem,
} from '../api/tenant-invitations'
import {
  fetchTenantSettings,
  updateTenantSettingsItem,
} from '../api/tenant-settings'
import type {
  CreateTenantInvitationRequestBodyWritable,
  CustomerSignalImportJobBody,
  EntitlementBody,
  EntitlementUpdateBody,
  TenantDataExportBody,
  TenantInvitationBody,
  TenantSettingsBody,
  TenantSettingsRequestBodyWritable,
  WebhookEndpointBody,
  WebhookEndpointRequestBodyWritable,
} from '../api/generated/types.gen'

const activeDataJobStatuses = new Set(['pending', 'processing'])

export const useTenantCommonStore = defineStore('tenantCommon', {
  state: () => ({
    invitations: [] as TenantInvitationBody[],
    exports: [] as TenantDataExportBody[],
    imports: [] as CustomerSignalImportJobBody[],
    entitlements: [] as EntitlementBody[],
    webhooks: [] as WebhookEndpointBody[],
    settings: null as TenantSettingsBody | null,
    loading: false,
    saving: false,
    errorMessage: '',
  }),

  getters: {
    hasActiveDataJobs: (state) => (
      state.exports.some((item) => activeDataJobStatuses.has(item.status)) ||
      state.imports.some((item) => activeDataJobStatuses.has(item.status))
    ),
  },

  actions: {
    reset() {
      this.invitations = []
      this.exports = []
      this.imports = []
      this.entitlements = []
      this.webhooks = []
      this.settings = null
      this.loading = false
      this.saving = false
      this.errorMessage = ''
    },

    async load(tenantSlug: string) {
      this.loading = true
      this.errorMessage = ''
      try {
        const [settings, invitations, exports, imports, entitlements, webhooks] = await Promise.all([
          fetchTenantSettings(tenantSlug),
          fetchTenantInvitations(tenantSlug),
          fetchTenantDataExports(tenantSlug),
          fetchCustomerSignalImports(tenantSlug).catch(() => []),
          fetchTenantEntitlements(tenantSlug).catch(() => []),
          fetchWebhooks(tenantSlug).catch(() => []),
        ])
        this.settings = settings
        this.invitations = invitations
        this.exports = exports
        this.imports = imports
        this.entitlements = entitlements
        this.webhooks = webhooks
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
      } finally {
        this.loading = false
      }
    },

    async refreshDataJobs(tenantSlug: string) {
      this.errorMessage = ''
      try {
        const [exports, imports] = await Promise.all([
          fetchTenantDataExports(tenantSlug),
          fetchCustomerSignalImports(tenantSlug).catch(() => []),
        ])
        this.exports = exports
        this.imports = imports
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async refreshTenantDataExports(tenantSlug: string) {
      this.errorMessage = ''
      try {
        this.exports = await fetchTenantDataExports(tenantSlug)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async refreshCustomerSignalImports(tenantSlug: string) {
      this.errorMessage = ''
      try {
        this.imports = await fetchCustomerSignalImports(tenantSlug)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    applyTenantDataExportUpdate(update: Partial<TenantDataExportBody> & { publicId: string }) {
      const index = this.exports.findIndex((item) => item.publicId === update.publicId)
      if (index < 0) {
        return false
      }
      this.exports[index] = {
        ...this.exports[index],
        ...update,
      }
      return true
    },

    applyCustomerSignalImportUpdate(update: Partial<CustomerSignalImportJobBody> & { publicId: string }) {
      const index = this.imports.findIndex((item) => item.publicId === update.publicId)
      if (index < 0) {
        return false
      }
      this.imports[index] = {
        ...this.imports[index],
        ...update,
      }
      return true
    },

    async updateSettings(tenantSlug: string, body: TenantSettingsRequestBodyWritable) {
      this.saving = true
      this.errorMessage = ''
      try {
        this.settings = await updateTenantSettingsItem(tenantSlug, body)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async createInvitation(tenantSlug: string, body: CreateTenantInvitationRequestBodyWritable) {
      this.saving = true
      this.errorMessage = ''
      try {
        const created = await createTenantInvitationItem(tenantSlug, body)
        this.invitations = [created, ...this.invitations]
        return created
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async revokeInvitation(tenantSlug: string, invitationPublicId: string) {
      this.saving = true
      this.errorMessage = ''
      try {
        await revokeTenantInvitationItem(tenantSlug, invitationPublicId)
        this.invitations = this.invitations.map((item) => (
          item.publicId === invitationPublicId ? { ...item, status: 'revoked' } : item
        ))
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async provisionInvitationIdentity(tenantSlug: string, invitationPublicId: string) {
      this.saving = true
      this.errorMessage = ''
      try {
        const updated = await provisionTenantInvitationIdentityItem(tenantSlug, invitationPublicId)
        this.invitations = this.invitations.map((item) => (
          item.publicId === invitationPublicId ? { ...item, ...updated } : item
        ))
        return updated
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async requestExport(tenantSlug: string) {
      this.saving = true
      this.errorMessage = ''
      try {
        const created = await createTenantDataExportItem(tenantSlug)
        this.exports = [created, ...this.exports]
        return created
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async requestCSVExport(tenantSlug: string) {
      this.saving = true
      this.errorMessage = ''
      try {
        const created = await createTenantDataExportItem(tenantSlug, 'csv')
        this.exports = [created, ...this.exports]
        return created
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async updateEntitlements(tenantSlug: string, items: EntitlementUpdateBody[]) {
      this.saving = true
      this.errorMessage = ''
      try {
        this.entitlements = await updateTenantEntitlementItems(tenantSlug, items)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async createWebhook(tenantSlug: string, body: WebhookEndpointRequestBodyWritable) {
      this.saving = true
      this.errorMessage = ''
      try {
        const created = await createWebhookItem(tenantSlug, body)
        this.webhooks = [created, ...this.webhooks]
        return created
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async createImport(tenantSlug: string, inputFilePublicId: string) {
      this.saving = true
      this.errorMessage = ''
      try {
        const created = await createCustomerSignalImportItem(tenantSlug, { inputFilePublicId })
        this.imports = [created, ...this.imports]
        return created
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },
  },
})
