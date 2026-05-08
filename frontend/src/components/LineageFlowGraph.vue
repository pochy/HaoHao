<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { GitBranch, Link2, Lock, Plus, Save, Trash2 } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { VueFlow, type Connection } from '@vue-flow/core'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'

import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'
import '@vue-flow/minimap/dist/style.css'

import type { DatasetLineageBody, DatasetLineageEdgeBody, DatasetLineageGraphSaveBodyWritable, DatasetLineageNodeBody } from '../api/generated/types.gen'
import { randomID } from '../utils/id'

const props = withDefaults(defineProps<{
  lineage: DatasetLineageBody | null
  editable?: boolean
  saving?: boolean
  draftSourceKind?: 'parser' | 'manual'
  hasDraft?: boolean
  canPublish?: boolean
}>(), {
  editable: false,
  saving: false,
  draftSourceKind: 'manual',
  hasDraft: false,
  canPublish: false,
})

const emit = defineEmits<{
  saveDraft: [body: DatasetLineageGraphSaveBodyWritable]
  publishDraft: []
  rejectDraft: []
}>()

const { t } = useI18n()
const router = useRouter()
const flowNodes = ref<FlowNode[]>([])
const flowEdges = ref<FlowEdge[]>([])
const selectedEdgeId = ref('')
const selectedEdgeLabel = ref('')
const selectedEdgeDescription = ref('')
const selectedEdgeRelation = ref('manual_dependency')

const selectedEdge = computed(() => flowEdges.value.find((edge) => edge.id === selectedEdgeId.value) ?? null)
const selectedEdgeLocked = computed(() => selectedEdge.value?.data?.sourceKind === 'metadata')
const nodeMap = computed(() => new Map((props.lineage?.nodes ?? []).map((node) => [node.id, node])))

type FlowNode = {
  id: string
  type?: string
  position: { x: number, y: number }
  data?: Record<string, unknown>
  draggable?: boolean
  selectable?: boolean
  class?: string
}

type FlowEdge = {
  id: string
  source: string
  target: string
  label?: string
  animated?: boolean
  selectable?: boolean
  data?: Record<string, unknown>
  class?: string
}

watch(
  () => props.lineage,
  () => resetGraph(),
  { immediate: true, deep: true },
)

function resetGraph() {
  const lineageNodes = props.lineage?.nodes ?? []
  const lineageEdges = props.lineage?.edges ?? []
  const positions = layoutPositions(lineageNodes, lineageEdges)
  flowNodes.value = lineageNodes.map((node) => ({
    id: node.id,
    type: 'default',
    position: node.position ?? positions.get(node.id) ?? { x: 0, y: 0 },
    data: {
      label: `${node.displayName}\n${resourceLabel(node.resourceType)}${node.columnName ? ` · ${node.columnName}` : ''}`,
      lineage: node,
      sourceKind: node.sourceKind ?? 'metadata',
    },
    draggable: props.editable,
    selectable: true,
    class: `lineage-flow-node lineage-flow-node-${node.nodeKind || 'resource'} lineage-flow-source-${node.sourceKind || 'metadata'}`,
  }))
  flowEdges.value = lineageEdges.map((edge) => flowEdge(edge))
  selectedEdgeId.value = ''
}

function flowEdge(edge: DatasetLineageEdgeBody): FlowEdge {
  const locked = edge.sourceKind === 'metadata' || edge.editable === false
  return {
    id: edge.id,
    source: edge.sourceNodeId,
    target: edge.targetNodeId,
    label: edge.label || relationLabel(edge.relationType),
    animated: !locked,
    selectable: true,
    data: {
      sourceKind: edge.sourceKind ?? 'metadata',
      relationType: edge.relationType,
      description: edge.description ?? '',
      expression: edge.expression ?? '',
      confidence: edge.confidence,
    },
    class: `lineage-flow-edge lineage-flow-source-${edge.sourceKind || 'metadata'}${locked ? ' locked' : ''}`,
  }
}

