<script setup lang="ts">
defineProps<{
  title?: string
  subtitle?: string
}>()
</script>

<template>
  <section class="data-card">
    <header v-if="title || subtitle || $slots.actions" class="data-card-header">
      <div>
        <h2 v-if="title">{{ title }}</h2>
        <p v-if="subtitle">{{ subtitle }}</p>
      </div>
      <div v-if="$slots.actions" class="action-row">
        <slot name="actions" />
      </div>
    </header>
    <slot />
  </section>
</template>
