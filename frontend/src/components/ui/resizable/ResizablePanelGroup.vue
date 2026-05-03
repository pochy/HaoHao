<script setup lang="ts">
import { SplitterGroup, type SplitterGroupEmits, type SplitterGroupProps } from 'reka-ui'

const props = defineProps<SplitterGroupProps>()
const emits = defineEmits<SplitterGroupEmits>()
</script>

<template>
  <SplitterGroup v-bind="props" class="resizable-panel-group" @layout="emits('layout', $event)">
    <slot />
  </SplitterGroup>
</template>
