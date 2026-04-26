<script setup lang="ts">
import { useRoute } from 'vue-router'

type SectionTab = {
  to: string
  label: string
}

defineProps<{
  tabs: SectionTab[]
}>()

const route = useRoute()

function isActive(tab: SectionTab) {
  if (tab.to === '/drive') {
    return route.path === '/drive' || route.path.startsWith('/drive/folders/')
  }
  return route.path === tab.to
}
</script>

<template>
  <nav class="section-tabs" aria-label="Section navigation">
    <RouterLink
      v-for="tab in tabs"
      :key="tab.to"
      :to="tab.to"
      :class="{ active: isActive(tab) }"
    >
      {{ tab.label }}
    </RouterLink>
  </nav>
</template>
