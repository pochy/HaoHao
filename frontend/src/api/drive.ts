import {
  acceptDriveShareInvitation,
  addDriveGroupMember,
  copyDriveFile,
  copyDriveFolder,
  createDriveFileShare,
  createDriveFileShareInvitation,
  createDriveFileShareLink,
  createDriveFolder,
  createDriveFolderShare,
  createDriveFolderShareInvitation,
  createDriveFolderShareLink,
  createDriveGroup,
  createDriveOcrJob,
  createDriveProductExtractionJob,
  createDriveWorkspace,
  deleteDriveFile,
  deleteDriveFileShare,
  deleteDriveFolder,
  deleteDriveFolderShare,
  deleteDriveGroup,
  deleteDriveGroupMember,
  deleteDriveShareLink,
  getCsrf,
  getDriveFolderTree,
  getDriveFile,
  getDriveStorageUsage,
  getDriveFilePermissions,
  getDriveFolder,
  getDriveFolderPermissions,
  getDriveGroup,
  getDriveOcr,
  getPublicDriveShareLink,
  listDriveFileActivity,
  listDriveFolderActivity,
  listDriveGroups,
  listDriveItems,
  listDriveProductExtractions,
  listDriveRecent,
  listPublicDriveShareLinkChildren,
  listDriveShareInvitations,
  listDriveShareTargets,
  listDriveSharedWithMe,
  listDriveStarred,
  permanentlyDeleteDriveFile,
  permanentlyDeleteDriveFolder,
  listDriveTrashItems,
  listDriveWorkspaces,
  restoreDriveFile,
  restoreDriveFolder,
  revokeDriveShareInvitation,
  searchDriveDocuments,
  searchDriveItems,
  starDriveFile,
  starDriveFolder,
  transferDriveFileOwner,
  transferDriveFolderOwner,
  unstarDriveFile,
  unstarDriveFolder,
  updateDriveFile,
  updateDriveFileShare,
  updateDriveFolder,
  updateDriveFolderShare,
  updateDriveGroup,
  updateDriveShareLink,
} from './generated/sdk.gen'
import type {
  CreateDriveFolderBodyWritable,
  CreateDriveGroupBodyWritable,
  CreateDriveShareBodyWritable,
  CreateDriveShareInvitationBodyWritable,
  CreateDriveShareLinkBodyWritable,
  CreateDriveWorkspaceBodyWritable,
  DriveActivityBody,
  DriveArchiveBodyWritable,
  DriveCopyBodyWritable,
  DriveFileBody,
  DriveFolderBody,
  DriveFolderTreeBody,
  DriveGroupBody,
  DriveItemBody,
  DriveOcrJobBody,
  DriveOcrOutputBody,
  DriveProductExtractionJobBody,
  DriveSearchResultBody,
  DrivePermissionsBody,
  DriveProductExtractionItemBody,
  DriveShareBody,
  DriveShareInvitationBody,
  DriveShareLinkBody,
  DriveShareTargetBody,
  DriveStorageUsageBody,
  DriveWorkspaceBody,
  DriveOwnerTransferBodyWritable,
  PublicDriveShareLinkOutputBody,
  RestoreDriveResourceBodyWritable,
  UpdateDriveFileBodyWritable,
  UpdateDriveFolderBodyWritable,
  UpdateDriveShareBodyWritable,
  UpdateDriveShareLinkBodyWritable,
} from './generated/types.gen'
import { apiErrorFromResponse, readCookie } from './client'

export type DriveResourceType = 'file' | 'folder'

export type DriveResourceRef = {
  type: DriveResourceType
  publicId: string
}

export type DriveListFilters = {
  type?: 'all' | 'file' | 'folder'
  owner?: 'all' | 'me' | 'shared_with_me'
  source?: 'all' | 'upload' | 'external' | 'generated' | 'sync'
  sort?: 'name' | 'updated_at' | 'size'
  direction?: 'asc' | 'desc'
}

export type DriveDownloadedFile = {
  blob: Blob
  filename: string
}

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

function encodePath(value: string) {
  return encodeURIComponent(value)
}