function layoutPositions(nodes: DatasetLineageNodeBody[], edges: DatasetLineageEdgeBody[]) {
  const indegree = new Map(nodes.map((node) => [node.id, 0]))
  const outgoing = new Map<string, string[]>()
  for (const edge of edges) {
    indegree.set(edge.targetNodeId, (indegree.get(edge.targetNodeId) ?? 0) + 1)
    outgoing.set(edge.sourceNodeId, [...(outgoing.get(edge.sourceNodeId) ?? []), edge.targetNodeId])
  }
  const depth = new Map<string, number>()
  const queue = nodes.filter((node) => (indegree.get(node.id) ?? 0) === 0).map((node) => node.id)
  if (queue.length === 0 && props.lineage?.root.id) {
    queue.push(props.lineage.root.id)
  }
  for (const id of queue) {
    depth.set(id, 0)
  }
  for (let cursor = 0; cursor < queue.length; cursor += 1) {
    const id = queue[cursor]
    const nextDepth = (depth.get(id) ?? 0) + 1
    for (const target of outgoing.get(id) ?? []) {
      if (!depth.has(target) || nextDepth < (depth.get(target) ?? 0)) {
        depth.set(target, nextDepth)
        queue.push(target)
      }
    }
  }
  const byDepth = new Map<number, DatasetLineageNodeBody[]>()
  for (const node of nodes) {
    const itemDepth = depth.get(node.id) ?? 0
    byDepth.set(itemDepth, [...(byDepth.get(itemDepth) ?? []), node])
  }
  const positions = new Map<string, { x: number, y: number }>()
  for (const [itemDepth, items] of byDepth.entries()) {
    items
      .sort((a, b) => a.displayName.localeCompare(b.displayName))
      .forEach((node, index) => positions.set(node.id, { x: itemDepth * 280, y: index * 110 }))
  }
  return positions
}

function addCustomNode() {
  const id = `custom:${randomID()}`
  flowNodes.value = [
    ...flowNodes.value,
    {
      id,
      type: 'default',
      position: { x: 80, y: 80 + flowNodes.value.length * 24 },
      data: {
        label: t('datasets.lineageCustomNode'),
        lineage: {
          id,
          displayName: t('datasets.lineageCustomNode'),
          resourceType: 'custom',
          nodeKind: 'custom',
          sourceKind: 'manual',
          editable: true,
        },
        sourceKind: 'manual',
      },
      draggable: true,
      selectable: true,
      class: 'lineage-flow-node lineage-flow-node-custom lineage-flow-source-manual',
    },
  ]
}

function onConnect(connection: Connection) {
  if (!props.editable || !connection.source || !connection.target || connection.source === connection.target) {
    return
  }
  const id = `edge:${connection.source}:${connection.target}:${randomID()}`
  flowEdges.value = [
    ...flowEdges.value,
    {
      id,
      source: connection.source,
      target: connection.target,
      label: relationLabel('manual_dependency'),
      animated: true,
      selectable: true,
      data: {
        sourceKind: 'manual',
        relationType: 'manual_dependency',
        description: '',
        expression: '',
        confidence: 'manual',
      },
      class: 'lineage-flow-edge lineage-flow-source-manual',
    },
  ]
}

function onNodeClick(event: { node?: FlowNode } | any) {
  const lineage = event.node?.data?.lineage as DatasetLineageNodeBody | undefined
  if (!lineage?.publicId || lineage.resourceType !== 'dataset') {
    return
  }
  router.push({ name: 'dataset-detail', params: { datasetPublicId: lineage.publicId } })
}

function onEdgeClick(event: { edge?: FlowEdge } | any) {
  const edge = event.edge
  if (!edge) {
    return
  }
  selectedEdgeId.value = edge.id
  selectedEdgeLabel.value = String(edge.label ?? '')
  selectedEdgeDescription.value = String(edge.data?.description ?? '')
  selectedEdgeRelation.value = String(edge.data?.relationType ?? 'manual_dependency')
}

function updateSelectedEdge() {
  if (!selectedEdge.value || selectedEdgeLocked.value) {
    return
  }
  flowEdges.value = flowEdges.value.map((edge) => {
    if (edge.id !== selectedEdgeId.value) {
      return edge
    }
    return {
      ...edge,
      label: selectedEdgeLabel.value.trim() || relationLabel(selectedEdgeRelation.value),
      data: {
        ...edge.data,
        relationType: selectedEdgeRelation.value.trim() || 'manual_dependency',
        description: selectedEdgeDescription.value.trim(),
      },
    }
  })
}

function removeSelectedEdge() {
  if (!selectedEdge.value || selectedEdgeLocked.value) {
    return
  }
  flowEdges.value = flowEdges.value.filter((edge) => edge.id !== selectedEdgeId.value)
  selectedEdgeId.value = ''
}

function saveDraft() {
  emit('saveDraft', {
    nodes: flowNodes.value.map((node) => nodeWriteBody(node)),
    edges: flowEdges.value
      .filter((edge) => edge.data?.sourceKind !== 'metadata')
      .map((edge) => edgeWriteBody(edge)),
  })
}

function nodeWriteBody(node: FlowNode) {
  const lineage = (node.data?.lineage as DatasetLineageNodeBody | undefined) ?? nodeMap.value.get(node.id)
  const sourceKind = lineage?.sourceKind === 'parser' ? 'parser' : props.draftSourceKind
  return {
    id: node.id,
    sourceKind,
    nodeKind: lineage?.nodeKind ?? 'custom',
    resourceType: lineage?.resourceType ?? 'custom',
    publicId: lineage?.publicId,
    columnName: lineage?.columnName,
    displayName: lineage?.displayName ?? String(node.data?.label ?? node.id),
    description: lineage?.description,
    metadata: lineage?.metadata,
    position: { x: node.position.x, y: node.position.y },
  }
}

