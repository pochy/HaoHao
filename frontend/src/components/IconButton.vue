<script setup lang="ts">
defineOptions({
  inheritAttrs: false,
})

withDefaults(defineProps<{
  label: string
  type?: 'button' | 'submit' | 'reset'
  variant?: 'default' | 'danger'
}>(), {
  type: 'button',
  variant: 'default',
})
</script>

<template>
  <button
    class="icon-button"
    :class="{ danger: variant === 'danger' }"
    :type="type"
    :aria-label="label"
    :title="label"
    v-bind="$attrs"
  >
    <slot />
  </button>
</template>
