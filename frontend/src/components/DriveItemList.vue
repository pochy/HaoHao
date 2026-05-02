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
import DriveFileTypeIcon from './DriveFileTypeIcon.vue'
import DriveItemMenu from './DriveItemMenu.vue'

const props = defineProps<{
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

const { t } = useI18n()

function searchResultFor(item: DriveItemBody) {
  return props.searchResultsByResourceId?.[driveItemPublicId(item)]
}

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
  <div class="drive-list" role="table" :aria-label="t('drive.tableLabel')">
    <div class="drive-list-header" role="row">
      <span role="columnheader">{{ t('drive.select') }}</span>
      <span role="columnheader">{{ t('drive.sort.name') }}</span>
      <span role="columnheader">{{ t('drive.typeLabel') }}</span>
      <span role="columnheader">{{ t('drive.sort.size') }}</span>
      <span role="columnheader">{{ t('common.updated') }}</span>
      <span role="columnheader">{{ t('drive.actions') }}</span>
    </div>
    <div v-if="loading" class="drive-list-row">
      <span class="drive-list-loading">{{ t('drive.loadingItems') }}</span>
    </div>
    <div
      v-for="item in items"
      v-else
      :key="driveItemPublicId(item)"
      class="drive-list-row"
      :class="{ selected: selectedResourceId === driveItemPublicId(item), 'archive-selected': selectedResourceIds.includes(driveItemPublicId(item)) }"
      role="row"
    >
      <label class="drive-list-select" role="cell">
        <input
          v-if="!trashMode"
          type="checkbox"
          :checked="selectedResourceIds.includes(driveItemPublicId(item))"
          :aria-label="t('drive.selectForArchive', { name: driveItemName(item) })"
          @change="emit('toggleSelect', item)"
        >
      </label>
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
        <button
          v-else-if="item.file && !trashMode"
          class="drive-name-button"
          type="button"
          @click="emit('openFile', item.file.publicId)"
        >
          {{ driveItemName(item) }}
        </button>
        <span v-else class="drive-file-name">{{ driveItemName(item) }}</span>
        <span class="cell-subtle monospace-cell">{{ driveItemPublicId(item) }}</span>
        <span v-if="searchResultFor(item)?.snippet" class="drive-search-list-snippet">
          {{ searchResultFor(item)?.snippet }}
        </span>
        <span v-if="visibleMatches(searchResultFor(item)).length > 0" class="drive-search-list-badges">
          <span
            v-for="match in visibleMatches(searchResultFor(item))"
            :key="`${match.resourceKind}:${match.resourcePublicId}`"
            class="status-pill"
          >
            {{ searchMatchLabel(match) }}
          </span>
        </span>
      </div>
      <span role="cell">{{ item.folder ? t('drive.folder') : driveItemKind(item) }}</span>
      <span class="tabular-cell" role="cell">{{ formatDriveSize(item.file?.byteSize) }}</span>
      <span class="tabular-cell" role="cell">{{ formatDriveDate(driveItemUpdatedAt(item)) }}</span>
      <div class="drive-list-actions" role="cell">
        <span v-if="trashMode" class="status-pill danger">{{ t('drive.deleted') }}</span>
        <span v-else-if="item.file?.locked" class="status-pill danger">{{ t('drive.locked') }}</span>
        <span v-else-if="item.starredByMe" class="status-pill">{{ t('drive.starred') }}</span>
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
          @toggle-star="emit('toggleStar', $event)"
          @copy-item="emit('copyItem', $event)"
          @download-archive="emit('downloadArchive', $event)"
          @edit-metadata-item="emit('editMetadataItem', $event)"
          @preview-item="emit('previewItem', $event)"
          @permanently-delete-item="emit('permanentlyDeleteItem', $event)"
        />
      </div>
    </div>
  </div>
</template>
