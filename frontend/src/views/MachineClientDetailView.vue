<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { toApiErrorMessage } from '../api/client'
import type { MachineClientBody, TenantBody } from '../api/generated/types.gen'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import { useMachineClientStore } from '../stores/machine-clients'
import { useTenantStore } from '../stores/tenants'

const route = useRoute()
const store = useMachineClientStore()
const tenantStore = useTenantStore()

const provider = ref('')
const providerClientId = ref('')
const displayName = ref('')
const defaultTenantId = ref('')
const allowedScopes = ref('')
const active = ref(true)
const message = ref('')
const errorMessage = ref('')

const clientId = computed(() => {
  const raw = Array.isArray(route.params.id) ? route.params.id[0] : route.params.id
  const parsed = Number(raw)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0
})

const tenantOptions = computed<TenantBody[]>(() => {
  const currentTenant = store.current?.defaultTenant
  if (!currentTenant || tenantStore.items.some((tenant) => tenant.id === currentTenant.id)) {
    return tenantStore.items
  }

  return [currentTenant, ...tenantStore.items]
})

const providerClientIdChanged = computed(() => (
  Boolean(store.current) &&
  providerClientId.value.trim() !== store.current?.providerClientId
))

const canSubmit = computed(() => (
  clientId.value > 0 &&
  providerClientId.value.trim() !== '' &&
  displayName.value.trim() !== '' &&
  !store.saving
))

onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
  await loadCurrent()
})

watch(
  () => route.params.id,
  async () => {
    await loadCurrent()
  },
)

watch(
  () => store.current,
  (current) => syncForm(current),
)

async function loadCurrent() {
  message.value = ''
  errorMessage.value = ''
  if (clientId.value === 0) {
    errorMessage.value = 'Invalid machine client ID.'
    return
  }

  await store.loadOne(clientId.value)
}

function syncForm(current: MachineClientBody | null) {
  if (!current) {
    provider.value = ''
    providerClientId.value = ''
    displayName.value = ''
    defaultTenantId.value = ''
    allowedScopes.value = ''
    active.value = true
    return
  }

  provider.value = current.provider
  providerClientId.value = current.providerClientId
  displayName.value = current.displayName
  defaultTenantId.value = current.defaultTenant ? String(current.defaultTenant.id) : ''
  allowedScopes.value = current.allowedScopes?.join(' ') ?? ''
  active.value = current.active
}

function parseScopes(value: string): string[] {
  return value
    .split(/[\s,]+/)
    .map((item) => item.trim())
    .filter(Boolean)
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

async function submit() {
  if (!canSubmit.value) {
    return
  }

  message.value = ''
  errorMessage.value = ''

  try {
    await store.update(clientId.value, {
      provider: provider.value.trim() || 'zitadel',
      providerClientId: providerClientId.value.trim(),
      displayName: displayName.value.trim(),
      defaultTenantId: defaultTenantId.value ? Number(defaultTenantId.value) : undefined,
      allowedScopes: parseScopes(allowedScopes.value),
      active: active.value,
    })
    message.value = 'Machine client を更新しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function disableCurrent() {
  if (!store.current || !window.confirm('この machine client を無効化しますか？')) {
    return
  }

  message.value = ''
  errorMessage.value = ''

  try {
    await store.disable(store.current.id)
    active.value = false
    message.value = 'Machine client を無効化しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}
</script>

<template>
  <AdminAccessDenied v-if="store.status === 'forbidden'" />

  <section v-else class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">M2M</span>
        <h2>Machine Client Detail</h2>
      </div>
      <RouterLink class="secondary-button link-button" to="/machine-clients">
        Back
      </RouterLink>
    </div>

    <p v-if="store.status === 'loading'">
      Loading machine client...
    </p>
    <p v-if="errorMessage || store.errorMessage" class="error-message">
      {{ errorMessage || store.errorMessage }}
    </p>
    <p v-if="message" class="notice-message">
      {{ message }}
    </p>

    <form v-if="store.current" class="admin-form" @submit.prevent="submit">
      <label class="field">
        <span class="field-label">Provider</span>
        <input v-model="provider" class="field-input" autocomplete="off">
      </label>

      <label class="field">
        <span class="field-label">Provider client ID</span>
        <input v-model="providerClientId" class="field-input" autocomplete="off" required>
      </label>

      <label class="field">
        <span class="field-label">Display name</span>
        <input v-model="displayName" class="field-input" autocomplete="off" required>
      </label>

      <label class="field">
        <span class="field-label">Default tenant</span>
        <select v-model="defaultTenantId" class="field-input">
          <option value="">None</option>
          <option v-for="tenant in tenantOptions" :key="tenant.id" :value="String(tenant.id)">
            {{ tenant.displayName }} / {{ tenant.slug }}
          </option>
        </select>
      </label>

      <label class="field form-span">
        <span class="field-label">Allowed scopes</span>
        <textarea v-model="allowedScopes" class="field-input textarea-input" rows="4" />
      </label>

      <label class="checkbox-field form-span">
        <input v-model="active" type="checkbox">
        <span>Active</span>
      </label>

      <p v-if="providerClientIdChanged" class="warning-message form-span">
        Provider client ID を変更しようとしています。Zitadel 側の client ID と一致しているか確認してください。
      </p>

      <dl class="metadata-grid form-span">
        <div>
          <dt>Created</dt>
          <dd>{{ formatDate(store.current.createdAt) }}</dd>
        </div>
        <div>
          <dt>Updated</dt>
          <dd>{{ formatDate(store.current.updatedAt) }}</dd>
        </div>
      </dl>

      <div class="action-row form-span">
        <button class="primary-button" :disabled="!canSubmit" type="submit">
          {{ store.saving ? 'Saving...' : 'Save' }}
        </button>
        <button
          class="secondary-button danger-button"
          :disabled="store.saving || !store.current.active"
          type="button"
          @click="disableCurrent"
        >
          Disable
        </button>
      </div>
    </form>
  </section>
</template>