function contentDispositionFilename(header: string | null, fallback: string) {
  if (!header) {
    return fallback
  }

  const utf8Match = /filename\*=UTF-8''([^;]+)/i.exec(header)
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1])
    } catch {
      return utf8Match[1]
    }
  }

  const quotedMatch = /filename="([^"]+)"/i.exec(header)
  if (quotedMatch?.[1]) {
    return quotedMatch[1]
  }

  const plainMatch = /filename=([^;]+)/i.exec(header)
  return plainMatch?.[1]?.trim() || fallback
}

async function ensureCSRFCookie() {
  if (readCookie('XSRF-TOKEN')) {
    return
  }
  await getCsrf()
}

async function driveFetch(input: RequestInfo | URL, init: RequestInit = {}) {
  const method = (init.method ?? 'GET').toUpperCase()
  const headers = new Headers(init.headers)
  headers.set('Accept', headers.get('Accept') ?? 'application/json')

  if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
    await ensureCSRFCookie()
    const token = readCookie('XSRF-TOKEN')
    if (token) {
      headers.set('X-CSRF-Token', token)
    }
    if (method === 'POST' && !headers.get('Idempotency-Key')) {
      headers.set('Idempotency-Key', crypto.randomUUID())
    }
  }

  const response = await fetch(input, {
    ...init,
    credentials: 'include',
    headers,
  })

  if (!response.ok) {
    throw await apiErrorFromResponse(response, `Drive request failed (${response.status})`)
  }

  return response
}

export async function fetchDriveFolder(folderPublicId: string): Promise<DriveFolderBody> {
  return getDriveFolder({
    path: { folderPublicId },
  }) as unknown as Promise<DriveFolderBody>
}

export async function fetchDriveWorkspaces(): Promise<DriveWorkspaceBody[]> {
  const data = await listDriveWorkspaces() as unknown as { items: DriveWorkspaceBody[] | null }
  return data.items ?? []
}

export async function createDriveWorkspaceItem(body: CreateDriveWorkspaceBodyWritable): Promise<DriveWorkspaceBody> {
  return createDriveWorkspace({
    headers: csrfHeaders(),
    body,
  }) as unknown as Promise<DriveWorkspaceBody>
}

export async function fetchDriveItems(parentFolderPublicId = '', workspacePublicId = '', filters: DriveListFilters = {}): Promise<DriveItemBody[]> {
  const data = await listDriveItems({
    query: {
      ...(parentFolderPublicId ? { parentFolderPublicId } : {}),
      ...(workspacePublicId ? { workspacePublicId } : {}),
      ...(filters.type ? { type: filters.type } : {}),
      ...(filters.owner ? { owner: filters.owner } : {}),
      ...(filters.source ? { source: filters.source } : {}),
      ...(filters.sort ? { sort: filters.sort } : {}),
      ...(filters.direction ? { direction: filters.direction } : {}),
    },
  }) as unknown as { items: DriveItemBody[] | null }
  return data.items ?? []
}

export async function searchDriveItemsByKeyword(query: string, contentType = '', filters: DriveListFilters = {}): Promise<DriveItemBody[]> {
  const data = await searchDriveItems({
    query: {
      q: query,
      contentType,
      ...(filters.type ? { type: filters.type } : {}),
      ...(filters.owner ? { owner: filters.owner } : {}),
      ...(filters.source ? { source: filters.source } : {}),
      ...(filters.sort ? { sort: filters.sort } : {}),
      ...(filters.direction ? { direction: filters.direction } : {}),
    },
  }) as unknown as { items: DriveItemBody[] | null }
  return data.items ?? []
}

export async function searchDriveDocumentsByKeyword(query: string, contentType = '', filters: DriveListFilters = {}): Promise<DriveSearchResultBody[]> {
  const data = await searchDriveDocuments({
    query: {
      q: query,
      contentType,
      ...(filters.type ? { type: filters.type } : {}),
      ...(filters.owner ? { owner: filters.owner } : {}),
      ...(filters.source ? { source: filters.source } : {}),
      ...(filters.sort ? { sort: filters.sort } : {}),
      ...(filters.direction ? { direction: filters.direction } : {}),
    },
  }) as unknown as { items: DriveSearchResultBody[] | null }
  return data.items ?? []
}

