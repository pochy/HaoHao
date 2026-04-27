<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { useTenantAdminDetailContext } from '../../tenant-admin/detail-context'

const { t } = useI18n()
const {
  commonStore,
  createWebhook,
  webhookEvents,
  webhookName,
  webhookUrl,
} = useTenantAdminDetailContext()
</script>

<template>
  <section class="panel stack">
    <form class="admin-form" @submit.prevent="createWebhook">
      <div class="form-span">
        <span class="status-pill">{{ t('tenantAdmin.sections.webhooks') }}</span>
        <h2>{{ t('tenantAdmin.headings.outboundWebhooks') }}</h2>
      </div>

      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.name') }}</span>
        <input v-model="webhookName" class="field-input" autocomplete="off" :placeholder="t('tenantAdmin.placeholders.webhookName')">
      </label>

      <label class="field">
        <span class="field-label">URL</span>
        <input v-model="webhookUrl" class="field-input" autocomplete="url" placeholder="https://example.com/webhook">
      </label>

      <label class="field form-span">
        <span class="field-label">{{ t('tenantAdmin.fields.events') }}</span>
        <input v-model="webhookEvents" class="field-input" autocomplete="off">
      </label>

      <div class="action-row form-span">
        <button class="primary-button" :disabled="commonStore.saving || webhookName.trim() === '' || webhookUrl.trim() === ''" type="submit">
          {{ t('tenantAdmin.actions.createWebhook') }}
        </button>
      </div>
    </form>
  </section>

  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.sections.webhooks') }}</span>
        <h2>{{ t('tenantAdmin.headings.registeredEndpoints') }}</h2>
      </div>
    </div>

    <div v-if="commonStore.webhooks.length > 0" class="list-stack">
      <article v-for="item in commonStore.webhooks" :key="item.publicId" class="list-item">
        <div>
          <strong>{{ item.name }} / {{ item.active ? t('common.active') : t('tenantAdmin.status.inactive') }}</strong>
          <span class="cell-subtle">{{ item.url }}</span>
          <span class="cell-subtle">{{ item.eventTypes?.join(', ') }}</span>
        </div>
      </article>
    </div>

    <div v-else class="empty-state">
      <p>{{ t('tenantAdmin.empty.webhooks') }}</p>
    </div>
  </section>
</template>
