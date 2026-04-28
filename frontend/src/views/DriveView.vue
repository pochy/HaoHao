<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

import type { DriveFileBody, DriveItemBody, DrivePermissionBody } from '../api/generated/types.gen'
import { toApiErrorMessage } from '../api/client'
import ConfirmActionDialog from '../components/ConfirmActionDialog.vue'
import DriveCommandBar from '../components/DriveCommandBar.vue'
import DriveBreadcrumbs from '../components/DriveBreadcrumbs.vue'
import DriveDetailsPanel from '../components/DriveDetailsPanel.vue'
import DriveItemGrid from '../components/DriveItemGrid.vue'
import DriveItemList from '../components/DriveItemList.vue'
import DriveMetadataDialog from '../components/DriveMetadataDialog.vue'
import DrivePreviewDialog from '../components/DrivePreviewDialog.vue'
import DriveSideNav from '../components/DriveSideNav.vue'
import DriveShareDialog from '../components/DriveShareDialog.vue'
import DriveUploadQueue from '../components/DriveUploadQueue.vue'
import DriveWorkspaceLayout from '../components/DriveWorkspaceLayout.vue'
import EmptyState from '../components/EmptyState.vue'
import TextInputDialog from '../components/TextInputDialog.vue'
import {
  labelFromDriveItem,
  type DriveModifiedFilter,
  type DriveOwnerFilter,
  type DriveSortKey,
  type DriveSourceFilter,
  type DriveTypeFilter,
  useDriveStore,
} from '../stores/drive'
import { useTenantStore } from '../stores/tenants'
import {
  driveItemName,
  driveItemPublicId,
  driveItemSize,
  driveItemUpdatedAt,
  formatDriveSize,
} from '../utils/driveItems'

const route = useRoute()
const router = useRouter()
const tenantStore = useTenantStore()
const driveStore = useDriveStore()
const { t } = useI18n()

const actionMessage = ref('')
const actionErrorMessage = ref('')
const shareDialogOpen = ref(false)
const metadataDialogOpen = ref(false)
const workspaceDialogOpen = ref(false)
const createFolderDialogOpen = ref(false)
const detailsPanelOpen = ref(false)
const metadataTarget = ref<DriveItemBody | null>(null)
const previewTarget = ref<DriveItemBody | null>(null)
const pendingDelete = ref<DriveItemBody | null>(null)
const pendingRename = ref<DriveItemBody | null>(null)
const pendingMove = ref<DriveItemBody | null>(null)
const pendingOverwriteFile = ref<DriveFileBody | null>(null)
const overwriteInput = ref<HTMLInputElement | null>(null)
const searchMode = ref(false)
const trashMode = ref(false)
const sharedMode = ref(false)
const starredMode = ref(false)
const recentMode = ref(false)
const storageMode = ref(false)
const dropActive = ref(false)
const dragDepth = ref(0)

const routeFolderPublicId = computed(() => {
  const raw = route.params.folderPublicId
  return Array.isArray(raw) ? raw[0] : raw
})
const routeQueryFingerprint = computed(() => JSON.stringify(route.query))

const activeTenantLabel = computed(() => (
  tenantStore.activeTenant
    ? `${tenantStore.activeTenant.displayName} / ${tenantStore.activeTenant.slug}`
    : t('common.none')
))