export async function fetchDriveSharedWithMe(): Promise<DriveItemBody[]> {
  const data = await listDriveSharedWithMe() as unknown as { items: DriveItemBody[] | null }
  return data.items ?? []
}

export async function fetchDriveStarredItems(): Promise<DriveItemBody[]> {
  const data = await listDriveStarred() as unknown as { items: DriveItemBody[] | null }
  return data.items ?? []
}

export async function fetchDriveRecentItems(): Promise<DriveItemBody[]> {
  const data = await listDriveRecent() as unknown as { items: DriveItemBody[] | null }
  return data.items ?? []
}

export async function fetchDriveStorageUsage(): Promise<DriveStorageUsageBody> {
  return getDriveStorageUsage() as unknown as Promise<DriveStorageUsageBody>
}

export async function fetchDriveFolderTree(): Promise<DriveFolderTreeBody> {
  return getDriveFolderTree() as unknown as Promise<DriveFolderTreeBody>
}

export async function fetchPublicDriveShareLinkChildren(token: string): Promise<DriveItemBody[]> {
  const data = await listPublicDriveShareLinkChildren({
    path: { token },
  }) as unknown as { items: DriveItemBody[] | null }
  return data.items ?? []
}

export async function fetchDriveTrashItems(): Promise<DriveItemBody[]> {
  const data = await listDriveTrashItems() as unknown as { items: DriveItemBody[] | null }
  return data.items ?? []
}

export async function createDriveFolderItem(body: CreateDriveFolderBodyWritable): Promise<DriveFolderBody> {
  return createDriveFolder({
    headers: csrfHeaders(),
    body,
  }) as unknown as Promise<DriveFolderBody>
}

export async function updateDriveFolderItem(folderPublicId: string, body: UpdateDriveFolderBodyWritable): Promise<DriveFolderBody> {
  return updateDriveFolder({
    headers: csrfHeaders(),
    path: { folderPublicId },
    body,
  }) as unknown as Promise<DriveFolderBody>
}

export async function deleteDriveFolderItem(folderPublicId: string): Promise<void> {
  await deleteDriveFolder({
    headers: csrfHeaders(),
    path: { folderPublicId },
  })
}

export async function copyDriveItem(resource: DriveResourceRef, body: DriveCopyBodyWritable = {}): Promise<DriveItemBody> {
  if (resource.type === 'file') {
    return copyDriveFile({
      headers: csrfHeaders(),
      path: { filePublicId: resource.publicId },
      body,
    }) as unknown as Promise<DriveItemBody>
  }
  return copyDriveFolder({
    headers: csrfHeaders(),
    path: { folderPublicId: resource.publicId },
    body,
  }) as unknown as Promise<DriveItemBody>
}

export async function transferDriveOwner(resource: DriveResourceRef, body: DriveOwnerTransferBodyWritable): Promise<DriveItemBody> {
  if (resource.type === 'file') {
    return transferDriveFileOwner({
      headers: csrfHeaders(),
      path: { filePublicId: resource.publicId },
      body,
    }) as unknown as Promise<DriveItemBody>
  }
  return transferDriveFolderOwner({
    headers: csrfHeaders(),
    path: { folderPublicId: resource.publicId },
    body,
  }) as unknown as Promise<DriveItemBody>
}

