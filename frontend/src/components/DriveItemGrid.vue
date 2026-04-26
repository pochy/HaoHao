<script setup lang="ts">
import type { DriveFileBody, DriveItemBody } from '../api/generated/types.gen'
import DriveItemCard from './DriveItemCard.vue'

defineProps<{
  items: DriveItemBody[]
  loading: boolean
  busyResourceId: string
  deletingResourceId: string
  selectedResourceId: string
  trashMode?: boolean
}>()

const emit = defineEmits<{
  openFolder: [folderPublicId: string]
  downloadFile: [file: DriveFileBody]
  renameItem: [item: DriveItemBody]
  moveItem: [item: DriveItemBody]
  overwriteFile: [file: DriveFileBody]
  deleteItem: [item: DriveItemBody]
  shareItem: [item: DriveItemBody]
  restoreItem: [item: DriveItemBody]
  detailsItem: [item: DriveItemBody]
}>()
</script>

<template>
  <div v-if="loading" class="drive-grid">
    <div v-for="index in 8" :key="index" class="drive-grid-skeleton" />
  </div>
  <div v-else class="drive-grid">
    <DriveItemCard
      v-for="item in items"
      :key="item.file?.publicId ?? item.folder?.publicId"
      :item="item"
      :busy-resource-id="busyResourceId"
      :deleting-resource-id="deletingResourceId"
      :selected="selectedResourceId === (item.file?.publicId ?? item.folder?.publicId)"
      :trash-mode="trashMode"
      @open-folder="emit('openFolder', $event)"
      @download-file="emit('downloadFile', $event)"
      @rename-item="emit('renameItem', $event)"
      @move-item="emit('moveItem', $event)"
      @overwrite-file="emit('overwriteFile', $event)"
      @delete-item="emit('deleteItem', $event)"
      @share-item="emit('shareItem', $event)"
      @restore-item="emit('restoreItem', $event)"
      @details-item="emit('detailsItem', $event)"
    />
  </div>
</template>
