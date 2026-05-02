import { defineStore } from 'pinia'

import { toApiErrorMessage } from '../api/client'
import {
  fetchNotifications,
  markMatchingNotificationsRead,
  markNotificationItemRead,
  markNotificationItemsRead,
  type NotificationChannel,
  type NotificationListParams,
  type NotificationReadState,
} from '../api/notifications'
import type { NotificationBody } from '../api/generated/types.gen'

type NotificationStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'error'
export type NotificationChannelFilter = '' | NotificationChannel
export type NotificationCreatedAfterFilter = '' | 'today' | 'last_7_days' | 'last_30_days'

export const useNotificationStore = defineStore('notifications', {
  state: () => ({
    status: 'idle' as NotificationStatus,
    items: [] as NotificationBody[],
    query: '',
    filters: {
      readState: 'all' as NotificationReadState,
      channel: '' as NotificationChannelFilter,
      createdAfter: '' as NotificationCreatedAfterFilter,
    },
    summary: {
      totalCount: 0,
      filteredCount: 0,
      unreadCount: 0,
      readCount: 0,
    },
    nextCursor: '',
    selectedPublicIds: [] as string[],
    errorMessage: '',
    updatingPublicId: '',
    bulkUpdating: false,
    loadingMore: false,
  }),

  getters: {
    unreadCount: (state) => state.summary.unreadCount,
    selectedCount: (state) => state.selectedPublicIds.length,
    selectedUnreadPublicIds: (state) => state.items
      .filter((item) => state.selectedPublicIds.includes(item.publicId) && !item.readAt)
      .map((item) => item.publicId),
    hasActiveFilters: (state) => (
      state.query.trim() !== '' ||
      state.filters.readState !== 'all' ||
      state.filters.channel !== '' ||
      state.filters.createdAfter !== ''
    ),
  },

  actions: {
    async load(params: Pick<NotificationListParams, 'cursor' | 'limit'> = {}) {
      const loadingMore = Boolean(params.cursor)
      if (loadingMore) {
        this.loadingMore = true
      } else {
        this.status = 'loading'
        this.selectedPublicIds = []
      }
      this.errorMessage = ''
      try {
        const data = await fetchNotifications(this.listParams(params))
        this.items = loadingMore
          ? mergeNotifications(this.items, data.items)
          : data.items
        this.nextCursor = data.nextCursor
        this.summary = {
          totalCount: data.totalCount,
          filteredCount: data.filteredCount,
          unreadCount: data.unreadCount,
          readCount: data.readCount,
        }
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        if (!loadingMore) {
          this.items = []
          this.nextCursor = ''
          this.selectedPublicIds = []
        }
        this.status = 'error'
        this.errorMessage = toApiErrorMessage(error)
      } finally {
        this.loadingMore = false
      }
    },

    async markRead(publicId: string) {
      this.updatingPublicId = publicId
      this.errorMessage = ''
      try {
        const updated = await markNotificationItemRead(publicId)
        this.applyReadUpdates([updated])
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.updatingPublicId = ''
      }
    },

    async markSelectedRead() {
      const publicIds = this.selectedUnreadPublicIds
      if (publicIds.length === 0) {
        return []
      }
      this.bulkUpdating = true
      this.errorMessage = ''
      try {
        const updated = await markNotificationItemsRead(publicIds)
        this.applyReadUpdates(updated)
        return updated
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.bulkUpdating = false
      }
    },

    async markAllRead() {
      this.bulkUpdating = true
      this.errorMessage = ''
      try {
        const updatedCount = await markMatchingNotificationsRead(this.listParams())
        await this.load()
        return updatedCount
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.bulkUpdating = false
      }
    },

    toggleSelect(publicId: string) {
      this.selectedPublicIds = this.selectedPublicIds.includes(publicId)
        ? this.selectedPublicIds.filter((existing) => existing !== publicId)
        : [...this.selectedPublicIds, publicId]
    },

    selectVisible() {
      const selected = new Set(this.selectedPublicIds)
      for (const item of this.items) {
        selected.add(item.publicId)
      }
      this.selectedPublicIds = Array.from(selected)
    },

    clearSelection() {
      this.selectedPublicIds = []
    },

    clearFilters() {
      this.query = ''
      this.filters = {
        readState: 'all',
        channel: '',
        createdAfter: '',
      }
      this.selectedPublicIds = []
    },

    upsert(item: NotificationBody) {
      const existing = this.items.find((candidate) => candidate.publicId === item.publicId)
      const matches = notificationMatchesFilters(item, this.query, this.filters)
      if (existing) {
        this.adjustReadCounts(existing, item)
        this.items = this.items
          .map((candidate) => (candidate.publicId === item.publicId ? item : candidate))
          .filter((candidate) => notificationMatchesFilters(candidate, this.query, this.filters))
      } else {
        this.summary.totalCount += 1
        if (item.readAt) {
          this.summary.readCount += 1
        } else {
          this.summary.unreadCount += 1
        }
        if (matches) {
          this.summary.filteredCount += 1
          this.items = [item, ...this.items]
        }
      }
      this.status = this.items.length > 0 ? 'ready' : 'empty'
    },

    markReadFromRealtime(item: NotificationBody) {
      const existing = this.items.find((candidate) => candidate.publicId === item.publicId)
      if (existing) {
        this.applyReadUpdates([item])
        return
      }
      if (item.readAt && this.summary.unreadCount > 0) {
        this.summary.unreadCount -= 1
        this.summary.readCount += 1
      }
      this.status = this.items.length > 0 ? 'ready' : 'empty'
    },

    applyReadUpdates(updatedItems: NotificationBody[]) {
      if (updatedItems.length === 0) {
        return
      }
      const updatedByPublicId = new Map(updatedItems.map((item) => [item.publicId, item]))
      let newlyReadCount = 0
      this.items = this.items.flatMap((item) => {
        const updated = updatedByPublicId.get(item.publicId)
        if (!updated) {
          return [item]
        }
        if (!item.readAt && updated.readAt) {
          newlyReadCount += 1
        }
        if (updated.readAt && this.filters.readState === 'unread') {
          return []
        }
        return [updated]
      })
      this.selectedPublicIds = this.selectedPublicIds.filter((publicId) => !updatedByPublicId.has(publicId))
      this.applyReadCountDelta(newlyReadCount)
      this.status = this.items.length > 0 ? 'ready' : 'empty'
    },

    adjustReadCounts(previous: NotificationBody, next: NotificationBody) {
      if (!previous.readAt && next.readAt) {
        this.applyReadCountDelta(1)
      } else if (previous.readAt && !next.readAt) {
        this.summary.readCount = Math.max(0, this.summary.readCount - 1)
        this.summary.unreadCount += 1
      }
    },

    applyReadCountDelta(count: number) {
      if (count <= 0) {
        return
      }
      this.summary.unreadCount = Math.max(0, this.summary.unreadCount - count)
      this.summary.readCount += count
      if (this.filters.readState === 'unread') {
        this.summary.filteredCount = Math.max(0, this.summary.filteredCount - count)
      }
    },

    listParams(params: Pick<NotificationListParams, 'cursor' | 'limit'> = {}): NotificationListParams {
      return {
        q: this.query.trim() || undefined,
        readState: this.filters.readState,
        channel: this.filters.channel || undefined,
        createdAfter: createdAfterISO(this.filters.createdAfter),
        cursor: params.cursor,
        limit: params.limit ?? 25,
      }
    },
  },
})

