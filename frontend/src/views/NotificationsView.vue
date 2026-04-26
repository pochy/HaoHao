<script setup lang="ts">
import { onMounted } from 'vue'

import DataCard from '../components/DataCard.vue'
import EmptyState from '../components/EmptyState.vue'
import MetricTile from '../components/MetricTile.vue'
import PageHeader from '../components/PageHeader.vue'
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
  <section class="stack">
    <PageHeader
      eyebrow="Notifications"
      title="Notification Center"
      description="Tenant invitation や async job の通知を確認します。"
    >
      <template #actions>
      <button class="secondary-button" type="button" @click="store.load">
        Refresh
      </button>
      </template>
    </PageHeader>

    <div class="metric-grid">
      <MetricTile label="Notifications" :value="store.items.length" hint="Current page" />
      <MetricTile label="Unread" :value="store.items.filter((item) => !item.readAt).length" hint="Needs action" />
      <MetricTile label="Read" :value="store.items.filter((item) => item.readAt).length" hint="Completed" />
      <MetricTile label="Status" :value="store.status" hint="List loading state" />
    </div>

    <p v-if="store.status === 'loading'">
      Loading notifications...
    </p>
    <p v-if="store.errorMessage" class="error-message">
      {{ store.errorMessage }}
    </p>

    <DataCard v-if="store.items.length > 0" title="Notifications">
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
    </DataCard>

    <EmptyState v-else-if="store.status === 'empty'" title="No notifications" message="通知はありません。" />
  </section>
</template>
