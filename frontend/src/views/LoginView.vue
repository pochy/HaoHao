<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { useSessionStore } from '../stores/session'

const router = useRouter()
const sessionStore = useSessionStore()

const email = ref('demo@example.com')
const password = ref('changeme123')
const submitting = ref(false)

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
          foundation 段階の動作確認として、
          <code>demo@example.com / changeme123</code>
          でログインできます。
        </p>
      </div>

      <form class="stack" @submit.prevent="submit">
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

      <p v-if="sessionStore.errorMessage" class="error-message">
        {{ sessionStore.errorMessage }}
      </p>
    </div>

    <aside class="panel stack">
      <h2>Routes</h2>
      <p>backend は Huma から OpenAPI 3.1 を生成し、frontend は generated SDK を使います。</p>
      <div class="stack detail-list">
        <div>
          <strong>API</strong>
          <p><code>POST /api/v1/login</code></p>
        </div>
        <div>
          <strong>Session</strong>
          <p><code>GET /api/v1/session</code></p>
        </div>
        <div>
          <strong>Logout</strong>
          <p><code>POST /api/v1/logout</code></p>
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
</style>

