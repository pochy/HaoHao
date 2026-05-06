<script setup lang="ts">
import { DropdownMenuSeparator, type DropdownMenuSeparatorProps, useForwardProps } from 'reka-ui'

const props = defineProps<DropdownMenuSeparatorProps>()
const forwarded = useForwardProps(props)
</script>

<template>
  <DropdownMenuSeparator v-bind="forwarded" class="ui-dropdown-menu-separator" />
</template>
