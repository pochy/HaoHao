<script setup lang="ts">
import { computed } from 'vue'

import { useSessionStore } from './stores/session'

const sessionStore = useSessionStore()

const displayName = computed(() => sessionStore.user?.displayName ?? 'Guest')
const statusLabel = computed(() => {
  switch (sessionStore.status) {
    case 'authenticated':
      return 'Authenticated'
    case 'anonymous':
      return 'Anonymous'
    case 'loading':
      return 'Checking'
    default:
      return 'Idle'
  }
})
</script>

<template>
  <div class="app-shell">
    <header class="app-header">
      <div>
        <p class="eyebrow">Foundation Tutorial Build</p>
        <h1>HaoHao</h1>
      </div>

      <div class="identity-card">
        <span class="identity-label">Current identity</span>
        <strong>{{ displayName }}</strong>
        <span class="identity-status">{{ statusLabel }}</span>
      </div>
    </header>

    <main class="app-main">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  width: min(960px, calc(100vw - 32px));
  margin: 0 auto;
  padding: 40px 0 64px;
}

.app-header {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 24px;
  margin-bottom: 28px;
}

.eyebrow {
  margin: 0 0 10px;
  font-size: 0.78rem;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--muted);
}

h1 {
  margin: 0;
  font-size: clamp(2.5rem, 5vw, 4rem);
  line-height: 0.96;
}

.identity-card {
  min-width: 210px;
  padding: 14px 16px;
  border: 1px solid var(--border-strong);
  border-radius: 18px;
  background: rgba(248, 239, 227, 0.78);
  backdrop-filter: blur(12px);
}

.identity-card strong {
  display: block;
  color: var(--text-strong);
  font-size: 1.05rem;
}

.identity-label,
.identity-status {
  display: block;
  font-size: 0.76rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted);
}

.identity-label {
  margin-bottom: 6px;
}

.identity-status {
  margin-top: 8px;
}

@media (max-width: 720px) {
  .app-shell {
    width: min(100vw - 24px, 960px);
    padding-top: 24px;
  }

  .app-header {
    flex-direction: column;
    align-items: stretch;
  }

  .identity-card {
    min-width: 0;
  }
}
</style>

