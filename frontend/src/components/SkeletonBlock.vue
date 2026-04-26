<script setup lang="ts">
withDefaults(defineProps<{
  rows?: number
}>(), {
  rows: 3,
})
</script>

<template>
  <div class="skeleton-stack" aria-label="Loading">
    <span v-for="row in rows" :key="row" class="skeleton-block" />
  </div>
</template>
