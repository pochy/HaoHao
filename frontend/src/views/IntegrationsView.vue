<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { toApiErrorMessage } from '../api/client'
import type { IntegrationStatusBody, VerifyIntegrationBody } from '../api/generated/types.gen'
import {
  fetchIntegrations,
  revokeIntegrationGrant,
  startIntegrationConnect,
  verifyIntegration,
} from '../api/integrations'
import { useTenantStore } from '../stores/tenants'

const route = useRoute()
const router = useRouter()
const tenantStore = useTenantStore()

const items = ref<IntegrationStatusBody[]>([])
const loading = ref(false)
const busyResource = ref('')
const errorMessage = ref('')
const verifyResult = ref<VerifyIntegrationBody | null>(null)
const callbackNotice = ref('')
const callbackNoticeKind = ref<'success' | 'error' | ''>('')

const activeTenantLabel = computed(() => (
  tenantStore.activeTenant
    ? `${tenantStore.activeTenant.displayName} / ${tenantStore.activeTenant.slug}`
    : ''
))
const verifyExpired = computed(() => (
  Boolean(verifyResult.value?.accessExpiresAt) &&
  new Date(verifyResult.value?.accessExpiresAt ?? '').getTime() <= Date.now()
))

function readCallbackMessage() {
  if (route.query.connected) {
    callbackNoticeKind.value = 'success'
    return `${route.query.connected} integration connected.`
  }
  if (route.query.error === 'missing_session') {
    callbackNoticeKind.value = 'error'
    return 'Integration callback failed because the browser session was missing.'
  }
  if (route.query.error) {
    callbackNoticeKind.value = 'error'
    return `Integration callback failed: ${route.query.error}`
  }
  callbackNoticeKind.value = ''
  return ''
}

async function loadIntegrations() {
  if (!tenantStore.activeTenant) {
    items.value = []
    errorMessage.value = 'Integration を操作するには active tenant が必要です。'
    return
  }

  loading.value = true
  errorMessage.value = ''

  try {
    items.value = await fetchIntegrations()
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  } finally {
    loading.value = false
  }
}

function connect(resourceServer: string) {
  if (!tenantStore.activeTenant) {
    errorMessage.value = 'Integration を操作するには active tenant が必要です。'
    return
  }

  startIntegrationConnect(resourceServer)
}

async function verify(resourceServer: string) {
  busyResource.value = resourceServer
  errorMessage.value = ''
  verifyResult.value = null

  try {
    verifyResult.value = await verifyIntegration(resourceServer)
    await loadIntegrations()
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
    await loadIntegrations()
  } finally {
    busyResource.value = ''
  }
}

