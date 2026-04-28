import { defineStore } from 'pinia'

import {
  addDriveGroupMemberItem,
  copyDriveItem,
  createDriveOCRJobItem,
  createDriveProductExtractionJobItem,
  createDriveFolderItem,
  createDriveGroupItem,
  createDriveShareInvitationItem,
  createDriveShareItem,
  createDriveShareLinkItem,
  createDriveWorkspaceItem,
  deleteDriveFileItem,
  deleteDriveFolderItem,
  disableDriveShareLinkItem,
  downloadDriveArchiveItems,
  downloadDriveFileItem,
  fetchDriveActivity,
  fetchDriveFolder,
  fetchDriveFolderTree,
  fetchDriveFile,
  fetchDriveGroup,
  fetchDriveGroups,
  fetchDriveItems,
  fetchDriveOCR,
  fetchDriveProductExtractions,
  fetchDriveRecentItems,
  fetchDriveSharedWithMe,
  fetchDriveStarredItems,
  fetchDriveStorageUsage,
  fetchDriveTrashItems,
  fetchDriveWorkspaces,
  fetchDrivePermissions,
  overwriteDriveFileItem,
  permanentlyDeleteDriveItem,
  removeDriveGroupMemberItem,
  restoreDriveFileItem,
  restoreDriveFolderItem,
  revokeDriveShareItem,
  searchDriveItemsByKeyword,
  starDriveItem,
  transferDriveOwner,
  unstarDriveItem,
  updateDriveFileItem,
  updateDriveFolderItem,
  updateDriveShareItem,
  uploadDriveFileItem,
  type DriveListFilters,
  type DriveDownloadedFile,
  type DriveResourceRef,
} from '../api/drive'
import { toApiErrorMessage } from '../api/client'
import { presentDriveActionError, presentDriveUploadError } from '../utils/driveErrors'
import {
  driveOcrActionStatusFromRunStatus,
  isDriveOcrActiveStatus,
  isDriveOcrTerminalStatus,
  type DriveOcrActionStatus,
} from '../utils/driveOcrStatus'
import type {
  DriveFileBody,
  DriveActivityBody,
  DriveFolderBody,
  DriveFolderTreeBody,
  DriveGroupBody,
  DriveItemBody,
  DriveOcrOutputBody,
  DrivePermissionsBody,
  DriveProductExtractionItemBody,
  DriveShareBody,
  DriveShareInvitationBody,
  DriveShareLinkBody,
  DriveStorageUsageBody,
  DriveWorkspaceBody,
} from '../api/generated/types.gen'

type DriveStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'forbidden' | 'error'
type DriveActionStatus = 'idle' | 'working'
type DriveRole = 'owner' | 'editor' | 'viewer'
type DriveUploadStatus = 'queued' | 'uploading' | 'complete' | 'error'
export type DriveViewMode = 'grid' | 'list'
export type DriveTypeFilter = 'all' | 'file' | 'folder'
export type DriveOwnerFilter = 'all' | 'me' | 'shared_with_me'
export type DriveModifiedFilter = 'any' | 'today' | 'last_7_days' | 'last_30_days'
export type DriveSourceFilter = 'all' | 'upload' | 'external' | 'generated' | 'sync'
export type DriveSortKey = 'updated_at' | 'name' | 'size'
export type DriveSortDirection = 'asc' | 'desc'
export type DriveUploadQueueItem = {
  id: string
  file: File
  status: DriveUploadStatus
  progress: number
  errorMessage: string
  errorTitle: string
  errorAction: string
  errorRequestId: string
  retryable: boolean
}

const OCR_POLL_INTERVAL_MS = 2500
const OCR_PRODUCT_FOLLOWUP_POLLS = 6
let ocrPollTimer: ReturnType<typeof setTimeout> | null = null

function clearOCRPollTimer() {
  if (!ocrPollTimer) {
    return
  }
  clearTimeout(ocrPollTimer)
  ocrPollTimer = null
}

const rootFolder: DriveFolderBody = {
  publicId: 'root',
  name: 'Root',
  inheritanceEnabled: true,
  createdAt: new Date(0).toISOString(),
  updatedAt: new Date(0).toISOString(),
}

function driveItemWithDefaults(item: DriveItemBody): DriveItemBody {
  return {
    ...item,
    ownedByMe: item.ownedByMe ?? true,
    sharedWithMe: item.sharedWithMe ?? false,
    starredByMe: item.starredByMe ?? false,
    source: item.source ?? (item.file ? 'upload' : 'generated'),
  }
}

function statusFromError(error: unknown): DriveStatus {
  return /permission|forbidden|authorization|authz/i.test(toApiErrorMessage(error)) ? 'forbidden' : 'error'
}

