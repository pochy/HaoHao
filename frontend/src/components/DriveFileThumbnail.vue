<script setup lang="ts">
import { computed } from 'vue'

import type { DriveItemBody } from '../api/generated/types.gen'
import {
  driveItemKind,
  driveItemName,
} from '../utils/driveItems'
import DriveFileTypeIcon from './DriveFileTypeIcon.vue'

const props = defineProps<{
  item: DriveItemBody
}>()

const thumbnailUrl = computed(() => {
  const file = props.item.file
  if (!file?.publicId || !file.contentType?.startsWith('image/')) {
    return ''
  }
  return `/api/v1/drive/files/${encodeURIComponent(file.publicId)}/thumbnail`
})
</script>

<template>
  <div class="drive-thumbnail" :class="driveItemKind(item)" aria-hidden="true">
    <img v-if="thumbnailUrl" :src="thumbnailUrl" :alt="driveItemName(item)" loading="lazy">
    <template v-else>
      <DriveFileTypeIcon :kind="driveItemKind(item)" :size="30" />
      <span>{{ driveItemName(item).split('.').pop() || item.type }}</span>
    </template>
  </div>
</template>
