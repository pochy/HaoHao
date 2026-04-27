<script setup lang="ts">
import {
  ArrowDownAZ,
  ArrowUpAZ,
  Download,
  Grid3X3,
  List,
  RefreshCw,
  Search,
  SlidersHorizontal,
  X,
} from 'lucide-vue-next'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type {
  DriveModifiedFilter,
  DriveOwnerFilter,
  DriveSortKey,
  DriveSourceFilter,
  DriveTypeFilter,
  DriveViewMode,
} from '../stores/drive'

const props = defineProps<{
  busy: boolean
  disabled: boolean
  query: string
  viewMode: DriveViewMode
  typeFilter: DriveTypeFilter
  ownerFilter: DriveOwnerFilter
  modifiedFilter: DriveModifiedFilter
  sourceFilter: DriveSourceFilter
  sortKey: DriveSortKey
  sortDirection: 'asc' | 'desc'
  selectionCount: number
}>()

const { t } = useI18n()

const emit = defineEmits<{
  search: [query: string]
  clearFilters: []
  updateViewMode: [mode: DriveViewMode]
  updateTypeFilter: [filter: DriveTypeFilter]
  updateOwnerFilter: [filter: DriveOwnerFilter]
  updateModifiedFilter: [filter: DriveModifiedFilter]
  updateSourceFilter: [filter: DriveSourceFilter]
  updateSort: [key: DriveSortKey]
  refresh: []
  downloadArchive: []
  clearSelection: []
}>()

const localQuery = ref(props.query)

watch(
  () => props.query,
  (query) => {
    localQuery.value = query
  },
)

function submitSearch() {
  emit('search', localQuery.value.trim())
}

function clearAll() {
  localQuery.value = ''
  emit('clearFilters')
}
</script>

<template>
  <div class="drive-command-bar">
    <form class="drive-search-box" role="search" @submit.prevent="submitSearch">
      <Search :size="18" stroke-width="1.9" aria-hidden="true" />
      <label class="sr-only" for="drive-search-query">{{ t('drive.searchDrive') }}</label>
      <input
        id="drive-search-query"
        v-model="localQuery"
        autocomplete="off"
        :placeholder="t('drive.searchDrive')"
        :disabled="disabled || busy"
      >
      <button class="secondary-button compact-button" type="submit" :disabled="disabled || busy">
        {{ t('common.search') }}
      </button>
    </form>

    <div class="drive-filter-row" :aria-label="t('drive.filters')">
      <label class="drive-filter-chip">
        <SlidersHorizontal :size="15" stroke-width="1.9" aria-hidden="true" />
        <span>{{ t('drive.typeLabel') }}</span>
        <select :value="typeFilter" :disabled="disabled || busy" @change="emit('updateTypeFilter', ($event.target as HTMLSelectElement).value as DriveTypeFilter)">
          <option value="all">{{ t('drive.type.all') }}</option>
          <option value="file">{{ t('drive.type.file') }}</option>
          <option value="folder">{{ t('drive.type.folder') }}</option>
        </select>
      </label>
      <label class="drive-filter-chip">
        <span>{{ t('drive.owner') }}</span>
        <select :value="ownerFilter" :disabled="disabled || busy" @change="emit('updateOwnerFilter', ($event.target as HTMLSelectElement).value as DriveOwnerFilter)">
          <option value="all">{{ t('drive.type.all') }}</option>
          <option value="me">{{ t('drive.ownedByMe') }}</option>
          <option value="shared_with_me">{{ t('drive.sharedWithMe') }}</option>
        </select>
      </label>
      <label class="drive-filter-chip">
        <span>{{ t('drive.modified') }}</span>
        <select :value="modifiedFilter" :disabled="disabled || busy" @change="emit('updateModifiedFilter', ($event.target as HTMLSelectElement).value as DriveModifiedFilter)">
          <option value="any">{{ t('drive.anyTime') }}</option>
          <option value="today">{{ t('drive.today') }}</option>
          <option value="last_7_days">{{ t('drive.last7Days') }}</option>
          <option value="last_30_days">{{ t('drive.last30Days') }}</option>
        </select>
      </label>
      <label class="drive-filter-chip">
        <span>{{ t('signals.source') }}</span>
        <select :value="sourceFilter" :disabled="disabled || busy" @change="emit('updateSourceFilter', ($event.target as HTMLSelectElement).value as DriveSourceFilter)">
          <option value="all">{{ t('drive.type.all') }}</option>
          <option value="upload">{{ t('drive.source.upload') }}</option>
          <option value="generated">{{ t('drive.source.generated') }}</option>
          <option value="sync">{{ t('drive.source.sync') }}</option>
          <option value="external">{{ t('drive.source.external') }}</option>
        </select>
      </label>
      <button class="secondary-button compact-button" type="button" :disabled="disabled || busy" @click="clearAll">
        <X :size="15" stroke-width="1.9" aria-hidden="true" />
        {{ t('common.clear') }}
      </button>
    </div>

    <div class="drive-view-controls" :aria-label="t('drive.viewControls')">
      <button
        class="icon-button"
        :class="{ active: viewMode === 'grid' }"
        type="button"
        :aria-label="t('drive.gridView')"
        :title="t('drive.gridView')"
        :disabled="disabled || busy"
        @click="emit('updateViewMode', 'grid')"
      >
        <Grid3X3 :size="17" stroke-width="1.9" aria-hidden="true" />
      </button>
      <button
        class="icon-button"
        :class="{ active: viewMode === 'list' }"
        type="button"
        :aria-label="t('drive.listView')"
        :title="t('drive.listView')"
        :disabled="disabled || busy"
        @click="emit('updateViewMode', 'list')"
      >
        <List :size="17" stroke-width="1.9" aria-hidden="true" />
      </button>
      <button class="secondary-button compact-button" type="button" :disabled="disabled || busy" @click="emit('updateSort', sortKey)">
        <ArrowUpAZ v-if="sortDirection === 'asc'" :size="15" stroke-width="1.9" aria-hidden="true" />
        <ArrowDownAZ v-else :size="15" stroke-width="1.9" aria-hidden="true" />
        {{ t(`drive.sort.${sortKey}`) }}
      </button>
      <select class="drive-sort-select" :value="sortKey" :disabled="disabled || busy" :aria-label="t('drive.sortDriveItems')" @change="emit('updateSort', ($event.target as HTMLSelectElement).value as DriveSortKey)">
        <option value="updated_at">{{ t('drive.sort.updated_at') }}</option>
        <option value="name">{{ t('drive.sort.name') }}</option>
        <option value="size">{{ t('drive.sort.size') }}</option>
      </select>
      <button class="icon-button" type="button" :aria-label="t('drive.refreshDrive')" :title="t('drive.refreshDrive')" :disabled="disabled || busy" @click="emit('refresh')">
        <RefreshCw :size="17" stroke-width="1.9" aria-hidden="true" />
      </button>
    </div>

    <div v-if="selectionCount > 0" class="drive-selection-bar" aria-live="polite">
      <span>{{ t('drive.selected', { count: selectionCount }) }}</span>
      <button class="primary-button compact-button" type="button" :disabled="disabled || busy" @click="emit('downloadArchive')">
        <Download :size="15" stroke-width="1.9" aria-hidden="true" />
        {{ t('drive.downloadZip') }}
      </button>
      <button class="secondary-button compact-button" type="button" :disabled="disabled || busy" @click="emit('clearSelection')">
        {{ t('drive.clearSelection') }}
      </button>
    </div>
  </div>
</template>
