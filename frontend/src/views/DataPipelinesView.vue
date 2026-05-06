<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Plus, RefreshCw, Search, SlidersHorizontal, Workflow, X } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import { useDataPipelineStore } from '../stores/data-pipelines'
import { useTenantStore } from '../stores/tenants'

const store = useDataPipelineStore()
const tenantStore = useTenantStore()
const router = useRouter()
const route = useRoute()
const { d, t } = useI18n()

const newName = ref(t('dataPipelines.defaultPipelineName'))
const newDescription = ref('')
const createDialogOpen = ref(false)
const createDialogElement = ref<HTMLDialogElement | null>(null)
const query = ref('')
const status = ref('')
const publication = ref('all')
const runStatus = ref('')
const scheduleState = ref('all')
const sort = ref('updated_desc')
const cursor = ref('')
const limit = ref(25)
const knownStatusValues = new Set(['draft', 'pending', 'processing', 'completed', 'failed', 'skipped', 'ready', 'created', 'active', 'disabled', 'published', 'archived'])
const pipelineStatusOptions = ['draft', 'published']
const runStatusOptions = ['pending', 'processing', 'completed', 'failed', 'skipped']
const scheduleStateOptions = ['enabled', 'disabled', 'none']
const sortOptions = ['updated_desc', 'updated_asc', 'created_desc', 'created_asc', 'name_asc', 'name_desc', 'latest_run_desc']

const activeTenantLabel = computed(() => (
  tenantStore.activeTenant
    ? `${tenantStore.activeTenant.displayName} / ${tenantStore.activeTenant.slug}`
    : t('dataPipelines.noActiveTenant')
))
const hasActiveFilters = computed(() => Boolean(
  query.value.trim()
  || status.value
  || publication.value !== 'all'
  || runStatus.value
  || scheduleState.value !== 'all'
  || sort.value !== 'updated_desc',
))

onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
})

watch(
  () => tenantStore.activeTenant?.slug,
  async (slug) => {
    store.reset()
    if (slug) {
      await loadList()
    }
  },
  { immediate: true },
)

watch(() => route.query, async () => {
  syncFromRoute()
  if (tenantStore.activeTenant?.slug) {
    await loadList()
  }
})

watch(createDialogOpen, (open) => {
  const dialog = createDialogElement.value
  if (!dialog) return
  if (open && !dialog.open) {
    dialog.showModal()
  } else if (!open && dialog.open) {
    dialog.close()
  }
})

syncFromRoute()

async function createPipeline() {
  try {
    const item = await store.create(newName.value.trim(), newDescription.value.trim())
    newName.value = t('dataPipelines.defaultPipelineName')
    newDescription.value = ''
    createDialogOpen.value = false
    await router.push({ name: 'data-pipeline-detail', params: { pipelinePublicId: item.publicId } })
  } catch {
    // The store exposes the API error next to the create form.
  }
}

async function refreshList() {
  await loadList()
}

async function loadList() {
  await store.load(false, listParams())
}

async function applyFilters() {
  cursor.value = ''
  await router.push({ name: 'data-pipelines', query: routeQuery() })
}

async function clearFilters() {
  query.value = ''
  status.value = ''
  publication.value = 'all'
  runStatus.value = ''
  scheduleState.value = 'all'
  sort.value = 'updated_desc'
  cursor.value = ''
  await router.push({ name: 'data-pipelines', query: routeQuery() })
}

async function nextPage() {
  if (!store.nextCursor) return
  cursor.value = store.nextCursor
  await router.push({ name: 'data-pipelines', query: routeQuery() })
}

function openCreateDialog() {
  store.errorMessage = ''
  createDialogOpen.value = true
}

function closeCreateDialog() {
  if (store.actionLoading) return
  createDialogOpen.value = false
}

function syncFromRoute() {
  query.value = String(route.query.q ?? '')
  status.value = String(route.query.status ?? '')
  publication.value = String(route.query.publication ?? 'all')
  runStatus.value = String(route.query.runStatus ?? '')
  scheduleState.value = String(route.query.scheduleState ?? 'all')
  sort.value = String(route.query.sort ?? 'updated_desc')
  cursor.value = String(route.query.cursor ?? '')
  limit.value = Number(route.query.limit ?? 25)
}

function listParams() {
  return {
    q: query.value,
    status: status.value,
    publication: publication.value,
    runStatus: runStatus.value,
    scheduleState: scheduleState.value,
    sort: sort.value,
    cursor: cursor.value,
    limit: limit.value,
  }
}

