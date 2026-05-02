<script setup lang="ts">
import { computed } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { ArrowRight, GitBranch } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import type { DatasetLineageBody, DatasetLineageNodeBody } from '../api/generated/types.gen'

const props = defineProps<{
  lineage: DatasetLineageBody | null
}>()

const { t } = useI18n()

const nodes = computed(() => props.lineage?.nodes ?? [])
const edges = computed(() => props.lineage?.edges ?? [])
const nodeMap = computed(() => new Map(nodes.value.map((node) => [node.id, node])))

function nodeFor(id: string): DatasetLineageNodeBody | null {
  return nodeMap.value.get(id) ?? null
}

function nodeRoute(node: DatasetLineageNodeBody | null): RouteLocationRaw | null {
  if (!node?.publicId || node.resourceType !== 'dataset') {
    return null
  }
  return { name: 'dataset-detail', params: { datasetPublicId: node.publicId } }
}

function resourceLabel(resourceType?: string) {
  return t(`datasets.lineageResource.${resourceType || 'unknown'}`)
}

function relationLabel(relationType?: string) {
  return t(`datasets.lineageRelation.${relationType || 'unknown'}`)
}
</script>

<template>
  <div class="lineage-compact-graph">
    <div v-if="edges.length > 0" class="lineage-edge-list">
      <div v-for="edge in edges" :key="edge.id" class="lineage-edge-row">
        <RouterLink
          v-if="nodeRoute(nodeFor(edge.sourceNodeId))"
          class="lineage-node lineage-node-link"
          :to="nodeRoute(nodeFor(edge.sourceNodeId))!"
        >
          <GitBranch :size="15" aria-hidden="true" />
          <span>
            <strong>{{ nodeFor(edge.sourceNodeId)?.displayName || edge.sourceNodeId }}</strong>
            <small>{{ resourceLabel(nodeFor(edge.sourceNodeId)?.resourceType) }}</small>
          </span>
        </RouterLink>
        <span v-else class="lineage-node">
          <GitBranch :size="15" aria-hidden="true" />
          <span>
            <strong>{{ nodeFor(edge.sourceNodeId)?.displayName || edge.sourceNodeId }}</strong>
            <small>{{ resourceLabel(nodeFor(edge.sourceNodeId)?.resourceType) }}</small>
          </span>
        </span>

        <span class="lineage-edge-mid">
          <ArrowRight :size="15" aria-hidden="true" />
          <small>{{ relationLabel(edge.relationType) }}</small>
        </span>

        <RouterLink
          v-if="nodeRoute(nodeFor(edge.targetNodeId))"
          class="lineage-node lineage-node-link"
          :to="nodeRoute(nodeFor(edge.targetNodeId))!"
        >
          <GitBranch :size="15" aria-hidden="true" />
          <span>
            <strong>{{ nodeFor(edge.targetNodeId)?.displayName || edge.targetNodeId }}</strong>
            <small>{{ resourceLabel(nodeFor(edge.targetNodeId)?.resourceType) }}</small>
          </span>
        </RouterLink>
        <span v-else class="lineage-node">
          <GitBranch :size="15" aria-hidden="true" />
          <span>
            <strong>{{ nodeFor(edge.targetNodeId)?.displayName || edge.targetNodeId }}</strong>
            <small>{{ resourceLabel(nodeFor(edge.targetNodeId)?.resourceType) }}</small>
          </span>
        </span>
      </div>
    </div>

    <div v-else class="empty-state compact-empty-state">
      <p>{{ t('datasets.noLineageEdges') }}</p>
    </div>
  </div>
</template>