function createdAfterISO(filter: NotificationCreatedAfterFilter) {
  if (!filter) {
    return undefined
  }
  const now = new Date()
  if (filter === 'today') {
    return new Date(now.getFullYear(), now.getMonth(), now.getDate()).toISOString()
  }
  const days = filter === 'last_7_days' ? 7 : 30
  return new Date(now.getTime() - days * 24 * 60 * 60 * 1000).toISOString()
}

function mergeNotifications(existing: NotificationBody[], incoming: NotificationBody[]) {
  const seen = new Set<string>()
  const merged: NotificationBody[] = []
  for (const item of [...existing, ...incoming]) {
    if (seen.has(item.publicId)) {
      continue
    }
    seen.add(item.publicId)
    merged.push(item)
  }
  return merged
}

function notificationMatchesFilters(
  item: NotificationBody,
  query: string,
  filters: {
    readState: NotificationReadState
    channel: NotificationChannelFilter
    createdAfter: NotificationCreatedAfterFilter
  },
) {
  if (filters.readState === 'unread' && item.readAt) {
    return false
  }
  if (filters.readState === 'read' && !item.readAt) {
    return false
  }
  if (filters.channel && item.channel !== filters.channel) {
    return false
  }
  const createdAfter = createdAfterISO(filters.createdAfter)
  if (createdAfter && new Date(item.createdAt).getTime() < new Date(createdAfter).getTime()) {
    return false
  }
  const trimmedQuery = query.trim().toLowerCase()
  if (!trimmedQuery) {
    return true
  }
  return [
    item.subject,
    item.body,
    item.template,
  ].some((value) => value.toLowerCase().includes(trimmedQuery))
}
