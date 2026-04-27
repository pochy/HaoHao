<script setup lang="ts">
import {
  ChevronRight,
  Clock3,
  FolderTree,
  HardDrive,
  Plus,
  Share2,
  Star,
  Trash2,
  Upload,
} from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { DriveFolderBody, DriveFolderTreeBody, DriveFolderTreeNodeBody, DriveItemBody, DriveStorageUsageBody } from '../api/generated/types.gen'
import {
  driveItemName,
  driveItemPublicId,
} from '../utils/driveItems'
import DriveFileTypeIcon from './DriveFileTypeIcon.vue'

const props = defineProps<{
  currentFolder: DriveFolderBody
  children: DriveItemBody[]
  folderTree: DriveFolderTreeBody | null
  activeView: 'my-drive' | 'search' | 'shared' | 'starred' | 'recent' | 'storage' | 'trash' | 'groups'
  workspaceName: string
  storageUsage: DriveStorageUsageBody | null
  disabled: boolean
  busy: boolean
}>()

const { n, t } = useI18n()

const emit = defineEmits<{
  createFolder: []
  uploadFile: [file: File]
  uploadFiles: [files: File[]]
  openFolder: [folderPublicId: string]
}>()

type FolderTreeRow = DriveFolderTreeNodeBody & {
  depth: number
  section: 'owned' | 'shared'
}

const folderItems = computed(() => props.children.filter((item) => item.folder))
const manuallyExpandedIds = ref<Set<string>>(new Set())
const expandedIds = computed(() => {
  const ids = new Set(manuallyExpandedIds.value)
  ids.add(props.currentFolder.publicId)
  return ids
})
const storagePercent = computed(() => {
  const quota = props.storageUsage?.quotaBytes ?? 0
  if (quota <= 0) {
    return 0
  }
  return Math.min(100, Math.round(((props.storageUsage?.usedBytes ?? 0) / quota) * 100))
})

const treeRows = computed(() => [
  ...flattenTree(props.folderTree?.ownedRoots ?? [], 'owned', 0),
  ...flattenTree(props.folderTree?.sharedRoots ?? [], 'shared', 0),
])
const storageLabel = computed(() => {
  const used = props.storageUsage?.usedBytes ?? 0
  const count = props.storageUsage?.fileCount ?? 0
  return t('drive.storageLabel', {
    bytes: n(used, 'integer'),
    count,
  })
})

watch(
  () => props.currentFolder.publicId,
  (publicId) => {
    if (!publicId) {
      return
    }
    const next = new Set(manuallyExpandedIds.value)
    next.add(publicId)
    manuallyExpandedIds.value = next
  },
  { immediate: true },
)

function onFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files ?? [])
  if (files.length === 1 && files[0]) {
    emit('uploadFile', files[0])
  } else if (files.length > 1) {
    emit('uploadFiles', files)
  }
  target.value = ''
}

function flattenTree(nodes: DriveFolderTreeNodeBody[], section: 'owned' | 'shared', depth: number): FolderTreeRow[] {
  const rows: FolderTreeRow[] = []
  for (const node of nodes) {
    rows.push({ ...node, depth, section })
    if (expandedIds.value.has(node.publicId)) {
      rows.push(...flattenTree(node.children ?? [], section, depth + 1))
    }
  }
  return rows
}

function toggleTreeFolder(publicId: string) {
  const next = new Set(manuallyExpandedIds.value)
  if (next.has(publicId)) {
    next.delete(publicId)
  } else {
    next.add(publicId)
  }
  manuallyExpandedIds.value = next
}

function hasChildren(row: FolderTreeRow) {
  return (row.children?.length ?? 0) > 0
}
</script>

