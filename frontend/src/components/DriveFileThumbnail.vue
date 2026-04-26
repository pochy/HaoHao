<script setup lang="ts">
import type { DriveItemBody } from '../api/generated/types.gen'
import {
  driveItemKind,
  driveItemName,
} from '../utils/driveItems'
import DriveFileTypeIcon from './DriveFileTypeIcon.vue'

defineProps<{
  item: DriveItemBody
}>()
</script>

<template>
  <div class="drive-thumbnail" :class="driveItemKind(item)" aria-hidden="true">
    <DriveFileTypeIcon :kind="driveItemKind(item)" :size="30" />
    <span>{{ driveItemName(item).split('.').pop() || item.type }}</span>
  </div>
</template>
