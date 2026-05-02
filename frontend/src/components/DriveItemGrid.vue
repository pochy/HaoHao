<script setup lang="ts">
import type { DriveFileBody, DriveItemBody, DriveSearchResultBody } from '../api/generated/types.gen'
import DriveItemCard from './DriveItemCard.vue'

defineProps<{
  items: DriveItemBody[]
  loading: boolean
  busyResourceId: string
  deletingResourceId: string
  selectedResourceId: string
  selectedResourceIds: string[]
  searchResultsByResourceId?: Record<string, DriveSearchResultBody>
  trashMode?: boolean
}>()

const emit = defineEmits<{
  openFolder: [folderPublicId: string]
  openFile: [filePublicId: string]
  downloadFile: [file: DriveFileBody]
  renameItem: [item: DriveItemBody]
  moveItem: [item: DriveItemBody]
  overwriteFile: [file: DriveFileBody]
  deleteItem: [item: DriveItemBody]
  shareItem: [item: DriveItemBody]
  restoreItem: [item: DriveItemBody]
  detailsItem: [item: DriveItemBody]
  toggleStar: [item: DriveItemBody]
  copyItem: [item: DriveItemBody]
  downloadArchive: [item: DriveItemBody]
  editMetadataItem: [item: DriveItemBody]
  previewItem: [item: DriveItemBody]
  toggleSelect: [item: DriveItemBody]
  permanentlyDeleteItem: [item: DriveItemBody]
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
      :selected-for-archive="selectedResourceIds.includes(item.file?.publicId ?? item.folder?.publicId ?? '')"
      :search-result="searchResultsByResourceId?.[item.file?.publicId ?? item.folder?.publicId ?? '']"
      :trash-mode="trashMode"
      @open-folder="emit('openFolder', $event)"
      @open-file="emit('openFile', $event)"
      @download-file="emit('downloadFile', $event)"
      @rename-item="emit('renameItem', $event)"
      @move-item="emit('moveItem', $event)"
      @overwrite-file="emit('overwriteFile', $event)"
      @delete-item="emit('deleteItem', $event)"
      @share-item="emit('shareItem', $event)"
      @restore-item="emit('restoreItem', $event)"
      @details-item="emit('detailsItem', $event)"
      @toggle-star="emit('toggleStar', $event)"
      @copy-item="emit('copyItem', $event)"
      @download-archive="emit('downloadArchive', $event)"
      @edit-metadata-item="emit('editMetadataItem', $event)"
      @preview-item="emit('previewItem', $event)"
      @toggle-select="emit('toggleSelect', $event)"
      @permanently-delete-item="emit('permanentlyDeleteItem', $event)"
    />
  </div>
</template>
