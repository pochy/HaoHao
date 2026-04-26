<script setup lang="ts">
defineProps<{
  detailsOpen: boolean
}>()
</script>

<template>
  <section class="drive-workspace-layout" :class="{ 'details-open': detailsOpen }">
    <slot name="side" />
    <div class="drive-workspace-main">
      <slot name="header" />
      <slot name="command" />
      <slot />
    </div>
    <slot name="details" />
  </section>
</template>
