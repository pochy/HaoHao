<script setup lang="ts">
import {
  Download,
  Edit3,
  Eye,
  FolderInput,
  MoreVertical,
  RefreshCw,
  RotateCcw,
  Share2,
  Trash2,
} from 'lucide-vue-next'

import type { DriveFileBody, DriveItemBody } from '../api/generated/types.gen'
import {
  driveItemName,
  driveItemPublicId,
} from '../utils/driveItems'

const props = defineProps<{
  item: DriveItemBody
  busyResourceId: string
  deletingResourceId: string
  trashMode?: boolean
}>()

const emit = defineEmits<{
  downloadFile: [file: DriveFileBody]
  renameItem: [item: DriveItemBody]
  moveItem: [item: DriveItemBody]
  overwriteFile: [file: DriveFileBody]
  deleteItem: [item: DriveItemBody]
  shareItem: [item: DriveItemBody]
  restoreItem: [item: DriveItemBody]
  detailsItem: [item: DriveItemBody]
}>()

function itemBusy() {
  return props.busyResourceId === driveItemPublicId(props.item)
}

function deleteBusy() {
  return props.deletingResourceId === driveItemPublicId(props.item)
}
</script>

<template>
  <details class="drive-item-menu">
    <summary :aria-label="`Actions for ${driveItemName(item)}`" title="Actions">
      <MoreVertical :size="17" stroke-width="1.9" aria-hidden="true" />
    </summary>
    <div class="drive-item-menu-popover" role="menu">
      <button
        v-if="trashMode"
        type="button"
        role="menuitem"
        :disabled="itemBusy()"
        @click="emit('restoreItem', item)"
      >
        <RotateCcw :size="16" stroke-width="1.8" aria-hidden="true" />
        Restore
      </button>
      <template v-else>
        <button
          v-if="item.file"
          type="button"
          role="menuitem"
          :disabled="busyResourceId === item.file.publicId"
          @click="emit('downloadFile', item.file)"
        >
          <Download :size="16" stroke-width="1.8" aria-hidden="true" />
          Download
        </button>
        <button
          type="button"
          role="menuitem"
          :disabled="itemBusy() || item.file?.locked"
          @click="emit('shareItem', item)"
        >
          <Share2 :size="16" stroke-width="1.8" aria-hidden="true" />
          Share
        </button>
        <button
          type="button"
          role="menuitem"
          :disabled="itemBusy() || item.file?.locked"
          @click="emit('renameItem', item)"
        >
          <Edit3 :size="16" stroke-width="1.8" aria-hidden="true" />
          Rename
        </button>
        <button
          type="button"
          role="menuitem"
          :disabled="itemBusy() || item.file?.locked"
          @click="emit('moveItem', item)"
        >
          <FolderInput :size="16" stroke-width="1.8" aria-hidden="true" />
          Move
        </button>
        <button
          v-if="item.file"
          type="button"
          role="menuitem"
          :disabled="busyResourceId === item.file.publicId || item.file.locked"
          @click="emit('overwriteFile', item.file)"
        >
          <RefreshCw :size="16" stroke-width="1.8" aria-hidden="true" />
          Replace
        </button>
        <button
          class="danger"
          type="button"
          role="menuitem"
          :disabled="deleteBusy() || item.file?.locked"
          @click="emit('deleteItem', item)"
        >
          <Trash2 :size="16" stroke-width="1.8" aria-hidden="true" />
          Delete
        </button>
      </template>
      <button type="button" role="menuitem" @click="emit('detailsItem', item)">
        <Eye :size="16" stroke-width="1.8" aria-hidden="true" />
        Details
      </button>
    </div>
  </details>
</template>
