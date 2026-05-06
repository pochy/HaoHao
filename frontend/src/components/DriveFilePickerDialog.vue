<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { FilePlus2, Folder, Search, Trash2, X } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import {
  fetchDriveFile,
  fetchDriveFolder,
  fetchDriveItems,
  fetchDriveRecentItems,
  searchDriveItemsByKeyword,
} from '../api/drive'
import type { DriveFileBody, DriveFolderBody, DriveItemBody } from '../api/generated/types.gen'
import { driveItemContentType, driveItemName } from '../utils/driveItems'
import DriveFileTypeIcon from './DriveFileTypeIcon.vue'

type SelectedDriveFile = {
  publicId: string
  file: DriveFileBody | null
}

const props = withDefaults(defineProps<{
  open: boolean
  selectedFileIds: string[]
  multiple?: boolean
  spreadsheetMode?: boolean
}>(), {
  multiple: false,
  spreadsheetMode: false,
})

const emit = defineEmits<{
  close: []
  apply: [filePublicIds: string[]]
}>()

const { t } = useI18n()
const dialogRef = ref<HTMLDialogElement | null>(null)
const query = ref('')
const currentFolderPublicId = ref('')
const currentFolder = ref<DriveFolderBody | null>(null)
const folderItems = ref<DriveItemBody[]>([])
const recentItems = ref<DriveItemBody[]>([])
const searchResults = ref<DriveItemBody[]>([])
const selectedItems = ref<SelectedDriveFile[]>([])
const loading = ref(false)
const selectedLoading = ref(false)
const errorMessage = ref('')
let searchToken = 0

const hasQuery = computed(() => query.value.trim().length > 0)
const visibleItems = computed(() => prioritizedItems(hasQuery.value ? searchResults.value : folderItems.value))
const visibleRecentItems = computed(() => prioritizedItems(recentItems.value).slice(0, 8))
const selectedIds = computed(() => selectedItems.value.map((item) => item.publicId))

watch(
  () => props.open,
  async (open) => {
    await nextTick()
    const dialog = dialogRef.value
    if (!dialog) {
      return
    }
    if (open && !dialog.open) {
      dialog.showModal()
      await initializeDialog()
      return
    }
    if (!open && dialog.open) {
      dialog.close()
    }
  },
  { immediate: true },
)

watch(
  () => props.selectedFileIds,
  (ids) => {
    if (props.open) {
      void hydrateSelectedItems(ids)
    }
  },
)

watch(query, () => {
  void runSearch()
})

onBeforeUnmount(() => {
  if (dialogRef.value?.open) {
    dialogRef.value.close()
  }
})

async function initializeDialog() {
  query.value = ''
  errorMessage.value = ''
  currentFolderPublicId.value = ''
  await Promise.all([
    hydrateSelectedItems(props.selectedFileIds),
    loadFolder(''),
    loadRecent(),
  ])
}

async function hydrateSelectedItems(ids: string[]) {
  selectedLoading.value = true
  const uniqueIds = props.multiple ? uniqueStrings(ids) : uniqueStrings(ids).slice(0, 1)
  const existing = new Map(selectedItems.value.map((item) => [item.publicId, item]))
  try {
    selectedItems.value = await Promise.all(uniqueIds.map(async (publicId) => {
      const cached = existing.get(publicId)
      if (cached?.file) {
        return cached
      }
      try {
        return { publicId, file: await fetchDriveFile(publicId) }
      } catch {
        return { publicId, file: null }
      }
    }))
  } finally {
    selectedLoading.value = false
  }
}

async function loadFolder(folderPublicId: string) {
  loading.value = true
  errorMessage.value = ''
  try {
    currentFolderPublicId.value = folderPublicId
    const [folder, items] = await Promise.all([
      folderPublicId ? fetchDriveFolder(folderPublicId).catch(() => null) : Promise.resolve(null),
      fetchDriveItems(folderPublicId, '', { type: 'all', sort: 'name', direction: 'asc' }),
    ])
    currentFolder.value = folder
    folderItems.value = items
  } catch (error) {
    folderItems.value = []
    errorMessage.value = error instanceof Error ? error.message : t('dataPipelines.drivePickerLoadFailed')
  } finally {
    loading.value = false
  }
}

async function loadRecent() {
  try {
    recentItems.value = await fetchDriveRecentItems()
  } catch {
    recentItems.value = []
  }
}

