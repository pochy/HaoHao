<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { toApiErrorMessage } from '../api/client'
import type { CustomerSignalBody } from '../api/generated/types.gen'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import ConfirmActionDialog from '../components/ConfirmActionDialog.vue'
import DataCard from '../components/DataCard.vue'
import EmptyState from '../components/EmptyState.vue'
import MetricTile from '../components/MetricTile.vue'
import PageHeader from '../components/PageHeader.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { useCustomerSignalStore } from '../stores/customer-signals'
import { useTenantStore } from '../stores/tenants'

const sourceOptions = ['support', 'sales', 'customer_success', 'research', 'internal', 'other'] as const
const priorityOptions = ['low', 'medium', 'high', 'urgent'] as const
const statusOptions = ['new', 'triaged', 'planned', 'closed'] as const

const tenantStore = useTenantStore()
const signalStore = useCustomerSignalStore()
const { d, t } = useI18n()

const customerName = ref('')
const title = ref('')
const body = ref('')
const source = ref<typeof sourceOptions[number]>('support')
const priority = ref<typeof priorityOptions[number]>('medium')
const status = ref<typeof statusOptions[number]>('new')
const actionErrorMessage = ref('')
const savedFilterName = ref('')
const pendingFilterDelete = ref<{ publicId: string, name: string } | null>(null)

const activeTenantLabel = computed(() => (
  tenantStore.activeTenant
    ? `${tenantStore.activeTenant.displayName} / ${tenantStore.activeTenant.slug}`
    : t('common.none')
))

const openCount = computed(() => signalStore.items.filter((item) => item.status !== 'closed').length)
const urgentCount = computed(() => signalStore.items.filter((item) => item.priority === 'urgent').length)
const savedFilterCount = computed(() => signalStore.savedFilters.length)

const canCreate = computed(() => (
  Boolean(tenantStore.activeTenant) &&
  customerName.value.trim() !== '' &&
  title.value.trim() !== '' &&
  !signalStore.creating &&
  signalStore.status !== 'loading'
))

onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
})

watch(
  () => tenantStore.activeTenant?.slug,
  async (slug) => {
    actionErrorMessage.value = ''
    signalStore.reset()

    if (slug) {
      await signalStore.load()
      await signalStore.loadSavedFilters()
    }
  },
  { immediate: true },
)

function formatDate(value: string) {
  return d(new Date(value), 'long')
}

function sourceLabel(value: string) {
  return t(`signals.options.source.${value}`)
}

function priorityLabel(value: string) {
  return t(`signals.options.priority.${value}`)
}

function statusLabel(value: string) {
  return t(`signals.options.status.${value}`)
}

function previewText(item: CustomerSignalBody) {
  return item.body || t('signals.noDetails')
}

