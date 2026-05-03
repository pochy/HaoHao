<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { RefreshCw, Search, SlidersHorizontal, Square, X } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { fetchSystemJobs, stopSystemJob, type SystemJobBody } from '../api/jobs'
import { toApiErrorMessage } from '../api/client'
import DataCard from '../components/DataCard.vue'
import EmptyState from '../components/EmptyState.vue'
import MetricTile from '../components/MetricTile.vue'
import PageHeader from '../components/PageHeader.vue'
import StatusBadge from '../components/StatusBadge.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const items = ref<SystemJobBody[]>([])
const total = ref(0)
const loading = ref(false)
const stoppingId = ref('')
const errorMessage = ref('')

const query = ref(String(route.query.query ?? ''))
const type = ref(String(route.query.type ?? ''))
const status = ref(String(route.query.status ?? ''))
const statusGroup = ref(String(route.query.statusGroup ?? ''))
const limit = ref(Number(route.query.limit ?? 25))
const offset = ref(Number(route.query.offset ?? 0))

const page = computed(() => Math.floor(offset.value / limit.value) + 1)
const pageCount = computed(() => Math.max(1, Math.ceil(total.value / limit.value)))
const activeCount = computed(() => items.value.filter((item) => item.statusGroup === 'active').length)
const failedCount = computed(() => items.value.filter((item) => /failed|dead|denied/i.test(item.status)).length)
const hasActiveFilters = computed(() => Boolean(query.value.trim() || type.value || status.value || statusGroup.value))
const visibleRange = computed(() => {
  if (total.value === 0) {
    return '0'
  }
  const start = offset.value + 1
  const end = Math.min(offset.value + items.value.length, total.value)
  return `${start}-${end}`
})

const jobTypeOptions = [
  'outbox_event',
  'drive_ocr',
  'data_pipeline_run',
  'local_search_index',
  'dataset_import',
  'dataset_query',
  'dataset_sync',
  'dataset_work_table_export',
  'dataset_gold_publish',
  'dataset_lineage_parse',
  'tenant_data_export',
  'customer_signal_import',
  'webhook_delivery',
  'drive_index',
  'drive_ai',
  'drive_key_rotation',
  'drive_region_migration',
  'drive_clean_room',
]

const statusOptions = ['pending', 'queued', 'processing', 'running', 'failed', 'completed', 'ready', 'sent', 'dead', 'skipped', 'succeeded']

watch(() => route.query, () => {
  query.value = String(route.query.query ?? '')
  type.value = String(route.query.type ?? '')
  status.value = String(route.query.status ?? '')
  statusGroup.value = String(route.query.statusGroup ?? '')
  limit.value = Number(route.query.limit ?? 25)
  offset.value = Number(route.query.offset ?? 0)
  loadJobs()
})

onMounted(loadJobs)

function jobTypeLabel(value: string) {
  return value.replace(/_/g, ' ')
}

function statusTone(status: string): 'success' | 'warning' | 'danger' | 'neutral' {
  if (/failed|dead|denied/i.test(status)) return 'danger'
  if (/completed|ready|sent|succeeded|delivered/i.test(status)) return 'success'
  if (/pending|queued|processing|running/i.test(status)) return 'warning'
  return 'neutral'
}

function formatDate(value?: string) {
  return value ? new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '-'
}

function requesterLabel(item: SystemJobBody) {
  return item.requestedByDisplayName || item.requestedByEmail || t('jobs.system')
}

function jobSummary(item: SystemJobBody) {
  if (item.errorMessage) {
    return item.errorMessage
  }
  if (item.action) {
    return item.action
  }
  if (item.subjectType || item.subjectPublicId) {
    return [item.subjectType, item.subjectPublicId].filter(Boolean).join(' / ')
  }
  return item.publicId
}

