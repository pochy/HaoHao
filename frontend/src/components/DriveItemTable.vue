<script setup lang="ts">
import type { DriveFileBody, DriveItemBody } from '../api/generated/types.gen'
import { labelFromDriveItem } from '../stores/drive'

defineProps<{
  items: DriveItemBody[]
  loading: boolean
  busyResourceId: string
  deletingResourceId: string
}>()

const emit = defineEmits<{
  openFolder: [folderPublicId: string]
  downloadFile: [file: DriveFileBody]
  renameItem: [item: DriveItemBody]
  moveItem: [item: DriveItemBody]
  overwriteFile: [file: DriveFileBody]
  deleteItem: [item: DriveItemBody]
  shareItem: [item: DriveItemBody]
}>()

function itemPublicId(item: DriveItemBody) {
  return item.file?.publicId ?? item.folder?.publicId ?? ''
}

function itemUpdatedAt(item: DriveItemBody) {
  return item.file?.updatedAt ?? item.folder?.updatedAt ?? ''
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
              v-if="item.folder"
              class="drive-name-button"
              type="button"
              @click="emit('openFolder', item.folder.publicId)"
            >
              {{ item.folder.name }}
            </button>
            <span v-else class="drive-file-name">{{ labelFromDriveItem(item) }}</span>
            <span class="cell-subtle monospace-cell">{{ itemPublicId(item) }}</span>
          </td>
          <td>{{ item.type }}</td>
          <td class="tabular-cell">{{ formatSize(item.file?.byteSize) }}</td>
          <td>
            <span v-if="item.file?.locked" class="status-pill danger">Locked</span>
            <span v-else-if="item.file" class="status-pill">File</span>
            <span v-else class="status-pill">Folder</span>
            <span v-if="item.file?.status" class="cell-subtle">{{ item.file.status }}</span>
          </td>
          <td class="tabular-cell">{{ formatDate(itemUpdatedAt(item)) }}</td>
          <td class="drive-actions-cell">
            <div class="drive-row-actions">
              <button
                v-if="item.file"
                class="secondary-button compact-button"
                type="button"
                :disabled="busyResourceId === item.file.publicId"
                @click="emit('downloadFile', item.file)"
              >
                Download
              </button>
              <button
                class="secondary-button compact-button"
                type="button"
                :disabled="busyResourceId === itemPublicId(item) || item.file?.locked"
                @click="emit('renameItem', item)"
              >
                Rename
              </button>
              <button
                class="secondary-button compact-button"
                type="button"
                :disabled="busyResourceId === itemPublicId(item) || item.file?.locked"
                @click="emit('moveItem', item)"
              >
                Move
              </button>
              <button
                v-if="item.file"
                class="secondary-button compact-button"
                type="button"
                :disabled="busyResourceId === item.file.publicId || item.file.locked"
                @click="emit('overwriteFile', item.file)"
              >
                Replace
              </button>
              <button
                class="secondary-button compact-button"
                type="button"
                :disabled="busyResourceId === itemPublicId(item) || item.file?.locked"
                @click="emit('shareItem', item)"
              >
                Share
              </button>
              <button
                class="secondary-button compact-button danger-button"
                type="button"
                :disabled="deletingResourceId === itemPublicId(item) || item.file?.locked"
                @click="emit('deleteItem', item)"
              >
                Delete
              </button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
