<script setup lang="ts">
import {
  ArrowDownAZ,
  ArrowUpAZ,
  Grid3X3,
  List,
  RefreshCw,
  Search,
  SlidersHorizontal,
  X,
} from 'lucide-vue-next'
import { ref, watch } from 'vue'

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
}>()

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
      <label class="sr-only" for="drive-search-query">Search Drive</label>
      <input
        id="drive-search-query"
        v-model="localQuery"
        autocomplete="off"
        placeholder="Search Drive"
        :disabled="disabled || busy"
      >
      <button class="secondary-button compact-button" type="submit" :disabled="disabled || busy">
        Search
      </button>
    </form>

    <div class="drive-filter-row" aria-label="Drive filters">
      <label class="drive-filter-chip">
        <SlidersHorizontal :size="15" stroke-width="1.9" aria-hidden="true" />
        <span>Type</span>
        <select :value="typeFilter" :disabled="disabled || busy" @change="emit('updateTypeFilter', ($event.target as HTMLSelectElement).value as DriveTypeFilter)">
          <option value="all">All</option>
          <option value="folder">Folders</option>
          <option value="document">Documents</option>
          <option value="image">Images</option>
          <option value="archive">Archives</option>
          <option value="other">Other</option>
        </select>
      </label>
      <label class="drive-filter-chip">
        <span>Owner</span>
        <select :value="ownerFilter" :disabled="disabled || busy" @change="emit('updateOwnerFilter', ($event.target as HTMLSelectElement).value as DriveOwnerFilter)">
          <option value="all">All</option>
          <option value="owned_by_me">Owned by me</option>
          <option value="shared_with_me">Shared with me</option>
        </select>
      </label>
      <label class="drive-filter-chip">
        <span>Modified</span>
        <select :value="modifiedFilter" :disabled="disabled || busy" @change="emit('updateModifiedFilter', ($event.target as HTMLSelectElement).value as DriveModifiedFilter)">
          <option value="any">Any time</option>
          <option value="today">Today</option>
          <option value="last_7_days">Last 7 days</option>
          <option value="last_30_days">Last 30 days</option>
        </select>
      </label>
      <label class="drive-filter-chip">
        <span>Source</span>
        <select :value="sourceFilter" :disabled="disabled || busy" @change="emit('updateSourceFilter', ($event.target as HTMLSelectElement).value as DriveSourceFilter)">
          <option value="all">All</option>
          <option value="uploaded">Uploaded</option>
          <option value="external">External</option>
        </select>
      </label>
      <button class="secondary-button compact-button" type="button" :disabled="disabled || busy" @click="clearAll">
        <X :size="15" stroke-width="1.9" aria-hidden="true" />
        Clear
      </button>
    </div>

    <div class="drive-view-controls" aria-label="Drive view controls">
      <button
        class="icon-button"
        :class="{ active: viewMode === 'grid' }"
        type="button"
        aria-label="Grid view"
        title="Grid view"
        :disabled="disabled || busy"
        @click="emit('updateViewMode', 'grid')"
      >
        <Grid3X3 :size="17" stroke-width="1.9" aria-hidden="true" />
      </button>
      <button
        class="icon-button"
        :class="{ active: viewMode === 'list' }"
        type="button"
        aria-label="List view"
        title="List view"
        :disabled="disabled || busy"
        @click="emit('updateViewMode', 'list')"
      >
        <List :size="17" stroke-width="1.9" aria-hidden="true" />
      </button>
      <button class="secondary-button compact-button" type="button" :disabled="disabled || busy" @click="emit('updateSort', sortKey)">
        <ArrowUpAZ v-if="sortDirection === 'asc'" :size="15" stroke-width="1.9" aria-hidden="true" />
        <ArrowDownAZ v-else :size="15" stroke-width="1.9" aria-hidden="true" />
        {{ sortKey.replace('_', ' ') }}
      </button>
      <select class="drive-sort-select" :value="sortKey" :disabled="disabled || busy" aria-label="Sort Drive items" @change="emit('updateSort', ($event.target as HTMLSelectElement).value as DriveSortKey)">
        <option value="updated_at">Updated</option>
        <option value="name">Name</option>
        <option value="size">Size</option>
        <option value="type">Type</option>
      </select>
      <button class="icon-button" type="button" aria-label="Refresh Drive" title="Refresh Drive" :disabled="disabled || busy" @click="emit('refresh')">
        <RefreshCw :size="17" stroke-width="1.9" aria-hidden="true" />
      </button>
    </div>
  </div>
</template>
