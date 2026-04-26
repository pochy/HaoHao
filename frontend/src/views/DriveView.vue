<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import type { DriveFileBody, DriveItemBody, DrivePermissionBody } from '../api/generated/types.gen'
import { toApiErrorMessage } from '../api/client'
import ConfirmActionDialog from '../components/ConfirmActionDialog.vue'
import DriveBreadcrumbs from '../components/DriveBreadcrumbs.vue'
import DriveItemTable from '../components/DriveItemTable.vue'
import DriveShareDialog from '../components/DriveShareDialog.vue'
import DriveToolbar from '../components/DriveToolbar.vue'
import {
  labelFromDriveItem,
  useDriveStore,
} from '../stores/drive'
import { useTenantStore } from '../stores/tenants'

const route = useRoute()
const router = useRouter()
const tenantStore = useTenantStore()
const driveStore = useDriveStore()

const actionMessage = ref('')
const actionErrorMessage = ref('')
const shareDialogOpen = ref(false)
const pendingDelete = ref<DriveItemBody | null>(null)
const pendingOverwriteFile = ref<DriveFileBody | null>(null)
const overwriteInput = ref<HTMLInputElement | null>(null)
const searchMode = ref(false)

const routeFolderPublicId = computed(() => {
  const raw = route.params.folderPublicId
  return Array.isArray(raw) ? raw[0] : raw
})

const activeTenantLabel = computed(() => (
  tenantStore.activeTenant
    ? `${tenantStore.activeTenant.displayName} / ${tenantStore.activeTenant.slug}`
    : 'None'
))

const visibleItems = computed(() => (searchMode.value ? driveStore.searchResults : driveStore.children))
const selectedLabel = computed(() => (driveStore.selectedItem ? labelFromDriveItem(driveStore.selectedItem) : 'Drive item'))
const selectedResource = computed(() => driveStore.selectedResource)
const currentWorkspaceId = computed(() => driveStore.currentWorkspace?.publicId ?? '')
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

    if (!slug) {
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
  router.push({ name: 'drive-folder', params: { folderPublicId } })
}

async function selectWorkspace(event: Event) {
  const workspacePublicId = (event.target as HTMLSelectElement).value
  await driveStore.selectWorkspace(workspacePublicId)
  if (route.name !== 'drive') {
    await router.push('/drive')
  }
}

async function createWorkspace() {
  const name = window.prompt('Workspace name', '')
  if (!name?.trim()) {
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

async function renameItem(item: DriveItemBody) {
  const currentName = labelFromDriveItem(item)
  const nextName = window.prompt('New name', currentName)
  if (!nextName || nextName.trim() === currentName) {
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

async function moveItem(item: DriveItemBody) {
  const target = window.prompt('Target folder public ID. 空欄なら root へ移動します。', '')
  if (target === null) {
    return
  }
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
</script>

<template>
  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">Drive</span>
        <h2>{{ searchMode ? 'Search Drive' : 'Drive Browser' }}</h2>
      </div>
      <RouterLink class="secondary-button link-button" to="/drive/groups">
        Groups
      </RouterLink>
    </div>

    <div v-if="!searchMode" class="action-row">
      <label class="field compact-field">
        <span class="field-label">Workspace</span>
        <select class="field-input" :value="currentWorkspaceId" :disabled="driveStore.isBusy" @change="selectWorkspace">
          <option v-for="workspace in driveStore.workspaces" :key="workspace.publicId" :value="workspace.publicId">
            {{ workspace.name }}
          </option>
        </select>
      </label>
      <button class="secondary-button compact-button" type="button" :disabled="driveStore.isBusy || !tenantStore.activeTenant" @click="createWorkspace">
        New workspace
      </button>
    </div>

    <dl class="metadata-grid">
      <div>
        <dt>Active tenant</dt>
        <dd>{{ activeTenantLabel }}</dd>
      </div>
      <div>
        <dt>Workspace</dt>
        <dd>{{ driveStore.currentWorkspace?.name ?? 'Default' }}</dd>
      </div>
      <div>
        <dt>Current folder</dt>
        <dd>{{ driveStore.currentFolder.name }}</dd>
      </div>
    </dl>

    <DriveBreadcrumbs v-if="!searchMode" :current-folder="driveStore.currentFolder" />

    <p v-if="tenantStore.status === 'empty'" class="warning-message">
      Active tenant がありません。tenant selector で tenant を選択してください。
    </p>
    <p v-if="tenantStore.status === 'error'" class="error-message">{{ tenantStore.errorMessage }}</p>
    <p v-if="actionErrorMessage || driveStore.errorMessage" class="error-message">
      {{ actionErrorMessage || driveStore.errorMessage }}
    </p>
    <p v-if="actionMessage" class="notice-message">{{ actionMessage }}</p>

    <DriveToolbar
      :busy="driveStore.isBusy"
      :disabled="!tenantStore.activeTenant"
      @create-folder="createFolder"
      @upload-file="uploadFile"
      @search="search"
      @refresh="driveStore.refreshCurrent()"
    />

    <div v-if="driveStore.status === 'forbidden'" class="empty-state">
      <p>Drive authorization が有効でない、またはこの tenant で Drive を表示する権限がありません。</p>
    </div>

    <DriveItemTable
      v-else-if="visibleItems.length > 0 || driveStore.status === 'loading'"
      :items="visibleItems"
      :loading="driveStore.status === 'loading'"
      :busy-resource-id="driveStore.busyResourceId"
      :deleting-resource-id="driveStore.deletingResourceId"
      @open-folder="navigateFolder"
      @download-file="downloadFile"
      @rename-item="renameItem"
      @move-item="moveItem"
      @overwrite-file="requestOverwrite"
      @delete-item="askDelete"
      @share-item="openShareDialog"
    />

    <div v-else class="empty-state">
      <p>{{ searchMode ? '検索結果はありません。' : 'この folder にはまだ item がありません。' }}</p>
      <button v-if="searchMode" class="secondary-button compact-button" type="button" @click="router.push('/drive')">
        Back to Drive
      </button>
    </div>

    <input ref="overwriteInput" class="drive-hidden-input" type="file" @change="onOverwriteFileChange">
  </section>

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
</template>
