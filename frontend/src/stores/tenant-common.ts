import { defineStore } from 'pinia'

import { toApiErrorMessage } from '../api/client'
import {
  createTenantDataExportItem,
  fetchTenantDataExports,
} from '../api/tenant-data-exports'
import {
  createTenantInvitationItem,
  fetchTenantInvitations,
  revokeTenantInvitationItem,
} from '../api/tenant-invitations'
import {
  fetchTenantSettings,
  updateTenantSettingsItem,
} from '../api/tenant-settings'
import type {
  CreateTenantInvitationRequestBodyWritable,
  TenantDataExportBody,
  TenantInvitationBody,
  TenantSettingsBody,
  TenantSettingsRequestBodyWritable,
} from '../api/generated/types.gen'

export const useTenantCommonStore = defineStore('tenantCommon', {
  state: () => ({
    invitations: [] as TenantInvitationBody[],
    exports: [] as TenantDataExportBody[],
    settings: null as TenantSettingsBody | null,
    loading: false,
    saving: false,
    errorMessage: '',
  }),

  actions: {
    async load(tenantSlug: string) {
      this.loading = true
      this.errorMessage = ''
      try {
        const [settings, invitations, exports] = await Promise.all([
          fetchTenantSettings(tenantSlug),
          fetchTenantInvitations(tenantSlug),
          fetchTenantDataExports(tenantSlug),
        ])
        this.settings = settings
        this.invitations = invitations
        this.exports = exports
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
      } finally {
        this.loading = false
      }
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
  },
})
