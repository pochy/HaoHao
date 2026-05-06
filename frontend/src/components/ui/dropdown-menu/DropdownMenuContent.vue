<script setup lang="ts">
import { DropdownMenuContent, DropdownMenuPortal, type DropdownMenuContentEmits, type DropdownMenuContentProps, useForwardPropsEmits } from 'reka-ui'

const props = withDefaults(defineProps<DropdownMenuContentProps>(), {
  align: 'end',
  sideOffset: 8,
})
const emits = defineEmits<DropdownMenuContentEmits>()
const forwarded = useForwardPropsEmits(props, emits)
</script>

<template>
  <DropdownMenuPortal>
    <DropdownMenuContent v-bind="forwarded" class="ui-dropdown-menu-content">
      <slot />
    </DropdownMenuContent>
  </DropdownMenuPortal>
</template>
