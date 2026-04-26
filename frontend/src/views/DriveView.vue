<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import type { DriveFileBody, DriveItemBody, DrivePermissionBody } from '../api/generated/types.gen'
import { toApiErrorMessage } from '../api/client'
import ConfirmActionDialog from '../components/ConfirmActionDialog.vue'
import DriveCommandBar from '../components/DriveCommandBar.vue'
import DriveBreadcrumbs from '../components/DriveBreadcrumbs.vue'
import DriveDetailsPanel from '../components/DriveDetailsPanel.vue'
import DriveItemGrid from '../components/DriveItemGrid.vue'
import DriveItemList from '../components/DriveItemList.vue'
import DriveSideNav from '../components/DriveSideNav.vue'
import DriveShareDialog from '../components/DriveShareDialog.vue'
import DriveWorkspaceLayout from '../components/DriveWorkspaceLayout.vue'
import EmptyState from '../components/EmptyState.vue'
import TextInputDialog from '../components/TextInputDialog.vue'
import {
  labelFromDriveItem,
  useDriveStore,
} from '../stores/drive'
import { useTenantStore } from '../stores/tenants'
import {
  driveItemKind,
  driveItemName,
  driveItemSize,
  driveItemUpdatedAt,
} from '../utils/driveItems'

const route = useRoute()
const router = useRouter()
const tenantStore = useTenantStore()
const driveStore = useDriveStore()

const actionMessage = ref('')
const actionErrorMessage = ref('')
const shareDialogOpen = ref(false)
const workspaceDialogOpen = ref(false)
const createFolderDialogOpen = ref(false)
const detailsPanelOpen = ref(false)
const pendingDelete = ref<DriveItemBody | null>(null)
const pendingRename = ref<DriveItemBody | null>(null)
const pendingMove = ref<DriveItemBody | null>(null)
const pendingOverwriteFile = ref<DriveFileBody | null>(null)
const overwriteInput = ref<HTMLInputElement | null>(null)
const searchMode = ref(false)
const trashMode = ref(false)

const routeFolderPublicId = computed(() => {
  const raw = route.params.folderPublicId
  return Array.isArray(raw) ? raw[0] : raw
})

const activeTenantLabel = computed(() => (
  tenantStore.activeTenant
    ? `${tenantStore.activeTenant.displayName} / ${tenantStore.activeTenant.slug}`
    : 'None'
))

const sourceItems = computed(() => {
  if (trashMode.value) {
    return driveStore.trashItems
  }
  return searchMode.value ? driveStore.searchResults : driveStore.children
})
const visibleItems = computed(() => {
  const now = Date.now()
  const filtered = sourceItems.value.filter((item) => {
    const query = driveStore.query.trim().toLowerCase()
    if (query && !driveItemName(item).toLowerCase().includes(query)) {
      return false
    }

    if (driveStore.typeFilter !== 'all' && driveItemKind(item) !== driveStore.typeFilter) {
      return false
    }

    if (driveStore.ownerFilter === 'shared_with_me') {
      return false
    }

    if (driveStore.sourceFilter === 'external') {
      return false
    }

    if (driveStore.modifiedFilter !== 'any') {
      const updatedAt = new Date(driveItemUpdatedAt(item)).getTime()
      if (Number.isNaN(updatedAt)) {
        return false
      }
      const dayMs = 24 * 60 * 60 * 1000
      const maxAge = driveStore.modifiedFilter === 'today'
        ? dayMs
        : driveStore.modifiedFilter === 'last_7_days'
          ? 7 * dayMs
          : 30 * dayMs
      if (now - updatedAt > maxAge) {
        return false
      }
    }

    return true
  })

  return [...filtered].sort((a, b) => {
    const direction = driveStore.sortDirection === 'asc' ? 1 : -1
    if (driveStore.sortKey === 'name') {
      return driveItemName(a).localeCompare(driveItemName(b)) * direction
    }
    if (driveStore.sortKey === 'size') {
      return ((driveItemSize(a) ?? -1) - (driveItemSize(b) ?? -1)) * direction
    }
    if (driveStore.sortKey === 'type') {
      return driveItemKind(a).localeCompare(driveItemKind(b)) * direction
    }
    return (new Date(driveItemUpdatedAt(a)).getTime() - new Date(driveItemUpdatedAt(b)).getTime()) * direction
  })
})
const selectedLabel = computed(() => (driveStore.selectedItem ? labelFromDriveItem(driveStore.selectedItem) : 'Drive item'))
const selectedResource = computed(() => driveStore.selectedResource)
const selectedResourceId = computed(() => driveStore.selectedResource?.publicId ?? '')
const currentWorkspaceId = computed(() => driveStore.currentWorkspace?.publicId ?? '')
const currentWorkspaceName = computed(() => driveStore.currentWorkspace?.name ?? 'Default workspace')
const itemCount = computed(() => visibleItems.value.length)
const fileCount = computed(() => visibleItems.value.filter((item) => item.file).length)
const folderCount = computed(() => visibleItems.value.filter((item) => item.folder).length)
const driveTitle = computed(() => {
  if (trashMode.value) {
    return 'Drive Trash'
  }
  return searchMode.value ? 'Search Drive' : 'Drive Browser'
})
const activeDriveView = computed(() => {
  if (trashMode.value) {
    return 'trash'
  }
  if (searchMode.value) {
    return 'search'
  }
  if (route.name === 'drive-groups') {
    return 'groups'
  }
  return 'my-drive'
})
const deleteTitle = computed(() => (
  pendingDelete.value?.type === 'folder' ? 'Delete folder' : 'Delete file'
))
const deleteMessage = computed(() => {
  const label = pendingDelete.value ? labelFromDriveItem(pendingDelete.value) : 'this item'
  if (pendingDelete.value?.type === 'folder') {
    return `${label} をゴミ箱へ移動します。配下の file/folder に削除不可のものがあれば API が拒否します。`
  }
  return `${label} をゴミ箱へ移動します。`
})
const renameInitialValue = computed(() => (pendingRename.value ? labelFromDriveItem(pendingRename.value) : ''))
const moveMessage = computed(() => {
  const label = pendingMove.value ? labelFromDriveItem(pendingMove.value) : 'this item'
  return `${label} の移動先 folder public ID を入力します。空欄なら root へ移動します。`
})
onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
})

