<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { fetchAuthSettings } from '../api/auth'
import { useSessionStore } from '../stores/session'

type AuthMode = 'local' | 'zitadel'

const route = useRoute()
const router = useRouter()
const sessionStore = useSessionStore()

const authMode = ref<AuthMode>('local')
const zitadelIssuer = ref('')
const email = ref('demo@example.com')
const password = ref('changeme123')
const submitting = ref(false)
const loadingSettings = ref(true)

const callbackErrorMessage = computed(() => {
  if (route.query.error === 'oidc_callback_failed') {
    return 'Zitadel callback の処理に失敗しました。設定値と redirect URI を確認してください。'
  }
  return ''
})

onMounted(async () => {
  try {
    const settings = await fetchAuthSettings()
    authMode.value = settings.mode as AuthMode
    zitadelIssuer.value = settings.zitadel?.issuer ?? ''
  } catch {
    authMode.value = 'local'
    zitadelIssuer.value = ''
  } finally {
    loadingSettings.value = false
  }
})

async function submit() {
  submitting.value = true

  try {
    await sessionStore.login(email.value, password.value)
    await router.push({ name: 'home' })
  } catch {
    // The store exposes the error message for the current view.
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <section class="split-grid">
    <div class="panel stack">
      <div class="stack intro">
        <span class="status-pill">Cookie Session</span>
        <h2>Login</h2>
        <p>
          認証 mode を backend から読み、<code>local</code> なら password form、
          <code>zitadel</code> なら browser redirect login を表示します。
        </p>
      </div>

      <p v-if="loadingSettings">Loading auth settings...</p>

      <form v-else-if="authMode === 'local'" class="stack" @submit.prevent="submit">
        <label class="field">
          <span class="field-label">Email</span>
          <input
            v-model="email"
            class="field-input"
            type="email"
            required
            autocomplete="username"
          />
        </label>

        <label class="field">
          <span class="field-label">Password</span>
          <input
            v-model="password"
            class="field-input"
            type="password"
            required
            minlength="8"
            autocomplete="current-password"
          />
        </label>

        <button class="primary-button" :disabled="submitting" type="submit">
          {{ submitting ? 'Signing in...' : 'Sign in' }}
        </button>
      </form>

      <div v-else class="stack">
        <p>
          <code>AUTH_MODE=zitadel</code> が有効です。browser は backend の
          <code>/api/v1/auth/login</code> へ遷移し、callback 後に local Cookie session を受け取ります。
        </p>
        <p v-if="zitadelIssuer">
          Issuer: <code>{{ zitadelIssuer }}</code>
        </p>
        <a class="primary-button zitadel-button" href="/api/v1/auth/login">
          Sign in with Zitadel
        </a>
      </div>

      <p v-if="callbackErrorMessage" class="error-message">
        {{ callbackErrorMessage }}
      </p>
      <p v-if="sessionStore.errorMessage" class="error-message">
        {{ sessionStore.errorMessage }}
      </p>
    </div>

    <aside class="panel stack">
      <h2>Routes</h2>
      <p>backend は Huma から OpenAPI 3.1 を生成し、frontend は generated SDK を使います。</p>
      <div class="stack detail-list">
        <div>
          <strong>Settings</strong>
          <p><code>GET /api/v1/auth/settings</code></p>
        </div>
        <div>
          <strong>OIDC</strong>
          <p><code>GET /api/v1/auth/login</code></p>
        </div>
        <div>
          <strong>Callback</strong>
          <p><code>GET /api/v1/auth/callback</code></p>
        </div>
        <div>
          <strong>Session</strong>
          <p><code>GET /api/v1/session</code></p>
        </div>
      </div>
    </aside>
  </section>
</template>

<style scoped>
.intro {
  gap: 10px;
}

.detail-list {
  gap: 14px;
}

.detail-list p,
.detail-list strong {
  margin: 0;
}

.zitadel-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  text-decoration: none;
}
</style>
