<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import AppSidebar from './components/AppSidebar.vue'
import AppTopbar from './components/AppTopbar.vue'
import SupportAccessBanner from './components/SupportAccessBanner.vue'

const route = useRoute()
const sidebarCollapsedStorageKey = 'haohao.sidebar.collapsed'
const sidebarCollapsed = ref(
  typeof window !== 'undefined' && window.localStorage.getItem(sidebarCollapsedStorageKey) === 'true',
)
const wideMain = computed(() => (
  route.path === '/datasets' ||
  route.path.startsWith('/datasets/') ||
  route.path === '/drive' ||
  route.path.startsWith('/drive/')
))

watch(sidebarCollapsed, (collapsed) => {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(sidebarCollapsedStorageKey, collapsed ? 'true' : 'false')
  }
})
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