const sourceItems = computed(() => {
  if (trashMode.value) {
    return driveStore.trashItems
  }
  if (sharedMode.value) {
    return driveStore.sharedItems
  }
  if (starredMode.value) {
    return driveStore.starredItems
  }
  if (recentMode.value) {
    return driveStore.recentItems
  }
  if (storageMode.value) {
    return []
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

    if (driveStore.typeFilter === 'file' && !item.file) {
      return false
    }
    if (driveStore.typeFilter === 'folder' && !item.folder) {
      return false
    }

    if (driveStore.ownerFilter === 'me' && !item.ownedByMe) {
      return false
    }
    if (driveStore.ownerFilter === 'shared_with_me' && !item.sharedWithMe) {
      return false
    }
    if (driveStore.sourceFilter !== 'all' && item.source !== driveStore.sourceFilter) {
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
    return (new Date(driveItemUpdatedAt(a)).getTime() - new Date(driveItemUpdatedAt(b)).getTime()) * direction
  })
})
const selectedArchiveItems = computed(() => visibleItems.value.filter((item) => (
  driveStore.selectedResourceIds.includes(driveItemPublicId(item))
)))
const selectedLabel = computed(() => (driveStore.selectedItem ? labelFromDriveItem(driveStore.selectedItem) : t('drive.selectedItemFallback')))
const selectedResource = computed(() => driveStore.selectedResource)
const selectedResourceId = computed(() => driveStore.selectedResource?.publicId ?? '')
const currentWorkspaceId = computed(() => driveStore.currentWorkspace?.publicId ?? '')
const currentWorkspaceName = computed(() => driveStore.currentWorkspace?.name ?? t('drive.defaultWorkspace'))
const itemCount = computed(() => visibleItems.value.length)
const fileCount = computed(() => visibleItems.value.filter((item) => item.file).length)
const folderCount = computed(() => visibleItems.value.filter((item) => item.folder).length)
const driveTitle = computed(() => {
  if (trashMode.value) {
    return t('routes.driveTrash')
  }
  if (sharedMode.value) {
    return t('routes.driveShared')
  }
  if (starredMode.value) {
    return t('routes.driveStarred')
  }
  if (recentMode.value) {
    return t('drive.recent')
  }
  if (storageMode.value) {
    return t('drive.storage')
  }
  return searchMode.value ? t('drive.searchDrive') : t('drive.browser')
})
const activeDriveView = computed(() => {
  if (trashMode.value) {
    return 'trash'
  }
  if (sharedMode.value) {
    return 'shared'
  }
  if (starredMode.value) {
    return 'starred'
  }
  if (recentMode.value) {
    return 'recent'
  }
  if (storageMode.value) {
    return 'storage'
  }
  if (searchMode.value) {
    return 'search'
  }
  if (route.name === 'drive-groups') {
    return 'groups'
  }
  return 'my-drive'
})
const canUploadToCurrentView = computed(() => (
  Boolean(tenantStore.activeTenant)
  && !trashMode.value
  && !sharedMode.value
  && !starredMode.value
  && !recentMode.value
  && !storageMode.value
  && !searchMode.value
))
const deleteTitle = computed(() => (
  pendingDelete.value?.type === 'folder' ? t('drive.deleteFolderTitle') : t('drive.deleteFileTitle')
))
const deleteMessage = computed(() => {
  const name = pendingDelete.value ? labelFromDriveItem(pendingDelete.value) : t('drive.deleteFallback')
  if (pendingDelete.value?.type === 'folder') {
    return t('drive.deleteFolderMessage', { name })
  }
  return t('drive.deleteFileMessage', { name })
})
const renameInitialValue = computed(() => (pendingRename.value ? labelFromDriveItem(pendingRename.value) : ''))
const moveMessage = computed(() => {
  const name = pendingMove.value ? labelFromDriveItem(pendingMove.value) : t('drive.deleteFallback')
  return t('drive.dialogs.moveMessage', { name })
})
const emptyTitle = computed(() => {
  if (trashMode.value) {
    return t('drive.empty.trashTitle')
  }
  if (sharedMode.value) {
    return t('drive.empty.sharedTitle')
  }
  if (starredMode.value) {
    return t('drive.empty.starredTitle')
  }
  if (recentMode.value) {
    return t('drive.empty.recentTitle')
  }
  if (searchMode.value) {
    return t('drive.empty.searchTitle')
  }
  return t('drive.empty.defaultTitle')
})
const emptyMessage = computed(() => {
  if (trashMode.value) {
    return t('drive.empty.trashMessage')
  }
  if (sharedMode.value) {
    return t('drive.empty.sharedMessage')
  }
  if (starredMode.value) {
    return t('drive.empty.starredMessage')
  }
  if (recentMode.value) {
    return t('drive.empty.recentMessage')
  }
  if (searchMode.value) {
    return t('drive.empty.searchMessage')
  }
  return t('drive.empty.defaultMessage')
})

const validTypeFilters: DriveTypeFilter[] = ['all', 'file', 'folder']
const validOwnerFilters: DriveOwnerFilter[] = ['all', 'me', 'shared_with_me']
const validModifiedFilters: DriveModifiedFilter[] = ['any', 'today', 'last_7_days', 'last_30_days']
const validSourceFilters: DriveSourceFilter[] = ['all', 'upload', 'external', 'generated', 'sync']
const validSortKeys: DriveSortKey[] = ['updated_at', 'name', 'size']

