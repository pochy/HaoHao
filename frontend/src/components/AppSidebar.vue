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

type NavigationItem = {
  to: string
  label: string
  icon: Component
}

type NavigationGroup = {
  label: string
  items: NavigationItem[]
}

const navigationGroups: NavigationGroup[] = [
  {
    label: 'Workspace',
    items: [
      { to: '/', label: 'Session', icon: Home },
      { to: '/notifications', label: 'Notifications', icon: Bell },
    ],
  },
  {
    label: 'Work',
    items: [
      { to: '/customer-signals', label: 'Signals', icon: RadioTower },
      { to: '/drive', label: 'Drive', icon: FolderOpen },
      { to: '/todos', label: 'TODO', icon: CircleCheckBig },
    ],
  },
  {
    label: 'Admin',
    items: [
      { to: '/tenant-admin', label: 'Tenants', icon: Building2 },
      { to: '/machine-clients', label: 'Machine Clients', icon: KeyRound },
      { to: '/integrations', label: 'Integrations', icon: PlugZap },
    ],
  },
]
</script>

<template>
  <aside class="app-sidebar" aria-label="Primary">
    <RouterLink class="app-brand" to="/">
      <span class="app-brand-mark" aria-hidden="true">H</span>
      <span>
        <strong>HaoHao</strong>
        <span>Workspace OS</span>
      </span>
    </RouterLink>

    <nav class="sidebar-nav">
      <section v-for="group in navigationGroups" :key="group.label" class="sidebar-group">
        <h2>{{ group.label }}</h2>
        <RouterLink v-for="item in group.items" :key="item.to" class="sidebar-link" :to="item.to">
          <component :is="item.icon" class="sidebar-link-icon" :size="17" stroke-width="1.8" aria-hidden="true" />
          <span>{{ item.label }}</span>
        </RouterLink>
      </section>
    </nav>

    <RouterLink class="sidebar-doc-link" to="/drive/groups">
      <FileText :size="17" stroke-width="1.8" aria-hidden="true" />
      <span>Drive groups</span>
    </RouterLink>
  </aside>
</template>
