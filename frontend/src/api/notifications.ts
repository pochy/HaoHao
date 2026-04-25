import { readCookie } from './client'
import { listNotifications, markNotificationRead } from './generated/sdk.gen'
import type { NotificationBody } from './generated/types.gen'

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export async function fetchNotifications(): Promise<NotificationBody[]> {
  const data = await listNotifications({
    query: { limit: 50 },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: NotificationBody[] | null }

  return data.items ?? []
}

export async function markNotificationItemRead(notificationPublicId: string): Promise<NotificationBody> {
  return markNotificationRead({
    headers: csrfHeaders(),
    path: { notificationPublicId },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<NotificationBody>
}
