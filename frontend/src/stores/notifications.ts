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
  },
})
