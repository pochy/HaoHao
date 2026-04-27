<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { toApiErrorMessage } from '../api/client'
import { endSupportAccessSession } from '../api/support-access'
import { useSessionStore } from '../stores/session'

const sessionStore = useSessionStore()
const { t } = useI18n()

async function endAccess() {
  try {
    await endSupportAccessSession()
    sessionStore.supportAccess = null
    sessionStore.status = 'idle'
    await sessionStore.bootstrap()
  } catch (error) {
    sessionStore.errorMessage = toApiErrorMessage(error)
  }
}
</script>

<template>
  <div v-if="sessionStore.supportAccess" class="support-banner" data-testid="support-access-banner">
    <div>
      <strong>{{ t('supportAccess.active') }}</strong>
      <span>
        {{ sessionStore.supportAccess.supportUserEmail }} {{ t('supportAccess.as') }}
        {{ sessionStore.supportAccess.impersonatedUserEmail }} /
        {{ sessionStore.supportAccess.tenantSlug }}
      </span>
    </div>
    <button class="secondary-button compact-button" type="button" @click="endAccess">
      {{ t('supportAccess.end') }}
    </button>
  </div>
</template>

<style scoped>
.support-banner {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  padding: 12px 16px;
  border: 1px solid rgba(174, 45, 42, 0.35);
  border-radius: 12px;
  background: rgba(174, 45, 42, 0.09);
  color: var(--text-strong);
}

.support-banner strong,
.support-banner span {
  display: block;
}

.support-banner span {
  color: var(--muted);
  font-size: 0.9rem;
}
</style>