async function createSignal() {
  if (!canCreate.value) {
    return
  }

  actionErrorMessage.value = ''

  try {
    await signalStore.create({
      customerName: customerName.value.trim(),
      title: title.value.trim(),
      body: body.value.trim(),
      source: source.value,
      priority: priority.value,
      status: status.value,
    })
    customerName.value = ''
    title.value = ''
    body.value = ''
    source.value = 'support'
    priority.value = 'medium'
    status.value = 'new'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function applySearch() {
  actionErrorMessage.value = ''
  await signalStore.load()
}

async function loadMore() {
  if (!signalStore.nextCursor) {
    return
  }
  await signalStore.load({ cursor: signalStore.nextCursor })
}

async function saveFilter() {
  if (savedFilterName.value.trim() === '') {
    return
  }
  actionErrorMessage.value = ''
  try {
    await signalStore.saveCurrentFilter(savedFilterName.value.trim())
    savedFilterName.value = ''
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

function requestDeleteSavedFilter(filter: { publicId: string, name: string }) {
  pendingFilterDelete.value = filter
}

function cancelDeleteSavedFilter() {
  pendingFilterDelete.value = null
}

async function confirmDeleteSavedFilter() {
  if (!pendingFilterDelete.value) {
    return
  }
  const target = pendingFilterDelete.value
  pendingFilterDelete.value = null
  await signalStore.deleteSavedFilter(target.publicId)
}

async function applySavedFilter(filter: { query?: string, filters?: Record<string, unknown> }) {
  signalStore.query = filter.query ?? ''
  signalStore.filters.status = typeof filter.filters?.status === 'string' ? filter.filters.status : ''
  signalStore.filters.priority = typeof filter.filters?.priority === 'string' ? filter.filters.priority : ''
  signalStore.filters.source = typeof filter.filters?.source === 'string' ? filter.filters.source : ''
  await signalStore.load()
}
</script>

<template>
  <AdminAccessDenied
    v-if="signalStore.status === 'forbidden'"
    :title="t('access.signalTitle')"
    :message="t('access.signalMessage')"
    role-label="customer_signal_user"
  />

  <section v-else class="stack">
    <PageHeader
      :eyebrow="t('signals.eyebrow')"
      :title="t('signals.title')"
      :description="t('signals.description')"
    >
      <template #actions>
      <button
        class="secondary-button"
        :disabled="signalStore.status === 'loading' || !tenantStore.activeTenant"
        type="button"
        @click="signalStore.load()"
      >
        {{ signalStore.status === 'loading' ? t('common.refreshing') : t('common.refresh') }}
      </button>
      </template>
    </PageHeader>

    <div class="metric-grid">
      <MetricTile :label="t('common.activeTenant')" :value="activeTenantLabel" :hint="t('signals.activeTenantHint')" />
      <MetricTile :label="t('signals.openSignals')" :value="openCount" :hint="t('signals.openSignalsHint')" />
      <MetricTile :label="t('signals.urgent')" :value="urgentCount" :hint="t('signals.urgentHint')" />
      <MetricTile :label="t('signals.savedFilters')" :value="savedFilterCount" :hint="t('signals.savedFiltersHint')" />
    </div>

    <p v-if="tenantStore.status === 'empty'" class="warning-message">
      {{ t('signals.noTenantMessage') }}
    </p>
    <p v-if="tenantStore.status === 'error'" class="error-message">
      {{ tenantStore.errorMessage }}
    </p>
    <p v-if="actionErrorMessage || signalStore.errorMessage" class="error-message">
      {{ actionErrorMessage || signalStore.errorMessage }}
    </p>

    <DataCard :title="t('signals.searchCardTitle')" :subtitle="t('signals.searchCardSubtitle')">
    <form class="admin-form" @submit.prevent="applySearch">
      <label class="field">
        <span class="field-label">{{ t('common.search') }}</span>
        <input
          v-model="signalStore.query"
          class="field-input"
          autocomplete="off"
          :placeholder="t('signals.searchPlaceholder')"
        >
      </label>

      <label class="field">
        <span class="field-label">{{ t('signals.statusFilter') }}</span>
        <select v-model="signalStore.filters.status" class="field-input">
          <option value="">{{ t('common.any') }}</option>
          <option v-for="item in statusOptions" :key="item" :value="item">
            {{ statusLabel(item) }}
          </option>
        </select>
      </label>

      <label class="field">
        <span class="field-label">{{ t('signals.priorityFilter') }}</span>
        <select v-model="signalStore.filters.priority" class="field-input">
          <option value="">{{ t('common.any') }}</option>
          <option v-for="item in priorityOptions" :key="item" :value="item">
            {{ priorityLabel(item) }}
          </option>
        </select>
      </label>

      <label class="field">
        <span class="field-label">{{ t('signals.sourceFilter') }}</span>
        <select v-model="signalStore.filters.source" class="field-input">
          <option value="">{{ t('common.any') }}</option>
          <option v-for="item in sourceOptions" :key="item" :value="item">
            {{ sourceLabel(item) }}
          </option>
        </select>
      </label>

      <div class="action-row form-span">
        <button
          class="primary-button"
          :disabled="signalStore.status === 'loading'"
          type="submit"
          :aria-label="t('signals.applySearch')"
        >
          {{ t('common.apply') }}
        </button>
        <input
          v-model="savedFilterName"
          class="field-input inline-input"
          autocomplete="off"
          :placeholder="t('common.filterName')"
        >
        <button class="secondary-button" :disabled="savedFilterName.trim() === ''" type="button" @click="saveFilter">
          {{ t('signals.saveFilter') }}
        </button>
      </div>
    </form>
    </DataCard>

    <DataCard v-if="signalStore.savedFilters.length > 0" :title="t('signals.savedFilters')">
      <article v-for="filter in signalStore.savedFilters" :key="filter.publicId" class="list-item">
        <div>
          <strong>{{ filter.name }}</strong>
          <span class="cell-subtle">{{ filter.query || t('signals.noSearchText') }}</span>
        </div>
        <div class="action-row">
          <button
            class="secondary-button compact-button"
            type="button"
            :aria-label="t('signals.applySavedFilter', { name: filter.name })"
            @click="applySavedFilter(filter)"
          >
            {{ t('common.apply') }}
          </button>
          <button class="secondary-button danger-button compact-button" type="button" @click="requestDeleteSavedFilter(filter)">
            {{ t('common.delete') }}
          </button>
        </div>
      </article>
    </DataCard>

    <DataCard :title="t('signals.addCardTitle')" :subtitle="t('signals.addCardSubtitle')">
    <form class="admin-form" @submit.prevent="createSignal">
      <label class="field">
        <span class="field-label">{{ t('signals.customer') }}</span>
        <input
          v-model="customerName"
          class="field-input"
          autocomplete="organization"
          maxlength="120"
          :placeholder="t('signals.customerPlaceholder')"
          required
          :disabled="!tenantStore.activeTenant || signalStore.creating"
        >
      </label>

      <label class="field">
        <span class="field-label">{{ t('common.title') }}</span>
        <input
          v-model="title"
          class="field-input"
          autocomplete="off"
          maxlength="200"
          :placeholder="t('signals.titlePlaceholder')"
          required
          :disabled="!tenantStore.activeTenant || signalStore.creating"
        >
      </label>

      <label class="field">
        <span class="field-label">{{ t('signals.source') }}</span>
        <select v-model="source" class="field-input" :disabled="signalStore.creating">
          <option v-for="item in sourceOptions" :key="item" :value="item">
            {{ sourceLabel(item) }}
          </option>
        </select>
      </label>

      <label class="field">
        <span class="field-label">{{ t('signals.priority') }}</span>
        <select v-model="priority" class="field-input" :disabled="signalStore.creating">
          <option v-for="item in priorityOptions" :key="item" :value="item">
            {{ priorityLabel(item) }}
          </option>
        </select>
      </label>

      <label class="field">
        <span class="field-label">{{ t('signals.status') }}</span>
        <select v-model="status" class="field-input" :disabled="signalStore.creating">
          <option v-for="item in statusOptions" :key="item" :value="item">
            {{ statusLabel(item) }}
          </option>
        </select>
      </label>

      <label class="field form-span">
        <span class="field-label">{{ t('signals.details') }}</span>
        <textarea
          v-model="body"
          class="field-input textarea-input"
          maxlength="4000"
          :placeholder="t('signals.detailsPlaceholder')"
          :disabled="!tenantStore.activeTenant || signalStore.creating"
        />
      </label>

      <div class="action-row form-span">
        <button class="primary-button" :disabled="!canCreate" type="submit">
          {{ signalStore.creating ? t('common.adding') : t('signals.addSignal') }}
        </button>
      </div>
    </form>
    </DataCard>

    <p v-if="signalStore.status === 'loading'" class="todo-loading">
      {{ t('signals.loading') }}
    </p>

    <DataCard v-else-if="signalStore.items.length > 0" :title="t('signals.listTitle')">
      <article v-for="item in signalStore.items" :key="item.publicId" class="signal-item">
        <div class="signal-item-main">
          <div class="signal-title-row">
            <RouterLink class="text-link signal-title" :to="`/customer-signals/${item.publicId}`">
              {{ item.title }}
            </RouterLink>
            <StatusBadge :tone="item.status === 'closed' ? 'danger' : 'neutral'">
              {{ statusLabel(item.status) }}
            </StatusBadge>
          </div>
          <p class="signal-preview">
            {{ previewText(item) }}
          </p>
          <div class="signal-meta-row">
            <span>{{ item.customerName }}</span>
            <span>{{ sourceLabel(item.source) }}</span>
            <span>{{ priorityLabel(item.priority) }}</span>
            <span>{{ t('signals.updatedAt', { date: formatDate(item.updatedAt) }) }}</span>
          </div>
        </div>
        <RouterLink class="secondary-button link-button compact-button" :to="`/customer-signals/${item.publicId}`">
          {{ t('common.open') }}
        </RouterLink>
      </article>
      <div v-if="signalStore.nextCursor" class="action-row">
        <button class="secondary-button" type="button" @click="loadMore">
          {{ t('signals.loadMore') }}
        </button>
      </div>
    </DataCard>

    <EmptyState
      v-else-if="signalStore.status === 'empty'"
      :title="t('signals.emptyTitle')"
      :message="t('signals.emptyMessage')"
    />

    <ConfirmActionDialog
      :open="pendingFilterDelete !== null"
      :title="t('signals.deleteSavedFilterTitle')"
      :message="t('signals.deleteSavedFilterMessage', { name: pendingFilterDelete?.name ?? t('signals.deleteSavedFilterFallback') })"
      :confirm-label="t('common.delete')"
      @cancel="cancelDeleteSavedFilter"
      @confirm="confirmDeleteSavedFilter"
    />
  </section>
</template>
