<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { toApiErrorMessage } from '../api/client'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import ConfirmActionDialog from '../components/ConfirmActionDialog.vue'
import { useCustomerSignalStore } from '../stores/customer-signals'
import { useTenantStore } from '../stores/tenants'

const sourceOptions = ['support', 'sales', 'customer_success', 'research', 'internal', 'other'] as const
const priorityOptions = ['low', 'medium', 'high', 'urgent'] as const
const statusOptions = ['new', 'triaged', 'planned', 'closed'] as const

const route = useRoute()
const router = useRouter()
const tenantStore = useTenantStore()
const signalStore = useCustomerSignalStore()

const customerName = ref('')
const title = ref('')
const body = ref('')
const source = ref<typeof sourceOptions[number]>('support')
const priority = ref<typeof priorityOptions[number]>('medium')
const status = ref<typeof statusOptions[number]>('new')
const message = ref('')
const errorMessage = ref('')
const confirmingDelete = ref(false)

const signalPublicId = computed(() => {
  const raw = Array.isArray(route.params.signalPublicId)
    ? route.params.signalPublicId[0]
    : route.params.signalPublicId
  return raw ?? ''
})

const signal = computed(() => signalStore.current)
const activeTenantLabel = computed(() => (
  tenantStore.activeTenant
    ? `${tenantStore.activeTenant.displayName} / ${tenantStore.activeTenant.slug}`
    : 'None'
))

const canSave = computed(() => (
  Boolean(signal.value) &&
  customerName.value.trim() !== '' &&
  title.value.trim() !== '' &&
  !signalStore.updating
))

const deleteMessage = computed(() => (
  `Customer Signal "${signal.value?.title ?? signalPublicId.value}" を削除します。record は soft delete され、audit event は残ります。`
))

onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
})

watch(
  () => [tenantStore.activeTenant?.slug, signalPublicId.value],
  async ([slug]) => {
    message.value = ''
    errorMessage.value = ''
    signalStore.reset()

    if (slug && signalPublicId.value) {
      await signalStore.loadOne(signalPublicId.value)
      syncForm()
    }
  },
  { immediate: true },
)

watch(
  () => signalStore.current,
  () => syncForm(),
)

function syncForm() {
  if (!signalStore.current) {
    customerName.value = ''
    title.value = ''
    body.value = ''
    source.value = 'support'
    priority.value = 'medium'
    status.value = 'new'
    return
  }

  customerName.value = signalStore.current.customerName
  title.value = signalStore.current.title
  body.value = signalStore.current.body
  source.value = signalStore.current.source
  priority.value = signalStore.current.priority
  status.value = signalStore.current.status
}

function formatDate(value?: string) {
  if (!value) {
    return 'Never'
  }

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

function sourceLabel(value: string) {
  return value.replaceAll('_', ' ')
}

async function saveSignal() {
  if (!signal.value || !canSave.value) {
    return
  }

  message.value = ''
  errorMessage.value = ''

  try {
    await signalStore.update(signal.value.publicId, {
      customerName: customerName.value.trim(),
      title: title.value.trim(),
      body: body.value.trim(),
      source: source.value,
      priority: priority.value,
      status: status.value,
    })
    message.value = 'Customer Signal を更新しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

function askDelete() {
  confirmingDelete.value = true
}

function cancelDelete() {
  confirmingDelete.value = false
}

async function confirmDelete() {
  if (!signal.value) {
    confirmingDelete.value = false
    return
  }

  const publicId = signal.value.publicId
  confirmingDelete.value = false
  message.value = ''
  errorMessage.value = ''

  try {
    await signalStore.remove(publicId)
    await router.push('/customer-signals')
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
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
        <h2>Signal Detail</h2>
      </div>
      <RouterLink class="secondary-button link-button" to="/customer-signals">
        Back
      </RouterLink>
    </div>

    <p v-if="signalStore.status === 'loading'">
      Loading customer signal...
    </p>
    <p v-if="errorMessage || signalStore.errorMessage" class="error-message">
      {{ errorMessage || signalStore.errorMessage }}
    </p>
    <p v-if="message" class="notice-message">
      {{ message }}
    </p>

    <template v-if="signal">
      <dl class="metadata-grid">
        <div>
          <dt>Active tenant</dt>
          <dd>{{ activeTenantLabel }}</dd>
        </div>
        <div>
          <dt>Public ID</dt>
          <dd>{{ signal.publicId }}</dd>
        </div>
        <div>
          <dt>Created</dt>
          <dd>{{ formatDate(signal.createdAt) }}</dd>
        </div>
        <div>
          <dt>Updated</dt>
          <dd>{{ formatDate(signal.updatedAt) }}</dd>
        </div>
      </dl>

      <form class="admin-form" @submit.prevent="saveSignal">
        <label class="field">
          <span class="field-label">Customer</span>
          <input v-model="customerName" class="field-input" autocomplete="organization" maxlength="120" required>
        </label>

        <label class="field">
          <span class="field-label">Title</span>
          <input v-model="title" class="field-input" autocomplete="off" maxlength="200" required>
        </label>

        <label class="field">
          <span class="field-label">Source</span>
          <select v-model="source" class="field-input">
            <option v-for="item in sourceOptions" :key="item" :value="item">
              {{ sourceLabel(item) }}
            </option>
          </select>
        </label>

        <label class="field">
          <span class="field-label">Priority</span>
          <select v-model="priority" class="field-input">
            <option v-for="item in priorityOptions" :key="item" :value="item">
              {{ item }}
            </option>
          </select>
        </label>

        <label class="field">
          <span class="field-label">Status</span>
          <select v-model="status" class="field-input">
            <option v-for="item in statusOptions" :key="item" :value="item">
              {{ item }}
            </option>
          </select>
        </label>

        <label class="field form-span">
          <span class="field-label">Details</span>
          <textarea v-model="body" class="field-input textarea-input signal-detail-body" maxlength="4000" />
        </label>

        <div class="action-row form-span">
          <button class="primary-button" :disabled="!canSave" type="submit">
            {{ signalStore.updating ? 'Saving...' : 'Save' }}
          </button>
          <button
            class="secondary-button danger-button"
            :disabled="signalStore.deletingPublicId === signal.publicId"
            type="button"
            @click="askDelete"
          >
            {{ signalStore.deletingPublicId === signal.publicId ? 'Deleting...' : 'Delete' }}
          </button>
        </div>
      </form>
    </template>

    <div v-else-if="signalStore.status === 'error'" class="empty-state">
      <p>Customer Signal を読み込めませんでした。</p>
    </div>
  </section>

  <ConfirmActionDialog
    :open="confirmingDelete"
    title="Delete customer signal"
    :message="deleteMessage"
    confirm-label="Delete"
    @cancel="cancelDelete"
    @confirm="confirmDelete"
  />
</template>