export function resourceFromDriveItem(item: DriveItemBody): DriveResourceRef | null {
  if (item.type === 'file' && item.file) {
    return { type: 'file', publicId: item.file.publicId }
  }
  if (item.type === 'folder' && item.folder) {
    return { type: 'folder', publicId: item.folder.publicId }
  }
  return null
}

export function labelFromDriveItem(item: DriveItemBody) {
  if (item.type === 'file') {
    return item.file?.originalFilename ?? 'Untitled file'
  }
  return item.folder?.name ?? 'Untitled folder'
}

export const useDriveStore = defineStore('drive', {
  state: () => ({
    status: 'idle' as DriveStatus,
    actionStatus: 'idle' as DriveActionStatus,
    currentFolder: rootFolder as DriveFolderBody,
    workspaces: [] as DriveWorkspaceBody[],
    currentWorkspace: null as DriveWorkspaceBody | null,
    children: [] as DriveItemBody[],
    searchResults: [] as DriveItemBody[],
    sharedItems: [] as DriveItemBody[],
    starredItems: [] as DriveItemBody[],
    recentItems: [] as DriveItemBody[],
    trashItems: [] as DriveItemBody[],
    storageUsage: null as DriveStorageUsageBody | null,
    folderTree: null as DriveFolderTreeBody | null,
    activityItems: [] as DriveActivityBody[],
    selectedItem: null as DriveItemBody | null,
    selectedResource: null as DriveResourceRef | null,
    permissions: null as DrivePermissionsBody | null,
    ocrResult: null as DriveOcrOutputBody | null,
    productExtractionItems: [] as DriveProductExtractionItemBody[],
    ocrLoading: false,
    ocrActionStatus: 'idle' as DriveOcrActionStatus,
    ocrActionResourceId: '',
    ocrErrorMessage: '',
    ocrPollingResourceId: '',
    ocrProductFollowupPolls: 0,
    productExtractionActionStatus: 'idle' as DriveOcrActionStatus,
    productExtractionActionResourceId: '',
    productExtractionErrorMessage: '',
    groups: [] as DriveGroupBody[],
    currentGroup: null as DriveGroupBody | null,
    invitations: [] as DriveShareInvitationBody[],
    errorMessage: '',
    lastRawShareLink: null as DriveShareLinkBody | null,
    deletingResourceId: '',
    busyResourceId: '',
    selectedResourceIds: [] as string[],
    uploadQueue: [] as DriveUploadQueueItem[],
    viewMode: 'grid' as DriveViewMode,
    query: '',
    typeFilter: 'all' as DriveTypeFilter,
    ownerFilter: 'all' as DriveOwnerFilter,
    modifiedFilter: 'any' as DriveModifiedFilter,
    sourceFilter: 'all' as DriveSourceFilter,
    sortKey: 'updated_at' as DriveSortKey,
    sortDirection: 'desc' as DriveSortDirection,
  }),

  getters: {
    isBusy: (state) => state.actionStatus === 'working' || state.status === 'loading',
  },

  actions: {
    clearOCRPolling() {
      clearOCRPollTimer()
      this.ocrPollingResourceId = ''
      this.ocrProductFollowupPolls = 0
    },

    clearOCRActionState() {
      this.clearOCRPolling()
      this.ocrActionStatus = 'idle'
      this.ocrActionResourceId = ''
      this.ocrErrorMessage = ''
      this.productExtractionActionStatus = 'idle'
      this.productExtractionActionResourceId = ''
      this.productExtractionErrorMessage = ''
    },

    resetSelection() {
      this.selectedItem = null
      this.selectedResource = null
      this.permissions = null
      this.ocrResult = null
      this.productExtractionItems = []
      this.clearOCRActionState()
      this.lastRawShareLink = null
      this.activityItems = []
    },

    setViewMode(mode: DriveViewMode) {
      this.viewMode = mode
    },

    setQuery(query: string) {
      this.query = query
    },

    setTypeFilter(filter: DriveTypeFilter) {
      this.typeFilter = filter
    },

    setOwnerFilter(filter: DriveOwnerFilter) {
      this.ownerFilter = filter
    },

    setModifiedFilter(filter: DriveModifiedFilter) {
      this.modifiedFilter = filter
    },

    setSourceFilter(filter: DriveSourceFilter) {
      this.sourceFilter = filter
    },

    setSort(key: DriveSortKey) {
      if (this.sortKey === key) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc'
        return
      }
      this.sortKey = key
      this.sortDirection = key === 'name' ? 'asc' : 'desc'
    },

    clearFilters() {
      this.query = ''
      this.typeFilter = 'all'
      this.ownerFilter = 'all'
      this.modifiedFilter = 'any'
      this.sourceFilter = 'all'
    },

    clearSelection() {
      this.selectedResourceIds = []
    },

    toggleSelectedItem(item: DriveItemBody) {
      const resource = resourceFromDriveItem(item)
      if (!resource) {
        return
      }
      if (this.selectedResourceIds.includes(resource.publicId)) {
        this.selectedResourceIds = this.selectedResourceIds.filter((id) => id !== resource.publicId)
        return
      }
      this.selectedResourceIds = [...this.selectedResourceIds, resource.publicId]
    },

    clearCompletedUploads() {
      this.uploadQueue = this.uploadQueue.filter((item) => item.status !== 'complete')
    },

    listFilters(): DriveListFilters {
      return {
        type: this.typeFilter,
        owner: this.ownerFilter,
        source: this.sourceFilter,
        sort: this.sortKey,
        direction: this.sortDirection,
      }
    },

    setError(error: unknown) {
      this.errorMessage = toApiErrorMessage(error)
      this.status = statusFromError(error)
    },

    async loadRoot() {
      await this.loadFolder('root')
    },

    async loadFolder(folderPublicId: string) {
      this.status = 'loading'
      this.errorMessage = ''
      this.resetSelection()
      try {
        if (this.workspaces.length === 0) {
          this.workspaces = await fetchDriveWorkspaces()
          this.currentWorkspace = this.workspaces[0] ?? null
        }
        this.currentFolder = folderPublicId === 'root' ? rootFolder : await fetchDriveFolder(folderPublicId)
        this.children = await fetchDriveItems(
          folderPublicId === 'root' ? '' : folderPublicId,
          folderPublicId === 'root' ? this.currentWorkspace?.publicId ?? '' : '',
          this.listFilters(),
        )
        await this.loadStorage()
        await this.loadFolderTree()
        this.status = this.children.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.currentFolder = rootFolder
        this.children = []
        this.setError(error)
      }
    },

    async loadFileDetail(filePublicId: string) {
      if (!filePublicId) {
        return
      }
      this.status = 'loading'
      this.errorMessage = ''
      this.resetSelection()
      try {
        if (this.workspaces.length === 0) {
          this.workspaces = await fetchDriveWorkspaces()
          this.currentWorkspace = this.workspaces[0] ?? null
        }
        const file = await fetchDriveFile(filePublicId)
        const item = driveItemWithDefaults({ type: 'file', file } as DriveItemBody)
        const resource: DriveResourceRef = { type: 'file', publicId: file.publicId }
        this.currentFolder = rootFolder
        this.children = [item]
        this.selectedItem = item
        this.selectedResource = resource
        await Promise.all([
          this.loadStorage(),
          this.loadFolderTree(),
          this.loadPermissions(resource),
          this.loadActivity(resource),
          this.loadOCR(resource),
        ])
        this.status = 'ready'
      } catch (error) {
        this.children = []
        this.setError(error)
      }
    },

    async refreshCurrent() {
      await this.loadFolder(this.currentFolder.publicId)
    },

    async loadSharedWithMe() {
      this.status = 'loading'
      this.errorMessage = ''
      this.resetSelection()
      try {
        this.sharedItems = await fetchDriveSharedWithMe()
        await this.loadStorage()
        await this.loadFolderTree()
        this.status = this.sharedItems.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.sharedItems = []
        this.setError(error)
      }
    },

    async loadStarred() {
      this.status = 'loading'
      this.errorMessage = ''
      this.resetSelection()
      try {
        this.starredItems = await fetchDriveStarredItems()
        await this.loadStorage()
        await this.loadFolderTree()
        this.status = this.starredItems.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.starredItems = []
        this.setError(error)
      }
    },

    async loadRecent() {
      this.status = 'loading'
      this.errorMessage = ''
      this.resetSelection()
      try {
        this.recentItems = await fetchDriveRecentItems()
        await this.loadStorage()
        await this.loadFolderTree()
        this.status = this.recentItems.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.recentItems = []
        this.setError(error)
      }
    },

    async loadStorage() {
      try {
        this.storageUsage = await fetchDriveStorageUsage()
      } catch {
        this.storageUsage = null
      }
    },

    async loadFolderTree() {
      try {
        this.folderTree = await fetchDriveFolderTree()
      } catch {
        this.folderTree = null
      }
    },

    async loadTrash() {
      this.status = 'loading'
      this.errorMessage = ''
      this.resetSelection()
      try {
        this.trashItems = await fetchDriveTrashItems()
        await this.loadStorage()
        await this.loadFolderTree()
        this.status = this.trashItems.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.trashItems = []
        this.setError(error)
      }
    },

    async selectWorkspace(workspacePublicId: string) {
      const workspace = this.workspaces.find((item) => item.publicId === workspacePublicId) ?? null
      this.currentWorkspace = workspace
      await this.loadFolder('root')
    },

    async createWorkspace(name: string) {
      const trimmed = name.trim()
      if (!trimmed) {
        return
      }
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        const workspace = await createDriveWorkspaceItem({ name: trimmed })
        this.workspaces = [...this.workspaces, workspace].sort((a, b) => a.name.localeCompare(b.name))
        this.currentWorkspace = workspace
        await this.loadFolder('root')
        return workspace
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async createFolder(name: string) {
      const trimmed = name.trim()
      if (!trimmed) {
        return
      }

      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        const folder = await createDriveFolderItem({
          name: trimmed,
          workspacePublicId: this.currentFolder.publicId === 'root' ? this.currentWorkspace?.publicId : undefined,
          parentFolderPublicId: this.currentFolder.publicId === 'root' ? undefined : this.currentFolder.publicId,
        })
        this.children = [driveItemWithDefaults({ type: 'folder', folder } as DriveItemBody), ...this.children]
        this.status = 'ready'
        return folder
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async uploadFile(file: File) {
      const uploaded = await this.uploadFiles([file])
      return uploaded[0]?.file
    },

    async uploadFiles(files: File[]) {
      const queue = files.filter(Boolean)
      if (queue.length === 0) {
        return []
      }
      this.actionStatus = 'working'
      this.errorMessage = ''
      const queueItems: DriveUploadQueueItem[] = queue.map((file) => ({
        id: crypto.randomUUID(),
        file,
        status: 'queued',
        progress: 0,
        errorMessage: '',
        errorTitle: '',
        errorAction: '',
        errorRequestId: '',
        retryable: true,
      }))
      this.uploadQueue = [...this.uploadQueue, ...queueItems]
      const uploadedItems: DriveItemBody[] = []
      try {
        let cursor = 0
        const worker = async () => {
          while (cursor < queueItems.length) {
            const item = queueItems[cursor]
            cursor += 1
            if (!item) {
              continue
            }
            const uploaded = await this.uploadQueueItem(item)
            if (uploaded) {
              uploadedItems.push(uploaded)
            }
          }
        }
        await Promise.all(Array.from({ length: Math.min(3, queueItems.length) }, () => worker()))
        this.children = [...uploadedItems, ...this.children]
        await this.loadStorage()
        this.status = this.children.length > 0 ? 'ready' : this.status
        return uploadedItems
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async uploadQueueItem(item: DriveUploadQueueItem) {
      this.uploadQueue = this.uploadQueue.map((candidate) => (
        candidate.id === item.id
          ? { ...candidate, status: 'uploading', progress: 10, errorMessage: '', errorTitle: '', errorAction: '', errorRequestId: '', retryable: true }
          : candidate
      ))
      try {
        const uploaded = await uploadDriveFileItem(
          item.file,
          this.currentFolder.publicId === 'root' ? '' : this.currentFolder.publicId,
          this.currentFolder.publicId === 'root' ? this.currentWorkspace?.publicId ?? '' : '',
        )
        const driveItem = driveItemWithDefaults({ type: 'file', file: uploaded } as DriveItemBody)
        this.uploadQueue = this.uploadQueue.map((candidate) => (
          candidate.id === item.id
            ? { ...candidate, status: 'complete', progress: 100, errorMessage: '', errorTitle: '', errorAction: '', errorRequestId: '', retryable: true }
            : candidate
        ))
        return driveItem
      } catch (error) {
        const presentation = presentDriveUploadError(error)
        this.uploadQueue = this.uploadQueue.map((candidate) => (
          candidate.id === item.id
            ? {
              ...candidate,
              status: 'error',
              progress: 0,
              errorMessage: presentation.reason,
              errorTitle: presentation.title,
              errorAction: presentation.action,
              errorRequestId: presentation.requestId,
              retryable: presentation.retryable,
            }
            : candidate
        ))
        return null
      }
    },

    async retryUpload(id: string) {
      const item = this.uploadQueue.find((candidate) => candidate.id === id)
      if (!item || item.status !== 'error' || item.retryable === false) {
        return
      }
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        const uploaded = await this.uploadQueueItem(item)
        if (uploaded) {
          this.children = [uploaded, ...this.children]
          await this.loadStorage()
          this.status = 'ready'
        }
      } finally {
        this.actionStatus = 'idle'
      }
    },

    cancelUpload(id: string) {
      this.uploadQueue = this.uploadQueue.filter((item) => item.id !== id || item.status === 'uploading')
    },

    async downloadFile(file: DriveFileBody): Promise<DriveDownloadedFile> {
      this.busyResourceId = file.publicId
      this.errorMessage = ''
      try {
        return await downloadDriveFileItem(file)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.busyResourceId = ''
      }
    },

    async renameFile(file: DriveFileBody, originalFilename: string) {
      const name = originalFilename.trim()
      if (!name || name === file.originalFilename) {
        return file
      }
      this.busyResourceId = file.publicId
      this.errorMessage = ''
      try {
        const updated = await updateDriveFileItem(file.publicId, { originalFilename: name })
        this.children = this.children.map((item) => (
          item.file?.publicId === updated.publicId ? { ...item, file: updated } : item
        ))
        return updated
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.busyResourceId = ''
      }
    },

    async moveFile(file: DriveFileBody, targetFolderPublicId: string) {
      this.busyResourceId = file.publicId
      this.errorMessage = ''
      try {
        const updated = await updateDriveFileItem(file.publicId, {
          parentFolderPublicId: targetFolderPublicId.trim() || 'root',
        })
        this.children = this.children.filter((item) => item.file?.publicId !== updated.publicId)
        this.status = this.children.length > 0 ? 'ready' : 'empty'
        return updated
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.busyResourceId = ''
      }
    },

    async overwriteFile(file: DriveFileBody, body: File) {
      this.busyResourceId = file.publicId
      this.errorMessage = ''
      try {
        const updated = await overwriteDriveFileItem(file.publicId, body)
        this.children = this.children.map((item) => (
          item.file?.publicId === updated.publicId ? { ...item, file: updated } : item
        ))
        return updated
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.busyResourceId = ''
      }
    },

    async renameFolder(folder: DriveFolderBody, name: string) {
      const trimmed = name.trim()
      if (!trimmed || trimmed === folder.name) {
        return folder
      }
      this.busyResourceId = folder.publicId
      this.errorMessage = ''
      try {
        const updated = await updateDriveFolderItem(folder.publicId, { name: trimmed })
        if (this.currentFolder.publicId === updated.publicId) {
          this.currentFolder = updated
        }
        this.children = this.children.map((item) => (
          item.folder?.publicId === updated.publicId ? { ...item, folder: updated } : item
        ))
        return updated
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.busyResourceId = ''
      }
    },

    async moveFolder(folder: DriveFolderBody, targetFolderPublicId: string) {
      this.busyResourceId = folder.publicId
      this.errorMessage = ''
      try {
        const updated = await updateDriveFolderItem(folder.publicId, {
          parentFolderPublicId: targetFolderPublicId.trim() || 'root',
        })
        this.children = this.children.filter((item) => item.folder?.publicId !== updated.publicId)
        this.status = this.children.length > 0 ? 'ready' : 'empty'
        return updated
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.busyResourceId = ''
      }
    },

    async deleteItem(item: DriveItemBody) {
      const resource = resourceFromDriveItem(item)
      if (!resource) {
        return
      }

      this.deletingResourceId = resource.publicId
      this.errorMessage = ''
      try {
        if (resource.type === 'file') {
          await deleteDriveFileItem(resource.publicId)
          this.children = this.children.filter((child) => child.file?.publicId !== resource.publicId)
        } else {
          await deleteDriveFolderItem(resource.publicId)
          this.children = this.children.filter((child) => child.folder?.publicId !== resource.publicId)
        }
        this.status = this.children.length > 0 ? 'ready' : 'empty'
        await this.loadStorage()
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.deletingResourceId = ''
      }
    },

    async copyItem(item: DriveItemBody) {
      const resource = resourceFromDriveItem(item)
      if (!resource) {
        return
      }
      this.busyResourceId = resource.publicId
      this.errorMessage = ''
      try {
        const copied = driveItemWithDefaults(await copyDriveItem(resource))
        this.children = [copied, ...this.children]
        await this.loadStorage()
        this.status = 'ready'
        return copied
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.busyResourceId = ''
      }
    },

    async downloadArchive(items: DriveItemBody[], filename = 'drive-archive.zip'): Promise<DriveDownloadedFile> {
      const resources = items
        .map(resourceFromDriveItem)
        .filter((item): item is NonNullable<ReturnType<typeof resourceFromDriveItem>> => Boolean(item))
      if (resources.length === 0) {
        throw new Error('archive items are required')
      }
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        return await downloadDriveArchiveItems({
          filename,
          items: resources.map((resource) => ({
            type: resource.type,
            publicId: resource.publicId,
          })),
        })
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async restoreItem(item: DriveItemBody) {
      const resource = resourceFromDriveItem(item)
      if (!resource) {
        return
      }

      this.busyResourceId = resource.publicId
      this.errorMessage = ''
      try {
        const restored = resource.type === 'file'
          ? await restoreDriveFileItem(resource.publicId)
          : await restoreDriveFolderItem(resource.publicId)
        this.trashItems = this.trashItems.filter((child) => resourceFromDriveItem(child)?.publicId !== resource.publicId)
        await this.loadStorage()
        this.status = this.trashItems.length > 0 ? 'ready' : 'empty'
        return restored
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.busyResourceId = ''
      }
    },

    async permanentlyDeleteItem(item: DriveItemBody) {
      const resource = resourceFromDriveItem(item)
      if (!resource) {
        return
      }
      this.deletingResourceId = resource.publicId
      this.errorMessage = ''
      try {
        await permanentlyDeleteDriveItem(resource)
        this.trashItems = this.trashItems.filter((child) => resourceFromDriveItem(child)?.publicId !== resource.publicId)
        await this.loadStorage()
        this.status = this.trashItems.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.deletingResourceId = ''
      }
    },

    async search(query: string, contentType = '') {
      this.status = 'loading'
      this.errorMessage = ''
      try {
        this.searchResults = await searchDriveItemsByKeyword(query, contentType, this.listFilters())
        this.status = this.searchResults.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.searchResults = []
        this.setError(error)
      }
    },

    async selectItem(item: DriveItemBody) {
      const resource = resourceFromDriveItem(item)
      this.clearOCRActionState()
      this.selectedItem = item
      this.selectedResource = resource
      this.lastRawShareLink = null
      if (resource) {
        await this.loadPermissions(resource)
        await this.loadActivity(resource)
        await this.loadOCR(resource)
        this.syncOCRPollingForResource(resource)
      }
    },

    async loadOCR(resource?: DriveResourceRef | null, options: { showLoading?: boolean } = {}) {
      const target = resource ?? this.selectedResource
      if (!target || target.type !== 'file') {
        this.ocrResult = null
        this.productExtractionItems = []
        return
      }
      const showLoading = options.showLoading ?? true
      if (showLoading) {
        this.ocrLoading = true
      }
      try {
        const [ocr, products] = await Promise.all([
          fetchDriveOCR(target.publicId).catch(() => null),
          fetchDriveProductExtractions(target.publicId).catch(() => []),
        ])
        this.ocrResult = ocr
        this.productExtractionItems = products
      } finally {
        if (showLoading) {
          this.ocrLoading = false
        }
      }
    },

    async requestOCR(file: DriveFileBody) {
      this.clearOCRPolling()
      this.ocrActionStatus = 'requesting'
      this.ocrActionResourceId = file.publicId
      this.ocrErrorMessage = ''
      this.productExtractionActionStatus = 'idle'
      this.productExtractionActionResourceId = ''
      this.productExtractionErrorMessage = ''
      this.busyResourceId = file.publicId
      this.errorMessage = ''
      try {
        const job = await createDriveOCRJobItem(file.publicId)
        this.ocrActionStatus = driveOcrActionStatusFromRunStatus(job.status)
        if (this.selectedResource?.type === 'file' && this.selectedResource.publicId === file.publicId) {
          await this.loadOCR({ type: 'file', publicId: file.publicId }, { showLoading: false })
          const status = this.ocrResult?.run.status || job.status
          this.ocrActionStatus = driveOcrActionStatusFromRunStatus(status)
          if (!isDriveOcrTerminalStatus(status)) {
            this.startOCRPolling(file.publicId)
          }
        }
        return job
      } catch (error) {
        const message = presentDriveActionError(error)
        this.ocrActionStatus = 'failed'
        this.ocrErrorMessage = message
        this.errorMessage = message
        throw error
      } finally {
        this.busyResourceId = ''
      }
    },

    async requestProductExtraction(file: DriveFileBody) {
      this.productExtractionActionStatus = 'requesting'
      this.productExtractionActionResourceId = file.publicId
      this.productExtractionErrorMessage = ''
      this.busyResourceId = file.publicId
      this.errorMessage = ''
      try {
        this.productExtractionActionStatus = 'polling'
        await createDriveProductExtractionJobItem(file.publicId)
        if (this.selectedResource?.type === 'file' && this.selectedResource.publicId === file.publicId) {
          await this.loadOCR({ type: 'file', publicId: file.publicId }, { showLoading: false })
        }
        this.productExtractionActionStatus = 'succeeded'
      } catch (error) {
        const message = presentDriveActionError(error)
        this.productExtractionActionStatus = 'failed'
        this.productExtractionErrorMessage = message
        this.errorMessage = message
        throw error
      } finally {
        this.busyResourceId = ''
      }
    },

    syncOCRPollingForResource(resource?: DriveResourceRef | null) {
      if (!resource || resource.type !== 'file') {
        this.clearOCRPolling()
        return
      }
      const status = this.ocrResult?.run.status
      if (isDriveOcrActiveStatus(status)) {
        this.ocrActionResourceId = resource.publicId
        this.ocrActionStatus = driveOcrActionStatusFromRunStatus(status)
        this.ocrErrorMessage = ''
        this.startOCRPolling(resource.publicId)
        return
      }
      this.clearOCRPolling()
      if (this.ocrActionResourceId === resource.publicId && status) {
        this.ocrActionStatus = driveOcrActionStatusFromRunStatus(status)
      }
    },

    startOCRPolling(filePublicId: string) {
      this.clearOCRPolling()
      this.ocrPollingResourceId = filePublicId
      this.ocrActionResourceId = filePublicId
      this.ocrActionStatus = 'polling'
      this.scheduleOCRPoll(filePublicId)
    },

    scheduleOCRPoll(filePublicId: string) {
      clearOCRPollTimer()
      ocrPollTimer = setTimeout(() => {
        void this.pollOCR(filePublicId)
      }, OCR_POLL_INTERVAL_MS)
    },

    async pollOCR(filePublicId: string) {
      if (this.ocrPollingResourceId !== filePublicId) {
        return
      }
      if (this.selectedResource?.type !== 'file' || this.selectedResource.publicId !== filePublicId) {
        this.clearOCRPolling()
        return
      }
      this.ocrActionStatus = 'polling'
      try {
        await this.loadOCR({ type: 'file', publicId: filePublicId }, { showLoading: false })
        const status = this.ocrResult?.run.status
        if (status) {
          this.ocrActionStatus = driveOcrActionStatusFromRunStatus(status)
        }
        if (status === 'completed' && this.productExtractionItems.length === 0 && this.ocrProductFollowupPolls < OCR_PRODUCT_FOLLOWUP_POLLS) {
          this.ocrActionStatus = 'polling'
          this.ocrProductFollowupPolls += 1
          this.scheduleOCRPoll(filePublicId)
          return
        }
        if (isDriveOcrTerminalStatus(status)) {
          this.clearOCRPolling()
          return
        }
        this.scheduleOCRPoll(filePublicId)
      } catch (error) {
        this.ocrActionStatus = 'failed'
        this.ocrErrorMessage = presentDriveActionError(error)
        this.clearOCRPolling()
      }
    },

    async loadActivity(resource?: DriveResourceRef | null) {
      const target = resource ?? this.selectedResource
      if (!target) {
        this.activityItems = []
        return
      }
      try {
        this.activityItems = await fetchDriveActivity(target)
      } catch {
        this.activityItems = []
      }
    },

    async toggleStar(item: DriveItemBody) {
      const resource = resourceFromDriveItem(item)
      if (!resource) {
        return
      }
      this.busyResourceId = resource.publicId
      this.errorMessage = ''
      try {
        if (item.starredByMe) {
          await unstarDriveItem(resource)
        } else {
          await starDriveItem(resource)
        }
        const update = (candidate: DriveItemBody) => (
          resourceFromDriveItem(candidate)?.publicId === resource.publicId
            ? { ...candidate, starredByMe: !item.starredByMe }
            : candidate
        )
        this.children = this.children.map(update)
        this.searchResults = this.searchResults.map(update)
        this.sharedItems = this.sharedItems.map(update)
        this.recentItems = this.recentItems.map(update)
        if (item.starredByMe) {
          this.starredItems = this.starredItems.filter((candidate) => resourceFromDriveItem(candidate)?.publicId !== resource.publicId)
        } else {
          this.starredItems = this.starredItems.map(update)
        }
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.busyResourceId = ''
      }
    },

    async loadPermissions(resource?: DriveResourceRef | null) {
      const target = resource ?? this.selectedResource
      if (!target) {
        this.permissions = null
        return
      }
      this.errorMessage = ''
      try {
        this.permissions = await fetchDrivePermissions(target)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      }
    },

    async createUserShare(resource: DriveResourceRef, subjectPublicId: string, role: string): Promise<DriveShareBody> {
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        const share = await createDriveShareItem(resource, {
          subjectType: 'user',
          subjectPublicId: subjectPublicId.trim(),
          role: role as DriveRole,
        })
        await this.loadPermissions(resource)
        return share
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async createGroupShare(resource: DriveResourceRef, groupPublicId: string, role: string): Promise<DriveShareBody> {
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        const share = await createDriveShareItem(resource, {
          subjectType: 'group',
          subjectPublicId: groupPublicId,
          role: role as DriveRole,
        })
        await this.loadPermissions(resource)
        return share
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async createExternalInvitation(resource: DriveResourceRef, inviteeEmail: string, role: string): Promise<DriveShareInvitationBody> {
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        return await createDriveShareInvitationItem(resource, {
          inviteeEmail: inviteeEmail.trim(),
          role: role as DriveRole,
        })
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async revokeShare(resource: DriveResourceRef, sharePublicId: string) {
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        await revokeDriveShareItem(resource, sharePublicId)
        await this.loadPermissions(resource)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async updateShareRole(resource: DriveResourceRef, sharePublicId: string, role: string) {
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        await updateDriveShareItem(resource, sharePublicId, { role: role as 'editor' | 'viewer' })
        await this.loadPermissions(resource)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async transferOwner(resource: DriveResourceRef, newOwnerUserPublicId: string, revokePreviousOwnerAccess: boolean) {
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        const item = driveItemWithDefaults(await transferDriveOwner(resource, {
          newOwnerUserPublicId: newOwnerUserPublicId.trim(),
          revokePreviousOwnerAccess,
        }))
        const update = (candidate: DriveItemBody) => (
          resourceFromDriveItem(candidate)?.publicId === resource.publicId ? item : candidate
        )
        this.children = this.children.map(update)
        this.searchResults = this.searchResults.map(update)
        this.sharedItems = this.sharedItems.map(update)
        this.starredItems = this.starredItems.map(update)
        this.recentItems = this.recentItems.map(update)
        if (this.selectedResource?.publicId === resource.publicId) {
          this.selectedItem = item
          await this.loadPermissions(resource)
        }
        return item
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async updateItemMetadata(item: DriveItemBody, description: string, tags: string[]) {
      const resource = resourceFromDriveItem(item)
      if (!resource) {
        return
      }
      this.busyResourceId = resource.publicId
      this.errorMessage = ''
      try {
        const body = {
          description,
          tags,
        }
        const updatedItem = resource.type === 'file'
          ? driveItemWithDefaults({ ...item, file: await updateDriveFileItem(resource.publicId, body) })
          : driveItemWithDefaults({ ...item, folder: await updateDriveFolderItem(resource.publicId, body) })
        updatedItem.tags = tags
        const update = (candidate: DriveItemBody) => (
          resourceFromDriveItem(candidate)?.publicId === resource.publicId ? updatedItem : candidate
        )
        this.children = this.children.map(update)
        this.searchResults = this.searchResults.map(update)
        this.sharedItems = this.sharedItems.map(update)
        this.starredItems = this.starredItems.map(update)
        this.recentItems = this.recentItems.map(update)
        if (this.selectedResource?.publicId === resource.publicId) {
          this.selectedItem = updatedItem
        }
        return updatedItem
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.busyResourceId = ''
      }
    },

    async createShareLink(resource: DriveResourceRef, expiresAt: string, canDownload: boolean, password = '', role = 'viewer') {
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        const link = await createDriveShareLinkItem(resource, {
          expiresAt: new Date(expiresAt).toISOString(),
          canDownload,
          password: password.trim() || undefined,
          role: (role === 'editor' ? 'editor' : 'viewer') as 'editor' | 'viewer',
        })
        this.lastRawShareLink = link
        await this.loadPermissions(resource)
        return link
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async disableShareLink(resource: DriveResourceRef, shareLinkPublicId: string) {
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        await disableDriveShareLinkItem(shareLinkPublicId)
        if (this.lastRawShareLink?.publicId === shareLinkPublicId) {
          this.lastRawShareLink = null
        }
        await this.loadPermissions(resource)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async loadGroups() {
      this.errorMessage = ''
      try {
        this.groups = await fetchDriveGroups()
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      }
    },

    async loadGroup(groupPublicId: string) {
      this.errorMessage = ''
      this.currentGroup = await fetchDriveGroup(groupPublicId)
      return this.currentGroup
    },

    async createGroup(name: string, description = '') {
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        const group = await createDriveGroupItem({ name: name.trim(), description: description.trim() })
        this.groups = [group, ...this.groups]
        return group
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async addGroupMember(group: DriveGroupBody, userPublicId: string) {
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        await addDriveGroupMemberItem(group.publicId, userPublicId.trim())
        await this.loadGroup(group.publicId)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },

    async removeGroupMember(group: DriveGroupBody, userPublicId: string) {
      this.actionStatus = 'working'
      this.errorMessage = ''
      try {
        await removeDriveGroupMemberItem(group.publicId, userPublicId)
        await this.loadGroup(group.publicId)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.actionStatus = 'idle'
      }
    },
  },
})
