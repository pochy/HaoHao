import { defineStore } from 'pinia'

import { toApiErrorMessage } from '../api/client'
import { fetchNotifications, markNotificationItemRead } from '../api/notifications'
import type { NotificationBody } from '../api/generated/types.gen'

type NotificationStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'error'

export const useNotificationStore = defineStore('notifications', {
  state: () => ({
    status: 'idle' as NotificationStatus,
    items: [] as NotificationBody[],
    errorMessage: '',
    updatingPublicId: '',
  }),

  getters: {
    unreadCount: (state) => state.items.filter((item) => !item.readAt).length,
  },

  actions: {
    async load() {
      this.status = 'loading'
      this.errorMessage = ''
      try {
        this.items = await fetchNotifications()
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.items = []
        this.status = 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async markRead(publicId: string) {
      this.updatingPublicId = publicId
      this.errorMessage = ''
      try {
        const updated = await markNotificationItemRead(publicId)
        this.items = this.items.map((item) => (item.publicId === publicId ? updated : item))
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.updatingPublicId = ''
      }
    },

    upsert(item: NotificationBody) {
      this.items = [item, ...this.items.filter((existing) => existing.publicId !== item.publicId)]
      this.status = this.items.length > 0 ? 'ready' : 'empty'
    },

    markReadFromRealtime(item: NotificationBody) {
      this.items = this.items.map((existing) => (existing.publicId === item.publicId ? item : existing))
      if (!this.items.some((existing) => existing.publicId === item.publicId)) {
        this.items = [item, ...this.items]
      }
      this.status = this.items.length > 0 ? 'ready' : 'empty'
    },
  },
})
