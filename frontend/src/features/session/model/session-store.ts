import type { SessionBody } from '@/api/generated/types.gen'
import { fetchSession } from '@/features/session/api/get-session'
import { defineStore } from 'pinia'

type SessionState = {
  data: SessionBody | null
  error: string | null
  loading: boolean
}

export const useSessionStore = defineStore('session', {
  state: (): SessionState => ({
    data: null,
    error: null,
    loading: false,
  }),
  getters: {
    authenticated: (state) => state.data?.authenticated ?? false,
  },
  actions: {
    async bootstrap() {
      this.loading = true
      this.error = null

      try {
        this.data = await fetchSession()
      } catch (error) {
        this.error =
          error instanceof Error ? error.message : 'failed to bootstrap session'
      } finally {
        this.loading = false
      }
    },
  },
})

