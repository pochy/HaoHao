<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { toApiErrorMessage } from '../../api/client'
import {
  createTenantAdminDriveLocalSearchRebuildJob,
  fetchTenantAdminDriveLocalSearchJobs,
  fetchTenantAdminSchemaMappingExamples,
  rebuildTenantAdminSchemaMappingSearchIndex,
  updateTenantAdminSchemaMappingExampleSharedScope,
} from '../../api/tenant-admin'
import type { LocalSearchIndexJobBody, TenantAdminSchemaMappingExampleListItemBody } from '../../api/generated/types.gen'
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

const evidenceItems = ref<TenantAdminSchemaMappingExampleListItemBody[]>([])
const evidenceLoading = ref(false)
const evidenceSavingId = ref('')
const evidenceError = ref('')
const evidenceMessage = ref('')
const evidenceQuery = ref('')
const evidenceSharedScope = ref<'all' | 'private' | 'tenant'>('all')
const evidenceDecision = ref<'all' | 'accepted' | 'rejected'>('all')
const rebuildResult = ref<{ indexed: number, schemaColumnsIndexed: number, mappingExamplesIndexed: number } | null>(null)
const localSearchJobs = ref<LocalSearchIndexJobBody[]>([])
const localSearchLoading = ref(false)
const localSearchSaving = ref(false)
const localSearchError = ref('')
const localSearchMessage = ref('')

const evidenceFilters = computed(() => ({
  q: evidenceQuery.value,
  sharedScope: evidenceSharedScope.value === 'all' ? undefined : evidenceSharedScope.value,
  decision: evidenceDecision.value === 'all' ? undefined : evidenceDecision.value,
  limit: 100,
}))

let refreshTimer: number | undefined

