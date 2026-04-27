<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { useTenantAdminDetailContext } from '../../tenant-admin/detail-context'

const { t } = useI18n()
const {
  browserRateLimit,
  canSaveCommonSettings,
  commonStore,
  fileQuotaBytes,
  notificationsEnabled,
  saveCommonSettings,
} = useTenantAdminDetailContext()
</script>

<template>
  <section class="panel stack">
    <form class="admin-form" @submit.prevent="saveCommonSettings">
      <div class="form-span">
        <span class="status-pill">{{ t('tenantAdmin.labels.common') }}</span>
        <h2>{{ t('tenantAdmin.headings.settingsAndQuota') }}</h2>
      </div>

      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.fileQuotaBytes') }}</span>
        <input v-model.number="fileQuotaBytes" data-testid="tenant-file-quota" class="field-input" min="0" type="number">
      </label>

      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.browserApiLimit') }}</span>
        <input v-model.number="browserRateLimit" data-testid="tenant-browser-rate-limit" class="field-input" min="1" type="number">
      </label>

      <label class="checkbox-field form-span">
        <input v-model="notificationsEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.notificationsEnabled') }}</span>
      </label>

      <div class="action-row form-span">
        <button class="primary-button" :disabled="!canSaveCommonSettings" type="submit">
          {{ commonStore.saving ? t('common.saving') : t('tenantAdmin.actions.saveCommonSettings') }}
        </button>
      </div>
    </form>
  </section>
</template>
