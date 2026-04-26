<script setup lang="ts">
withDefaults(defineProps<{
  tone?: 'neutral' | 'success' | 'warning' | 'danger'
}>(), {
  tone: 'neutral',
})
</script>

<template>
  <span :class="['status-pill', tone !== 'neutral' ? tone : '']">
    <slot />
  </span>
</template>
