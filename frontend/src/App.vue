<script setup lang="ts">
import AppSidebar from './components/AppSidebar.vue'
import AppTopbar from './components/AppTopbar.vue'
import SupportAccessBanner from './components/SupportAccessBanner.vue'
</script>

<template>
  <div class="app-layout">
    <AppSidebar />

    <div class="app-workspace">
      <AppTopbar />
      <main class="app-main">
        <SupportAccessBanner />
        <RouterView />
      </main>
    </div>
  </div>
</template>
