import { defineStore } from 'pinia'

import { toApiErrorMessage } from '../api/client'
import { deleteFileItem, fetchFiles, uploadFile } from '../api/files'
import type { FileObjectBody } from '../api/generated/types.gen'

type FileStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'error'

export const useFileStore = defineStore('files', {
  state: () => ({
    status: 'idle' as FileStatus,
    items: [] as FileObjectBody[],
    errorMessage: '',
    uploading: false,
    deletingPublicId: '',
  }),

  actions: {
    async load(attachedToType: string, attachedToId: string) {
      this.status = 'loading'
      this.errorMessage = ''
      try {
        this.items = await fetchFiles(attachedToType, attachedToId)
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.items = []
        this.status = 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async upload(form: FormData) {
      this.uploading = true
      this.errorMessage = ''
      try {
        const created = await uploadFile(form)
        this.items = [created, ...this.items]
        this.status = 'ready'
        return created
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.uploading = false
      }
    },

    async remove(publicId: string) {
      this.deletingPublicId = publicId
      this.errorMessage = ''
      try {
        await deleteFileItem(publicId)
        this.items = this.items.filter((item) => item.publicId !== publicId)
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.deletingPublicId = ''
      }
    },
  },
})
