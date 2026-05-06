<script setup lang="ts">
withDefaults(defineProps<{
  orientation?: 'horizontal' | 'vertical'
}>(), {
  orientation: 'horizontal',
})
</script>

<template>
  <div class="ui-button-group" :class="`ui-button-group-${orientation}`" role="group">
    <slot />
  </div>
</template>
