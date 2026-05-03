<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import AppSidebar from './components/AppSidebar.vue'
import AppTopbar from './components/AppTopbar.vue'
import SupportAccessBanner from './components/SupportAccessBanner.vue'
import { useNotificationStore } from './stores/notifications'
import { useRealtimeStore } from './stores/realtime'
import { useSessionStore } from './stores/session'
import { useTenantStore } from './stores/tenants'

const route = useRoute()
const notificationStore = useNotificationStore()
const realtimeStore = useRealtimeStore()
const sessionStore = useSessionStore()
const tenantStore = useTenantStore()
const sidebarCollapsedStorageKey = 'haohao.sidebar.collapsed'
const sidebarCollapsed = ref(
  typeof window !== 'undefined' && window.localStorage.getItem(sidebarCollapsedStorageKey) === 'true',
)
const wideMain = computed(() => (
  route.path === '/datasets' ||
  route.path.startsWith('/datasets/') ||
  route.path === '/data-pipelines' ||
  route.path.startsWith('/data-pipelines/') ||
  route.path === '/drive' ||
  route.path.startsWith('/drive/')
))

watch(sidebarCollapsed, (collapsed) => {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(sidebarCollapsedStorageKey, collapsed ? 'true' : 'false')
  }
})

watch(
  () => sessionStore.status,
  async (status) => {
    if (status !== 'authenticated') {
      realtimeStore.stop()
      return
    }
    if (tenantStore.status === 'idle') {
      await tenantStore.load()
    }
    if (notificationStore.status === 'idle') {
      await notificationStore.load()
    }
    realtimeStore.start()
  },
  { immediate: true },
)

watch(
  () => tenantStore.activeTenant?.slug,
  async () => {
    if (sessionStore.status !== 'authenticated') {
      return
    }
    await notificationStore.load()
    realtimeStore.start()
  },
)
</script>

<template>
  <div class="app-layout" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
    <AppSidebar :collapsed="sidebarCollapsed" @toggle-collapsed="sidebarCollapsed = !sidebarCollapsed" />

    <div class="app-workspace">
      <AppTopbar />
      <main class="app-main" :class="{ 'app-main-wide': wideMain }">
        <SupportAccessBanner />
        <RouterView />
      </main>
    </div>
  </div>
</template>
