<script setup lang="ts">
import {
  Download,
  Edit3,
  FolderInput,
  RefreshCw,
  RotateCcw,
  Share2,
  Trash2,
} from 'lucide-vue-next'

import type { DriveFileBody, DriveItemBody } from '../api/generated/types.gen'
import { labelFromDriveItem } from '../stores/drive'
import IconButton from './IconButton.vue'

defineProps<{
  items: DriveItemBody[]
  loading: boolean
  busyResourceId: string
  deletingResourceId: string
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
}>()

function itemPublicId(item: DriveItemBody) {
  return item.file?.publicId ?? item.folder?.publicId ?? ''
}

function itemUpdatedAt(item: DriveItemBody) {
  return item.file?.updatedAt ?? item.folder?.updatedAt ?? ''
}

function itemDeletedAt(item: DriveItemBody) {
  return item.file?.deletedAt ?? item.folder?.deletedAt ?? ''
}

function formatDate(value: string) {
  if (!value) {
    return '-'
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

function formatSize(value?: number) {
  if (value === undefined) {
    return '-'
  }
  return new Intl.NumberFormat(undefined, {
    style: 'unit',
    unit: 'byte',
    unitDisplay: 'narrow',
  }).format(value)
}
</script>

<template>
  <div class="admin-table drive-table">
    <table>
      <thead>
        <tr>
          <th scope="col">Name</th>
          <th scope="col">Type</th>
          <th scope="col">Size</th>
          <th scope="col">State</th>
          <th scope="col">Updated</th>
          <th scope="col" class="drive-actions-cell">Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="loading">
          <td colspan="6">Loading Drive items...</td>
        </tr>
        <tr v-for="item in items" v-else :key="itemPublicId(item)">
          <td>
            <button
              v-if="item.folder && !trashMode"
              class="drive-name-button"
              type="button"
              @click="emit('openFolder', item.folder.publicId)"
            >
              {{ item.folder.name }}
            </button>
            <span v-else-if="item.folder" class="drive-file-name">{{ item.folder.name }}</span>
            <span v-else class="drive-file-name">{{ labelFromDriveItem(item) }}</span>
            <span class="cell-subtle monospace-cell">{{ itemPublicId(item) }}</span>
          </td>
          <td>{{ item.type }}</td>
          <td class="tabular-cell">{{ formatSize(item.file?.byteSize) }}</td>
          <td>
            <span v-if="trashMode" class="status-pill danger">Deleted</span>
            <span v-else-if="item.file?.locked" class="status-pill danger">Locked</span>
            <span v-else-if="item.file" class="status-pill">File</span>
            <span v-else class="status-pill">Folder</span>
            <span v-if="item.file?.status" class="cell-subtle">{{ item.file.status }}</span>
          </td>
          <td class="tabular-cell">{{ formatDate(trashMode ? itemDeletedAt(item) : itemUpdatedAt(item)) }}</td>
          <td class="drive-actions-cell">
            <div v-if="trashMode" class="drive-row-actions">
              <IconButton
                label="Restore"
                :disabled="busyResourceId === itemPublicId(item)"
                @click="emit('restoreItem', item)"
              >
                <RotateCcw :size="16" stroke-width="1.8" aria-hidden="true" />
              </IconButton>
            </div>
            <div v-else class="drive-row-actions">
              <IconButton
                v-if="item.file"
                label="Download"
                :disabled="busyResourceId === item.file.publicId"
                @click="emit('downloadFile', item.file)"
              >
                <Download :size="16" stroke-width="1.8" aria-hidden="true" />
              </IconButton>
              <IconButton
                label="Rename"
                :disabled="busyResourceId === itemPublicId(item) || item.file?.locked"
                @click="emit('renameItem', item)"
              >
                <Edit3 :size="16" stroke-width="1.8" aria-hidden="true" />
              </IconButton>
              <IconButton
                label="Move"
                :disabled="busyResourceId === itemPublicId(item) || item.file?.locked"
                @click="emit('moveItem', item)"
              >
                <FolderInput :size="16" stroke-width="1.8" aria-hidden="true" />
              </IconButton>
              <IconButton
                v-if="item.file"
                label="Replace"
                :disabled="busyResourceId === item.file.publicId || item.file.locked"
                @click="emit('overwriteFile', item.file)"
              >
                <RefreshCw :size="16" stroke-width="1.8" aria-hidden="true" />
              </IconButton>
              <IconButton
                label="Share"
                :disabled="busyResourceId === itemPublicId(item) || item.file?.locked"
                @click="emit('shareItem', item)"
              >
                <Share2 :size="16" stroke-width="1.8" aria-hidden="true" />
              </IconButton>
              <IconButton
                label="Delete"
                variant="danger"
                :disabled="deletingResourceId === itemPublicId(item) || item.file?.locked"
                @click="emit('deleteItem', item)"
              >
                <Trash2 :size="16" stroke-width="1.8" aria-hidden="true" />
              </IconButton>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
