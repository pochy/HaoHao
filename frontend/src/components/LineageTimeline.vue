<script setup lang="ts">
import { computed } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { Clock3 } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import type { DatasetLineageBody, DatasetLineageNodeBody, DatasetLineageTimelineItemBody } from '../api/generated/types.gen'

const props = defineProps<{
  lineage: DatasetLineageBody | null
}>()

const { d, t } = useI18n()

const nodeMap = computed(() => new Map((props.lineage?.nodes ?? []).map((node) => [node.id, node])))
const items = computed(() => [...(props.lineage?.timeline ?? [])].sort(compareTimelineDesc))

function compareTimelineDesc(a: DatasetLineageTimelineItemBody, b: DatasetLineageTimelineItemBody) {
  return new Date(b.occurredAt || '').getTime() - new Date(a.occurredAt || '').getTime()
}

function nodeFor(item: DatasetLineageTimelineItemBody): DatasetLineageNodeBody | null {
  return nodeMap.value.get(item.nodeId) ?? null
}

function nodeRoute(node: DatasetLineageNodeBody | null): RouteLocationRaw | null {
  if (!node?.publicId || node.resourceType !== 'dataset') {
    return null
  }
  return { name: 'dataset-detail', params: { datasetPublicId: node.publicId } }
}

function formatDate(value?: string) {
  return value ? d(new Date(value), 'long') : '-'
}

function statusClass(status?: string) {
  if (['active', 'ready', 'completed', 'enabled'].includes(status || '')) {
    return 'success'
  }
  if (['dropped', 'failed', 'deleted', 'disabled'].includes(status || '')) {
    return 'danger'
  }
  return 'warning'
}

function relationLabel(relationType?: string) {
  return t(`datasets.lineageRelation.${relationType || 'unknown'}`)
}

function resourceLabel(resourceType?: string) {
  return t(`datasets.lineageResource.${resourceType || 'unknown'}`)
}
</script>

<template>
  <div class="lineage-timeline">
    <div v-if="items.length > 0" class="lineage-timeline-list">
      <div v-for="item in items" :key="item.id" class="lineage-timeline-row">
        <Clock3 :size="16" aria-hidden="true" />
        <div class="lineage-timeline-copy">
          <div class="lineage-timeline-title">
            <RouterLink v-if="nodeRoute(nodeFor(item))" :to="nodeRoute(nodeFor(item))!" class="lineage-inline-link">
              {{ nodeFor(item)?.displayName || item.publicId || item.nodeId }}
            </RouterLink>
            <strong v-else>{{ nodeFor(item)?.displayName || item.publicId || item.nodeId }}</strong>
            <span class="status-pill" :class="statusClass(item.status)">{{ item.status || resourceLabel(item.resourceType) }}</span>
          </div>
          <small>{{ relationLabel(item.relationType) }} · {{ formatDate(item.occurredAt) }}</small>
        </div>
      </div>
    </div>

    <div v-else class="empty-state compact-empty-state">
      <p>{{ t('datasets.noLineageTimeline') }}</p>
    </div>
  </div>
</template>
