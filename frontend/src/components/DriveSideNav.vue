<script setup lang="ts">
import {
  Clock3,
  FolderTree,
  HardDrive,
  Plus,
  Share2,
  Star,
  Trash2,
  Upload,
} from 'lucide-vue-next'
import { computed } from 'vue'

import type { DriveFolderBody, DriveItemBody } from '../api/generated/types.gen'
import {
  driveItemName,
  driveItemPublicId,
} from '../utils/driveItems'
import DriveFileTypeIcon from './DriveFileTypeIcon.vue'

const props = defineProps<{
  currentFolder: DriveFolderBody
  children: DriveItemBody[]
  activeView: 'my-drive' | 'search' | 'trash' | 'groups'
  workspaceName: string
  disabled: boolean
  busy: boolean
}>()

const emit = defineEmits<{
  createFolder: []
  uploadFile: [file: File]
  openFolder: [folderPublicId: string]
}>()

const folderItems = computed(() => props.children.filter((item) => item.folder))

function onFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (file) {
    emit('uploadFile', file)
  }
  target.value = ''
}
</script>

<template>
  <aside class="drive-side-nav" aria-label="Drive navigation">
    <div class="drive-create-stack">
      <button class="primary-button drive-new-button" type="button" :disabled="disabled || busy" @click="emit('createFolder')">
        <Plus :size="18" stroke-width="2" aria-hidden="true" />
        New folder
      </button>
      <label class="secondary-button compact-button drive-upload-button">
        <Upload :size="16" stroke-width="1.8" aria-hidden="true" />
        <span>Upload file</span>
        <input class="drive-hidden-input" type="file" :disabled="disabled || busy" @change="onFileChange">
      </label>
    </div>

    <nav class="drive-local-nav">
      <RouterLink class="drive-local-link" :class="{ active: activeView === 'my-drive' }" to="/drive">
        <HardDrive :size="17" stroke-width="1.9" aria-hidden="true" />
        My Drive
      </RouterLink>
      <RouterLink class="drive-local-link" :class="{ active: activeView === 'search' }" to="/drive/search">
        <Clock3 :size="17" stroke-width="1.9" aria-hidden="true" />
        Recent / Search
      </RouterLink>
      <button class="drive-local-link muted" type="button" disabled>
        <Share2 :size="17" stroke-width="1.9" aria-hidden="true" />
        Shared with me
      </button>
      <button class="drive-local-link muted" type="button" disabled>
        <Star :size="17" stroke-width="1.9" aria-hidden="true" />
        Starred
      </button>
      <RouterLink class="drive-local-link" :class="{ active: activeView === 'trash' }" to="/drive/trash">
        <Trash2 :size="17" stroke-width="1.9" aria-hidden="true" />
        Trash
      </RouterLink>
    </nav>

    <section class="drive-folder-tree" aria-label="Current folder tree">
      <div class="drive-side-heading">
        <FolderTree :size="15" stroke-width="1.9" aria-hidden="true" />
        <span>{{ workspaceName }}</span>
      </div>
      <button class="drive-tree-row active" type="button" @click="emit('openFolder', currentFolder.publicId)">
        <DriveFileTypeIcon kind="folder" :size="16" />
        <span>{{ currentFolder.publicId === 'root' ? 'Root' : currentFolder.name }}</span>
      </button>
      <button
        v-for="item in folderItems"
        :key="driveItemPublicId(item)"
        class="drive-tree-row"
        type="button"
        @click="item.folder && emit('openFolder', item.folder.publicId)"
      >
        <DriveFileTypeIcon kind="folder" :size="16" />
        <span>{{ driveItemName(item) }}</span>
      </button>
      <p v-if="folderItems.length === 0" class="cell-subtle">
        No child folders in this view.
      </p>
    </section>

    <section class="drive-storage-summary">
      <div>
        <strong>Storage</strong>
        <span>Quota provider pending</span>
      </div>
      <div class="drive-storage-bar" aria-hidden="true">
        <span />
      </div>
    </section>
  </aside>
</template>
