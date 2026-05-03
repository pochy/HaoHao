<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { ArrowLeft, RefreshCw, Square } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { fetchSystemJob, stopSystemJob, type SystemJobBody } from '../api/jobs'
import { toApiErrorMessage } from '../api/client'

const { t } = useI18n()
const route = useRoute()

const job = ref<SystemJobBody | null>(null)
const loading = ref(false)
const stopping = ref(false)
const errorMessage = ref('')

const jobType = computed(() => String(route.params.jobType ?? ''))
const jobPublicId = computed(() => String(route.params.jobPublicId ?? ''))
const metadataRows = computed(() => Object.entries(job.value?.metadata ?? {}).filter(([, value]) => value !== null && value !== undefined && value !== ''))

onMounted(loadJob)

function statusTone(status: string) {
  if (/failed|dead|denied/i.test(status)) return 'danger'
  if (/completed|ready|sent|succeeded|delivered/i.test(status)) return 'success'
  if (/pending|queued|processing|running/i.test(status)) return 'warning'
  return ''
}

function formatDate(value?: string) {
  return value ? new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value)) : '-'
}

function formatValue(value: unknown) {
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  return JSON.stringify(value)
}

async function loadJob() {
  loading.value = true
  errorMessage.value = ''
  try {
    job.value = await fetchSystemJob(jobType.value, jobPublicId.value)
  } catch (error) {
    job.value = null
    errorMessage.value = toApiErrorMessage(error)
  } finally {
    loading.value = false
  }
}

async function stopJob() {
  if (!job.value) return
  stopping.value = true
  errorMessage.value = ''
  try {
    job.value = await stopSystemJob(job.value.type, job.value.publicId)
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  } finally {
    stopping.value = false
  }
}
</script>

<template>
  <main class="page-shell jobs-page">
    <section class="page-hero compact">
      <div>
        <RouterLink class="text-link" to="/jobs">
          <ArrowLeft :size="15" aria-hidden="true" />
          {{ t('jobs.backToJobs') }}
        </RouterLink>
        <span class="eyebrow">{{ job?.type?.replace(/_/g, ' ') || t('jobs.detailEyebrow') }}</span>
        <h1>{{ job?.title || t('jobs.detailTitle') }}</h1>
        <p>{{ job?.publicId || jobPublicId }}</p>
      </div>
      <div class="jobs-detail-actions">
        <button class="secondary-button" type="button" :disabled="loading" @click="loadJob">
          <RefreshCw :size="16" aria-hidden="true" />
          {{ t('common.refresh') }}
        </button>
        <button class="secondary-button danger" type="button" :disabled="!job?.canStop || stopping" @click="stopJob">
          <Square :size="15" aria-hidden="true" />
          {{ t('jobs.stop') }}
        </button>
      </div>
    </section>

    <p v-if="errorMessage" class="error-message">{{ errorMessage }}</p>
    <p v-if="loading" class="cell-subtle">{{ t('common.loading') }}</p>

    <section v-if="job" class="jobs-detail-grid">
      <div class="panel stack">
        <div class="section-header">
          <div>
            <span class="status-pill" :class="statusTone(job.status)">{{ job.status }}</span>
            <h2>{{ t('jobs.sections.summary') }}</h2>
          </div>
        </div>
        <dl class="detail-list">
          <div>
            <dt>{{ t('jobs.fields.when') }}</dt>
            <dd>{{ formatDate(job.createdAt) }}</dd>
          </div>
          <div>
            <dt>{{ t('jobs.fields.startedAt') }}</dt>
            <dd>{{ formatDate(job.startedAt) }}</dd>
          </div>
          <div>
            <dt>{{ t('jobs.fields.completedAt') }}</dt>
            <dd>{{ formatDate(job.completedAt) }}</dd>
          </div>
          <div>
            <dt>{{ t('jobs.fields.who') }}</dt>
            <dd>{{ job.requestedByDisplayName || job.requestedByEmail || '-' }}</dd>
          </div>
          <div>
            <dt>{{ t('jobs.fields.what') }}</dt>
            <dd>{{ job.action || job.title }}</dd>
          </div>
          <div>
            <dt>{{ t('jobs.fields.status') }}</dt>
            <dd>{{ job.status }}</dd>
          </div>
          <div v-if="job.errorMessage">
            <dt>{{ t('jobs.fields.error') }}</dt>
            <dd class="error-message">{{ job.errorMessage }}</dd>
          </div>
        </dl>
      </div>

      <div class="panel stack">
        <div class="section-header">
          <div>
            <span class="status-pill">{{ t('jobs.sections.identifiers') }}</span>
            <h2>{{ t('common.details') }}</h2>
          </div>
        </div>
        <dl class="detail-list">
          <div>
            <dt>{{ t('jobs.fields.type') }}</dt>
            <dd>{{ job.type }}</dd>
          </div>
          <div>
            <dt>{{ t('common.publicId') }}</dt>
            <dd class="mono-cell">{{ job.publicId }}</dd>
          </div>
          <div>
            <dt>{{ t('jobs.fields.subject') }}</dt>
            <dd>{{ job.subjectType || '-' }} <span v-if="job.subjectPublicId" class="cell-subtle mono-cell">/ {{ job.subjectPublicId }}</span></dd>
          </div>
          <div>
            <dt>{{ t('jobs.fields.outbox') }}</dt>
            <dd class="mono-cell">{{ job.outboxEventPublicId || '-' }}</dd>
          </div>
          <div>
            <dt>{{ t('common.updated') }}</dt>
            <dd>{{ formatDate(job.updatedAt) }}</dd>
          </div>
        </dl>
      </div>
    </section>

    <section v-if="job" class="panel stack">
      <div class="section-header">
        <div>
          <span class="status-pill">{{ t('jobs.sections.metadata') }}</span>
          <h2>{{ t('jobs.metadata') }}</h2>
        </div>
      </div>
      <div class="admin-table">
        <table>
          <tbody>
            <tr v-if="metadataRows.length === 0">
              <td>{{ t('common.none') }}</td>
            </tr>
            <tr v-for="[key, value] in metadataRows" :key="key">
              <td>{{ key }}</td>
              <td class="mono-cell">{{ formatValue(value) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </main>
</template>
