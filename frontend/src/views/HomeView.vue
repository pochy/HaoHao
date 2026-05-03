<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import { toApiErrorMessage } from '../api/client'
import { refreshCurrentSession } from '../api/session'
import { useSessionStore } from '../stores/session'

const router = useRouter()
const sessionStore = useSessionStore()

const userJson = computed(() => JSON.stringify(sessionStore.user, null, 2))
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
    <div class="split-grid">
      <section class="panel stack">
        <span class="status-pill">Authenticated</span>
        <h2>Current Session</h2>
        <p>Cookie セッションが復元できていれば、現在ユーザーがここに表示されます。</p>
        <pre class="json-card">{{ userJson }}</pre>

        <div class="action-row">
          <button class="secondary-button" :disabled="refreshing" type="button" @click="rotateSession">
            {{ refreshing ? 'Refreshing...' : 'Refresh Session' }}
          </button>
          <button class="secondary-button" type="button" @click="signOut">
            Logout
          </button>
          <a class="secondary-button docs-link" href="/docs" target="_blank" rel="noreferrer">
            Open Docs
          </a>
        </div>

        <p v-if="refreshMessage">{{ refreshMessage }}</p>
        <p v-if="refreshErrorMessage" class="error-message">
          {{ refreshErrorMessage }}
        </p>
        <p v-if="sessionStore.errorMessage" class="error-message">
          {{ sessionStore.errorMessage }}
        </p>
      </section>

      <aside class="panel stack">
        <h2>Verification</h2>
        <p>この画面が出ていれば、frontend は generated SDK 経由で session を読めています。</p>
        <ul class="check-list">
          <li>Cookie が browser に保存される</li>
          <li><code>/api/v1/session</code> が 200 を返す</li>
          <li><code>POST /api/v1/session/refresh</code> が Cookie を rotate する</li>
          <li><code>/docs</code> で OpenAPI 由来の docs を確認できる</li>
        </ul>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.docs-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  text-decoration: none;
}

.check-list {
  margin: 0;
  padding-left: 1.2rem;
}
</style>
