<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { toApiErrorMessage } from '../api/client'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import { useTenantAdminStore } from '../stores/tenant-admin'

const router = useRouter()
const store = useTenantAdminStore()
const { t } = useI18n()

const slug = ref('')
const displayName = ref('')
const errorMessage = ref('')

const canSubmit = computed(() => (
  slug.value.trim() !== '' &&
  displayName.value.trim() !== '' &&
  !store.saving
))

async function submit() {
  if (!canSubmit.value) {
    return
  }

  errorMessage.value = ''

  try {
    const created = await store.create({
      slug: slug.value.trim(),
      displayName: displayName.value.trim(),
    })

    await router.push({ name: 'tenant-admin-detail', params: { tenantSlug: created.slug } })
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}
</script>

<template>
  <AdminAccessDenied
    v-if="store.status === 'forbidden'"
    :title="t('tenantAdmin.accessRequiredTitle')"
    :message="t('tenantAdmin.accessRequiredMessage')"
    role-label="tenant_admin"
  />

  <section v-else class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.actions.newTenant') }}</span>
        <h2>{{ t('tenantAdmin.actions.newTenant') }}</h2>
      </div>
      <RouterLink class="secondary-button link-button" to="/tenant-admin">
        {{ t('common.back') }}
      </RouterLink>
    </div>

    <form class="admin-form" @submit.prevent="submit">
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.slug') }}</span>
        <input v-model="slug" class="field-input" autocomplete="off" required>
      </label>

      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.displayName') }}</span>
        <input v-model="displayName" class="field-input" autocomplete="off" required>
      </label>

      <p v-if="errorMessage || store.errorMessage" class="error-message form-span">
        {{ errorMessage || store.errorMessage }}
      </p>

      <div class="action-row form-span">
        <button class="primary-button" :disabled="!canSubmit" type="submit">
          {{ store.saving ? t('common.saving') : t('tenantAdmin.actions.create') }}
        </button>
        <RouterLink class="secondary-button link-button" to="/tenant-admin">
          {{ t('tenantAdmin.actions.cancel') }}
        </RouterLink>
      </div>
    </form>
  </section>
</template>
