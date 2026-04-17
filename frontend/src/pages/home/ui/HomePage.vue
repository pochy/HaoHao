<script setup lang="ts">
import type { HealthBody } from '@/api/generated/types.gen'
import { useSessionStore } from '@/features/session/model/session-store'
import { fetchHealth } from '@/features/system/api/get-health'
import { onMounted, ref } from 'vue'

const sessionStore = useSessionStore()

const health = ref<HealthBody | null>(null)
const healthError = ref<string | null>(null)
const healthLoading = ref(false)

async function loadHealth() {
  healthLoading.value = true
  healthError.value = null

  try {
    health.value = await fetchHealth()
  } catch (error) {
    healthError.value =
      error instanceof Error ? error.message : 'failed to load health'
  } finally {
    healthLoading.value = false
  }
}

async function refreshAll() {
  await Promise.all([sessionStore.bootstrap(), loadHealth()])
}

onMounted(() => {
  void refreshAll()
})
</script>

<template>
  <main class="page-shell">
    <section class="hero">
      <div>
        <p class="eyebrow">HaoHao</p>
        <h1>Minimal runnable skeleton</h1>
        <p class="lead">
          OpenAPI 3.1 artifact を backend から export し、generated client を
          Vue から Pinia 経由で呼ぶ最小構成です。
        </p>
      </div>

      <button class="refresh-button" type="button" @click="refreshAll">
        Refresh bootstrap
      </button>
    </section>

    <section class="grid">
      <article class="card">
        <header class="card-header">
          <h2>Health</h2>
          <span class="pill" :data-state="health?.status ?? 'unknown'">
            {{ health?.status ?? 'unknown' }}
          </span>
        </header>

        <dl v-if="health" class="detail-list">
          <div>
            <dt>Service</dt>
            <dd>{{ health.service }}</dd>
          </div>
          <div>
            <dt>Version</dt>
            <dd>{{ health.version }}</dd>
          </div>
          <div>
            <dt>Time</dt>
            <dd>{{ health.time }}</dd>
          </div>
        </dl>

        <p v-else-if="healthLoading" class="muted">Loading health…</p>
        <p v-else-if="healthError" class="error">{{ healthError }}</p>
        <p v-else class="muted">No data yet.</p>
      </article>

      <article class="card">
        <header class="card-header">
          <h2>Session</h2>
          <span class="pill" :data-state="sessionStore.authenticated ? 'on' : 'off'">
            {{ sessionStore.authenticated ? 'authenticated' : 'anonymous' }}
          </span>
        </header>

        <dl v-if="sessionStore.data" class="detail-list">
          <div>
            <dt>API surface</dt>
            <dd>{{ sessionStore.data.apiSurface }}</dd>
          </div>
          <div>
            <dt>Auth mode</dt>
            <dd>{{ sessionStore.data.authMode }}</dd>
          </div>
          <div>
            <dt>CSRF cookie</dt>
            <dd>{{ sessionStore.data.csrfCookie }}</dd>
          </div>
        </dl>

        <p v-else-if="sessionStore.loading" class="muted">Loading session…</p>
        <p v-else-if="sessionStore.error" class="error">{{ sessionStore.error }}</p>
        <p v-else class="muted">No data yet.</p>
      </article>
    </section>
  </main>
</template>

<style scoped>
.page-shell {
  min-height: 100vh;
  padding: 2rem;
  display: grid;
  gap: 1.5rem;
  align-content: start;
}

.hero {
  width: min(1080px, 100%);
  margin: 0 auto;
  display: flex;
  gap: 1rem;
  justify-content: space-between;
  align-items: flex-start;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 24px;
  padding: 2rem;
  background: rgba(255, 255, 255, 0.84);
  box-shadow: 0 24px 60px rgba(15, 23, 42, 0.08);
}

.grid {
  width: min(1080px, 100%);
  margin: 0 auto;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
}

.card {
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 20px;
  padding: 1.25rem;
  background: rgba(255, 255, 255, 0.88);
  box-shadow: 0 12px 30px rgba(15, 23, 42, 0.06);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
}

.eyebrow {
  margin: 0 0 0.5rem;
  font-size: 0.9rem;
  font-weight: 700;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: #0f766e;
}

h1,
h2 {
  margin: 0;
}

h1 {
  font-size: clamp(2.2rem, 5vw, 4rem);
  line-height: 1;
}

.lead {
  margin: 1rem 0 0;
  max-width: 42rem;
  color: #334155;
  font-size: 1.05rem;
}

.detail-list {
  margin: 1rem 0 0;
  display: grid;
  gap: 0.75rem;
}

.detail-list div {
  display: grid;
  gap: 0.25rem;
}

dt {
  color: #64748b;
  font-size: 0.9rem;
}

dd {
  margin: 0;
  font-weight: 600;
}

.pill {
  border-radius: 999px;
  padding: 0.35rem 0.75rem;
  font-size: 0.9rem;
  font-weight: 700;
  background: #e2e8f0;
}

.pill[data-state='ok'],
.pill[data-state='on'] {
  background: #dcfce7;
  color: #166534;
}

.pill[data-state='off'],
.pill[data-state='unknown'] {
  background: #e2e8f0;
  color: #334155;
}

.refresh-button {
  border: none;
  border-radius: 999px;
  padding: 0.85rem 1.2rem;
  background: #0f172a;
  color: white;
  cursor: pointer;
}

.muted {
  color: #64748b;
}

.error {
  color: #b91c1c;
}

@media (max-width: 800px) {
  .hero {
    flex-direction: column;
  }

  .grid {
    grid-template-columns: 1fr;
  }
}
</style>
