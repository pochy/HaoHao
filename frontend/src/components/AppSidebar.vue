<script setup lang="ts">
import type { Component } from 'vue'
import {
  Bell,
  Building2,
  CircleCheckBig,
  FileText,
  FolderOpen,
  Home,
  KeyRound,
  PlugZap,
  RadioTower,
} from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

type NavigationItem = {
  to: string
  labelKey: string
  icon: Component
}

type NavigationGroup = {
  labelKey: string
  items: NavigationItem[]
}

const { t } = useI18n()

const navigationGroups: NavigationGroup[] = [
  {
    labelKey: 'nav.groups.workspace',
    items: [
      { to: '/', labelKey: 'nav.items.session', icon: Home },
      { to: '/notifications', labelKey: 'nav.items.notifications', icon: Bell },
    ],
  },
  {
    labelKey: 'nav.groups.work',
    items: [
      { to: '/customer-signals', labelKey: 'nav.items.signals', icon: RadioTower },
      { to: '/drive', labelKey: 'nav.items.drive', icon: FolderOpen },
      { to: '/todos', labelKey: 'nav.items.todos', icon: CircleCheckBig },
    ],
  },
  {
    labelKey: 'nav.groups.admin',
    items: [
      { to: '/tenant-admin', labelKey: 'nav.items.tenants', icon: Building2 },
      { to: '/machine-clients', labelKey: 'nav.items.machineClients', icon: KeyRound },
      { to: '/integrations', labelKey: 'nav.items.integrations', icon: PlugZap },
    ],
  },
]
</script>

<template>
  <aside class="app-sidebar" :aria-label="t('nav.primary')">
    <RouterLink class="app-brand" to="/">
      <span class="app-brand-mark" aria-hidden="true">H</span>
      <span>
        <strong>{{ t('app.name') }}</strong>
        <span>{{ t('app.tagline') }}</span>
      </span>
    </RouterLink>

    <nav class="sidebar-nav">
      <section v-for="group in navigationGroups" :key="group.labelKey" class="sidebar-group">
        <h2>{{ t(group.labelKey) }}</h2>
        <RouterLink v-for="item in group.items" :key="item.to" class="sidebar-link" :to="item.to">
          <component :is="item.icon" class="sidebar-link-icon" :size="17" stroke-width="1.8" aria-hidden="true" />
          <span>{{ t(item.labelKey) }}</span>
        </RouterLink>
      </section>
    </nav>

    <RouterLink class="sidebar-doc-link" to="/drive/groups">
      <FileText :size="17" stroke-width="1.8" aria-hidden="true" />
      <span>{{ t('nav.items.driveGroups') }}</span>
    </RouterLink>
  </aside>
</template>
