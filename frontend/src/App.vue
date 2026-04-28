<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import AppSidebar from './components/AppSidebar.vue'
import AppTopbar from './components/AppTopbar.vue'
import SupportAccessBanner from './components/SupportAccessBanner.vue'

const route = useRoute()
const wideMain = computed(() => route.path === '/drive' || route.path.startsWith('/drive/'))
</script>

<template>
  <div class="app-layout">
    <AppSidebar />

    <div class="app-workspace">
      <AppTopbar />
      <main class="app-main" :class="{ 'app-main-wide': wideMain }">
        <SupportAccessBanner />
        <RouterView />
      </main>
    </div>
  </div>
</template>