function routeString(value: unknown) {
  return Array.isArray(value) ? value[0] ?? '' : typeof value === 'string' ? value : ''
}

function applyRouteQueryFilters() {
  const query = routeString(route.query.q)
  if (driveStore.query !== query) {
    driveStore.setQuery(query)
  }

  const type = routeString(route.query.type)
  driveStore.setTypeFilter(validTypeFilters.includes(type as DriveTypeFilter) ? type as DriveTypeFilter : 'all')

  const owner = routeString(route.query.owner)
  driveStore.setOwnerFilter(validOwnerFilters.includes(owner as DriveOwnerFilter) ? owner as DriveOwnerFilter : 'all')

  const modified = routeString(route.query.modified)
  driveStore.setModifiedFilter(validModifiedFilters.includes(modified as DriveModifiedFilter) ? modified as DriveModifiedFilter : 'any')

  const source = routeString(route.query.source)
  driveStore.setSourceFilter(validSourceFilters.includes(source as DriveSourceFilter) ? source as DriveSourceFilter : 'all')

  const sort = routeString(route.query.sort)
  driveStore.sortKey = validSortKeys.includes(sort as DriveSortKey) ? sort as DriveSortKey : 'updated_at'

  const direction = routeString(route.query.direction)
  driveStore.sortDirection = direction === 'asc' || direction === 'desc' ? direction : 'desc'
}

function driveFilterQuery() {
  return {
    ...(driveStore.query ? { q: driveStore.query } : {}),
    ...(driveStore.typeFilter !== 'all' ? { type: driveStore.typeFilter } : {}),
    ...(driveStore.ownerFilter !== 'all' ? { owner: driveStore.ownerFilter } : {}),
    ...(driveStore.modifiedFilter !== 'any' ? { modified: driveStore.modifiedFilter } : {}),
    ...(driveStore.sourceFilter !== 'all' ? { source: driveStore.sourceFilter } : {}),
    ...(driveStore.sortKey !== 'updated_at' ? { sort: driveStore.sortKey } : {}),
    ...(driveStore.sortDirection !== 'desc' ? { direction: driveStore.sortDirection } : {}),
  }
}

async function replaceDriveQuery() {
  await router.replace({ query: driveFilterQuery() })
}

onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
})

