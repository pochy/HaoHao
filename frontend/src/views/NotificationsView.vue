<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

import DataCard from '../components/DataCard.vue'
import EmptyState from '../components/EmptyState.vue'
import MetricTile from '../components/MetricTile.vue'
import PageHeader from '../components/PageHeader.vue'
import { useNotificationStore } from '../stores/notifications'

const store = useNotificationStore()
const { d, t } = useI18n()

onMounted(async () => {
  await store.load()
})

function formatDate(value?: string) {
  if (!value) {
    return t('common.never')
  }
  return d(new Date(value), 'long')
}
</script>

<template>
  <section class="stack">
    <PageHeader
      :eyebrow="t('notifications.eyebrow')"
      :title="t('notifications.title')"
      :description="t('notifications.description')"
    >
      <template #actions>
      <button class="secondary-button" type="button" @click="store.load">
        {{ t('common.refresh') }}
      </button>
      </template>
    </PageHeader>

    <div class="metric-grid">
      <MetricTile :label="t('nav.items.notifications')" :value="store.items.length" :hint="t('common.currentPage')" />
      <MetricTile :label="t('notifications.unread')" :value="store.items.filter((item) => !item.readAt).length" :hint="t('notifications.unreadHint')" />
      <MetricTile :label="t('common.read')" :value="store.items.filter((item) => item.readAt).length" :hint="t('notifications.readHint')" />
      <MetricTile :label="t('common.status')" :value="store.status" :hint="t('notifications.statusHint')" />
    </div>

    <p v-if="store.status === 'loading'">
      {{ t('notifications.loading') }}
    </p>
    <p v-if="store.errorMessage" class="error-message">
      {{ store.errorMessage }}
    </p>

    <DataCard v-if="store.items.length > 0" :title="t('nav.items.notifications')">
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
          {{ item.readAt ? t('common.read') : t('notifications.markRead') }}
        </button>
      </article>
    </DataCard>

    <EmptyState v-else-if="store.status === 'empty'" :title="t('notifications.emptyTitle')" :message="t('notifications.emptyMessage')" />
  </section>
</template>
