<script setup lang="ts">
import { ref, watch } from 'vue'
import { VueFlow, type Connection } from '@vue-flow/core'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'
import { useI18n } from 'vue-i18n'

import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'
import '@vue-flow/minimap/dist/style.css'

import type { DataPipelineGraph, DataPipelineStepType } from '../api/data-pipelines'
import DataPipelineNode from './DataPipelineNode.vue'

const props = defineProps<{
  graph: DataPipelineGraph
  selectedNodeId: string
}>()

const emit = defineEmits<{
  'update:graph': [graph: DataPipelineGraph]
  'select-node': [nodeId: string]
}>()

const nodes = ref(clone(props.graph.nodes))
const edges = ref(clone(props.graph.edges))
const { t } = useI18n()

watch(
  () => props.graph,
  (graph) => {
    const nextNodes = clone(graph.nodes)
    const nextEdges = clone(graph.edges)
    if (JSON.stringify(nextNodes) !== JSON.stringify(nodes.value)) {
      nodes.value = nextNodes
    }
    if (JSON.stringify(nextEdges) !== JSON.stringify(edges.value)) {
      edges.value = nextEdges
    }
  },
  { deep: true },
)

watch([nodes, edges], () => {
  emit('update:graph', {
    nodes: clone(nodes.value),
    edges: clone(edges.value),
  })
}, { deep: true })

function addNode(stepType: DataPipelineStepType) {
  if (stepType === 'input') {
    const input = nodes.value.find((node) => node.data?.stepType === 'input')
    if (input) {
      emit('select-node', input.id)
      return
    }
  }
  if (stepType === 'output') {
    const output = nodes.value.find((node) => node.data?.stepType === 'output')
    if (output) {
      emit('select-node', output.id)
      return
    }
  }
  const count = nodes.value.filter((node) => node.data?.stepType === stepType).length + 1
  const id = `${stepType}_${count}_${randomId()}`
  const position = insertionPosition()
  nodes.value = [
    ...nodes.value,
    {
      id,
      type: 'pipelineStep',
      position,
      data: {
        label: labelForStep(stepType),
        stepType,
        config: defaultConfig(stepType),
      },
    },
  ]
  edges.value = insertionEdges(id, stepType)
  emit('select-node', id)
}

function insertionPosition() {
  const selected = nodes.value.find((node) => node.id === props.selectedNodeId)
  if (!selected) {
    return { x: 180 + nodes.value.length * 24, y: 120 + nodes.value.length * 36 }
  }
  if (selected.data?.stepType === 'output') {
    return { x: Math.max(40, selected.position.x - 180), y: selected.position.y }
  }
  return { x: selected.position.x + 180, y: selected.position.y }
}

function insertionEdges(nodeId: string, stepType: DataPipelineStepType) {
  if (stepType === 'input') {
    return edges.value
  }
  const { source, target } = insertionLink()
  if (!source && !target) {
    return edges.value
  }
  const nextEdges = edges.value.filter((edge) => !(source && target && edge.source === source && edge.target === target))
  if (source) {
    nextEdges.push({
      id: `edge_${source}_${nodeId}_${randomId()}`,
      source,
      target: nodeId,
    })
  }
  if (target && stepType !== 'output') {
    nextEdges.push({
      id: `edge_${nodeId}_${target}_${randomId()}`,
      source: nodeId,
      target,
    })
  }
  return nextEdges
}

function insertionLink() {
  const selected = nodes.value.find((node) => node.id === props.selectedNodeId)
  const input = nodes.value.find((node) => node.data?.stepType === 'input')
  const output = nodes.value.find((node) => node.data?.stepType === 'output')
  if (selected?.data?.stepType === 'output') {
    const incoming = edges.value.find((edge) => edge.target === selected.id)
    return { source: incoming?.source || input?.id || '', target: selected.id }
  }

  const source = selected?.id || input?.id || ''
  if (!source) {
    return { source: '', target: output?.id || '' }
  }
  const directOutputEdge = output ? edges.value.find((edge) => edge.source === source && edge.target === output.id) : undefined
  const outgoing = directOutputEdge ?? edges.value.find((edge) => edge.source === source)
  return { source, target: outgoing?.target || output?.id || '' }
}

function onConnect(connection: Connection) {
  if (!connection.source || !connection.target || connection.source === connection.target) {
    return
  }
  const exists = edges.value.some((edge) => edge.source === connection.source && edge.target === connection.target)
  if (exists) {
    return
  }
  edges.value = [
    ...edges.value,
    {
      id: `edge_${connection.source}_${connection.target}_${randomId()}`,
      source: connection.source,
      target: connection.target,
    },
  ]
}

function onNodeClick(event: any) {
  const nodeId = event?.node?.id
  if (nodeId) {
    emit('select-node', nodeId)
  }
}

function labelForStep(type: DataPipelineStepType) {
  return type
    .split('_')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

function defaultConfig(type: DataPipelineStepType): Record<string, unknown> {
  switch (type) {
  case 'input':
    return { sourceKind: 'dataset', datasetPublicId: '' }
  case 'clean':
    return { rules: [{ operation: 'drop_null_rows', columns: [] }] }
  case 'normalize':
    return { rules: [] }
  case 'validate':
    return { rules: [] }
  case 'schema_mapping':
    return { mappings: [] }
  case 'schema_completion':
    return { rules: [] }
  case 'enrich_join':
    return { rightSourceKind: 'dataset', rightDatasetPublicId: '', joinType: 'left', leftKeys: [], rightKeys: [], selectColumns: [] }
  case 'transform':
    return { operation: 'select_columns', columns: [] }
  case 'output':
    return { displayName: t('dataPipelines.defaultOutputDisplayName'), writeMode: 'replace', engine: 'MergeTree' }
  default:
    return {}
  }
}

function randomId() {
  return Math.random().toString(36).slice(2, 8)
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

defineExpose({ addNode })
</script>

<template>
  <div class="data-pipeline-flow-builder">
    <VueFlow
      v-model:nodes="nodes"
      v-model:edges="edges"
      fit-view-on-init
      :nodes-draggable="true"
      :nodes-connectable="true"
      :elements-selectable="true"
      @connect="onConnect"
      @node-click="onNodeClick"
    >
      <template #node-pipelineStep="nodeProps">
        <DataPipelineNode v-bind="nodeProps" :selected="nodeProps.id === selectedNodeId" />
      </template>
      <Controls />
      <MiniMap pannable zoomable />
    </VueFlow>
  </div>
</template>