async function runSearch() {
  const token = ++searchToken
  const text = query.value.trim()
  if (!text) {
    searchResults.value = []
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    const results = await searchDriveItemsByKeyword(text, '', { type: 'all', sort: 'updated_at', direction: 'desc' })
    if (token === searchToken) {
      searchResults.value = results
    }
  } catch (error) {
    if (token === searchToken) {
      searchResults.value = []
      errorMessage.value = error instanceof Error ? error.message : t('dataPipelines.drivePickerSearchFailed')
    }
  } finally {
    if (token === searchToken) {
      loading.value = false
    }
  }
}

function openFolder(item: DriveItemBody) {
  if (item.folder?.publicId) {
    query.value = ''
    void loadFolder(item.folder.publicId)
  }
}

function toggleFile(item: DriveItemBody) {
  const file = item.file
  if (!file?.publicId) {
    return
  }
  const exists = selectedItems.value.some((selected) => selected.publicId === file.publicId)
  if (exists) {
    removeSelected(file.publicId)
    return
  }
  if (!props.multiple) {
    selectedItems.value = [{ publicId: file.publicId, file }]
    return
  }
  selectedItems.value = [...selectedItems.value, { publicId: file.publicId, file }]
}

function removeSelected(publicId: string) {
  selectedItems.value = selectedItems.value.filter((item) => item.publicId !== publicId)
}

function isSelected(item: DriveItemBody) {
  return Boolean(item.file?.publicId && selectedIds.value.includes(item.file.publicId))
}

function applySelection() {
  emit('apply', selectedIds.value)
}

function handleClose() {
  if (props.open) {
    emit('close')
  }
}

function itemPublicId(item: DriveItemBody) {
  return item.file?.publicId ?? item.folder?.publicId ?? ''
}

function itemSubtitle(item: DriveItemBody) {
  if (item.folder) {
    return t('dataPipelines.drivePickerFolder')
  }
  return [item.file?.contentType, item.file?.byteSize ? formatSize(item.file.byteSize) : ''].filter(Boolean).join(' · ')
}

function selectedName(item: SelectedDriveFile) {
  return item.file?.originalFilename ?? t('dataPipelines.unknownDriveFile')
}

function selectedSubtitle(item: SelectedDriveFile) {
  return item.file ? [item.file.contentType, item.publicId].filter(Boolean).join(' · ') : item.publicId
}

function itemKind(item: DriveItemBody) {
  if (item.folder) {
    return 'folder'
  }
  const contentType = driveItemContentType(item)
  if (contentType.startsWith('image/')) {
    return 'image'
  }
  if (contentType.includes('zip') || contentType.includes('archive')) {
    return 'archive'
  }
  return 'document'
}

function prioritizedItems(items: DriveItemBody[]) {
  const files = items.filter((item) => item.file)
  const folders = items.filter((item) => item.folder)
  if (!props.spreadsheetMode) {
    return [...folders, ...files]
  }
  const spreadsheetFiles = files.filter(isSpreadsheetFile)
  const otherFiles = files.filter((item) => !isSpreadsheetFile(item))
  return [...spreadsheetFiles, ...folders, ...otherFiles]
}

function isSpreadsheetFile(item: DriveItemBody) {
  const name = driveItemName(item).toLowerCase()
  const contentType = driveItemContentType(item).toLowerCase()
  return name.endsWith('.xls')
    || name.endsWith('.xlsx')
    || contentType === 'application/vnd.ms-excel'
    || contentType.includes('spreadsheetml')
}

function uniqueStrings(values: string[]) {
  return Array.from(new Set(values.map((value) => value.trim()).filter(Boolean)))
}

function formatSize(value: number) {
  return new Intl.NumberFormat(undefined, {
    style: 'unit',
    unit: 'byte',
    unitDisplay: 'narrow',
  }).format(value)
}
</script>