function routeQuery() {
  const out: Record<string, string> = {
    limit: String(limit.value),
  }
  if (query.value.trim()) out.q = query.value.trim()
  if (status.value) out.status = status.value
  if (publication.value !== 'all') out.publication = publication.value
  if (runStatus.value) out.runStatus = runStatus.value
  if (scheduleState.value !== 'all') out.scheduleState = scheduleState.value
  if (sort.value !== 'updated_desc') out.sort = sort.value
  if (cursor.value) out.cursor = cursor.value
  return out
}

function statusClass(status = '') {
  if (['published', 'completed', 'active'].includes(status)) return 'success'
  if (['failed', 'archived'].includes(status)) return 'danger'
  return 'warning'
}

function statusLabel(status = '') {
  return knownStatusValues.has(status) ? t(`dataPipelines.statusValue.${status}`) : status
}

function sortLabel(value: string) {
  return t(`dataPipelines.sortOptions.${value}`)
}

function publicationLabel(value: string) {
  return t(`dataPipelines.publicationOptions.${value}`)
}

function scheduleStateLabel(value = '') {
  return value ? t(`dataPipelines.scheduleStateOptions.${value}`) : '-'
}

function formatDate(value?: string | null) {
  return value ? d(new Date(value), 'long') : '-'
}
</script>

