<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import { toApiErrorMessage } from '../api/client'
import { refreshCurrentSession } from '../api/session'
import DataCard from '../components/DataCard.vue'
import DocsLink from '../components/DocsLink.vue'
import MetricTile from '../components/MetricTile.vue'
import PageHeader from '../components/PageHeader.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { useSessionStore } from '../stores/session'

const router = useRouter()
const sessionStore = useSessionStore()

const userJson = computed(() => JSON.stringify(sessionStore.user, null, 2))
const userEmail = computed(() => sessionStore.user?.email ?? 'No email')
const userId = computed(() => sessionStore.user?.publicId ?? 'Anonymous')
const refreshing = ref(false)
const refreshMessage = ref('')
const refreshErrorMessage = ref('')

async function signOut() {
  try {
    const postLogoutURL = await sessionStore.logout()
    if (postLogoutURL) {
      window.location.assign(postLogoutURL)
      return
    }
    await router.push({ name: 'login' })
  } catch {
    // The store exposes the error message for the current view.
  }
}

async function rotateSession() {
  refreshing.value = true
  refreshMessage.value = ''
  refreshErrorMessage.value = ''

  try {
    await refreshCurrentSession()
    refreshMessage.value = 'Session ID と CSRF token を再発行しました。'
  } catch (error) {
    refreshErrorMessage.value = toApiErrorMessage(error)
  } finally {
    refreshing.value = false
  }
}
</script>

<template>
  <section class="stack">
    <PageHeader
      eyebrow="Workspace"
      title="Session"
      description="Cookie session、active identity、generated SDK の接続状態を確認します。"
    >
      <template #actions>
        <button class="secondary-button" :disabled="refreshing" type="button" @click="rotateSession">
          {{ refreshing ? 'Refreshing...' : 'Refresh Session' }}
        </button>
        <button class="secondary-button" type="button" @click="signOut">
          Logout
        </button>
      </template>
    </PageHeader>

    <div class="metric-grid">
      <MetricTile label="Status" :value="sessionStore.status" hint="Current browser session" />
      <MetricTile label="User" :value="sessionStore.user?.displayName ?? 'Guest'" :hint="userEmail" />
      <MetricTile label="User public ID" :value="userId" hint="API subject" />
      <MetricTile label="Support access" :value="sessionStore.supportAccess ? 'Active' : 'Off'" hint="Impersonation state" />
    </div>

    <div class="split-grid">
      <DataCard title="Current Session" subtitle="現在の user payload を API から取得しています。">
        <StatusBadge tone="success">Authenticated</StatusBadge>
        <pre class="json-card">{{ userJson }}</pre>

        <div class="action-row">
          <RouterLink class="secondary-button link-button" to="/todos">
            Open TODO
          </RouterLink>
          <DocsLink />
        </div>

        <p v-if="refreshMessage">{{ refreshMessage }}</p>
        <p v-if="refreshErrorMessage" class="error-message">
          {{ refreshErrorMessage }}
        </p>
        <p v-if="sessionStore.errorMessage" class="error-message">
          {{ sessionStore.errorMessage }}
        </p>
      </DataCard>

      <DataCard title="Verification" subtitle="frontend、session API、OpenAPI docs の疎通確認です。">
        <ul class="check-list">
          <li>Cookie が browser に保存される</li>
          <li><code>/api/v1/session</code> が 200 を返す</li>
          <li><code>POST /api/v1/session/refresh</code> が Cookie を rotate する</li>
          <li><code>/docs</code> で OpenAPI 由来の docs を確認できる</li>
        </ul>
      </DataCard>
    </div>
  </section>
</template>

<style scoped>
.check-list {
  margin: 0;
  padding-left: 1.2rem;
}
</style>
