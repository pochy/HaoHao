<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'

import { toApiErrorMessage } from '../api/client'
import { acceptTenantInvitationItem } from '../api/tenant-invitations'

const route = useRoute()
const token = ref(String(route.query.token ?? ''))
const accepting = ref(false)
const message = ref('')
const errorMessage = ref('')

const canAccept = computed(() => token.value.trim() !== '' && !accepting.value)

async function acceptInvitation() {
  if (!canAccept.value) {
    return
  }
  accepting.value = true
  message.value = ''
  errorMessage.value = ''
  try {
    const invitation = await acceptTenantInvitationItem(token.value.trim())
    message.value = `Invitation accepted for tenant ${invitation.tenantId}.`
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  } finally {
    accepting.value = false
  }
}
</script>

<template>
  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">Invitation</span>
        <h2>Accept Tenant Invitation</h2>
      </div>
    </div>

    <p v-if="errorMessage" class="error-message">
      {{ errorMessage }}
    </p>
    <p v-if="message" class="notice-message">
      {{ message }}
    </p>

    <form class="admin-form" @submit.prevent="acceptInvitation">
      <label class="field form-span">
        <span class="field-label">Invitation token</span>
        <input v-model="token" class="field-input" autocomplete="off" required>
      </label>

      <div class="action-row form-span">
        <button class="primary-button" type="submit" :disabled="!canAccept">
          {{ accepting ? 'Accepting...' : 'Accept' }}
        </button>
      </div>
    </form>
  </section>
</template>
