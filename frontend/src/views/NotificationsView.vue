<script setup lang="ts">
import { onMounted } from 'vue'

import { useNotificationStore } from '../stores/notifications'

const store = useNotificationStore()

onMounted(async () => {
  await store.load()
})

function formatDate(value?: string) {
  if (!value) {
    return 'Never'
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}
</script>

<template>
  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">Notifications</span>
        <h2>Notification Center</h2>
      </div>
      <button class="secondary-button" type="button" @click="store.load">
        Refresh
      </button>
    </div>

    <p v-if="store.status === 'loading'">
      Loading notifications...
    </p>
    <p v-if="store.errorMessage" class="error-message">
      {{ store.errorMessage }}
    </p>

    <div v-if="store.items.length > 0" class="list-stack">
      <article v-for="item in store.items" :key="item.publicId" class="list-item">
        <div>
          <strong>{{ item.subject || item.template }}</strong>
          <p>{{ item.body }}</p>
          <span class="cell-subtle">{{ formatDate(item.createdAt) }}</span>
        </div>
        <button
          class="secondary-button compact-button"
          type="button"
          :disabled="Boolean(item.readAt) || store.updatingPublicId === item.publicId"
          @click="store.markRead(item.publicId)"
        >
          {{ item.readAt ? 'Read' : 'Mark read' }}
        </button>
      </article>
    </div>

    <div v-else-if="store.status === 'empty'" class="empty-state">
      <p>通知はありません。</p>
    </div>
  </section>
</template>