watch(
  () => [tenantStore.activeTenant?.slug, route.name, routeFolderPublicId.value],
  async ([slug]) => {
    actionMessage.value = ''
    actionErrorMessage.value = ''
    shareDialogOpen.value = false
    searchMode.value = route.name === 'drive-search'
    trashMode.value = route.name === 'drive-trash'
    detailsPanelOpen.value = false

    if (!slug) {
      return
    }

    if (trashMode.value) {
      await driveStore.loadTrash()
      return
    }

    if (searchMode.value) {
      driveStore.status = 'idle'
      driveStore.searchResults = []
      return
    }

    await driveStore.loadFolder(routeFolderPublicId.value || 'root')
  },
  { immediate: true },
)

function navigateFolder(folderPublicId: string) {
  if (folderPublicId === 'root') {
    router.push('/drive')
    return
  }
  router.push({ name: 'drive-folder', params: { folderPublicId } })
}

async function selectWorkspace(event: Event) {
  const workspacePublicId = (event.target as HTMLSelectElement).value
  await driveStore.selectWorkspace(workspacePublicId)
  if (route.name !== 'drive') {
    await router.push('/drive')
  }
}

function requestCreateWorkspace() {
  workspaceDialogOpen.value = true
}

function requestCreateFolder() {
  createFolderDialogOpen.value = true
}

function cancelCreateFolder() {
  createFolderDialogOpen.value = false
}

function cancelCreateWorkspace() {
  workspaceDialogOpen.value = false
}

