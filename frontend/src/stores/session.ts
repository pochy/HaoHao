import { defineStore } from 'pinia'

import { toApiErrorMessage } from '../api/client'
import type { UserResponse } from '../api/generated/types.gen'
import {
  fetchCurrentSession,
  loginWithPassword,
  logoutCurrentSession,
} from '../api/session'
import { useTenantStore } from './tenants'

type AuthStatus = 'idle' | 'loading' | 'authenticated' | 'anonymous'

export const useSessionStore = defineStore('session', {
  state: () => ({
    status: 'idle' as AuthStatus,
    user: null as UserResponse | null,
    errorMessage: '',
  }),

  actions: {
    async bootstrap() {
      if (this.status !== 'idle') {
        return
      }

      this.status = 'loading'
      this.errorMessage = ''

      try {
        const data = await fetchCurrentSession()
        this.user = data.user
        this.status = 'authenticated'
      } catch {
        this.user = null
        this.status = 'anonymous'
      }
    },

    async login(email: string, password: string) {
      this.status = 'loading'
      this.errorMessage = ''

      try {
        const data = await loginWithPassword(email, password)
        this.user = data.user
        this.status = 'authenticated'
      } catch (error) {
        this.user = null
        this.status = 'anonymous'
        this.errorMessage = toApiErrorMessage(error)
        throw error
      }
    },

    async logout() {
      this.errorMessage = ''

      try {
        const data = await logoutCurrentSession()
        const tenantStore = useTenantStore()
        tenantStore.reset()
        this.user = null
        this.status = 'anonymous'
        return data.postLogoutURL ?? ''
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      }
    },
  },
})
