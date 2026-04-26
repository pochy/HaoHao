import type { DriveItemBody } from '../api/generated/types.gen'

export function driveItemPublicId(item: DriveItemBody) {
  return item.file?.publicId ?? item.folder?.publicId ?? ''
}

export function driveItemName(item: DriveItemBody) {
  if (item.type === 'file') {
    return item.file?.originalFilename ?? 'Untitled file'
  }
  return item.folder?.name ?? 'Untitled folder'
}

export function driveItemUpdatedAt(item: DriveItemBody) {
  return item.file?.updatedAt ?? item.folder?.updatedAt ?? ''
}

export function driveItemDeletedAt(item: DriveItemBody) {
  return item.file?.deletedAt ?? item.folder?.deletedAt ?? ''
}

export function driveItemSize(item: DriveItemBody) {
  return item.file?.byteSize
}

export function driveItemContentType(item: DriveItemBody) {
  return item.file?.contentType ?? ''
}

export function driveItemKind(item: DriveItemBody) {
  if (item.folder) {
    return 'folder'
  }
  const contentType = driveItemContentType(item)
  const name = driveItemName(item).toLowerCase()
  if (contentType.startsWith('image/')) {
    return 'image'
  }
  if (contentType.includes('pdf') || contentType.includes('text') || name.endsWith('.md') || name.endsWith('.txt')) {
    return 'document'
  }
  if (
    contentType.includes('zip') ||
    contentType.includes('gzip') ||
    name.endsWith('.zip') ||
    name.endsWith('.tar.gz') ||
    name.endsWith('.tgz')
  ) {
    return 'archive'
  }
  return 'other'
}

export function formatDriveDate(value: string) {
  if (!value) {
    return '-'
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

export function formatDriveSize(value?: number) {
  if (value === undefined) {
    return '-'
  }
  return new Intl.NumberFormat(undefined, {
    style: 'unit',
    unit: 'byte',
    unitDisplay: 'narrow',
  }).format(value)
}
