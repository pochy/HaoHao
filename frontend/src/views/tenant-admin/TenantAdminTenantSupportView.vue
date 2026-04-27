<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { useTenantAdminDetailContext } from '../../tenant-admin/detail-context'

const { t } = useI18n()
const {
  startSupportAccess,
  supportReason,
  supportUserPublicId,
} = useTenantAdminDetailContext()
</script>

<template>
  <section class="panel stack">
    <form class="admin-form" @submit.prevent="startSupportAccess">
      <div class="form-span">
        <span class="status-pill">{{ t('tenantAdmin.sections.support') }}</span>
        <h2>{{ t('tenantAdmin.headings.supportAccess') }}</h2>
      </div>

      <label class="field">
        <span class="field-label">{{ t('common.userPublicId') }}</span>
        <input v-model="supportUserPublicId" class="field-input" autocomplete="off">
      </label>

      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.reason') }}</span>
        <input v-model="supportReason" class="field-input" autocomplete="off">
      </label>

      <div class="action-row form-span">
        <button class="secondary-button" :disabled="supportUserPublicId.trim() === '' || supportReason.trim().length < 8" type="submit">
          {{ t('tenantAdmin.actions.startSupportAccess') }}
        </button>
      </div>
    </form>
  </section>
</template>
