<script setup lang="ts">
import type { Component } from 'vue'
import {
  Bell,
  Building2,
  CircleCheckBig,
  Database,
  FileText,
  FolderOpen,
  Home,
  KeyRound,
  ListChecks,
  PanelLeftClose,
  PanelLeftOpen,
  PlugZap,
  RadioTower,
  Workflow,
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

defineProps<{
  collapsed: boolean
}>()

defineEmits<{
  toggleCollapsed: []
}>()

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
      { to: '/datasets', labelKey: 'nav.items.datasets', icon: Database },
      { to: '/data-pipelines', labelKey: 'nav.items.dataPipelines', icon: Workflow },
      { to: '/drive', labelKey: 'nav.items.drive', icon: FolderOpen },
      { to: '/todos', labelKey: 'nav.items.todos', icon: CircleCheckBig },
    ],
  },
  {
    labelKey: 'nav.groups.admin',
    items: [
      { to: '/tenant-admin', labelKey: 'nav.items.tenants', icon: Building2 },
      { to: '/jobs', labelKey: 'nav.items.jobs', icon: ListChecks },
      { to: '/machine-clients', labelKey: 'nav.items.machineClients', icon: KeyRound },
      { to: '/integrations', labelKey: 'nav.items.integrations', icon: PlugZap },
    ],
  },
]
</script>

<template>
  <aside class="app-sidebar" :class="{ collapsed }" :aria-label="t('nav.primary')">
    <div class="app-sidebar-header">
      <RouterLink
        class="app-brand"
        to="/"
        :aria-label="collapsed ? t('app.name') : undefined"
        :title="collapsed ? t('app.name') : undefined"
      >
        <span class="app-brand-mark" aria-hidden="true">H</span>
        <span class="app-brand-copy">
          <strong>{{ t('app.name') }}</strong>
          <span>{{ t('app.tagline') }}</span>
        </span>
      </RouterLink>

      <button
        class="sidebar-collapse-button"
        type="button"
        :aria-label="collapsed ? t('nav.expandSidebar') : t('nav.collapseSidebar')"
        :title="collapsed ? t('nav.expandSidebar') : t('nav.collapseSidebar')"
        @click="$emit('toggleCollapsed')"
      >
        <PanelLeftOpen v-if="collapsed" :size="17" stroke-width="1.9" aria-hidden="true" />
        <PanelLeftClose v-else :size="17" stroke-width="1.9" aria-hidden="true" />
      </button>
    </div>

    <nav class="sidebar-nav">
      <section v-for="group in navigationGroups" :key="group.labelKey" class="sidebar-group">
        <h2>{{ t(group.labelKey) }}</h2>
        <RouterLink
          v-for="item in group.items"
          :key="item.to"
          class="sidebar-link"
          :to="item.to"
          :aria-label="collapsed ? t(item.labelKey) : undefined"
          :title="collapsed ? t(item.labelKey) : undefined"
        >
          <component :is="item.icon" class="sidebar-link-icon" :size="17" stroke-width="1.8" aria-hidden="true" />
          <span class="sidebar-link-label">{{ t(item.labelKey) }}</span>
        </RouterLink>
      </section>
    </nav>

    <RouterLink
      class="sidebar-doc-link"
      to="/drive/groups"
      :aria-label="collapsed ? t('nav.items.driveGroups') : undefined"
      :title="collapsed ? t('nav.items.driveGroups') : undefined"
    >
      <FileText :size="17" stroke-width="1.8" aria-hidden="true" />
      <span class="sidebar-link-label">{{ t('nav.items.driveGroups') }}</span>
    </RouterLink>
  </aside>
</template>
