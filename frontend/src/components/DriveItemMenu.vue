<script setup lang="ts">
import {
  Copy,
  Download,
  Edit3,
  Eye,
  FolderInput,
  MoreVertical,
  RefreshCw,
  RotateCcw,
  Share2,
  Star,
  Trash2,
} from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

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

const { t } = useI18n()

const emit = defineEmits<{
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
  permanentlyDeleteItem: [item: DriveItemBody]
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
    <summary :aria-label="t('drive.actionsFor', { name: driveItemName(item) })" :title="t('drive.actions')">
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
        {{ t('drive.restore') }}
      </button>
      <button
        v-if="trashMode"
        class="danger"
        type="button"
        role="menuitem"
        :disabled="deleteBusy()"
        @click="emit('permanentlyDeleteItem', item)"
      >
        <Trash2 :size="16" stroke-width="1.8" aria-hidden="true" />
        {{ t('drive.deletePermanently') }}
      </button>
      <template v-else>
        <button
          v-if="item.file"
          type="button"
          role="menuitem"
          :disabled="busyResourceId === item.file.publicId"
          @click="emit('previewItem', item)"
        >
          <Eye :size="16" stroke-width="1.8" aria-hidden="true" />
          {{ t('drive.preview') }}
        </button>
        <button
          v-if="item.file"
          type="button"
          role="menuitem"
          :disabled="busyResourceId === item.file.publicId"
          @click="emit('downloadFile', item.file)"
        >
          <Download :size="16" stroke-width="1.8" aria-hidden="true" />
          {{ t('common.download') }}
        </button>
        <button
          type="button"
          role="menuitem"
          :disabled="itemBusy()"
          @click="emit('downloadArchive', item)"
        >
          <Download :size="16" stroke-width="1.8" aria-hidden="true" />
          {{ t('drive.downloadZip') }}
        </button>
        <button
          type="button"
          role="menuitem"
          :disabled="itemBusy()"
          @click="emit('toggleStar', item)"
        >
          <Star :size="16" stroke-width="1.8" aria-hidden="true" />
          {{ item.starredByMe ? t('drive.removeStar') : t('drive.addStar') }}
        </button>
        <button
          type="button"
          role="menuitem"
          :disabled="itemBusy() || item.file?.locked"
          @click="emit('copyItem', item)"
        >
          <Copy :size="16" stroke-width="1.8" aria-hidden="true" />
          {{ t('drive.copy') }}
        </button>
        <button
          type="button"
          role="menuitem"
          :disabled="itemBusy() || item.file?.locked"
          @click="emit('shareItem', item)"
        >
          <Share2 :size="16" stroke-width="1.8" aria-hidden="true" />
          {{ t('drive.share') }}
        </button>
        <button
          type="button"
          role="menuitem"
          :disabled="itemBusy() || item.file?.locked"
          @click="emit('renameItem', item)"
        >
          <Edit3 :size="16" stroke-width="1.8" aria-hidden="true" />
          {{ t('drive.rename') }}
        </button>
        <button
          type="button"
          role="menuitem"
          :disabled="itemBusy() || item.file?.locked"
          @click="emit('editMetadataItem', item)"
        >
          <Edit3 :size="16" stroke-width="1.8" aria-hidden="true" />
          {{ t('drive.metadata') }}
        </button>
        <button
          type="button"
          role="menuitem"
          :disabled="itemBusy() || item.file?.locked"
          @click="emit('moveItem', item)"
        >
          <FolderInput :size="16" stroke-width="1.8" aria-hidden="true" />
          {{ t('drive.move') }}
        </button>
        <button
          v-if="item.file"
          type="button"
          role="menuitem"
          :disabled="busyResourceId === item.file.publicId || item.file.locked"
          @click="emit('overwriteFile', item.file)"
        >
          <RefreshCw :size="16" stroke-width="1.8" aria-hidden="true" />
          {{ t('drive.replace') }}
        </button>
        <button
          class="danger"
          type="button"
          role="menuitem"
          :disabled="deleteBusy() || item.file?.locked"
          @click="emit('deleteItem', item)"
        >
          <Trash2 :size="16" stroke-width="1.8" aria-hidden="true" />
          {{ t('common.delete') }}
        </button>
      </template>
      <button type="button" role="menuitem" @click="emit('detailsItem', item)">
        <Eye :size="16" stroke-width="1.8" aria-hidden="true" />
        {{ t('common.details') }}
      </button>
    </div>
  </details>
</template>
