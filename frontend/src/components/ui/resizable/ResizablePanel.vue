<script setup lang="ts">
import { SplitterPanel, type SplitterPanelEmits, type SplitterPanelProps } from 'reka-ui'

const props = defineProps<SplitterPanelProps>()
const emits = defineEmits<SplitterPanelEmits>()
</script>

<template>
  <SplitterPanel
    v-bind="props"
    class="resizable-panel"
    @collapse="emits('collapse')"
    @expand="emits('expand')"
    @resize="(size, prevSize) => emits('resize', size, prevSize)"
  >
    <slot />
  </SplitterPanel>
</template>