<template>
  <dialog ref="dialogRef" class="confirm-dialog drive-file-picker-dialog" @close="handleClose" @cancel.prevent="emit('close')">
    <div class="confirm-dialog-panel drive-file-picker-panel">
      <header class="drive-file-picker-header">
        <div>
          <span class="status-pill">{{ t('dataPipelines.drivePicker') }}</span>
          <h2>{{ t('dataPipelines.selectDriveFiles') }}</h2>
        </div>
        <button class="secondary-button compact-button" type="button" @click="emit('close')">
          <X :size="16" stroke-width="1.9" aria-hidden="true" />
          {{ t('common.close') }}
        </button>
      </header>

      <label class="field drive-file-picker-search">
        <span>{{ t('drive.searchDrive') }}</span>
        <span class="drive-file-picker-search-input">
          <Search :size="16" stroke-width="1.9" aria-hidden="true" />
          <input v-model="query" autocomplete="off" :placeholder="t('dataPipelines.drivePickerSearchPlaceholder')">
        </span>
      </label>

      <p v-if="errorMessage" class="error-message">{{ errorMessage }}</p>

      <div class="drive-file-picker-layout">
        <section class="drive-file-picker-browser">
          <div class="drive-file-picker-section-header">
            <h3>{{ hasQuery ? t('dataPipelines.searchResults') : (currentFolder?.name || t('drive.myDrive')) }}</h3>
            <button v-if="currentFolderPublicId && !hasQuery" class="secondary-button compact-button" type="button" @click="loadFolder('')">
              {{ t('dataPipelines.rootFolder') }}
            </button>
          </div>
          <div class="drive-file-picker-list" role="listbox" :aria-label="t('dataPipelines.selectDriveFiles')">
            <p v-if="loading" class="cell-subtle">{{ t('common.loading') }}</p>
            <p v-else-if="visibleItems.length === 0" class="cell-subtle">{{ t('dataPipelines.noDriveFiles') }}</p>
            <button
              v-for="item in visibleItems"
              :key="itemPublicId(item)"
              class="drive-file-picker-row"
              :class="{ selected: isSelected(item), folder: Boolean(item.folder) }"
              type="button"
              @click="item.folder ? openFolder(item) : toggleFile(item)"
            >
              <DriveFileTypeIcon :kind="itemKind(item)" :size="18" />
              <span class="drive-file-picker-row-text">
                <strong>{{ driveItemName(item) }}</strong>
                <span>{{ itemSubtitle(item) }}</span>
              </span>
              <span v-if="item.file" class="status-pill">{{ isSelected(item) ? t('dataPipelines.selected') : t('dataPipelines.select') }}</span>
              <Folder v-else :size="16" stroke-width="1.9" aria-hidden="true" />
            </button>
          </div>
        </section>

        <aside class="drive-file-picker-side">
          <section>
            <div class="drive-file-picker-section-header">
              <h3>{{ t('dataPipelines.selectedDriveFiles') }}</h3>
              <span class="status-pill">{{ selectedItems.length }}</span>
            </div>
            <div class="drive-file-picker-selected-list">
              <p v-if="selectedLoading" class="cell-subtle">{{ t('common.loading') }}</p>
              <p v-else-if="selectedItems.length === 0" class="cell-subtle">{{ t('dataPipelines.noSelectedDriveFiles') }}</p>
              <div v-for="item in selectedItems" :key="item.publicId" class="drive-file-picker-selected-item">
                <FilePlus2 :size="16" stroke-width="1.9" aria-hidden="true" />
                <span>
                  <strong>{{ selectedName(item) }}</strong>
                  <small>{{ selectedSubtitle(item) }}</small>
                </span>
                <button class="icon-button danger" type="button" :aria-label="t('dataPipelines.removeDriveFile', { name: selectedName(item) })" @click="removeSelected(item.publicId)">
                  <Trash2 :size="14" stroke-width="1.9" aria-hidden="true" />
                </button>
              </div>
            </div>
          </section>

          <section>
            <div class="drive-file-picker-section-header">
              <h3>{{ t('drive.recent') }}</h3>
            </div>
            <div class="drive-file-picker-recent-list">
              <button
                v-for="item in visibleRecentItems"
                :key="`recent-${itemPublicId(item)}`"
                class="drive-file-picker-recent-item"
                type="button"
                :disabled="!item.file"
                @click="item.file ? toggleFile(item) : undefined"
              >
                <DriveFileTypeIcon :kind="itemKind(item)" :size="16" />
                <span>{{ driveItemName(item) }}</span>
              </button>
              <p v-if="visibleRecentItems.length === 0" class="cell-subtle">{{ t('dataPipelines.noRecentDriveFiles') }}</p>
            </div>
          </section>
        </aside>
      </div>

      <footer class="action-row data-pipeline-dialog-actions">
        <button class="secondary-button" type="button" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button class="primary-button" type="button" @click="applySelection">
          {{ t('dataPipelines.applyDriveFiles', { count: selectedItems.length }) }}
        </button>
      </footer>
    </div>
  </dialog>
</template>