export async function downloadDriveArchiveItems(body: DriveArchiveBodyWritable): Promise<DriveDownloadedFile> {
  const response = await driveFetch('/api/v1/drive/downloads/archive', {
    method: 'POST',
    headers: {
      Accept: 'application/zip',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  })
  return {
    blob: await response.blob(),
    filename: contentDispositionFilename(response.headers.get('Content-Disposition'), body.filename || 'drive-archive.zip'),
  }
}

export async function permanentlyDeleteDriveItem(resource: DriveResourceRef): Promise<void> {
  if (resource.type === 'file') {
    await permanentlyDeleteDriveFile({
      headers: csrfHeaders(),
      path: { filePublicId: resource.publicId },
    })
    return
  }
  await permanentlyDeleteDriveFolder({
    headers: csrfHeaders(),
    path: { folderPublicId: resource.publicId },
  })
}

export async function restoreDriveFolderItem(folderPublicId: string, parentFolderPublicId = ''): Promise<DriveFolderBody> {
  const body: RestoreDriveResourceBodyWritable = parentFolderPublicId
    ? { parentFolderPublicId }
    : {}
  return restoreDriveFolder({
    headers: csrfHeaders(),
    path: { folderPublicId },
    body,
  }) as unknown as Promise<DriveFolderBody>
}

export async function uploadDriveFileItem(file: File, parentFolderPublicId = '', workspacePublicId = ''): Promise<DriveFileBody> {
  const form = new FormData()
  if (workspacePublicId) {
    form.append('workspacePublicId', workspacePublicId)
  }
  if (parentFolderPublicId && parentFolderPublicId !== 'root') {
    form.append('parentFolderPublicId', parentFolderPublicId)
  }
  form.append('file', file)
  const response = await driveFetch('/api/v1/drive/files', {
    method: 'POST',
    body: form,
  })
  return response.json() as Promise<DriveFileBody>
}

export async function fetchDriveFile(filePublicId: string): Promise<DriveFileBody> {
  return getDriveFile({
    path: { filePublicId },
  }) as unknown as Promise<DriveFileBody>
}

export async function updateDriveFileItem(filePublicId: string, body: UpdateDriveFileBodyWritable): Promise<DriveFileBody> {
  return updateDriveFile({
    headers: csrfHeaders(),
    path: { filePublicId },
    body,
  }) as unknown as Promise<DriveFileBody>
}

export async function overwriteDriveFileItem(filePublicId: string, file: File): Promise<DriveFileBody> {
  const form = new FormData()
  form.append('file', file)
  const response = await driveFetch(`/api/v1/drive/files/${encodePath(filePublicId)}/content`, {
    method: 'PUT',
    body: form,
  })
  return response.json() as Promise<DriveFileBody>
}

export async function downloadDriveFileItem(file: DriveFileBody): Promise<DriveDownloadedFile> {
  const response = await driveFetch(`/api/v1/drive/files/${encodePath(file.publicId)}/content`, {
    method: 'GET',
    headers: { Accept: file.contentType || 'application/octet-stream' },
  })
  return {
    blob: await response.blob(),
    filename: contentDispositionFilename(response.headers.get('Content-Disposition'), file.originalFilename),
  }
}

export async function createDriveOCRJobItem(filePublicId: string): Promise<DriveOcrJobBody> {
  return createDriveOcrJob({
    headers: csrfHeaders(),
    path: { filePublicId },
  }) as unknown as Promise<DriveOcrJobBody>
}

export async function createDriveProductExtractionJobItem(filePublicId: string): Promise<DriveProductExtractionJobBody> {
  return createDriveProductExtractionJob({
    headers: csrfHeaders(),
    path: { filePublicId },
  }) as unknown as Promise<DriveProductExtractionJobBody>
}

export async function fetchDriveOCR(filePublicId: string): Promise<DriveOcrOutputBody> {
  return getDriveOcr({
    path: { filePublicId },
  }) as unknown as Promise<DriveOcrOutputBody>
}

export async function fetchDriveProductExtractions(filePublicId: string): Promise<DriveProductExtractionItemBody[]> {
  const data = await listDriveProductExtractions({
    path: { filePublicId },
  }) as unknown as { items: DriveProductExtractionItemBody[] | null }
  return data.items ?? []
}

export async function deleteDriveFileItem(filePublicId: string): Promise<void> {
  await deleteDriveFile({
    headers: csrfHeaders(),
    path: { filePublicId },
  })
}

export async function starDriveItem(resource: DriveResourceRef): Promise<void> {
  if (resource.type === 'file') {
    await starDriveFile({
      headers: csrfHeaders(),
      path: { filePublicId: resource.publicId },
    })
    return
  }
  await starDriveFolder({
    headers: csrfHeaders(),
    path: { folderPublicId: resource.publicId },
  })
}

export async function unstarDriveItem(resource: DriveResourceRef): Promise<void> {
  if (resource.type === 'file') {
    await unstarDriveFile({
      headers: csrfHeaders(),
      path: { filePublicId: resource.publicId },
    })
    return
  }
  await unstarDriveFolder({
    headers: csrfHeaders(),
    path: { folderPublicId: resource.publicId },
  })
}

export async function fetchDriveActivity(resource: DriveResourceRef): Promise<DriveActivityBody[]> {
  if (resource.type === 'file') {
    const data = await listDriveFileActivity({
      path: { filePublicId: resource.publicId },
    }) as unknown as { items: DriveActivityBody[] | null }
    return data.items ?? []
  }
  const data = await listDriveFolderActivity({
    path: { folderPublicId: resource.publicId },
  }) as unknown as { items: DriveActivityBody[] | null }
  return data.items ?? []
}

export async function fetchDriveShareTargets(query: string): Promise<DriveShareTargetBody[]> {
  const data = await listDriveShareTargets({
    query: { q: query.trim() },
  }) as unknown as { items: DriveShareTargetBody[] | null }
  return data.items ?? []
}

export async function restoreDriveFileItem(filePublicId: string, parentFolderPublicId = ''): Promise<DriveFileBody> {
  const body: RestoreDriveResourceBodyWritable = parentFolderPublicId
    ? { parentFolderPublicId }
    : {}
  return restoreDriveFile({
    headers: csrfHeaders(),
    path: { filePublicId },
    body,
  }) as unknown as Promise<DriveFileBody>
}

export async function fetchDrivePermissions(resource: DriveResourceRef): Promise<DrivePermissionsBody> {
  if (resource.type === 'file') {
    return getDriveFilePermissions({
      path: { filePublicId: resource.publicId },
    }) as unknown as Promise<DrivePermissionsBody>
  }
  return getDriveFolderPermissions({
    path: { folderPublicId: resource.publicId },
  }) as unknown as Promise<DrivePermissionsBody>
}

export async function createDriveShareItem(resource: DriveResourceRef, body: CreateDriveShareBodyWritable): Promise<DriveShareBody> {
  if (resource.type === 'file') {
    return createDriveFileShare({
      headers: csrfHeaders(),
      path: { filePublicId: resource.publicId },
      body,
    }) as unknown as Promise<DriveShareBody>
  }
  return createDriveFolderShare({
    headers: csrfHeaders(),
    path: { folderPublicId: resource.publicId },
    body,
  }) as unknown as Promise<DriveShareBody>
}

export async function createDriveShareInvitationItem(
  resource: DriveResourceRef,
  body: CreateDriveShareInvitationBodyWritable,
): Promise<DriveShareInvitationBody> {
  if (resource.type === 'file') {
    return createDriveFileShareInvitation({
      headers: csrfHeaders(),
      path: { filePublicId: resource.publicId },
      body,
    }) as unknown as Promise<DriveShareInvitationBody>
  }
  return createDriveFolderShareInvitation({
    headers: csrfHeaders(),
    path: { folderPublicId: resource.publicId },
    body,
  }) as unknown as Promise<DriveShareInvitationBody>
}

export async function revokeDriveShareItem(resource: DriveResourceRef, sharePublicId: string): Promise<void> {
  if (resource.type === 'file') {
    await deleteDriveFileShare({
      headers: csrfHeaders(),
      path: {
        filePublicId: resource.publicId,
        sharePublicId,
      },
    })
    return
  }
  await deleteDriveFolderShare({
    headers: csrfHeaders(),
    path: {
      folderPublicId: resource.publicId,
      sharePublicId,
    },
  })
}

export async function updateDriveShareItem(resource: DriveResourceRef, sharePublicId: string, body: UpdateDriveShareBodyWritable): Promise<DriveShareBody> {
  if (resource.type === 'file') {
    return updateDriveFileShare({
      headers: csrfHeaders(),
      path: {
        filePublicId: resource.publicId,
        sharePublicId,
      },
      body,
    }) as unknown as Promise<DriveShareBody>
  }
  return updateDriveFolderShare({
    headers: csrfHeaders(),
    path: {
      folderPublicId: resource.publicId,
      sharePublicId,
    },
    body,
  }) as unknown as Promise<DriveShareBody>
}

export async function createDriveShareLinkItem(
  resource: DriveResourceRef,
  body: CreateDriveShareLinkBodyWritable,
): Promise<DriveShareLinkBody> {
  if (resource.type === 'file') {
    return createDriveFileShareLink({
      headers: csrfHeaders(),
      path: { filePublicId: resource.publicId },
      body,
    }) as unknown as Promise<DriveShareLinkBody>
  }
  return createDriveFolderShareLink({
    headers: csrfHeaders(),
    path: { folderPublicId: resource.publicId },
    body,
  }) as unknown as Promise<DriveShareLinkBody>
}

export async function updateDriveShareLinkItem(
  shareLinkPublicId: string,
  body: UpdateDriveShareLinkBodyWritable,
): Promise<DriveShareLinkBody> {
  return updateDriveShareLink({
    headers: csrfHeaders(),
    path: { shareLinkPublicId },
    body,
  }) as unknown as Promise<DriveShareLinkBody>
}

export async function disableDriveShareLinkItem(shareLinkPublicId: string): Promise<void> {
  await deleteDriveShareLink({
    headers: csrfHeaders(),
    path: { shareLinkPublicId },
  })
}

export async function fetchDriveShareInvitations(): Promise<DriveShareInvitationBody[]> {
  const data = await listDriveShareInvitations() as unknown as { items: DriveShareInvitationBody[] | null }
  return data.items ?? []
}

export async function acceptDriveShareInvitationItem(invitationPublicId: string, acceptToken: string): Promise<DriveShareBody> {
  return acceptDriveShareInvitation({
    headers: csrfHeaders(),
    path: { invitationPublicId },
    body: { acceptToken },
  }) as unknown as Promise<DriveShareBody>
}

export async function revokeDriveShareInvitationItem(invitationPublicId: string): Promise<void> {
  await revokeDriveShareInvitation({
    headers: csrfHeaders(),
    path: { invitationPublicId },
  })
}

export async function fetchDriveGroups(): Promise<DriveGroupBody[]> {
  const data = await listDriveGroups() as unknown as { items: DriveGroupBody[] | null }
  return data.items ?? []
}

export async function fetchDriveGroup(groupPublicId: string): Promise<DriveGroupBody> {
  return getDriveGroup({
    path: { groupPublicId },
  }) as unknown as Promise<DriveGroupBody>
}

export async function createDriveGroupItem(body: CreateDriveGroupBodyWritable): Promise<DriveGroupBody> {
  return createDriveGroup({
    headers: csrfHeaders(),
    body,
  }) as unknown as Promise<DriveGroupBody>
}

export async function updateDriveGroupItem(groupPublicId: string, body: CreateDriveGroupBodyWritable): Promise<DriveGroupBody> {
  return updateDriveGroup({
    headers: csrfHeaders(),
    path: { groupPublicId },
    body,
  }) as unknown as Promise<DriveGroupBody>
}

export async function deleteDriveGroupItem(groupPublicId: string): Promise<void> {
  await deleteDriveGroup({
    headers: csrfHeaders(),
    path: { groupPublicId },
  })
}

export async function addDriveGroupMemberItem(groupPublicId: string, userPublicId: string): Promise<void> {
  await addDriveGroupMember({
    headers: csrfHeaders(),
    path: { groupPublicId },
    body: { userPublicId },
  })
}

export async function removeDriveGroupMemberItem(groupPublicId: string, userPublicId: string): Promise<void> {
  await deleteDriveGroupMember({
    headers: csrfHeaders(),
    path: {
      groupPublicId,
      userPublicId,
    },
  })
}

export async function fetchPublicDriveShareLink(token: string): Promise<PublicDriveShareLinkOutputBody> {
  return getPublicDriveShareLink({
    path: { token },
  }) as unknown as Promise<PublicDriveShareLinkOutputBody>
}

export async function verifyPublicDriveShareLinkPassword(token: string, password: string): Promise<void> {
  await driveFetch(`/api/public/drive/share-links/${encodePath(token)}/password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  })
}
