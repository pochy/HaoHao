<script setup lang="ts">
import { X } from 'lucide-vue-next'

import type { DriveUploadQueueItem } from '../stores/drive'

defineProps<{
  items: DriveUploadQueueItem[]
  busy: boolean
}>()

const emit = defineEmits<{
  retry: [id: string]
  cancel: [id: string]
  clearCompleted: []
}>()
</script>

<template>
  <aside v-if="items.length > 0" class="drive-upload-queue" aria-label="Upload queue">
    <header>
      <strong>Uploads</strong>
      <button class="icon-button" type="button" aria-label="Clear completed uploads" title="Clear completed uploads" @click="emit('clearCompleted')">
        <X :size="16" stroke-width="1.9" aria-hidden="true" />
      </button>
    </header>
    <div class="drive-upload-queue-list">
      <article v-for="item in items" :key="item.id" class="drive-upload-row">
        <div>
          <strong>{{ item.file.name }}</strong>
          <span>{{ item.status }} · {{ item.progress }}%</span>
          <div v-if="item.errorMessage" class="drive-upload-error" role="alert">
            <strong>{{ item.errorTitle || 'アップロードできませんでした' }}</strong>
            <p>{{ item.errorMessage }}</p>
            <p v-if="item.errorAction">{{ item.errorAction }}</p>
            <code v-if="item.errorRequestId">Request ID: {{ item.errorRequestId }}</code>
          </div>
        </div>
        <div class="drive-upload-progress" :aria-label="`${item.progress}% uploaded`">
          <span :style="{ width: `${item.progress}%` }" />
        </div>
        <div class="drive-row-actions">
          <button v-if="item.status === 'error' && item.retryable !== false" class="secondary-button compact-button" type="button" :disabled="busy" @click="emit('retry', item.id)">
            Retry
          </button>
          <button v-if="item.status !== 'uploading'" class="secondary-button compact-button" type="button" :disabled="busy && item.status === 'queued'" @click="emit('cancel', item.id)">
            Remove
          </button>
        </div>
      </article>
    </div>
  </aside>
</template>
