<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { useTenantAdminDetailContext } from '../../tenant-admin/detail-context'

const { t } = useI18n()
const {
  commonStore,
  saveEntitlements,
} = useTenantAdminDetailContext()
</script>

<template>
  <section class="panel stack">
    <form class="admin-form" @submit.prevent="saveEntitlements">
      <div class="form-span">
        <span class="status-pill">{{ t('tenantAdmin.sections.entitlements') }}</span>
        <h2>{{ t('tenantAdmin.headings.featureGates') }}</h2>
      </div>

      <label
        v-for="item in commonStore.entitlements"
        :key="item.featureCode"
        class="checkbox-field"
      >
        <input v-model="item.enabled" type="checkbox">
        <span>{{ item.featureCode }}</span>
      </label>

      <div v-if="commonStore.entitlements.length === 0" class="empty-state form-span">
        <p>{{ t('tenantAdmin.empty.entitlements') }}</p>
      </div>

      <div class="action-row form-span">
        <button class="primary-button" :disabled="commonStore.saving" type="submit">
          {{ t('tenantAdmin.actions.saveEntitlements') }}
        </button>
      </div>
    </form>
  </section>
</template>
