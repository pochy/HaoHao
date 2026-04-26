<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Bell, CircleUserRound, HelpCircle, LogIn } from 'lucide-vue-next'

import { useSessionStore } from '../stores/session'
import TenantSelector from './TenantSelector.vue'

const route = useRoute()
const sessionStore = useSessionStore()

const displayName = computed(() => sessionStore.user?.displayName ?? 'Guest')
const routeLabel = computed(() => String(route.meta.title ?? route.name ?? 'HaoHao'))
const routeGroup = computed(() => String(route.meta.group ?? 'Application'))
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
  <header class="app-topbar">
    <div class="topbar-breadcrumb" aria-label="Current location">
      <span>{{ routeGroup }}</span>
      <span aria-hidden="true">/</span>
      <strong>{{ routeLabel }}</strong>
    </div>

    <div class="topbar-actions">
      <RouterLink class="icon-button" to="/notifications" aria-label="Open inbox">
        <Bell :size="17" stroke-width="1.8" aria-hidden="true" />
      </RouterLink>
      <a class="icon-button" href="/docs" aria-label="Open API docs">
        <HelpCircle :size="17" stroke-width="1.8" aria-hidden="true" />
      </a>

      <TenantSelector v-if="sessionStore.status === 'authenticated'" compact />

      <RouterLink v-if="sessionStore.status !== 'authenticated'" class="secondary-button compact-button link-button" to="/login">
        <LogIn :size="16" stroke-width="1.8" aria-hidden="true" />
        Sign in
      </RouterLink>

      <div class="identity-card" data-testid="identity-card">
        <CircleUserRound :size="20" stroke-width="1.8" aria-hidden="true" />
        <span>
          <strong>{{ displayName }}</strong>
          <span data-testid="identity-status">{{ statusLabel }}</span>
        </span>
      </div>
    </div>
  </header>
</template>
