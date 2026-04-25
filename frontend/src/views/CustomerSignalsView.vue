<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'

import { toApiErrorMessage } from '../api/client'
import type { CustomerSignalBody } from '../api/generated/types.gen'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import { useCustomerSignalStore } from '../stores/customer-signals'
import { useTenantStore } from '../stores/tenants'

const sourceOptions = ['support', 'sales', 'customer_success', 'research', 'internal', 'other'] as const
const priorityOptions = ['low', 'medium', 'high', 'urgent'] as const
const statusOptions = ['new', 'triaged', 'planned', 'closed'] as const

const tenantStore = useTenantStore()
const signalStore = useCustomerSignalStore()

const customerName = ref('')
const title = ref('')
const body = ref('')
const source = ref<typeof sourceOptions[number]>('support')
const priority = ref<typeof priorityOptions[number]>('medium')
const status = ref<typeof statusOptions[number]>('new')
const actionErrorMessage = ref('')

const activeTenantLabel = computed(() => (
  tenantStore.activeTenant
    ? `${tenantStore.activeTenant.displayName} / ${tenantStore.activeTenant.slug}`
    : 'None'
))

const openCount = computed(() => signalStore.items.filter((item) => item.status !== 'closed').length)

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
    }
  },
  { immediate: true },
)

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

function sourceLabel(value: string) {
  return value.replaceAll('_', ' ')
}

function previewText(item: CustomerSignalBody) {
  return item.body || 'No details recorded.'
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
</script>

<template>
  <AdminAccessDenied
    v-if="signalStore.status === 'forbidden'"
    title="Customer Signal role required"
    message="この画面を使うには active tenant の customer_signal_user role が必要です。"
    role-label="customer_signal_user"
  />

  <section v-else class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">Customer Signals</span>
        <h2>Signals</h2>
      </div>
      <button
        class="secondary-button"
        :disabled="signalStore.status === 'loading' || !tenantStore.activeTenant"
        type="button"
        @click="signalStore.load()"
      >
        {{ signalStore.status === 'loading' ? 'Refreshing...' : 'Refresh' }}
      </button>
    </div>

    <dl class="metadata-grid">
      <div>
        <dt>Active tenant</dt>
        <dd>{{ activeTenantLabel }}</dd>
      </div>
      <div>
        <dt>Open signals</dt>
        <dd>{{ openCount }}</dd>
      </div>
    </dl>

    <p v-if="tenantStore.status === 'empty'" class="warning-message">
      Active tenant がありません。tenant membership を seed してから再ログインしてください。
    </p>
    <p v-if="tenantStore.status === 'error'" class="error-message">
      {{ tenantStore.errorMessage }}
    </p>
    <p v-if="actionErrorMessage || signalStore.errorMessage" class="error-message">
      {{ actionErrorMessage || signalStore.errorMessage }}
    </p>

    <form class="admin-form" @submit.prevent="createSignal">
      <label class="field">
        <span class="field-label">Customer</span>
        <input
          v-model="customerName"
          class="field-input"
          autocomplete="organization"
          maxlength="120"
          placeholder="Acme"
          required
          :disabled="!tenantStore.activeTenant || signalStore.creating"
        >
      </label>

      <label class="field">
        <span class="field-label">Title</span>
        <input
          v-model="title"
          class="field-input"
          autocomplete="off"
          maxlength="200"
          placeholder="Export CSV from reports"
          required
          :disabled="!tenantStore.activeTenant || signalStore.creating"
        >
      </label>

      <label class="field">
        <span class="field-label">Source</span>
        <select v-model="source" class="field-input" :disabled="signalStore.creating">
          <option v-for="item in sourceOptions" :key="item" :value="item">
            {{ sourceLabel(item) }}
          </option>
        </select>
      </label>

      <label class="field">
        <span class="field-label">Priority</span>
        <select v-model="priority" class="field-input" :disabled="signalStore.creating">
          <option v-for="item in priorityOptions" :key="item" :value="item">
            {{ item }}
          </option>
        </select>
      </label>

      <label class="field">
        <span class="field-label">Status</span>
        <select v-model="status" class="field-input" :disabled="signalStore.creating">
          <option v-for="item in statusOptions" :key="item" :value="item">
            {{ item }}
          </option>
        </select>
      </label>

      <label class="field form-span">
        <span class="field-label">Details</span>
        <textarea
          v-model="body"
          class="field-input textarea-input"
          maxlength="4000"
          placeholder="Customer asked for monthly report export."
          :disabled="!tenantStore.activeTenant || signalStore.creating"
        />
      </label>

      <div class="action-row form-span">
        <button class="primary-button" :disabled="!canCreate" type="submit">
          {{ signalStore.creating ? 'Adding...' : 'Add Signal' }}
        </button>
      </div>
    </form>

    <p v-if="signalStore.status === 'loading'" class="todo-loading">
      Loading customer signals...
    </p>

    <div v-else-if="signalStore.items.length > 0" class="signal-list">
      <article v-for="item in signalStore.items" :key="item.publicId" class="signal-item">
        <div class="signal-item-main">
          <div class="signal-title-row">
            <RouterLink class="text-link signal-title" :to="`/customer-signals/${item.publicId}`">
              {{ item.title }}
            </RouterLink>
            <span :class="['status-pill', item.status === 'closed' ? 'danger' : '']">
              {{ item.status }}
            </span>
          </div>
          <p class="signal-preview">
            {{ previewText(item) }}
          </p>
          <div class="signal-meta-row">
            <span>{{ item.customerName }}</span>
            <span>{{ sourceLabel(item.source) }}</span>
            <span>{{ item.priority }}</span>
            <span>Updated {{ formatDate(item.updatedAt) }}</span>
          </div>
        </div>
        <RouterLink class="secondary-button link-button compact-button" :to="`/customer-signals/${item.publicId}`">
          Open
        </RouterLink>
      </article>
    </div>

    <div v-else-if="signalStore.status === 'empty'" class="empty-state">
      <p>この tenant の Customer Signal はまだありません。</p>
    </div>
  </section>
</template>
