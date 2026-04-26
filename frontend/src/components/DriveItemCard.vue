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
import DriveFileThumbnail from './DriveFileThumbnail.vue'
import DriveFileTypeIcon from './DriveFileTypeIcon.vue'
import DriveItemMenu from './DriveItemMenu.vue'

defineProps<{
  item: DriveItemBody
  busyResourceId: string
  deletingResourceId: string
  selected?: boolean
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
  <article class="drive-item-card" :class="{ selected }">
    <button
      v-if="item.folder && !trashMode"
      class="drive-item-card-open"
      type="button"
      @click="emit('openFolder', item.folder.publicId)"
    >
      <span class="drive-item-card-heading">
        <DriveFileTypeIcon :kind="driveItemKind(item)" :size="18" />
        <span>{{ driveItemName(item) }}</span>
      </span>
    </button>
    <div v-else class="drive-item-card-heading">
      <DriveFileTypeIcon :kind="driveItemKind(item)" :size="18" />
      <span>{{ driveItemName(item) }}</span>
    </div>

    <DriveFileThumbnail v-if="item.file" :item="item" />
    <div v-else class="drive-folder-card-body">
      <DriveFileTypeIcon :kind="driveItemKind(item)" :size="34" />
      <span>{{ trashMode ? 'Deleted folder' : 'Folder' }}</span>
    </div>

    <footer class="drive-item-card-meta">
      <span>{{ formatDriveDate(driveItemUpdatedAt(item)) }}</span>
      <span>{{ item.file ? formatDriveSize(item.file.byteSize) : 'Folder' }}</span>
      <span v-if="item.file?.locked" class="status-pill danger">Locked</span>
      <span v-else-if="item.file?.status" class="status-pill">{{ item.file.status }}</span>
    </footer>

    <DriveItemMenu
      class="drive-item-card-menu"
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
    <span class="drive-item-public-id monospace-cell">{{ driveItemPublicId(item) }}</span>
  </article>
</template>
