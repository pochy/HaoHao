<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { useTenantAdminDetailContext } from '../../tenant-admin/detail-context'

const { t } = useI18n()
const {
  canInvite,
  commonStore,
  createInvitation,
  formatDate,
  invitationEmail,
  invitationRoleCode,
  revokeInvitation,
  tenantRoleOptions,
} = useTenantAdminDetailContext()
</script>

<template>
  <section class="panel stack">
    <form class="admin-form" @submit.prevent="createInvitation">
      <div class="form-span">
        <span class="status-pill">{{ t('tenantAdmin.labels.invitation') }}</span>
        <h2>{{ t('tenantAdmin.headings.inviteUser') }}</h2>
      </div>

      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.email') }}</span>
        <input
          v-model="invitationEmail"
          data-testid="tenant-invitation-email"
          class="field-input"
          autocomplete="email"
          type="email"
          required
        >
      </label>

      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.role') }}</span>
        <select v-model="invitationRoleCode" data-testid="tenant-invitation-role" class="field-input">
          <option v-for="role in tenantRoleOptions" :key="role" :value="role">
            {{ role }}
          </option>
        </select>
      </label>

      <div class="action-row form-span">
        <button class="primary-button" :disabled="!canInvite" type="submit">
          {{ commonStore.saving ? t('common.saving') : t('tenantAdmin.actions.invite') }}
        </button>
      </div>
    </form>
  </section>

  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.sections.invitations') }}</span>
        <h2>{{ t('tenantAdmin.headings.pendingInvites') }}</h2>
      </div>
    </div>

    <div v-if="commonStore.invitations.length > 0" class="list-stack">
      <article v-for="invitation in commonStore.invitations" :key="invitation.publicId" class="list-item">
        <div>
          <strong>{{ invitation.inviteeEmailNormalized }}</strong>
          <span class="cell-subtle">{{ invitation.roleCodes?.join(', ') }} / {{ invitation.status }}</span>
          <span class="cell-subtle">{{ t('tenantAdmin.labels.expiresAt', { date: formatDate(invitation.expiresAt) }) }}</span>
        </div>
        <button
          class="secondary-button danger-button compact-button"
          type="button"
          :disabled="invitation.status !== 'pending' || commonStore.saving"
          @click="revokeInvitation(invitation.publicId)"
        >
          {{ t('tenantAdmin.actions.revoke') }}
        </button>
      </article>
    </div>

    <div v-else class="empty-state">
      <p>{{ t('tenantAdmin.empty.invitations') }}</p>
    </div>
  </section>
</template>