<template>
  <section class="data-pipeline-page">
    <header class="page-header">
      <div class="page-header-copy">
        <span class="status-pill">
          <Workflow :size="14" stroke-width="1.9" aria-hidden="true" />
          {{ activeTenantLabel }}
        </span>
        <h1>{{ t('routes.dataPipelines') }}</h1>
      </div>
      <div class="page-header-actions">
        <button class="primary-button" type="button" @click="openCreateDialog">
          <Plus :size="16" stroke-width="1.9" aria-hidden="true" />
          {{ t('dataPipelines.newPipeline') }}
        </button>
        <button class="secondary-button" type="button" :disabled="store.status === 'loading'" @click="refreshList">
          <RefreshCw :size="16" stroke-width="1.9" aria-hidden="true" />
          {{ t('common.refresh') }}
        </button>
      </div>
    </header>

    <AdminAccessDenied
      v-if="store.status === 'forbidden'"
      :title="t('dataPipelines.accessRequiredTitle')"
      role-label="data_pipeline_user"
      :message="t('dataPipelines.accessRequiredMessage')"
    />

    <div v-else class="data-pipeline-list-layout">
      <main class="data-pipeline-list-main">
        <section class="data-pipeline-list-panel">
          <header class="panel-header">
            <h2>{{ t('dataPipelines.pipelines') }}</h2>
            <span class="status-pill">{{ store.items.length }}</span>
          </header>

          <form class="notification-command-bar data-pipeline-list-command-bar" role="search" @submit.prevent="applyFilters">
            <div class="notification-search-box">
              <Search :size="18" stroke-width="1.9" aria-hidden="true" />
              <label class="sr-only" for="data-pipeline-search-query">{{ t('dataPipelines.searchPlaceholder') }}</label>
              <input
                id="data-pipeline-search-query"
                v-model="query"
                type="search"
                autocomplete="off"
                :placeholder="t('dataPipelines.searchPlaceholder')"
                :disabled="store.status === 'loading'"
              >
              <button class="secondary-button compact-button" type="submit" :disabled="store.status === 'loading'">
                {{ t('common.search') }}
              </button>
            </div>

            <div class="notification-filter-row" :aria-label="t('dataPipelines.filters')">
              <label class="notification-filter-chip">
                <SlidersHorizontal :size="15" stroke-width="1.9" aria-hidden="true" />
                <span>{{ t('dataPipelines.status') }}</span>
                <select v-model="status" :disabled="store.status === 'loading'" @change="applyFilters">
                  <option value="">{{ t('common.all') }}</option>
                  <option v-for="option in pipelineStatusOptions" :key="option" :value="option">{{ statusLabel(option) }}</option>
                </select>
              </label>

              <label class="notification-filter-chip">
                <span>{{ t('dataPipelines.publication') }}</span>
                <select v-model="publication" :disabled="store.status === 'loading'" @change="applyFilters">
                  <option value="all">{{ publicationLabel('all') }}</option>
                  <option value="published">{{ publicationLabel('published') }}</option>
                  <option value="unpublished">{{ publicationLabel('unpublished') }}</option>
                </select>
              </label>

              <label class="notification-filter-chip">
                <span>{{ t('dataPipelines.latestRun') }}</span>
                <select v-model="runStatus" :disabled="store.status === 'loading'" @change="applyFilters">
                  <option value="">{{ t('common.all') }}</option>
                  <option v-for="option in runStatusOptions" :key="option" :value="option">{{ statusLabel(option) }}</option>
                </select>
              </label>

              <label class="notification-filter-chip">
                <span>{{ t('dataPipelines.scheduleState') }}</span>
                <select v-model="scheduleState" :disabled="store.status === 'loading'" @change="applyFilters">
                  <option value="all">{{ t('common.all') }}</option>
                  <option v-for="option in scheduleStateOptions" :key="option" :value="option">{{ scheduleStateLabel(option) }}</option>
                </select>
              </label>

              <label class="notification-filter-chip">
                <span>{{ t('dataPipelines.sort') }}</span>
                <select v-model="sort" :disabled="store.status === 'loading'" @change="applyFilters">
                  <option v-for="option in sortOptions" :key="option" :value="option">{{ sortLabel(option) }}</option>
                </select>
              </label>

              <button class="secondary-button compact-button" type="button" :disabled="store.status === 'loading' || !hasActiveFilters" @click="clearFilters">
                <X :size="15" stroke-width="1.9" aria-hidden="true" />
                {{ t('common.clear') }}
              </button>
            </div>
          </form>

          <div v-if="store.items.length > 0" class="dataset-list">
            <RouterLink
              v-for="item in store.items"
              :key="item.publicId"
              class="dataset-row data-pipeline-list-row"
              :to="{ name: 'data-pipeline-detail', params: { pipelinePublicId: item.publicId } }"
            >
              <Workflow :size="18" stroke-width="1.8" aria-hidden="true" />
              <span>
                <strong>{{ item.name }}</strong>
                <small>{{ item.description || item.publicId }}</small>
                <small>{{ t('common.updated') }}: {{ formatDate(item.updatedAt) }}</small>
                <small>{{ t('dataPipelines.latestRun') }}: {{ item.latestRunStatus ? `${statusLabel(item.latestRunStatus)} / ${formatDate(item.latestRunAt)}` : '-' }}</small>
                <small>{{ t('dataPipelines.scheduleState') }}: {{ scheduleStateLabel(item.scheduleState) }} / {{ t('dataPipelines.nextRunAt') }}: {{ formatDate(item.nextRunAt) }}</small>
              </span>
              <span class="data-pipeline-list-badges">
                <span class="status-pill" :class="statusClass(item.status)">{{ statusLabel(item.status) }}</span>
                <span v-if="item.status !== 'published'" class="status-pill">{{ item.publishedVersionId ? publicationLabel('published') : publicationLabel('unpublished') }}</span>
              </span>
            </RouterLink>

            <div class="jobs-list-toolbar data-pipeline-pagination" aria-live="polite">
              <span>{{ cursor ? t('dataPipelines.pagedResults') : t('dataPipelines.firstPageResults') }}</span>
              <button class="secondary-button compact-button" type="button" :disabled="!store.nextCursor || store.status === 'loading'" @click="nextPage">
                {{ t('common.next') }}
              </button>
            </div>
          </div>

          <div v-else class="empty-state">
            <Workflow :size="28" stroke-width="1.8" aria-hidden="true" />
            <h2>{{ hasActiveFilters ? t('dataPipelines.noFilteredPipelines') : t('dataPipelines.noPipelines') }}</h2>
            <button v-if="hasActiveFilters" class="secondary-button compact-button" type="button" @click="clearFilters">
              {{ t('common.clear') }}
            </button>
          </div>
        </section>
      </main>
    </div>

    <dialog ref="createDialogElement" class="data-pipeline-create-dialog" @close="createDialogOpen = false">
      <form class="data-pipeline-create-dialog-panel" method="dialog" @submit.prevent="createPipeline">
        <header class="panel-header">
          <h2>{{ t('dataPipelines.newPipeline') }}</h2>
          <button class="secondary-button compact-button" type="button" :disabled="store.actionLoading" @click="closeCreateDialog">
            <X :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('common.close') }}
          </button>
        </header>
        <label class="field">
          <span>{{ t('dataPipelines.name') }}</span>
          <input v-model="newName" autocomplete="off">
        </label>
        <label class="field">
          <span>{{ t('dataPipelines.description') }}</span>
          <textarea v-model="newDescription" rows="4" />
        </label>
        <div class="data-pipeline-dialog-actions">
          <button class="secondary-button" type="button" :disabled="store.actionLoading" @click="closeCreateDialog">
            {{ t('common.cancel') }}
          </button>
          <button class="primary-button" type="submit" :disabled="store.actionLoading || !newName.trim()">
            <Plus :size="16" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.create') }}
          </button>
        </div>
        <p v-if="store.errorMessage" class="form-error">{{ store.errorMessage }}</p>
      </form>
    </dialog>
  </section>
</template>