async function loadJobs() {
  loading.value = true
  errorMessage.value = ''
  try {
    const result = await fetchSystemJobs({
      query: query.value,
      type: type.value,
      status: status.value,
      statusGroup: statusGroup.value,
      limit: limit.value,
      offset: offset.value,
    })
    items.value = result.items
    total.value = result.total
    limit.value = result.limit
    offset.value = result.offset
  } catch (error) {
    items.value = []
    total.value = 0
    errorMessage.value = toApiErrorMessage(error)
  } finally {
    loading.value = false
  }
}

async function applyFilters() {
  offset.value = 0
  await router.push({ name: 'jobs', query: routeQuery() })
}

async function clearFilters() {
  query.value = ''
  type.value = ''
  status.value = ''
  statusGroup.value = ''
  offset.value = 0
  await router.push({ name: 'jobs', query: routeQuery() })
}

async function changePage(nextOffset: number) {
  offset.value = Math.max(0, Math.min(nextOffset, Math.max(0, total.value - 1)))
  await router.push({ name: 'jobs', query: routeQuery() })
}

async function stopJob(item: SystemJobBody) {
  stoppingId.value = `${item.type}:${item.publicId}`
  errorMessage.value = ''
  try {
    const updated = await stopSystemJob(item.type, item.publicId)
    const index = items.value.findIndex((candidate) => candidate.type === item.type && candidate.publicId === item.publicId)
    if (index >= 0) {
      items.value[index] = updated
    }
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  } finally {
    stoppingId.value = ''
  }
}

function routeQuery() {
  const out: Record<string, string> = {
    limit: String(limit.value),
    offset: String(offset.value),
  }
  if (query.value.trim()) out.query = query.value.trim()
  if (type.value) out.type = type.value
  if (status.value) out.status = status.value
  if (statusGroup.value) out.statusGroup = statusGroup.value
  return out
}
</script>

