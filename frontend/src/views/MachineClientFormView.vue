<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { toApiErrorMessage } from '../api/client'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import { useMachineClientStore } from '../stores/machine-clients'
import { useTenantStore } from '../stores/tenants'

const router = useRouter()
const store = useMachineClientStore()
const tenantStore = useTenantStore()

const provider = ref('zitadel')
const providerClientId = ref('')
const displayName = ref('')
const defaultTenantId = ref('')
const allowedScopes = ref('m2m:read')
const active = ref(true)
const errorMessage = ref('')

const canSubmit = computed(() => (
  providerClientId.value.trim() !== '' &&
  displayName.value.trim() !== '' &&
  !store.saving
))

onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
  await store.loadList()
})

function parseScopes(value: string): string[] {
  return value
    .split(/[\s,]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}

async function submit() {
  if (!canSubmit.value) {
    return
  }

  errorMessage.value = ''

  try {
    const created = await store.create({
      provider: provider.value.trim() || 'zitadel',
      providerClientId: providerClientId.value.trim(),
      displayName: displayName.value.trim(),
      defaultTenantId: defaultTenantId.value ? Number(defaultTenantId.value) : undefined,
      allowedScopes: parseScopes(allowedScopes.value),
      active: active.value,
    })

    await router.push({ name: 'machine-client-detail', params: { id: created.id } })
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
        <span class="status-pill">New M2M</span>
        <h2>New Machine Client</h2>
      </div>
      <RouterLink class="secondary-button link-button" to="/machine-clients">
        Back
      </RouterLink>
    </div>

    <form class="admin-form" @submit.prevent="submit">
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
          <option v-for="tenant in tenantStore.items" :key="tenant.id" :value="String(tenant.id)">
            {{ tenant.displayName }} / {{ tenant.slug }}
          </option>
        </select>
      </label>

      <label class="field form-span">
        <span class="field-label">Allowed scopes</span>
        <textarea
          v-model="allowedScopes"
          class="field-input textarea-input"
          rows="4"
          placeholder="m2m:read m2m:write"
        />
      </label>

      <label class="checkbox-field form-span">
        <input v-model="active" type="checkbox">
        <span>Active</span>
      </label>

      <p v-if="errorMessage || store.errorMessage" class="error-message form-span">
        {{ errorMessage || store.errorMessage }}
      </p>

      <div class="action-row form-span">
        <button class="primary-button" :disabled="!canSubmit" type="submit">
          {{ store.saving ? 'Saving...' : 'Create' }}
        </button>
        <RouterLink class="secondary-button link-button" to="/machine-clients">
          Cancel
        </RouterLink>
      </div>
    </form>
  </section>
</template>
