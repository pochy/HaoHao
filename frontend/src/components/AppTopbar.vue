<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Bell, CircleUserRound, HelpCircle, LogIn } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { useSessionStore } from '../stores/session'
import LocaleSwitcher from './LocaleSwitcher.vue'
import TenantSelector from './TenantSelector.vue'

const route = useRoute()
const sessionStore = useSessionStore()
const { t, te } = useI18n()

const displayName = computed(() => sessionStore.user?.displayName ?? t('auth.guest'))
const routeLabel = computed(() => {
  const key = route.meta.titleKey
  if (typeof key === 'string' && te(key)) {
    return t(key)
  }

  return String(route.meta.title ?? route.name ?? t('app.name'))
})
const routeGroup = computed(() => {
  const key = route.meta.groupKey
  if (typeof key === 'string' && te(key)) {
    return t(key)
  }

  return String(route.meta.group ?? t('nav.groups.application'))
})
const statusLabel = computed(() => {
  switch (sessionStore.status) {
    case 'authenticated':
      return t('auth.status.authenticated')
    case 'anonymous':
      return t('auth.status.anonymous')
    case 'loading':
      return t('auth.status.loading')
    default:
      return t('auth.status.idle')
  }
})
</script>

<template>
  <header class="app-topbar">
    <div class="topbar-breadcrumb" :aria-label="t('topbar.breadcrumbLabel')">
      <span>{{ routeGroup }}</span>
      <span aria-hidden="true">/</span>
      <strong>{{ routeLabel }}</strong>
    </div>

    <div class="topbar-actions">
      <RouterLink class="icon-button" to="/notifications" :aria-label="t('topbar.openInbox')">
        <Bell :size="17" stroke-width="1.8" aria-hidden="true" />
      </RouterLink>
      <a class="icon-button" href="/docs" :aria-label="t('topbar.openApiDocs')">
        <HelpCircle :size="17" stroke-width="1.8" aria-hidden="true" />
      </a>

      <LocaleSwitcher />

      <TenantSelector v-if="sessionStore.status === 'authenticated'" compact />

      <RouterLink v-if="sessionStore.status !== 'authenticated'" class="secondary-button compact-button link-button" to="/login">
        <LogIn :size="16" stroke-width="1.8" aria-hidden="true" />
        {{ t('auth.signIn') }}
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