onMounted(() => {
  void loadEvidence()
  void loadLocalSearchJobs()
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

async function loadEvidence() {
  if (!tenant.value) {
    evidenceItems.value = []
    return
  }
  evidenceLoading.value = true
  evidenceError.value = ''
  try {
    evidenceItems.value = await fetchTenantAdminSchemaMappingExamples(tenant.value.slug, evidenceFilters.value)
  } catch (error) {
    evidenceItems.value = []
    evidenceError.value = toApiErrorMessage(error)
  } finally {
    evidenceLoading.value = false
  }
}

async function loadLocalSearchJobs() {
  if (!tenant.value) {
    localSearchJobs.value = []
    return
  }
  localSearchLoading.value = true
  localSearchError.value = ''
  try {
    localSearchJobs.value = await fetchTenantAdminDriveLocalSearchJobs(tenant.value.slug)
  } catch (error) {
    localSearchJobs.value = []
    localSearchError.value = toApiErrorMessage(error)
  } finally {
    localSearchLoading.value = false
  }
}

async function requestLocalSearchRebuild() {
  if (!tenant.value) {
    return
  }
  localSearchSaving.value = true
  localSearchError.value = ''
  localSearchMessage.value = ''
  try {
    const job = await createTenantAdminDriveLocalSearchRebuildJob(tenant.value.slug)
    localSearchJobs.value = [job, ...localSearchJobs.value]
    localSearchMessage.value = t('tenantAdmin.messages.localSearchRebuildQueued')
  } catch (error) {
    localSearchError.value = toApiErrorMessage(error)
  } finally {
    localSearchSaving.value = false
  }
}

async function updateEvidenceScope(item: TenantAdminSchemaMappingExampleListItemBody, sharedScope: 'private' | 'tenant') {
  if (!tenant.value || item.sharedScope === sharedScope) {
    return
  }
  evidenceSavingId.value = item.publicId
  evidenceError.value = ''
  evidenceMessage.value = ''
  try {
    await updateTenantAdminSchemaMappingExampleSharedScope(tenant.value.slug, item.publicId, sharedScope)
    await loadEvidence()
    evidenceMessage.value = t('tenantAdmin.messages.schemaMappingEvidenceUpdated')
  } catch (error) {
    evidenceError.value = toApiErrorMessage(error)
  } finally {
    evidenceSavingId.value = ''
  }
}

async function rebuildSchemaMappingSearchDocuments() {
  if (!tenant.value) {
    return
  }
  evidenceSavingId.value = 'rebuild'
  evidenceError.value = ''
  evidenceMessage.value = ''
  rebuildResult.value = null
  try {
    rebuildResult.value = await rebuildTenantAdminSchemaMappingSearchIndex(tenant.value.slug, { limit: 100 })
    evidenceMessage.value = t('tenantAdmin.messages.schemaMappingSearchRebuilt')
    await loadEvidence()
  } catch (error) {
    evidenceError.value = toApiErrorMessage(error)
  } finally {
    evidenceSavingId.value = ''
  }
}
</script>

<template>
  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.labels.localSearch') }}</span>
        <h2>{{ t('tenantAdmin.headings.localSearchIndex') }}</h2>
      </div>
      <div class="action-row">
        <button
          class="secondary-button compact-button"
          type="button"
          :disabled="localSearchLoading"
          @click="loadLocalSearchJobs"
        >
          {{ localSearchLoading ? t('common.refreshing') : t('common.refresh') }}
        </button>
        <button
          class="primary-button compact-button"
          type="button"
          :disabled="localSearchSaving"
          @click="requestLocalSearchRebuild"
        >
          {{ t('tenantAdmin.actions.rebuildLocalSearch') }}
        </button>
      </div>
    </div>

    <p v-if="localSearchError" class="form-error">{{ localSearchError }}</p>
    <p v-if="localSearchMessage" class="form-success">{{ localSearchMessage }}</p>

    <div v-if="localSearchJobs.length > 0" class="table-wrap">
      <table class="data-table">
        <thead>
          <tr>
            <th>{{ t('tenantAdmin.fields.status') }}</th>
            <th>{{ t('tenantAdmin.fields.reason') }}</th>
            <th>{{ t('tenantAdmin.fields.resourceKind') }}</th>
            <th>{{ t('tenantAdmin.fields.indexedCount') }}</th>
            <th>{{ t('tenantAdmin.fields.failedCount') }}</th>
            <th>{{ t('tenantAdmin.fields.completedAt') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="job in localSearchJobs.slice(0, 10)" :key="job.publicId">
            <td>
              <span :class="['status-pill', job.status === 'completed' ? 'success' : job.status === 'failed' ? 'warning' : '']">
                {{ job.status }}
              </span>
              <span v-if="job.lastError" class="cell-subtle">{{ job.lastError }}</span>
            </td>
            <td>{{ job.reason }}</td>
            <td>{{ job.resourceKind || t('common.all') }}</td>
            <td>{{ job.indexedCount }} / {{ job.skippedCount }}</td>
            <td>{{ job.failedCount }}</td>
            <td>{{ job.completedAt ? formatDate(job.completedAt) : '-' }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else class="empty-state">
      <p>{{ localSearchLoading ? t('common.loading') : t('tenantAdmin.empty.driveLocalSearchJobs') }}</p>
    </div>
  </section>

  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.labels.schemaMapping') }}</span>
        <h2>{{ t('tenantAdmin.headings.schemaMappingEvidence') }}</h2>
      </div>
      <div class="action-row">
        <button
          class="secondary-button compact-button"
          type="button"
          :disabled="evidenceLoading"
          @click="loadEvidence"
        >
          {{ evidenceLoading ? t('common.refreshing') : t('common.refresh') }}
        </button>
        <button
          class="primary-button compact-button"
          type="button"
          :disabled="evidenceSavingId !== ''"
          @click="rebuildSchemaMappingSearchDocuments"
        >
          {{ t('tenantAdmin.actions.rebuildSearchDocuments') }}
        </button>
      </div>
    </div>

    <form class="filter-bar" @submit.prevent="loadEvidence">
      <label class="field compact-field">
        <span class="field-label">{{ t('common.search') }}</span>
        <input
          v-model="evidenceQuery"
          class="field-input"
          type="search"
          :placeholder="t('tenantAdmin.placeholders.schemaMappingEvidenceSearch')"
        >
      </label>
      <label class="field compact-field">
        <span class="field-label">{{ t('tenantAdmin.fields.sharedScope') }}</span>
        <select v-model="evidenceSharedScope" class="field-input">
          <option value="all">{{ t('common.all') }}</option>
          <option value="private">{{ t('tenantAdmin.options.private') }}</option>
          <option value="tenant">{{ t('tenantAdmin.options.tenantShared') }}</option>
        </select>
      </label>
      <label class="field compact-field">
        <span class="field-label">{{ t('tenantAdmin.fields.decision') }}</span>
        <select v-model="evidenceDecision" class="field-input">
          <option value="all">{{ t('common.all') }}</option>
          <option value="accepted">{{ t('tenantAdmin.options.accepted') }}</option>
          <option value="rejected">{{ t('tenantAdmin.options.rejected') }}</option>
        </select>
      </label>
      <button class="secondary-button compact-button" type="submit" :disabled="evidenceLoading">
        {{ t('common.apply') }}
      </button>
    </form>

    <p v-if="evidenceError" class="form-error">{{ evidenceError }}</p>
    <p v-if="evidenceMessage" class="form-success">{{ evidenceMessage }}</p>
    <p v-if="rebuildResult" class="cell-subtle">
      {{ t('tenantAdmin.labels.schemaMappingRebuildResult', {
        indexed: rebuildResult.indexed,
        schemaColumns: rebuildResult.schemaColumnsIndexed,
        mappingExamples: rebuildResult.mappingExamplesIndexed,
      }) }}
    </p>

    <div v-if="evidenceItems.length > 0" class="table-wrap">
      <table class="data-table">
        <thead>
          <tr>
            <th>{{ t('tenantAdmin.fields.sourceColumn') }}</th>
            <th>{{ t('tenantAdmin.fields.targetColumn') }}</th>
            <th>{{ t('tenantAdmin.fields.pipeline') }}</th>
            <th>{{ t('tenantAdmin.fields.decision') }}</th>
            <th>{{ t('tenantAdmin.fields.sharedScope') }}</th>
            <th>{{ t('tenantAdmin.fields.searchDocument') }}</th>
            <th>{{ t('common.updated') }}</th>
            <th>{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in evidenceItems" :key="item.publicId">
            <td>
              <strong>{{ item.sourceColumn }}</strong>
              <span v-if="item.sampleValues?.length" class="cell-subtle">{{ item.sampleValues.slice(0, 3).join(', ') }}</span>
            </td>
            <td>
              <strong>{{ item.targetColumn }}</strong>
              <span class="cell-subtle">{{ item.domain }} / {{ item.schemaType }}</span>
            </td>
            <td>
              <span>{{ item.pipelineName }}</span>
              <span class="cell-subtle">{{ item.pipelinePublicId }}</span>
            </td>
            <td>
              <span :class="['status-pill', item.decision === 'accepted' ? 'success' : 'warning']">{{ item.decision }}</span>
            </td>
            <td>
              <span :class="['status-pill', item.sharedScope === 'tenant' ? 'success' : '']">{{ item.sharedScope }}</span>
              <span v-if="item.sharedAt" class="cell-subtle">{{ formatDate(item.sharedAt) }}</span>
            </td>
            <td>
              <span :class="['status-pill', item.searchDocumentMaterialized ? 'success' : 'warning']">
                {{ item.searchDocumentMaterialized ? t('tenantAdmin.status.materialized') : t('tenantAdmin.status.notMaterialized') }}
              </span>
            </td>
            <td>{{ formatDate(item.updatedAt) }}</td>
            <td>
              <div class="action-row">
                <button
                  class="secondary-button compact-button"
                  type="button"
                  :disabled="evidenceSavingId !== '' || item.sharedScope === 'tenant'"
                  @click="updateEvidenceScope(item, 'tenant')"
                >
                  {{ t('tenantAdmin.actions.shareToTenant') }}
                </button>
                <button
                  class="secondary-button compact-button"
                  type="button"
                  :disabled="evidenceSavingId !== '' || item.sharedScope === 'private'"
                  @click="updateEvidenceScope(item, 'private')"
                >
                  {{ t('tenantAdmin.actions.makePrivate') }}
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else class="empty-state">
      <p>{{ evidenceLoading ? t('common.loading') : t('tenantAdmin.empty.schemaMappingEvidence') }}</p>
    </div>
  </section>

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
