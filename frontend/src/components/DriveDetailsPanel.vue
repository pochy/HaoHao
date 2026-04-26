<script setup lang="ts">
import { computed, ref } from 'vue'
import { Info, Lock, ShieldCheck, X } from 'lucide-vue-next'

import type { DriveFolderBody, DriveItemBody, DrivePermissionsBody } from '../api/generated/types.gen'
import {
  driveItemContentType,
  driveItemName,
  driveItemPublicId,
  driveItemUpdatedAt,
  formatDriveDate,
  formatDriveSize,
} from '../utils/driveItems'

const props = defineProps<{
  open: boolean
  selectedItem: DriveItemBody | null
  currentFolder: DriveFolderBody
  permissions: DrivePermissionsBody | null
  itemCount: number
  fileCount: number
  folderCount: number
}>()

const emit = defineEmits<{
  close: []
  shareItem: [item: DriveItemBody]
}>()

const activeTab = ref<'details' | 'activity' | 'permissions'>('details')

const title = computed(() => (
  props.selectedItem ? driveItemName(props.selectedItem) : props.currentFolder.publicId === 'root' ? 'Root' : props.currentFolder.name
))
const directCount = computed(() => props.permissions?.direct?.length ?? 0)
const inheritedCount = computed(() => props.permissions?.inherited?.length ?? 0)
</script>

<template>
  <aside v-if="open" class="drive-details-panel" aria-label="Drive details panel">
    <header class="drive-details-header">
      <div>
        <span class="status-pill">Details</span>
        <h2>{{ title }}</h2>
      </div>
      <button class="icon-button" type="button" aria-label="Close details panel" title="Close details panel" @click="emit('close')">
        <X :size="17" stroke-width="1.9" aria-hidden="true" />
      </button>
    </header>

    <div class="drive-details-tabs" role="tablist" aria-label="Drive details sections">
      <button type="button" :class="{ active: activeTab === 'details' }" @click="activeTab = 'details'">
        <Info :size="15" stroke-width="1.9" aria-hidden="true" />
        Details
      </button>
      <button type="button" :class="{ active: activeTab === 'activity' }" @click="activeTab = 'activity'">
        <Lock :size="15" stroke-width="1.9" aria-hidden="true" />
        Activity
      </button>
      <button type="button" :class="{ active: activeTab === 'permissions' }" @click="activeTab = 'permissions'">
        <ShieldCheck :size="15" stroke-width="1.9" aria-hidden="true" />
        Permissions
      </button>
    </div>

    <div v-if="activeTab === 'details'" class="drive-details-section">
      <dl class="metadata-grid compact">
        <div>
          <dt>Public ID</dt>
          <dd class="monospace-cell">{{ selectedItem ? driveItemPublicId(selectedItem) : currentFolder.publicId }}</dd>
        </div>
        <div>
          <dt>Type</dt>
          <dd>{{ selectedItem?.folder ? 'Folder' : selectedItem?.file ? 'File' : 'Folder context' }}</dd>
        </div>
        <div v-if="selectedItem?.file">
          <dt>Size</dt>
          <dd>{{ formatDriveSize(selectedItem.file.byteSize) }}</dd>
        </div>
        <div v-if="selectedItem?.file">
          <dt>Content type</dt>
          <dd>{{ driveItemContentType(selectedItem) || '-' }}</dd>
        </div>
        <div>
          <dt>Updated</dt>
          <dd>{{ selectedItem ? formatDriveDate(driveItemUpdatedAt(selectedItem)) : formatDriveDate(currentFolder.updatedAt) }}</dd>
        </div>
        <div v-if="selectedItem?.file?.scanStatus">
          <dt>Scan</dt>
          <dd>{{ selectedItem.file.scanStatus }}</dd>
        </div>
      </dl>

      <div v-if="!selectedItem" class="drive-details-summary">
        <span>{{ itemCount }} items</span>
        <span>{{ fileCount }} files</span>
        <span>{{ folderCount }} folders</span>
      </div>
      <button v-if="selectedItem" class="primary-button compact-button" type="button" @click="emit('shareItem', selectedItem)">
        Share item
      </button>
    </div>

    <div v-else-if="activeTab === 'activity'" class="drive-details-section">
      <p class="cell-subtle">
        Activity feed will connect to Drive audit events in a later API phase.
      </p>
    </div>

    <div v-else class="drive-details-section">
      <div class="drive-details-summary">
        <span>{{ directCount }} direct</span>
        <span>{{ inheritedCount }} inherited</span>
      </div>
      <p class="cell-subtle">
        Open the share dialog to manage people, groups, links, and inherited access.
      </p>
      <button v-if="selectedItem" class="secondary-button compact-button" type="button" @click="emit('shareItem', selectedItem)">
        Manage access
      </button>
    </div>
  </aside>
</template>