<template>
  <aside class="drive-side-nav" :aria-label="t('drive.navigation')">
    <div class="drive-create-stack">
      <button class="primary-button drive-new-button" type="button" :disabled="disabled || busy" @click="emit('createFolder')">
        <Plus :size="18" stroke-width="2" aria-hidden="true" />
        {{ t('drive.newFolder') }}
      </button>
      <label class="secondary-button compact-button drive-upload-button">
        <Upload :size="16" stroke-width="1.8" aria-hidden="true" />
        <span>{{ t('drive.uploadFile') }}</span>
        <input class="drive-hidden-input" type="file" multiple :disabled="disabled || busy" @change="onFileChange">
      </label>
    </div>

    <nav class="drive-local-nav">
      <RouterLink class="drive-local-link" :class="{ active: activeView === 'my-drive' }" to="/drive">
        <HardDrive :size="17" stroke-width="1.9" aria-hidden="true" />
        {{ t('drive.myDrive') }}
      </RouterLink>
      <RouterLink class="drive-local-link" :class="{ active: activeView === 'recent' }" to="/drive/recent">
        <Clock3 :size="17" stroke-width="1.9" aria-hidden="true" />
        {{ t('drive.recent') }}
      </RouterLink>
      <RouterLink class="drive-local-link" :class="{ active: activeView === 'shared' }" to="/drive/shared">
        <Share2 :size="17" stroke-width="1.9" aria-hidden="true" />
        {{ t('drive.sharedWithMe') }}
      </RouterLink>
      <RouterLink class="drive-local-link" :class="{ active: activeView === 'starred' }" to="/drive/starred">
        <Star :size="17" stroke-width="1.9" aria-hidden="true" />
        {{ t('drive.starred') }}
      </RouterLink>
      <RouterLink class="drive-local-link" :class="{ active: activeView === 'search' }" to="/drive/search">
        <Clock3 :size="17" stroke-width="1.9" aria-hidden="true" />
        {{ t('common.search') }}
      </RouterLink>
      <RouterLink class="drive-local-link" :class="{ active: activeView === 'trash' }" to="/drive/trash">
        <Trash2 :size="17" stroke-width="1.9" aria-hidden="true" />
        {{ t('drive.trash') }}
      </RouterLink>
    </nav>

    <section class="drive-folder-tree" :aria-label="t('drive.folderTree')">
      <div class="drive-side-heading">
        <FolderTree :size="15" stroke-width="1.9" aria-hidden="true" />
        <span>{{ workspaceName }}</span>
      </div>
      <button class="drive-tree-row active" type="button" @click="emit('openFolder', currentFolder.publicId)">
        <DriveFileTypeIcon kind="folder" :size="16" />
        <span>{{ currentFolder.publicId === 'root' ? t('drive.root') : currentFolder.name }}</span>
      </button>
      <template v-if="treeRows.length > 0">
        <div class="drive-tree-section-label">{{ t('drive.ownedFolders') }}</div>
        <div
          v-for="row in treeRows.filter((item) => item.section === 'owned')"
          :key="`owned:${row.publicId}`"
          class="drive-tree-row"
          :class="{ active: row.publicId === currentFolder.publicId }"
          :style="{ paddingLeft: `${12 + row.depth * 14}px` }"
        >
          <button
            class="drive-tree-expander"
            type="button"
            :aria-label="expandedIds.has(row.publicId) ? t('drive.collapseFolder', { name: row.name }) : t('drive.expandFolder', { name: row.name })"
            :disabled="!hasChildren(row)"
            @click="toggleTreeFolder(row.publicId)"
          >
            <ChevronRight :size="13" stroke-width="1.9" aria-hidden="true" :class="{ expanded: expandedIds.has(row.publicId), muted: !hasChildren(row) }" />
          </button>
          <button class="drive-tree-link" type="button" @click="emit('openFolder', row.publicId)">
            <DriveFileTypeIcon kind="folder" :size="16" />
            <span>{{ row.name }}</span>
          </button>
        </div>
        <div v-if="treeRows.some((item) => item.section === 'shared')" class="drive-tree-section-label">{{ t('drive.sharedRoots') }}</div>
        <div
          v-for="row in treeRows.filter((item) => item.section === 'shared')"
          :key="`shared:${row.publicId}`"
          class="drive-tree-row"
          :class="{ active: row.publicId === currentFolder.publicId }"
          :style="{ paddingLeft: `${12 + row.depth * 14}px` }"
        >
          <button
            class="drive-tree-expander"
            type="button"
            :aria-label="expandedIds.has(row.publicId) ? t('drive.collapseFolder', { name: row.name }) : t('drive.expandFolder', { name: row.name })"
            :disabled="!hasChildren(row)"
            @click="toggleTreeFolder(row.publicId)"
          >
            <ChevronRight :size="13" stroke-width="1.9" aria-hidden="true" :class="{ expanded: expandedIds.has(row.publicId), muted: !hasChildren(row) }" />
          </button>
          <button class="drive-tree-link" type="button" @click="emit('openFolder', row.publicId)">
            <DriveFileTypeIcon kind="folder" :size="16" />
            <span>{{ row.name }}</span>
          </button>
        </div>
      </template>
      <template v-else>
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
      </template>
      <p v-if="folderItems.length === 0" class="cell-subtle">
        {{ t('drive.noChildFolders') }}
      </p>
    </section>

    <RouterLink class="drive-storage-summary" :class="{ active: activeView === 'storage' }" to="/drive/storage">
      <div>
        <strong>{{ t('drive.storage') }}</strong>
        <span>{{ storageLabel }}</span>
      </div>
      <div class="drive-storage-bar" :aria-label="t('drive.storageUsed', { percent: storagePercent })">
        <span :style="{ width: `${storagePercent}%` }" />
      </div>
    </RouterLink>
  </aside>
</template>