function edgeWriteBody(edge: FlowEdge) {
  const sourceKind = edge.data?.sourceKind === 'parser' ? 'parser' : props.draftSourceKind
  const confidence: 'parser_partial' | 'manual' = sourceKind === 'parser' ? 'parser_partial' : 'manual'
  return {
    id: edge.id,
    sourceKind,
    confidence,
    sourceNodeId: edge.source,
    targetNodeId: edge.target,
    relationType: String(edge.data?.relationType ?? 'manual_dependency'),
    label: String(edge.label ?? ''),
    description: String(edge.data?.description ?? ''),
    expression: String(edge.data?.expression ?? ''),
  }
}

function resourceLabel(resourceType?: string) {
  return t(`datasets.lineageResource.${resourceType || 'unknown'}`)
}

function relationLabel(relationType?: string) {
  return t(`datasets.lineageRelation.${relationType || 'unknown'}`)
}
</script>

<template>
  <div class="lineage-flow-panel">
    <div class="lineage-flow-toolbar">
      <div class="lineage-flow-toolbar-copy">
        <GitBranch :size="16" aria-hidden="true" />
        <span>{{ lineage?.nodes?.length ?? 0 }} {{ t('datasets.lineageNodes') }}</span>
        <span>{{ lineage?.edges?.length ?? 0 }} {{ t('datasets.lineageEdges') }}</span>
      </div>
      <div v-if="editable" class="lineage-flow-actions">
        <button class="secondary-button compact-button" type="button" :disabled="saving" @click="addCustomNode">
          <Plus :size="15" aria-hidden="true" />
          {{ t('datasets.lineageAddCustomNode') }}
        </button>
        <button class="secondary-button compact-button" type="button" :disabled="saving" @click="saveDraft">
          <Save :size="15" aria-hidden="true" />
          {{ saving ? t('common.saving') : t('datasets.lineageSaveDraft') }}
        </button>
        <button v-if="hasDraft && canPublish" class="secondary-button compact-button" type="button" :disabled="saving" @click="emit('publishDraft')">
          {{ t('datasets.lineagePublish') }}
        </button>
        <button v-if="hasDraft && canPublish" class="secondary-button danger-button compact-button" type="button" :disabled="saving" @click="emit('rejectDraft')">
          {{ t('datasets.lineageReject') }}
        </button>
      </div>
    </div>

    <div v-if="flowNodes.length > 0" class="lineage-flow-canvas">
      <VueFlow
        v-model:nodes="flowNodes"
        v-model:edges="flowEdges"
        :nodes-draggable="editable"
        :nodes-connectable="editable"
        :elements-selectable="true"
        fit-view-on-init
        @connect="onConnect"
        @node-click="onNodeClick"
        @edge-click="onEdgeClick"
      >
        <Controls />
        <MiniMap />
      </VueFlow>
    </div>
    <div v-else class="empty-state compact-empty-state">
      <p>{{ t('datasets.noLineageEdges') }}</p>
    </div>

    <div v-if="editable && selectedEdge" class="lineage-edge-editor">
      <span class="status-pill" :class="{ success: !selectedEdgeLocked }">
        <Lock v-if="selectedEdgeLocked" :size="13" aria-hidden="true" />
        {{ selectedEdgeLocked ? t('datasets.lineageLockedEdge') : t('datasets.lineageEdgeEditor') }}
      </span>
      <label class="field compact-field">
        <span class="field-label">{{ t('datasets.lineageRelationLabel') }}</span>
        <input v-model="selectedEdgeLabel" class="field-input" :disabled="selectedEdgeLocked">
      </label>
      <label class="field compact-field">
        <span class="field-label">{{ t('datasets.lineageRelationType') }}</span>
        <input v-model="selectedEdgeRelation" class="field-input" :disabled="selectedEdgeLocked">
      </label>
      <label class="field compact-field">
        <span class="field-label">{{ t('datasets.description') }}</span>
        <input v-model="selectedEdgeDescription" class="field-input" :disabled="selectedEdgeLocked">
      </label>
      <button class="secondary-button compact-button" type="button" :disabled="selectedEdgeLocked" @click="updateSelectedEdge">
        <Link2 :size="15" aria-hidden="true" />
        {{ t('common.save') }}
      </button>
      <button class="secondary-button danger-button compact-button" type="button" :disabled="selectedEdgeLocked" @click="removeSelectedEdge">
        <Trash2 :size="15" aria-hidden="true" />
        {{ t('common.delete') }}
      </button>
    </div>
  </div>
</template>
