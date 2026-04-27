<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { useTenantAdminDetailContext } from '../../tenant-admin/detail-context'

const { t } = useI18n()
const {
  active,
  askDeactivate,
  canSaveSettings,
  displayName,
  formatDate,
  saveSettings,
  store,
  tenant,
} = useTenantAdminDetailContext()
</script>

<template>
  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.sections.overview') }}</span>
        <h2>{{ t('tenantAdmin.headings.tenantSettings') }}</h2>
      </div>
    </div>

    <form v-if="tenant" class="admin-form" @submit.prevent="saveSettings">
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.slug') }}</span>
        <input :value="tenant.slug" class="field-input" autocomplete="off" disabled>
      </label>

      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.displayName') }}</span>
        <input v-model="displayName" class="field-input" autocomplete="off" required>
      </label>

      <label class="checkbox-field form-span">
        <input v-model="active" type="checkbox">
        <span>{{ t('common.active') }}</span>
      </label>

      <dl class="metadata-grid form-span">
        <div>
          <dt>{{ t('tenantAdmin.activeMembers') }}</dt>
          <dd>{{ tenant.activeMemberCount }}</dd>
        </div>
        <div>
          <dt>{{ t('common.updated') }}</dt>
          <dd>{{ formatDate(tenant.updatedAt) }}</dd>
        </div>
      </dl>

      <div class="action-row form-span">
        <button class="primary-button" :disabled="!canSaveSettings" type="submit">
          {{ store.saving ? t('common.saving') : t('common.save') }}
        </button>
        <button
          class="secondary-button danger-button"
          :disabled="store.saving || !tenant.active"
          type="button"
          @click="askDeactivate"
        >
          {{ t('tenantAdmin.actions.deactivate') }}
        </button>
      </div>
    </form>
  </section>
</template>
