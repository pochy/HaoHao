<script setup lang="ts">
import type { DriveFileBody, DriveItemBody } from '../api/generated/types.gen'
import {
  driveItemKind,
  driveItemName,
  driveItemPublicId,
  driveItemUpdatedAt,
  formatDriveDate,
  formatDriveSize,
} from '../utils/driveItems'
import DriveFileTypeIcon from './DriveFileTypeIcon.vue'
import DriveItemMenu from './DriveItemMenu.vue'

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
  <div class="drive-list" role="table" aria-label="Drive items">
    <div class="drive-list-header" role="row">
      <span role="columnheader">Name</span>
      <span role="columnheader">Type</span>
      <span role="columnheader">Size</span>
      <span role="columnheader">Updated</span>
      <span role="columnheader">Actions</span>
    </div>
    <div v-if="loading" class="drive-list-row">
      <span class="drive-list-loading">Loading Drive items...</span>
    </div>
    <div
      v-for="item in items"
      v-else
      :key="driveItemPublicId(item)"
      class="drive-list-row"
      :class="{ selected: selectedResourceId === driveItemPublicId(item) }"
      role="row"
    >
      <div class="drive-list-name" role="cell">
        <DriveFileTypeIcon :kind="driveItemKind(item)" :size="18" />
        <button
          v-if="item.folder && !trashMode"
          class="drive-name-button"
          type="button"
          @click="emit('openFolder', item.folder.publicId)"
        >
          {{ driveItemName(item) }}
        </button>
        <span v-else class="drive-file-name">{{ driveItemName(item) }}</span>
        <span class="cell-subtle monospace-cell">{{ driveItemPublicId(item) }}</span>
      </div>
      <span role="cell">{{ item.folder ? 'Folder' : driveItemKind(item) }}</span>
      <span class="tabular-cell" role="cell">{{ formatDriveSize(item.file?.byteSize) }}</span>
      <span class="tabular-cell" role="cell">{{ formatDriveDate(driveItemUpdatedAt(item)) }}</span>
      <div class="drive-list-actions" role="cell">
        <span v-if="trashMode" class="status-pill danger">Deleted</span>
        <span v-else-if="item.file?.locked" class="status-pill danger">Locked</span>
        <span v-else-if="item.file?.status" class="status-pill">{{ item.file.status }}</span>
        <DriveItemMenu
          :item="item"
          :busy-resource-id="busyResourceId"
          :deleting-resource-id="deletingResourceId"
          :trash-mode="trashMode"
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
    </div>
  </div>
</template>
