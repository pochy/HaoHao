<script setup lang="ts">
import { CheckCheck, RefreshCw, Search, SlidersHorizontal, X } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import ConfirmActionDialog from '../components/ConfirmActionDialog.vue'
import DataCard from '../components/DataCard.vue'
import EmptyState from '../components/EmptyState.vue'
import MetricTile from '../components/MetricTile.vue'
import PageHeader from '../components/PageHeader.vue'
import StatusBadge from '../components/StatusBadge.vue'
import {
  useNotificationStore,
  type NotificationChannelFilter,
  type NotificationCreatedAfterFilter,
} from '../stores/notifications'
import type { NotificationReadState } from '../api/notifications'

const store = useNotificationStore()
const { d, t } = useI18n()
const readStateOptions = ['all', 'unread', 'read'] as const
const channelOptions = ['', 'in_app', 'email'] as const
const createdAfterOptions = ['', 'today', 'last_7_days', 'last_30_days'] as const
const confirmReadAllOpen = ref(false)
const busy = computed(() => store.status === 'loading' || store.bulkUpdating)
const selectedUnreadCount = computed(() => store.selectedUnreadPublicIds.length)
const allVisibleSelected = computed(() => (
  store.items.length > 0 &&
  store.items.every((item) => store.selectedPublicIds.includes(item.publicId))
))
const canMarkFilteredUnread = computed(() => store.summary.unreadCount > 0 && store.filters.readState !== 'read')

onMounted(async () => {
  await store.load()
})

function formatDate(value?: string) {
  if (!value) {
    return t('common.never')
  }
  return d(new Date(value), 'long')
}

function readStateLabel(value: NotificationReadState) {
  return t(`notifications.readState.${value}`)
}

function channelLabel(value: NotificationChannelFilter) {
  return value ? t(`notifications.channelOptions.${value}`) : t('common.any')
}

function notificationChannelLabel(value: string) {
  return value === 'in_app' || value === 'email'
    ? channelLabel(value)
    : value
}

function createdAfterLabel(value: NotificationCreatedAfterFilter) {
  return value ? t(`notifications.createdAfterOptions.${value}`) : t('notifications.createdAfterOptions.any')
}

async function applySearch() {
  await store.load()
}

async function clearFilters() {
  store.clearFilters()
  await store.load()
}

async function loadMore() {
  if (!store.nextCursor) {
    return
  }
  await store.load({ cursor: store.nextCursor })
}

function toggleVisibleSelection() {
  if (allVisibleSelected.value) {
    store.clearSelection()
    return
  }
  store.selectVisible()
}

async function markSelectedRead() {
  await store.markSelectedRead()
}

function requestMarkAllRead() {
  confirmReadAllOpen.value = true
}

function cancelMarkAllRead() {
  confirmReadAllOpen.value = false
}

