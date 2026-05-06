<script setup lang="ts">
import { DropdownMenuLabel, type DropdownMenuLabelProps, useForwardProps } from 'reka-ui'

const props = defineProps<DropdownMenuLabelProps>()
const forwarded = useForwardProps(props)
</script>

<template>
  <DropdownMenuLabel v-bind="forwarded" class="ui-dropdown-menu-label">
    <slot />
  </DropdownMenuLabel>
</template>
