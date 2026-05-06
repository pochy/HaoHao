<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  variant?: 'primary' | 'secondary' | 'ghost'
  size?: 'default' | 'sm' | 'icon'
  as?: string | object
  type?: 'button' | 'submit' | 'reset'
}>(), {
  variant: 'secondary',
  size: 'default',
  as: 'button',
  type: 'button',
})

const buttonClass = computed(() => [
  'ui-button',
  `ui-button-${props.variant}`,
  `ui-button-${props.size}`,
])

const buttonType = computed(() => (props.as === 'button' ? props.type : undefined))
</script>

<template>
  <component :is="as" :type="buttonType" :class="buttonClass">
    <slot />
  </component>
</template>