async function confirmMarkAllRead() {
  confirmReadAllOpen.value = false
  await store.markAllRead()
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
        <button class="secondary-button" type="button" :disabled="busy" @click="store.load()">
          <RefreshCw :size="16" stroke-width="1.8" aria-hidden="true" />
          {{ busy ? t('common.refreshing') : t('common.refresh') }}
        </button>
      </template>
    </PageHeader>

    <div class="metric-grid">
      <MetricTile :label="t('notifications.total')" :value="store.summary.totalCount" :hint="t('notifications.totalHint')" />
      <MetricTile :label="t('notifications.unread')" :value="store.summary.unreadCount" :hint="t('notifications.unreadHint')" />
      <MetricTile :label="t('common.read')" :value="store.summary.readCount" :hint="t('notifications.readHint')" />
      <MetricTile :label="t('notifications.filtered')" :value="store.summary.filteredCount" :hint="t('notifications.filteredHint')" />
    </div>

    <p v-if="store.errorMessage" class="error-message">
      {{ store.errorMessage }}
    </p>

    <DataCard :title="t('notifications.searchCardTitle')" :subtitle="t('notifications.searchCardSubtitle')">
      <form class="notification-command-bar" role="search" @submit.prevent="applySearch">
        <div class="notification-search-box">
          <Search :size="18" stroke-width="1.9" aria-hidden="true" />
          <label class="sr-only" for="notification-search-query">{{ t('notifications.searchPlaceholder') }}</label>
          <input
            id="notification-search-query"
            v-model="store.query"
            autocomplete="off"
            :placeholder="t('notifications.searchPlaceholder')"
            :disabled="busy"
          >
          <button class="secondary-button compact-button" type="submit" :disabled="busy">
            {{ t('common.search') }}
          </button>
        </div>

        <div class="notification-filter-row" :aria-label="t('notifications.filters')">
          <label class="notification-filter-chip">
            <SlidersHorizontal :size="15" stroke-width="1.9" aria-hidden="true" />
            <span>{{ t('notifications.readStateLabel') }}</span>
            <select v-model="store.filters.readState" :disabled="busy">
              <option v-for="item in readStateOptions" :key="item" :value="item">
                {{ readStateLabel(item) }}
              </option>
            </select>
          </label>

          <label class="notification-filter-chip">
            <span>{{ t('notifications.channel') }}</span>
            <select v-model="store.filters.channel" :disabled="busy">
              <option v-for="item in channelOptions" :key="item || 'any'" :value="item">
                {{ channelLabel(item) }}
              </option>
            </select>
          </label>

          <label class="notification-filter-chip">
            <span>{{ t('notifications.createdAfter') }}</span>
            <select v-model="store.filters.createdAfter" :disabled="busy">
              <option v-for="item in createdAfterOptions" :key="item || 'any'" :value="item">
                {{ createdAfterLabel(item) }}
              </option>
            </select>
          </label>

          <button class="secondary-button compact-button" type="button" :disabled="busy" @click="clearFilters">
            <X :size="15" stroke-width="1.9" aria-hidden="true" />
            {{ t('common.clear') }}
          </button>
        </div>
      </form>
    </DataCard>

    <p v-if="store.status === 'loading' && store.items.length === 0">
      {{ t('notifications.loading') }}
    </p>

    <DataCard v-if="store.items.length > 0" :title="t('nav.items.notifications')">
      <div class="notification-selection-bar" aria-live="polite">
        <label class="checkbox-field">
          <input
            type="checkbox"
            :checked="allVisibleSelected"
            :disabled="busy"
            @change="toggleVisibleSelection"
          >
          <span>{{ t('notifications.selectVisible') }}</span>
        </label>
        <span>{{ t('notifications.selected', { count: store.selectedCount }) }}</span>
        <button
          class="primary-button compact-button"
          type="button"
          :disabled="busy || selectedUnreadCount === 0"
          @click="markSelectedRead"
        >
          <CheckCheck :size="15" stroke-width="1.9" aria-hidden="true" />
          {{ t('notifications.markSelectedRead', { count: selectedUnreadCount }) }}
        </button>
        <button class="secondary-button compact-button" type="button" :disabled="busy || store.selectedCount === 0" @click="store.clearSelection">
          {{ t('notifications.clearSelection') }}
        </button>
        <button
          class="secondary-button compact-button"
          type="button"
          :disabled="busy || !canMarkFilteredUnread"
          @click="requestMarkAllRead"
        >
          {{ t('notifications.markAllReadFiltered') }}
        </button>
      </div>

      <article
        v-for="item in store.items"
        :key="item.publicId"
        class="notification-list-item"
        :class="{ unread: !item.readAt }"
      >
        <label class="notification-checkbox">
          <input
            type="checkbox"
            :checked="store.selectedPublicIds.includes(item.publicId)"
            :aria-label="t('notifications.selectNotification', { subject: item.subject || item.template })"
            :disabled="busy"
            @change="store.toggleSelect(item.publicId)"
          >
        </label>

        <div class="notification-item-main">
          <div class="notification-title-row">
            <strong>{{ item.subject || item.template }}</strong>
            <StatusBadge :tone="item.readAt ? 'success' : 'warning'">
              {{ item.readAt ? t('common.read') : t('notifications.unread') }}
            </StatusBadge>
          </div>
          <p>{{ item.body || t('notifications.noBody') }}</p>
          <div class="notification-meta-row">
            <span>{{ notificationChannelLabel(item.channel) }}</span>
            <span>{{ item.template }}</span>
            <span>{{ formatDate(item.createdAt) }}</span>
          </div>
        </div>

        <button
          class="secondary-button compact-button"
          type="button"
          :disabled="Boolean(item.readAt) || store.updatingPublicId === item.publicId || store.bulkUpdating"
          @click="store.markRead(item.publicId)"
        >
          {{ item.readAt ? t('common.read') : t('notifications.markRead') }}
        </button>
      </article>

      <div v-if="store.nextCursor" class="action-row">
        <button class="secondary-button" type="button" :disabled="store.loadingMore || busy" @click="loadMore">
          {{ store.loadingMore ? t('notifications.loadingMore') : t('notifications.loadMore') }}
        </button>
      </div>
    </DataCard>

    <EmptyState
      v-else-if="store.status === 'empty'"
      :title="store.hasActiveFilters ? t('notifications.emptyFilteredTitle') : t('notifications.emptyTitle')"
      :message="store.hasActiveFilters ? t('notifications.emptyFilteredMessage') : t('notifications.emptyMessage')"
    >
      <template v-if="store.hasActiveFilters" #actions>
        <button class="secondary-button compact-button" type="button" @click="clearFilters">
          {{ t('notifications.clearFilters') }}
        </button>
      </template>
    </EmptyState>

    <ConfirmActionDialog
      :open="confirmReadAllOpen"
      :title="t('notifications.markAllReadTitle')"
      :message="t('notifications.markAllReadMessage')"
      :confirm-label="t('notifications.markAllReadConfirm')"
      @cancel="cancelMarkAllRead"
      @confirm="confirmMarkAllRead"
    />
  </section>
</template>
