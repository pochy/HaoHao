<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { DriveFileBody, DriveItemBody, DriveSearchResultBody, DriveSearchResultMatchBody } from '../api/generated/types.gen'
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
  selectedForArchive?: boolean
  searchResult?: DriveSearchResultBody
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

const { t } = useI18n()

function searchMatchLabel(match: DriveSearchResultMatchBody) {
  switch (match.resourceKind) {
    case 'ocr_run':
      return t('drive.searchMatch.ocr')
    case 'product_extraction':
      return t('drive.searchMatch.product')
    case 'gold_table':
      return t('drive.searchMatch.gold')
    default:
      return t('drive.searchMatch.file')
  }
}

function visibleMatches(result?: DriveSearchResultBody) {
  return result?.matches?.slice(0, 3) ?? []
}
</script>

<template>
  <article class="drive-item-card" :class="{ selected, 'archive-selected': selectedForArchive }">
    <label v-if="!trashMode" class="drive-select-checkbox">
      <input
        type="checkbox"
        :checked="selectedForArchive"
        :aria-label="t('drive.selectForArchive', { name: driveItemName(item) })"
        @change="emit('toggleSelect', item)"
      >
    </label>
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
    <button
      v-else-if="item.file && !trashMode"
      class="drive-item-card-open"
      type="button"
      @click="emit('openFile', item.file.publicId)"
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
      <span>{{ trashMode ? t('drive.deletedFolder') : t('drive.folder') }}</span>
    </div>

    <div v-if="searchResult?.snippet || visibleMatches(searchResult).length > 0" class="drive-search-summary">
      <p v-if="searchResult?.snippet" class="drive-search-snippet">{{ searchResult.snippet }}</p>
      <div v-if="visibleMatches(searchResult).length > 0" class="drive-search-badges">
        <span
          v-for="match in visibleMatches(searchResult)"
          :key="`${match.resourceKind}:${match.resourcePublicId}`"
          class="status-pill"
        >
          {{ searchMatchLabel(match) }}
        </span>
      </div>
    </div>

    <footer class="drive-item-card-meta">
      <span>{{ formatDriveDate(driveItemUpdatedAt(item)) }}</span>
      <span>{{ item.file ? formatDriveSize(item.file.byteSize) : t('drive.folder') }}</span>
      <span v-if="item.file?.locked" class="status-pill danger">{{ t('drive.locked') }}</span>
      <span v-else-if="item.starredByMe" class="status-pill">{{ t('drive.starred') }}</span>
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
      @toggle-star="emit('toggleStar', $event)"
      @copy-item="emit('copyItem', $event)"
      @download-archive="emit('downloadArchive', $event)"
      @edit-metadata-item="emit('editMetadataItem', $event)"
      @preview-item="emit('previewItem', $event)"
      @permanently-delete-item="emit('permanentlyDeleteItem', $event)"
    />
    <span class="drive-item-public-id monospace-cell">{{ driveItemPublicId(item) }}</span>
  </article>
</template>