watch(
  () => [tenantStore.activeTenant?.slug, route.name, routeFolderPublicId.value, routeQueryFingerprint.value],
  async ([slug]) => {
    actionMessage.value = ''
    actionErrorMessage.value = ''
    shareDialogOpen.value = false
    searchMode.value = route.name === 'drive-search'
    trashMode.value = route.name === 'drive-trash'
    sharedMode.value = route.name === 'drive-shared'
    starredMode.value = route.name === 'drive-starred'
    recentMode.value = route.name === 'drive-recent'
    storageMode.value = route.name === 'drive-storage'
    detailsPanelOpen.value = false
    applyRouteQueryFilters()

    if (!slug) {
      return
    }

    if (trashMode.value) {
      await driveStore.loadTrash()
      return
    }

    if (sharedMode.value) {
      await driveStore.loadSharedWithMe()
      return
    }

    if (starredMode.value) {
      await driveStore.loadStarred()
      return
    }

    if (recentMode.value) {
      await driveStore.loadRecent()
      return
    }

    if (storageMode.value) {
      await driveStore.loadStorage()
      await driveStore.loadFolderTree()
      driveStore.status = 'ready'
      return
    }

    if (searchMode.value) {
      driveStore.status = 'idle'
      driveStore.searchResults = []
      if (driveStore.query) {
        await driveStore.search(driveStore.query)
      }
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

async function uploadFiles(files: File[]) {
  try {
    await driveStore.uploadFiles(files)
    actionMessage.value = `${files.length} files を upload しました。`
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

function hasDraggedFiles(event: DragEvent) {
  return Array.from(event.dataTransfer?.types ?? []).includes('Files')
}

function onDragEnter(event: DragEvent) {
  if (!canUploadToCurrentView.value || !hasDraggedFiles(event)) {
    return
  }
  event.preventDefault()
  dragDepth.value += 1
  dropActive.value = true
}

function onDragOver(event: DragEvent) {
  if (!canUploadToCurrentView.value || !hasDraggedFiles(event)) {
    return
  }
  event.preventDefault()
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy'
  }
}

function onDragLeave(event: DragEvent) {
  if (!canUploadToCurrentView.value || !hasDraggedFiles(event)) {
    return
  }
  dragDepth.value = Math.max(0, dragDepth.value - 1)
  dropActive.value = dragDepth.value > 0
}

async function onDropFiles(event: DragEvent) {
  if (!canUploadToCurrentView.value || !hasDraggedFiles(event)) {
    return
  }
  event.preventDefault()
  dragDepth.value = 0
  dropActive.value = false
  const files = Array.from(event.dataTransfer?.files ?? [])
  if (files.length === 0) {
    return
  }
  await uploadFiles(files)
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

async function downloadArchive(items: DriveItemBody[] = selectedArchiveItems.value) {
  if (items.length === 0) {
    actionErrorMessage.value = 'ZIP に含める item を選択してください。'
    return
  }
  try {
    const download = await driveStore.downloadArchive(items)
    const href = URL.createObjectURL(download.blob)
    const anchor = document.createElement('a')
    anchor.href = href
    anchor.download = download.filename
    anchor.rel = 'noopener'
    anchor.click()
    URL.revokeObjectURL(href)
    driveStore.clearSelection()
    actionMessage.value = 'ZIP download を開始しました。'
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

async function copyItem(item: DriveItemBody) {
  try {
    await driveStore.copyItem(item)
    actionMessage.value = 'Copy を作成しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

function openMetadataDialog(item: DriveItemBody) {
  metadataTarget.value = item
  metadataDialogOpen.value = true
}

function closeMetadataDialog() {
  metadataDialogOpen.value = false
  metadataTarget.value = null
}

async function saveMetadata(description: string, tags: string[]) {
  if (!metadataTarget.value) {
    return
  }
  try {
    await driveStore.updateItemMetadata(metadataTarget.value, description, tags)
    actionMessage.value = 'Metadata を更新しました。'
    closeMetadataDialog()
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

function openPreviewDialog(item: DriveItemBody) {
  if (!item.file) {
    return
  }
  previewTarget.value = item
}

function closePreviewDialog() {
  previewTarget.value = null
}

async function restoreItem(item: DriveItemBody) {
  try {
    await driveStore.restoreItem(item)
    actionMessage.value = '復元しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function permanentlyDeleteItem(item: DriveItemBody) {
  if (!confirm('この item を完全に削除します。元に戻せません。')) {
    return
  }
  try {
    await driveStore.permanentlyDeleteItem(item)
    actionMessage.value = '完全削除しました。'
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

async function updateShareRole(permission: DrivePermissionBody, role: string) {
  if (!selectedResource.value || !permission.publicId || permission.role === role) {
    return
  }
  try {
    await driveStore.updateShareRole(selectedResource.value, permission.publicId, role)
    actionMessage.value = 'Share role を更新しました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function transferOwner(newOwnerUserPublicId: string, revokePreviousOwnerAccess: boolean) {
  if (!selectedResource.value) {
    return
  }
  if (!confirm(`Owner を ${newOwnerUserPublicId} に移管します。続行しますか？`)) {
    return
  }
  try {
    await driveStore.transferOwner(selectedResource.value, newOwnerUserPublicId, revokePreviousOwnerAccess)
    actionMessage.value = 'Owner を移管しました。'
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

async function requestOCR(filePublicId: string) {
  const file = driveStore.selectedItem?.file
  if (!file || file.publicId !== filePublicId) {
    return
  }
  try {
    await driveStore.requestOCR(file)
    actionMessage.value = t('drive.ocrRequested')
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function search(query: string) {
  driveStore.setQuery(query)
  if (!query) {
    searchMode.value = false
    await router.push({ name: 'drive', query: driveFilterQuery() })
    return
  }
  if (route.name !== 'drive-search') {
    await router.push({ name: 'drive-search', query: driveFilterQuery() })
  } else {
    await replaceDriveQuery()
  }
  searchMode.value = true
  await driveStore.search(query)
}

async function clearDriveFilters() {
  driveStore.clearFilters()
  if (searchMode.value) {
    searchMode.value = false
    await router.push('/drive')
    return
  }
  await replaceDriveQuery()
  await refreshDrive()
}

async function refreshDrive() {
  if (trashMode.value) {
    await driveStore.loadTrash()
    return
  }
  if (sharedMode.value) {
    await driveStore.loadSharedWithMe()
    return
  }
  if (starredMode.value) {
    await driveStore.loadStarred()
    return
  }
  if (recentMode.value) {
    await driveStore.loadRecent()
    return
  }
  if (storageMode.value) {
    await driveStore.loadStorage()
    return
  }
  if (searchMode.value && driveStore.query) {
    await driveStore.search(driveStore.query)
    return
  }
  await driveStore.refreshCurrent()
}

async function updateTypeFilter(filter: DriveTypeFilter) {
  driveStore.setTypeFilter(filter)
  await replaceDriveQuery()
  await refreshDrive()
}

async function updateOwnerFilter(filter: DriveOwnerFilter) {
  driveStore.setOwnerFilter(filter)
  await replaceDriveQuery()
  await refreshDrive()
}

async function updateModifiedFilter(filter: DriveModifiedFilter) {
  driveStore.setModifiedFilter(filter)
  await replaceDriveQuery()
  await refreshDrive()
}

async function updateSourceFilter(filter: DriveSourceFilter) {
  driveStore.setSourceFilter(filter)
  await replaceDriveQuery()
  await refreshDrive()
}

async function updateSort(key: DriveSortKey) {
  driveStore.setSort(key)
  await replaceDriveQuery()
  await refreshDrive()
}

async function toggleStar(item: DriveItemBody) {
  try {
    await driveStore.toggleStar(item)
    actionMessage.value = item.starredByMe ? 'Star を外しました。' : 'Star を付けました。'
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}
</script>

<template>
  <DriveWorkspaceLayout :details-open="detailsPanelOpen">
    <template #side>
      <DriveSideNav
        :current-folder="driveStore.currentFolder"
        :children="driveStore.children"
        :folder-tree="driveStore.folderTree"
        :active-view="activeDriveView"
        :workspace-name="currentWorkspaceName"
        :storage-usage="driveStore.storageUsage"
        :disabled="!tenantStore.activeTenant"
        :busy="driveStore.isBusy"
        @create-folder="requestCreateFolder"
        @upload-file="uploadFile"
        @upload-files="uploadFiles"
        @open-folder="navigateFolder"
      />
    </template>

    <template #header>
      <header class="drive-workspace-header">
        <div class="drive-title-group">
          <span class="status-pill">Drive</span>
          <h1>{{ driveTitle }}</h1>
          <p>{{ t('drive.headerDescription') }}</p>
          <DriveBreadcrumbs v-if="!searchMode && !trashMode" :current-folder="driveStore.currentFolder" />
        </div>
        <div class="drive-header-actions">
          <label class="field compact-field">
            <span class="field-label">{{ t('drive.workspace') }}</span>
            <select class="field-input" :value="currentWorkspaceId" :disabled="driveStore.isBusy" @change="selectWorkspace">
              <option v-for="workspace in driveStore.workspaces" :key="workspace.publicId" :value="workspace.publicId">
                {{ workspace.name }}
              </option>
            </select>
          </label>
          <button class="secondary-button compact-button" type="button" :disabled="driveStore.isBusy || !tenantStore.activeTenant" @click="requestCreateWorkspace">
            {{ t('drive.newWorkspace') }}
          </button>
          <button class="secondary-button compact-button" type="button" @click="detailsPanelOpen = !detailsPanelOpen">
            {{ detailsPanelOpen ? t('drive.hideDetails') : t('drive.showDetails') }}
          </button>
        </div>
      </header>

      <div class="drive-quick-stats" :aria-label="t('drive.summary')">
        <span>{{ activeTenantLabel }}</span>
        <span>{{ t('drive.itemCount', { count: itemCount }) }}</span>
        <span>{{ t('drive.fileCount', { count: fileCount }) }}</span>
        <span>{{ t('drive.folderCount', { count: folderCount }) }}</span>
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
        :selection-count="selectedArchiveItems.length"
        @search="search"
        @clear-filters="clearDriveFilters"
        @update-view-mode="driveStore.setViewMode"
        @update-type-filter="updateTypeFilter"
        @update-modified-filter="updateModifiedFilter"
        @update-owner-filter="updateOwnerFilter"
        @update-source-filter="updateSourceFilter"
        @update-sort="updateSort"
        @refresh="refreshDrive"
        @download-archive="downloadArchive()"
        @clear-selection="driveStore.clearSelection"
      />
    </template>

    <div
      class="drive-workspace-content"
      :class="{ 'drop-active': dropActive }"
      @dragenter="onDragEnter"
      @dragover="onDragOver"
      @dragleave="onDragLeave"
      @drop="onDropFiles"
    >
      <div v-if="dropActive" class="drive-drop-overlay" aria-live="polite">
        {{ t('drive.dropFiles') }}
      </div>
      <p v-if="tenantStore.status === 'empty'" class="warning-message">
        {{ t('drive.noTenantMessage') }}
      </p>
      <p v-if="tenantStore.status === 'error'" class="error-message">{{ tenantStore.errorMessage }}</p>
      <p v-if="actionErrorMessage || driveStore.errorMessage" class="error-message">
        {{ actionErrorMessage || driveStore.errorMessage }}
      </p>
      <p v-if="actionMessage" class="notice-message">{{ actionMessage }}</p>

      <div v-if="trashMode" class="action-row">
        <RouterLink class="secondary-button link-button compact-button" to="/drive">
          {{ t('drive.backToDrive') }}
        </RouterLink>
        <button class="secondary-button compact-button" type="button" :disabled="driveStore.isBusy" @click="refreshDrive">
          {{ t('common.refresh') }}
        </button>
      </div>

      <section v-if="storageMode" class="data-card">
        <div class="card-header">
          <div>
            <h2>{{ t('drive.storageUsage') }}</h2>
            <p>{{ t('drive.storageDescription') }}</p>
          </div>
        </div>
        <dl class="metadata-grid">
          <div>
            <dt>{{ t('drive.storage') }}</dt>
            <dd class="tabular-cell">{{ formatDriveSize(driveStore.storageUsage?.usedBytes) }}</dd>
          </div>
          <div>
            <dt>{{ t('drive.trashBytes') }}</dt>
            <dd class="tabular-cell">{{ formatDriveSize(driveStore.storageUsage?.trashBytes) }}</dd>
          </div>
          <div>
            <dt>{{ t('common.files') }}</dt>
            <dd class="tabular-cell">{{ driveStore.storageUsage?.fileCount ?? 0 }}</dd>
          </div>
          <div>
            <dt>{{ t('drive.storageDriver') }}</dt>
            <dd>{{ driveStore.storageUsage?.storageDriver || '-' }}</dd>
          </div>
        </dl>
      </section>

      <EmptyState
        v-if="driveStore.status === 'forbidden'"
        :title="t('drive.accessDeniedTitle')"
        :message="t('drive.accessDeniedMessage')"
      />

      <template v-else-if="!storageMode && (visibleItems.length > 0 || driveStore.status === 'loading')">
        <DriveItemGrid
          v-if="driveStore.viewMode === 'grid'"
          :items="visibleItems"
          :loading="driveStore.status === 'loading'"
          :busy-resource-id="driveStore.busyResourceId"
          :deleting-resource-id="driveStore.deletingResourceId"
          :selected-resource-id="selectedResourceId"
          :selected-resource-ids="driveStore.selectedResourceIds"
          :trash-mode="trashMode"
          @open-folder="navigateFolder"
          @download-file="downloadFile"
          @rename-item="renameItem"
          @move-item="moveItem"
          @overwrite-file="requestOverwrite"
          @delete-item="askDelete"
          @copy-item="copyItem"
          @download-archive="(item) => downloadArchive([item])"
          @edit-metadata-item="openMetadataDialog"
          @preview-item="openPreviewDialog"
          @share-item="openShareDialog"
          @restore-item="restoreItem"
          @permanently-delete-item="permanentlyDeleteItem"
          @details-item="openDetailsPanel"
          @toggle-star="toggleStar"
          @toggle-select="driveStore.toggleSelectedItem"
        />
        <DriveItemList
          v-else
          :items="visibleItems"
          :loading="driveStore.status === 'loading'"
          :busy-resource-id="driveStore.busyResourceId"
          :deleting-resource-id="driveStore.deletingResourceId"
          :selected-resource-id="selectedResourceId"
          :selected-resource-ids="driveStore.selectedResourceIds"
          :trash-mode="trashMode"
          @open-folder="navigateFolder"
          @download-file="downloadFile"
          @rename-item="renameItem"
          @move-item="moveItem"
          @overwrite-file="requestOverwrite"
          @delete-item="askDelete"
          @copy-item="copyItem"
          @download-archive="(item) => downloadArchive([item])"
          @edit-metadata-item="openMetadataDialog"
          @preview-item="openPreviewDialog"
          @share-item="openShareDialog"
          @restore-item="restoreItem"
          @permanently-delete-item="permanentlyDeleteItem"
          @details-item="openDetailsPanel"
          @toggle-star="toggleStar"
          @toggle-select="driveStore.toggleSelectedItem"
        />
      </template>

      <EmptyState
        v-else-if="!storageMode"
        :title="emptyTitle"
        :message="emptyMessage"
      >
        <template #actions>
          <button v-if="!trashMode" class="primary-button compact-button" type="button" @click="requestCreateFolder">
            {{ t('drive.newFolder') }}
          </button>
          <button v-if="searchMode" class="secondary-button compact-button" type="button" @click="clearDriveFilters">
            {{ t('drive.backToDrive') }}
          </button>
        </template>
      </EmptyState>

      <input ref="overwriteInput" class="drive-hidden-input" type="file" @change="onOverwriteFileChange">
      <DriveUploadQueue
        :items="driveStore.uploadQueue"
        :busy="driveStore.isBusy"
        @retry="driveStore.retryUpload"
        @cancel="driveStore.cancelUpload"
        @clear-completed="driveStore.clearCompletedUploads"
      />
    </div>

    <template #details>
      <DriveDetailsPanel
        :open="detailsPanelOpen"
        :selected-item="driveStore.selectedItem"
        :current-folder="driveStore.currentFolder"
        :permissions="driveStore.permissions"
        :ocr-result="driveStore.ocrResult"
        :product-extraction-items="driveStore.productExtractionItems"
        :ocr-loading="driveStore.ocrLoading"
        :busy-resource-id="driveStore.busyResourceId"
        :activities="driveStore.activityItems"
        :item-count="itemCount"
        :file-count="fileCount"
        :folder-count="folderCount"
        @close="closeDetailsPanel"
        @share-item="openShareDialog"
        @request-ocr="requestOCR"
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
    @update-share-role="updateShareRole"
    @transfer-owner="transferOwner"
    @reload-permissions="reloadPermissions"
  />

  <DriveMetadataDialog
    :open="metadataDialogOpen"
    :item="metadataTarget"
    :busy="driveStore.isBusy"
    :error-message="actionErrorMessage || driveStore.errorMessage"
    @close="closeMetadataDialog"
    @save="saveMetadata"
  />

  <DrivePreviewDialog
    :open="previewTarget !== null"
    :item="previewTarget"
    @close="closePreviewDialog"
  />

  <ConfirmActionDialog
    :open="pendingDelete !== null"
    :title="deleteTitle"
    :message="deleteMessage"
    :confirm-label="t('common.delete')"
    @cancel="cancelDelete"
    @confirm="confirmDelete"
  />

  <TextInputDialog
    :open="workspaceDialogOpen"
    :title="t('drive.dialogs.newWorkspaceTitle')"
    :label="t('drive.dialogs.workspaceName')"
    :placeholder="t('drive.dialogs.workspacePlaceholder')"
    :confirm-label="t('drive.dialogs.createWorkspace')"
    @cancel="cancelCreateWorkspace"
    @confirm="createWorkspace"
  />

  <TextInputDialog
    :open="createFolderDialogOpen"
    :title="t('drive.dialogs.newFolderTitle')"
    :label="t('drive.dialogs.folderName')"
    :placeholder="t('drive.dialogs.folderPlaceholder')"
    :confirm-label="t('drive.dialogs.createFolder')"
    @cancel="cancelCreateFolder"
    @confirm="createFolder"
  />

  <TextInputDialog
    :open="pendingRename !== null"
    :title="t('drive.dialogs.renameTitle')"
    :label="t('drive.dialogs.newName')"
    :initial-value="renameInitialValue"
    :confirm-label="t('drive.dialogs.rename')"
    @cancel="cancelRename"
    @confirm="confirmRename"
  />

  <TextInputDialog
    :open="pendingMove !== null"
    :title="t('drive.dialogs.moveTitle')"
    :label="t('drive.dialogs.targetFolderPublicId')"
    :message="moveMessage"
    :placeholder="t('drive.dialogs.targetFolderPlaceholder')"
    :confirm-label="t('drive.dialogs.move')"
    allow-empty
    @cancel="cancelMove"
    @confirm="confirmMove"
  />
</template>
