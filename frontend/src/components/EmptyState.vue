<script setup lang="ts">
defineProps<{
  title: string
  message?: string
}>()
</script>

<template>
  <div class="empty-state">
    <strong>{{ title }}</strong>
    <p v-if="message">{{ message }}</p>
    <div v-if="$slots.actions" class="action-row">
      <slot name="actions" />
    </div>
  </div>
</template>