async function revoke(resourceServer: string) {
  busyResource.value = resourceServer
  errorMessage.value = ''
  verifyResult.value = null

  try {
    await revokeIntegrationGrant(resourceServer)
    await loadIntegrations()
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  } finally {
    busyResource.value = ''
  }
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

function clearCallbackQuery() {
  if (route.query.connected || route.query.error) {
    router.replace({ name: 'integrations' })
  }
}

onMounted(async () => {
  callbackNotice.value = readCallbackMessage()
  if (tenantStore.status === 'idle' || tenantStore.status === 'loading') {
    await tenantStore.load()
  }
  await loadIntegrations()
  clearCallbackQuery()
})

watch(
  () => tenantStore.activeTenant?.slug,
  async (slug, previous) => {
    if (slug && slug !== previous) {
      verifyResult.value = null
      await loadIntegrations()
    }
  },
)
</script>

<template>
  <section class="stack">
    <section class="panel stack">
      <div class="section-header">
        <div>
          <span class="status-pill">Delegated Auth</span>
          <h2>Integrations</h2>
        </div>
        <button class="secondary-button" :disabled="loading" type="button" @click="loadIntegrations">
          {{ loading ? 'Refreshing...' : 'Refresh' }}
        </button>
      </div>

      <p v-if="activeTenantLabel" class="notice-message">
        Active tenant: <strong>{{ activeTenantLabel }}</strong>
      </p>
      <p v-else class="error-message">
        Integration を操作するには active tenant が必要です。
      </p>

      <p
        v-if="callbackNotice"
        :class="callbackNoticeKind === 'error' ? 'error-message' : 'notice-message'"
      >
        {{ callbackNotice }}
      </p>
      <p v-if="errorMessage" class="error-message">
        {{ errorMessage }}
      </p>

      <div class="integration-list">
        <article v-for="item in items" :key="item.resourceServer" class="integration-card">
          <div class="integration-main">
            <div>
              <span class="field-label">{{ item.provider }}</span>
              <h3>{{ item.resourceServer }}</h3>
            </div>
            <span :class="['connection-state', item.revokedAt ? 'revoked' : item.connected ? 'connected' : 'disconnected']">
              {{ item.revokedAt ? 'Revoked' : item.connected ? 'Connected' : 'Disconnected' }}
            </span>
          </div>

          <dl class="metadata-grid">
            <div>
              <dt>Scopes</dt>
              <dd>{{ item.scopes?.join(' ') || 'None' }}</dd>
            </div>
            <div>
              <dt>Granted</dt>
              <dd>{{ formatDate(item.grantedAt) }}</dd>
            </div>
            <div>
              <dt>Last refresh</dt>
              <dd>{{ formatDate(item.lastRefreshedAt) }}</dd>
            </div>
            <div>
              <dt>Last error</dt>
              <dd>{{ item.lastErrorCode || 'None' }}</dd>
            </div>
            <div>
              <dt>Revoked</dt>
              <dd>{{ formatDate(item.revokedAt) }}</dd>
            </div>
          </dl>

          <div class="action-row">
            <button
              class="primary-button"
              :disabled="!tenantStore.activeTenant || busyResource === item.resourceServer"
              type="button"
              @click="connect(item.resourceServer)"
            >
              {{ item.connected ? 'Reconnect' : 'Connect' }}
            </button>
            <button
              class="secondary-button"
              :disabled="!tenantStore.activeTenant || !item.connected || busyResource === item.resourceServer"
              type="button"
              @click="verify(item.resourceServer)"
            >
              {{ busyResource === item.resourceServer ? 'Verifying...' : 'Verify' }}
            </button>
            <button
              class="secondary-button danger-button"
              :disabled="!tenantStore.activeTenant || !item.connected || busyResource === item.resourceServer"
              type="button"
              @click="revoke(item.resourceServer)"
            >
              Revoke
            </button>
          </div>
        </article>
      </div>
    </section>

    <section v-if="verifyResult" class="panel stack">
      <span :class="['status-pill', verifyResult.connected && !verifyExpired ? '' : 'danger']">
        {{ verifyResult.connected && !verifyExpired ? 'Verified' : 'Attention' }}
      </span>
      <h2>Access Check</h2>
      <p v-if="!verifyResult.connected" class="error-message">
        Access token を取得できませんでした。
      </p>
      <p v-else-if="verifyExpired" class="error-message">
        Access token は期限切れです。
      </p>
      <dl class="metadata-grid">
        <div>
          <dt>Resource</dt>
          <dd>{{ verifyResult.resourceServer }}</dd>
        </div>
        <div>
          <dt>Expires</dt>
          <dd>{{ formatDate(verifyResult.accessExpiresAt) }}</dd>
        </div>
        <div>
          <dt>Refreshed</dt>
          <dd>{{ formatDate(verifyResult.refreshedAt) }}</dd>
        </div>
        <div>
          <dt>Scopes</dt>
          <dd>{{ verifyResult.scopes?.join(' ') || 'None' }}</dd>
        </div>
      </dl>
    </section>
  </section>
</template>

<style scoped>
.section-header,
.integration-main {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 16px;
}

.integration-list {
  display: grid;
  gap: 16px;
}

.integration-card {
  display: grid;
  gap: 20px;
  padding: 20px;
  border: 1px solid var(--border);
  border-radius: 20px;
  background: rgba(255, 250, 243, 0.72);
}

h3 {
  margin: 4px 0 0;
  color: var(--text-strong);
  font-size: 1.3rem;
}

.connection-state {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  padding: 0 12px;
  border-radius: 999px;
  font-size: 0.8rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.connection-state.connected {
  color: var(--accent-strong);
  background: rgba(11, 93, 91, 0.1);
}

.connection-state.disconnected {
  color: var(--muted);
  background: rgba(124, 102, 88, 0.12);
}

.connection-state.revoked {
  color: var(--danger);
  background: rgba(174, 45, 42, 0.1);
}

.metadata-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  margin: 0;
}

.metadata-grid div {
  min-width: 0;
}

.metadata-grid dt {
  margin-bottom: 4px;
  color: var(--muted);
  font-size: 0.78rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.metadata-grid dd {
  margin: 0;
  overflow-wrap: anywhere;
  color: var(--text-strong);
}

.notice-message {
  margin: 0;
  color: var(--accent-strong);
}

.danger-button {
  color: var(--danger);
}

@media (max-width: 720px) {
  .section-header,
  .integration-main {
    align-items: stretch;
    flex-direction: column;
  }

  .metadata-grid {
    grid-template-columns: 1fr;
  }
}
</style>
