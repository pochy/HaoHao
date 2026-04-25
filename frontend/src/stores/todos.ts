import { defineStore } from 'pinia'

import { isApiForbidden, toApiErrorMessage, toApiErrorStatus } from '../api/client'
import {
  createTodoItem,
  deleteTodoItem,
  fetchTodos,
  updateTodoItem,
} from '../api/todos'
import type { TodoBody } from '../api/generated/types.gen'

type TodoStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'forbidden' | 'error'

export const useTodoStore = defineStore('todos', {
  state: () => ({
    status: 'idle' as TodoStatus,
    items: [] as TodoBody[],
    errorMessage: '',
    creating: false,
    updatingPublicId: '',
    deletingPublicId: '',
  }),

  actions: {
    async load() {
      this.status = 'loading'
      this.errorMessage = ''

      try {
        this.items = await fetchTodos()
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.items = []
        this.status = toApiErrorStatus(error) === 403 || isApiForbidden(error)
          ? 'forbidden'
          : 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async create(title: string) {
      const normalizedTitle = title.trim()
      if (!normalizedTitle) {
        this.errorMessage = 'TODO title is required.'
        return
      }

      this.creating = true
      this.errorMessage = ''

      try {
        const created = await createTodoItem({ title: normalizedTitle })
        this.items = [created, ...this.items]
        this.status = 'ready'
      } catch (error) {
        if (toApiErrorStatus(error) === 403 || isApiForbidden(error)) {
          this.status = 'forbidden'
        }
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.creating = false
      }
    },

    async toggle(todoPublicId: string, completed: boolean) {
      await this.update(todoPublicId, { completed })
    },

    async rename(todoPublicId: string, title: string) {
      const normalizedTitle = title.trim()
      if (!normalizedTitle) {
        this.errorMessage = 'TODO title is required.'
        return
      }

      await this.update(todoPublicId, { title: normalizedTitle })
    },

    async remove(todoPublicId: string) {
      this.deletingPublicId = todoPublicId
      this.errorMessage = ''

      try {
        await deleteTodoItem(todoPublicId)
        this.items = this.items.filter((item) => item.publicId !== todoPublicId)
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        if (toApiErrorStatus(error) === 403 || isApiForbidden(error)) {
          this.status = 'forbidden'
        }
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.deletingPublicId = ''
      }
    },

    reset() {
      this.status = 'idle'
      this.items = []
      this.errorMessage = ''
      this.creating = false
      this.updatingPublicId = ''
      this.deletingPublicId = ''
    },

    async update(
      todoPublicId: string,
      body: { title?: string, completed?: boolean },
    ) {
      this.updatingPublicId = todoPublicId
      this.errorMessage = ''

      try {
        const updated = await updateTodoItem(todoPublicId, body)
        this.items = this.items.map((item) => (
          item.publicId === todoPublicId ? updated : item
        ))
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        if (toApiErrorStatus(error) === 403 || isApiForbidden(error)) {
          this.status = 'forbidden'
        }
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.updatingPublicId = ''
      }
    },
  },
})
