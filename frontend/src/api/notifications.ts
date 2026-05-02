import { readCookie } from './client'
import {
  listNotifications,
  markAllNotificationsRead,
  markNotificationRead,
  markNotificationsRead,
} from './generated/sdk.gen'
import type { NotificationBody, NotificationListBody } from './generated/types.gen'

export type NotificationReadState = 'all' | 'unread' | 'read'
export type NotificationChannel = 'in_app' | 'email'

export type NotificationListParams = {
  q?: string
  readState?: NotificationReadState
  channel?: NotificationChannel
  createdAfter?: string
  cursor?: string
  limit?: number
}

export type NotificationListResult = {
  items: NotificationBody[]
  nextCursor: string
  totalCount: number
  filteredCount: number
  unreadCount: number
  readCount: number
}

export type NotificationReadAllParams = Pick<NotificationListParams, 'q' | 'readState' | 'channel' | 'createdAfter'>

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export async function fetchNotifications(params: NotificationListParams = {}): Promise<NotificationListResult> {
  const data = await listNotifications({
    query: {
      q: params.q || undefined,
      readState: params.readState ?? 'all',
      channel: params.channel,
      createdAfter: params.createdAfter,
      cursor: params.cursor,
      limit: params.limit ?? 25,
    },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as NotificationListBody

  return normalizeNotificationList(data)
}

export async function markNotificationItemRead(notificationPublicId: string): Promise<NotificationBody> {
  return markNotificationRead({
    headers: csrfHeaders(),
    path: { notificationPublicId },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<NotificationBody>
}

export async function markNotificationItemsRead(publicIds: string[]): Promise<NotificationBody[]> {
  const data = await markNotificationsRead({
    headers: csrfHeaders(),
    body: { publicIds },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: NotificationBody[] | null }

  return data.items ?? []
}

export async function markMatchingNotificationsRead(params: NotificationReadAllParams): Promise<number> {
  const data = await markAllNotificationsRead({
    headers: csrfHeaders(),
    body: {
      q: params.q || undefined,
      readState: params.readState ?? 'all',
      channel: params.channel,
      createdAfter: params.createdAfter,
    },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { updatedCount?: number }

  return data.updatedCount ?? 0
}

function normalizeNotificationList(data: NotificationListBody): NotificationListResult {
  return {
    items: data.items ?? [],
    nextCursor: data.nextCursor ?? '',
    totalCount: data.totalCount ?? 0,
    filteredCount: data.filteredCount ?? 0,
    unreadCount: data.unreadCount ?? 0,
    readCount: data.readCount ?? 0,
  }
}