async function createWorkspace(name: string) {
  workspaceDialogOpen.value = false
  if (!name.trim()) {
    return
  }
  try {
    await driveStore.createWorkspace(name)
    actionMessage.value = 'Workspace を作成しました。'
    await router.push('/drive')
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function createFolder(name: string) {
  createFolderDialogOpen.value = false
  try {
    await driveStore.createFolder(name)
    actionMessage.value = 'Folder を作成しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function uploadFile(file: File) {
  try {
    await driveStore.uploadFile(file)
    actionMessage.value = 'File を upload しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function downloadFile(file: DriveFileBody) {
  try {
    const download = await driveStore.downloadFile(file)
    const href = URL.createObjectURL(download.blob)
    const anchor = document.createElement('a')
    anchor.href = href
    anchor.download = download.filename
    anchor.rel = 'noopener'
    anchor.click()
    URL.revokeObjectURL(href)
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

function renameItem(item: DriveItemBody) {
  pendingRename.value = item
}

function cancelRename() {
  pendingRename.value = null
}

async function confirmRename(nextName: string) {
  if (!pendingRename.value) {
    return
  }
  const item = pendingRename.value
  pendingRename.value = null
  const currentName = labelFromDriveItem(item)
  if (!nextName.trim() || nextName.trim() === currentName) {
    return
  }
  try {
    if (item.file) {
      await driveStore.renameFile(item.file, nextName)
    } else if (item.folder) {
      await driveStore.renameFolder(item.folder, nextName)
    }
    actionMessage.value = '名前を更新しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

function moveItem(item: DriveItemBody) {
  pendingMove.value = item
}

function cancelMove() {
  pendingMove.value = null
}

async function confirmMove(target: string) {
  if (!pendingMove.value) {
    return
  }
  const item = pendingMove.value
  pendingMove.value = null
  try {
    if (item.file) {
      await driveStore.moveFile(item.file, target)
    } else if (item.folder) {
      await driveStore.moveFolder(item.folder, target)
    }
    actionMessage.value = '移動しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

function requestOverwrite(file: DriveFileBody) {
  pendingOverwriteFile.value = file
  overwriteInput.value?.click()
}

async function onOverwriteFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  const current = pendingOverwriteFile.value
  target.value = ''
  pendingOverwriteFile.value = null
  if (!file || !current) {
    return
  }
  try {
    await driveStore.overwriteFile(current, file)
    actionMessage.value = 'File を置き換えました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

function askDelete(item: DriveItemBody) {
  pendingDelete.value = item
}

function cancelDelete() {
  pendingDelete.value = null
}

async function confirmDelete() {
  if (!pendingDelete.value) {
    return
  }
  const item = pendingDelete.value
  pendingDelete.value = null
  try {
    await driveStore.deleteItem(item)
    actionMessage.value = '削除しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function restoreItem(item: DriveItemBody) {
  try {
    await driveStore.restoreItem(item)
    actionMessage.value = '復元しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function openShareDialog(item: DriveItemBody) {
  actionErrorMessage.value = ''
  try {
    await driveStore.loadGroups()
    await driveStore.selectItem(item)
    shareDialogOpen.value = true
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function openDetailsPanel(item: DriveItemBody) {
  actionErrorMessage.value = ''
  try {
    await driveStore.selectItem(item)
    detailsPanelOpen.value = true
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

function closeDetailsPanel() {
  detailsPanelOpen.value = false
}

async function createUserShare(subjectPublicId: string, role: string) {
  if (!selectedResource.value) {
    return
  }
  try {
    await driveStore.createUserShare(selectedResource.value, subjectPublicId, role)
    actionMessage.value = 'User share を作成しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function createGroupShare(groupPublicId: string, role: string) {
  if (!selectedResource.value) {
    return
  }
  try {
    await driveStore.createGroupShare(selectedResource.value, groupPublicId, role)
    actionMessage.value = 'Group share を作成しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function createExternalInvitation(inviteeEmail: string, role: string) {
  if (!selectedResource.value) {
    return
  }
  try {
    const invitation = await driveStore.createExternalInvitation(selectedResource.value, inviteeEmail, role)
    actionMessage.value = invitation.acceptToken
      ? `External invitation を作成しました。Accept token: ${invitation.acceptToken}`
      : 'External invitation を作成しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function createShareLink(expiresAt: string, canDownload: boolean, password: string, role: string) {
  if (!selectedResource.value) {
    return
  }
  try {
    await driveStore.createShareLink(selectedResource.value, expiresAt, canDownload, password, role)
    actionMessage.value = 'Share link を作成しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function revokeShare(permission: DrivePermissionBody) {
  if (!selectedResource.value || !permission.publicId) {
    return
  }
  try {
    await driveStore.revokeShare(selectedResource.value, permission.publicId)
    actionMessage.value = 'Share を解除しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function disableShareLink(permission: DrivePermissionBody) {
  if (!selectedResource.value || !permission.publicId) {
    return
  }
  try {
    await driveStore.disableShareLink(selectedResource.value, permission.publicId)
    actionMessage.value = 'Share link を無効化しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function reloadPermissions() {
  if (!selectedResource.value) {
    return
  }
  try {
    await driveStore.loadPermissions(selectedResource.value)
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function search(query: string) {
  driveStore.setQuery(query)
  if (!query) {
    searchMode.value = false
    await router.push('/drive')
    return
  }
  if (route.name !== 'drive-search') {
    await router.push({ name: 'drive-search' })
  }
  searchMode.value = true
  await driveStore.search(query)
}

async function clearDriveFilters() {
  driveStore.clearFilters()
  if (searchMode.value) {
    searchMode.value = false
    await router.push('/drive')
  }
}

async function refreshDrive() {
  if (trashMode.value) {
    await driveStore.loadTrash()
    return
  }
  await driveStore.refreshCurrent()
}
</script>

<template>
  <DriveWorkspaceLayout :details-open="detailsPanelOpen">
    <template #side>
      <DriveSideNav
        :current-folder="driveStore.currentFolder"
        :children="driveStore.children"
        :active-view="activeDriveView"
        :workspace-name="currentWorkspaceName"
        :disabled="!tenantStore.activeTenant"
        :busy="driveStore.isBusy"
        @create-folder="requestCreateFolder"
        @upload-file="uploadFile"
        @open-folder="navigateFolder"
      />
    </template>

    <template #header>
      <header class="drive-workspace-header">
        <div class="drive-title-group">
          <span class="status-pill">Drive</span>
          <h1>{{ driveTitle }}</h1>
          <p>Workspace、folder、file、share link を tenant ごとに管理します。</p>
          <DriveBreadcrumbs v-if="!searchMode && !trashMode" :current-folder="driveStore.currentFolder" />
        </div>
        <div class="drive-header-actions">
          <label class="field compact-field">
            <span class="field-label">Workspace</span>
            <select class="field-input" :value="currentWorkspaceId" :disabled="driveStore.isBusy" @change="selectWorkspace">
              <option v-for="workspace in driveStore.workspaces" :key="workspace.publicId" :value="workspace.publicId">
                {{ workspace.name }}
              </option>
            </select>
          </label>
          <button class="secondary-button compact-button" type="button" :disabled="driveStore.isBusy || !tenantStore.activeTenant" @click="requestCreateWorkspace">
            New workspace
          </button>
          <button class="secondary-button compact-button" type="button" @click="detailsPanelOpen = !detailsPanelOpen">
            {{ detailsPanelOpen ? 'Hide details' : 'Show details' }}
          </button>
        </div>
      </header>

      <div class="drive-quick-stats" aria-label="Drive summary">
        <span>{{ activeTenantLabel }}</span>
        <span>{{ itemCount }} items</span>
        <span>{{ fileCount }} files</span>
        <span>{{ folderCount }} folders</span>
      </div>
    </template>

    <template #command>
      <DriveCommandBar
        :busy="driveStore.isBusy"
        :disabled="!tenantStore.activeTenant || trashMode"
        :query="driveStore.query"
        :view-mode="driveStore.viewMode"
        :type-filter="driveStore.typeFilter"
        :owner-filter="driveStore.ownerFilter"
        :modified-filter="driveStore.modifiedFilter"
        :source-filter="driveStore.sourceFilter"
        :sort-key="driveStore.sortKey"
        :sort-direction="driveStore.sortDirection"
        @search="search"
        @clear-filters="clearDriveFilters"
        @update-view-mode="driveStore.setViewMode"
        @update-type-filter="driveStore.setTypeFilter"
        @update-owner-filter="driveStore.setOwnerFilter"
        @update-modified-filter="driveStore.setModifiedFilter"
        @update-source-filter="driveStore.setSourceFilter"
        @update-sort="driveStore.setSort"
        @refresh="refreshDrive"
      />
    </template>

    <div class="drive-workspace-content">
      <p v-if="tenantStore.status === 'empty'" class="warning-message">
        Active tenant がありません。tenant selector で tenant を選択してください。
      </p>
      <p v-if="tenantStore.status === 'error'" class="error-message">{{ tenantStore.errorMessage }}</p>
      <p v-if="actionErrorMessage || driveStore.errorMessage" class="error-message">
        {{ actionErrorMessage || driveStore.errorMessage }}
      </p>
      <p v-if="actionMessage" class="notice-message">{{ actionMessage }}</p>

      <div v-if="trashMode" class="action-row">
        <RouterLink class="secondary-button link-button compact-button" to="/drive">
          Back to Drive
        </RouterLink>
        <button class="secondary-button compact-button" type="button" :disabled="driveStore.isBusy" @click="refreshDrive">
          Refresh
        </button>
      </div>

      <EmptyState
        v-if="driveStore.status === 'forbidden'"
        title="Drive access denied"
        message="Drive authorization が有効でない、またはこの tenant で Drive を表示する権限がありません。"
      />

      <template v-else-if="visibleItems.length > 0 || driveStore.status === 'loading'">
        <DriveItemGrid
          v-if="driveStore.viewMode === 'grid'"
          :items="visibleItems"
          :loading="driveStore.status === 'loading'"
          :busy-resource-id="driveStore.busyResourceId"
          :deleting-resource-id="driveStore.deletingResourceId"
          :selected-resource-id="selectedResourceId"
          :trash-mode="trashMode"
          @open-folder="navigateFolder"
          @download-file="downloadFile"
          @rename-item="renameItem"
          @move-item="moveItem"
          @overwrite-file="requestOverwrite"
          @delete-item="askDelete"
          @share-item="openShareDialog"
          @restore-item="restoreItem"
          @details-item="openDetailsPanel"
        />
        <DriveItemList
          v-else
          :items="visibleItems"
          :loading="driveStore.status === 'loading'"
          :busy-resource-id="driveStore.busyResourceId"
          :deleting-resource-id="driveStore.deletingResourceId"
          :selected-resource-id="selectedResourceId"
          :trash-mode="trashMode"
          @open-folder="navigateFolder"
          @download-file="downloadFile"
          @rename-item="renameItem"
          @move-item="moveItem"
          @overwrite-file="requestOverwrite"
          @delete-item="askDelete"
          @share-item="openShareDialog"
          @restore-item="restoreItem"
          @details-item="openDetailsPanel"
        />
      </template>

      <EmptyState
        v-else
        :title="trashMode ? 'Trash is empty' : searchMode ? 'No search results' : 'No items yet'"
        :message="trashMode ? 'ゴミ箱は空です。' : searchMode ? '検索結果はありません。' : 'この folder にはまだ item がありません。'"
      >
        <template #actions>
          <button v-if="!trashMode" class="primary-button compact-button" type="button" @click="requestCreateFolder">
            New folder
          </button>
          <button v-if="searchMode" class="secondary-button compact-button" type="button" @click="clearDriveFilters">
            Back to Drive
          </button>
        </template>
      </EmptyState>

      <input ref="overwriteInput" class="drive-hidden-input" type="file" @change="onOverwriteFileChange">
    </div>

    <template #details>
      <DriveDetailsPanel
        :open="detailsPanelOpen"
        :selected-item="driveStore.selectedItem"
        :current-folder="driveStore.currentFolder"
        :permissions="driveStore.permissions"
        :item-count="itemCount"
        :file-count="fileCount"
        :folder-count="folderCount"
        @close="closeDetailsPanel"
        @share-item="openShareDialog"
      />
    </template>
  </DriveWorkspaceLayout>

  <DriveShareDialog
    :open="shareDialogOpen"
    :resource="selectedResource"
    :label="selectedLabel"
    :groups="driveStore.groups"
    :permissions="driveStore.permissions"
    :last-raw-share-link="driveStore.lastRawShareLink"
    :busy="driveStore.isBusy"
    :error-message="actionErrorMessage || driveStore.errorMessage"
    @close="shareDialogOpen = false"
    @create-user-share="createUserShare"
    @create-group-share="createGroupShare"
    @create-external-invitation="createExternalInvitation"
    @create-share-link="createShareLink"
    @revoke-share="revokeShare"
    @disable-link="disableShareLink"
    @reload-permissions="reloadPermissions"
  />

  <ConfirmActionDialog
    :open="pendingDelete !== null"
    :title="deleteTitle"
    :message="deleteMessage"
    confirm-label="Delete"
    @cancel="cancelDelete"
    @confirm="confirmDelete"
  />

  <TextInputDialog
    :open="workspaceDialogOpen"
    title="New workspace"
    label="Workspace name"
    placeholder="Product team"
    confirm-label="Create workspace"
    @cancel="cancelCreateWorkspace"
    @confirm="createWorkspace"
  />

  <TextInputDialog
    :open="createFolderDialogOpen"
    title="New folder"
    label="Folder name"
    placeholder="Project files"
    confirm-label="Create folder"
    @cancel="cancelCreateFolder"
    @confirm="createFolder"
  />

  <TextInputDialog
    :open="pendingRename !== null"
    title="Rename item"
    label="New name"
    :initial-value="renameInitialValue"
    confirm-label="Rename"
    @cancel="cancelRename"
    @confirm="confirmRename"
  />

  <TextInputDialog
    :open="pendingMove !== null"
    title="Move item"
    label="Target folder public ID"
    :message="moveMessage"
    placeholder="folder public ID"
    confirm-label="Move"
    allow-empty
    @cancel="cancelMove"
    @confirm="confirmMove"
  />
</template>
