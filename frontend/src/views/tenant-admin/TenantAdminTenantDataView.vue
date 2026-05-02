<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

import { useTenantAdminDetailContext } from '../../tenant-admin/detail-context'

const DATA_JOB_REFRESH_INTERVAL_MS = 4000

const { t } = useI18n()
const {
  commonStore,
  formatDate,
  importFile,
  onImportFileChange,
  requestCSVExport,
  requestExport,
  tenant,
  uploadImportCSV,
} = useTenantAdminDetailContext()

let refreshTimer: number | undefined

onMounted(() => {
  refreshTimer = window.setInterval(() => {
    const slug = tenant.value?.slug
    if (slug && commonStore.hasActiveDataJobs) {
      void commonStore.refreshDataJobs(slug)
    }
  }, DATA_JOB_REFRESH_INTERVAL_MS)
})

onBeforeUnmount(() => {
  if (refreshTimer !== undefined) {
    window.clearInterval(refreshTimer)
  }
})
</script>

<template>
  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.labels.export') }}</span>
        <h2>{{ t('tenantAdmin.headings.tenantDataExports') }}</h2>
      </div>
      <div class="action-row">
        <button
          data-testid="tenant-request-export"
          class="primary-button compact-button"
          type="button"
          :disabled="commonStore.saving"
          @click="requestExport"
        >
          {{ t('tenantAdmin.actions.requestExport') }}
        </button>
        <button
          class="secondary-button compact-button"
          type="button"
          :disabled="commonStore.saving"
          @click="requestCSVExport"
        >
          {{ t('tenantAdmin.actions.requestCsv') }}
        </button>
      </div>
    </div>

    <div v-if="commonStore.exports.length > 0" class="list-stack">
      <article v-for="item in commonStore.exports" :key="item.publicId" class="list-item">
        <div>
          <strong>{{ item.format }} / {{ item.status }}</strong>
          <span class="cell-subtle">{{ item.publicId }}</span>
          <span class="cell-subtle">{{ t('tenantAdmin.labels.expiresAt', { date: formatDate(item.expiresAt) }) }}</span>
        </div>
        <a
          v-if="item.status === 'ready' && tenant"
          class="secondary-button compact-button link-button"
          :href="`/api/v1/admin/tenants/${tenant.slug}/exports/${item.publicId}/download`"
        >
          {{ t('common.download') }}
        </a>
      </article>
    </div>

    <div v-else class="empty-state">
      <p>{{ t('tenantAdmin.empty.exports') }}</p>
    </div>
  </section>

  <section class="panel stack">
    <form class="admin-form" @submit.prevent="uploadImportCSV">
      <div class="form-span">
        <span class="status-pill">{{ t('tenantAdmin.labels.import') }}</span>
        <h2>{{ t('tenantAdmin.headings.customerSignalsCsv') }}</h2>
      </div>

      <label class="field form-span">
        <span class="field-label">{{ t('tenantAdmin.fields.csvFile') }}</span>
        <input class="field-input" accept=".csv,text/csv" type="file" @change="onImportFileChange">
      </label>

      <div class="action-row form-span">
        <button class="primary-button" :disabled="commonStore.saving || !importFile" type="submit">
          {{ t('tenantAdmin.actions.uploadAndImport') }}
        </button>
      </div>
    </form>
  </section>

  <section v-if="commonStore.imports.length > 0" class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.labels.import') }}</span>
        <h2>{{ t('tenantAdmin.headings.importJobs') }}</h2>
      </div>
    </div>

    <div class="list-stack">
      <article v-for="item in commonStore.imports" :key="item.publicId" class="list-item">
        <div>
          <strong>{{ item.status }}</strong>
          <span class="cell-subtle">{{ item.publicId }}</span>
          <span class="cell-subtle">
            {{ t('tenantAdmin.labels.importRows', { validRows: item.validRows, totalRows: item.totalRows, invalidRows: item.invalidRows }) }}
          </span>
        </div>
      </article>
    </div>
  </section>
</template>
