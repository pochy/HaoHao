<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, RefreshCw, Workflow } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import { useDataPipelineStore } from '../stores/data-pipelines'
import { useTenantStore } from '../stores/tenants'

const store = useDataPipelineStore()
const tenantStore = useTenantStore()
const router = useRouter()
const { d, t } = useI18n()

const newName = ref(t('dataPipelines.defaultPipelineName'))
const newDescription = ref('')
const knownStatusValues = new Set(['pending', 'processing', 'completed', 'failed', 'ready', 'created', 'active', 'disabled', 'published', 'archived'])

const activeTenantLabel = computed(() => (
  tenantStore.activeTenant
    ? `${tenantStore.activeTenant.displayName} / ${tenantStore.activeTenant.slug}`
    : t('dataPipelines.noActiveTenant')
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
      await store.load(false)
    }
  },
  { immediate: true },
)

async function createPipeline() {
  try {
    const item = await store.create(newName.value.trim(), newDescription.value.trim())
    newName.value = t('dataPipelines.defaultPipelineName')
    newDescription.value = ''
    await router.push({ name: 'data-pipeline-detail', params: { pipelinePublicId: item.publicId } })
  } catch {
    // The store exposes the API error next to the create form.
  }
}

async function refreshList() {
  await store.load(false)
}

function statusClass(status = '') {
  if (['published', 'completed', 'active'].includes(status)) return 'success'
  if (['failed', 'archived'].includes(status)) return 'danger'
  return 'warning'
}

function statusLabel(status = '') {
  return knownStatusValues.has(status) ? t(`dataPipelines.statusValue.${status}`) : status
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
      <aside class="data-pipeline-sidebar data-pipeline-create-panel">
        <section class="sidebar-panel-section">
          <h2>{{ t('dataPipelines.newPipeline') }}</h2>
          <label class="field">
            <span>{{ t('dataPipelines.name') }}</span>
            <input v-model="newName" autocomplete="off">
          </label>
          <label class="field">
            <span>{{ t('dataPipelines.description') }}</span>
            <textarea v-model="newDescription" rows="4" />
          </label>
          <button class="primary-button full-width" type="button" :disabled="store.actionLoading || !newName.trim()" @click="createPipeline">
            <Plus :size="16" stroke-width="1.9" aria-hidden="true" />
            {{ t('dataPipelines.create') }}
          </button>
          <p v-if="store.errorMessage" class="form-error">{{ store.errorMessage }}</p>
        </section>
      </aside>

      <main class="data-pipeline-list-main">
        <section class="data-pipeline-list-panel">
          <header class="panel-header">
            <h2>{{ t('dataPipelines.pipelines') }}</h2>
            <span class="status-pill">{{ store.items.length }}</span>
          </header>

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
              </span>
              <span class="status-pill" :class="statusClass(item.status)">{{ statusLabel(item.status) }}</span>
            </RouterLink>
          </div>

          <div v-else class="empty-state">
            <Workflow :size="28" stroke-width="1.8" aria-hidden="true" />
            <h2>{{ t('dataPipelines.noPipelines') }}</h2>
          </div>
        </section>
      </main>
    </div>
  </section>
</template>