<template>
  <section class="stack jobs-page">
    <PageHeader
      :eyebrow="t('jobs.eyebrow')"
      :title="t('jobs.title')"
      :description="t('jobs.description')"
    >
      <template #actions>
        <button class="secondary-button" type="button" :disabled="loading" @click="loadJobs">
          <RefreshCw :size="16" stroke-width="1.8" aria-hidden="true" />
          {{ loading ? t('common.refreshing') : t('common.refresh') }}
        </button>
      </template>
    </PageHeader>

    <section class="metric-grid">
      <MetricTile :label="t('jobs.metrics.total')" :value="total" :hint="t('jobs.metrics.totalHint')" />
      <MetricTile :label="t('jobs.metrics.active')" :value="activeCount" :hint="t('jobs.metrics.activeHint')" />
      <MetricTile :label="t('jobs.metrics.failed')" :value="failedCount" :hint="t('jobs.metrics.failedHint')" />
    </section>

    <DataCard :title="t('jobs.searchCardTitle')" :subtitle="t('jobs.searchCardSubtitle')">
      <form class="notification-command-bar" role="search" @submit.prevent="applyFilters">
        <div class="notification-search-box">
          <Search :size="18" stroke-width="1.9" aria-hidden="true" />
          <label class="sr-only" for="jobs-search-query">{{ t('jobs.searchPlaceholder') }}</label>
          <input
            id="jobs-search-query"
            v-model="query"
            type="search"
            autocomplete="off"
            :placeholder="t('jobs.searchPlaceholder')"
            :disabled="loading"
          >
          <button class="secondary-button compact-button" type="submit" :disabled="loading">
            {{ t('common.search') }}
          </button>
        </div>

        <div class="notification-filter-row" :aria-label="t('jobs.filters.label')">
          <label class="notification-filter-chip">
            <SlidersHorizontal :size="15" stroke-width="1.9" aria-hidden="true" />
            <span>{{ t('jobs.filters.type') }}</span>
            <select v-model="type" :disabled="loading" @change="applyFilters">
              <option value="">{{ t('common.all') }}</option>
              <option v-for="option in jobTypeOptions" :key="option" :value="option">{{ jobTypeLabel(option) }}</option>
            </select>
          </label>

          <label class="notification-filter-chip">
            <span>{{ t('jobs.filters.state') }}</span>
            <select v-model="statusGroup" :disabled="loading" @change="applyFilters">
              <option value="">{{ t('common.all') }}</option>
              <option value="active">{{ t('jobs.active') }}</option>
              <option value="terminal">{{ t('jobs.terminal') }}</option>
            </select>
          </label>

          <label class="notification-filter-chip">
            <span>{{ t('jobs.filters.status') }}</span>
            <select v-model="status" :disabled="loading" @change="applyFilters">
              <option value="">{{ t('common.all') }}</option>
              <option v-for="option in statusOptions" :key="option" :value="option">{{ option }}</option>
            </select>
          </label>

          <button class="secondary-button compact-button" type="button" :disabled="loading || !hasActiveFilters" @click="clearFilters">
            <X :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('common.clear') }}
          </button>
        </div>
      </form>
    </DataCard>

    <p v-if="errorMessage" class="error-message">{{ errorMessage }}</p>

    <p v-if="loading && items.length === 0">
      {{ t('common.loading') }}
    </p>

    <DataCard v-if="items.length > 0" :title="t('jobs.resultsTitle')" :subtitle="t('jobs.resultsSubtitle', { range: visibleRange, total })">
      <div class="jobs-list-toolbar" aria-live="polite">
        <span>{{ t('jobs.pageCounter', { page, pageCount }) }}</span>
        <div class="jobs-pagination">
          <button class="secondary-button compact-button" type="button" :disabled="offset <= 0 || loading" @click="changePage(offset - limit)">
            {{ t('common.previous') }}
          </button>
          <button class="secondary-button compact-button" type="button" :disabled="offset + limit >= total || loading" @click="changePage(offset + limit)">
            {{ t('common.next') }}
          </button>
        </div>
      </div>

      <article
        v-for="item in items"
        :key="`${item.type}:${item.publicId}`"
        class="job-list-item"
        :class="{ active: item.statusGroup === 'active', failed: /failed|dead|denied/i.test(item.status) }"
      >
        <div class="job-item-main">
          <div class="notification-title-row">
            <RouterLink class="text-link jobs-title-link" :to="{ name: 'job-detail', params: { jobType: item.type, jobPublicId: item.publicId } }">
              {{ item.title }}
            </RouterLink>
            <StatusBadge :tone="statusTone(item.status)">
              {{ item.status }}
            </StatusBadge>
          </div>
          <p>{{ jobSummary(item) }}</p>
          <div class="notification-meta-row">
            <span>{{ jobTypeLabel(item.type) }}</span>
            <span>{{ requesterLabel(item) }}</span>
            <span>{{ t('jobs.createdAtValue', { value: formatDate(item.createdAt) }) }}</span>
            <span>{{ t('jobs.updatedAtValue', { value: formatDate(item.updatedAt) }) }}</span>
          </div>
        </div>

        <div class="job-item-actions">
          <RouterLink class="secondary-button compact-button link-button" :to="{ name: 'job-detail', params: { jobType: item.type, jobPublicId: item.publicId } }">
            {{ t('common.details') }}
          </RouterLink>
          <button class="secondary-button compact-button danger-button" type="button" :disabled="!item.canStop || stoppingId === `${item.type}:${item.publicId}`" @click="stopJob(item)">
            <Square :size="14" stroke-width="1.9" aria-hidden="true" />
            {{ t('jobs.stop') }}
          </button>
        </div>
      </article>
    </DataCard>

    <EmptyState
      v-else-if="!loading"
      :title="hasActiveFilters ? t('jobs.emptyFilteredTitle') : t('jobs.emptyTitle')"
      :message="hasActiveFilters ? t('jobs.emptyFilteredMessage') : t('jobs.emptyMessage')"
    >
      <template v-if="hasActiveFilters" #actions>
        <button class="secondary-button compact-button" type="button" @click="clearFilters">
          {{ t('common.clear') }}
        </button>
      </template>
    </EmptyState>
  </section>
</template>
