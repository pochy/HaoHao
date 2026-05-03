<script setup lang="ts">
import { computed } from 'vue'
import { SplitterResizeHandle, type SplitterResizeHandleEmits, type SplitterResizeHandleProps } from 'reka-ui'

const props = defineProps<SplitterResizeHandleProps & {
  withHandle?: boolean
}>()
const emits = defineEmits<SplitterResizeHandleEmits>()

const forwardedProps = computed(() => {
  const { withHandle: _withHandle, ...rest } = props
  return rest
})
</script>

<template>
  <SplitterResizeHandle
    v-bind="forwardedProps"
    class="resizable-handle"
    @dragging="emits('dragging', $event)"
  >
    <span v-if="withHandle" class="resizable-handle-grip" aria-hidden="true"></span>
    <slot />
  </SplitterResizeHandle>
</template>
