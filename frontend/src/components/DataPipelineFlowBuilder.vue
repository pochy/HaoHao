<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { VueFlow, useVueFlow, type Connection } from '@vue-flow/core'
import { ControlButton, Controls } from '@vue-flow/controls'
import { AlignHorizontalSpaceBetween, ArrowDownToLine, GitBranch, Plus, Search, SlidersHorizontal, X } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'

import {
  isDataPipelineAutoPreviewEnabled,
  sanitizeDataPipelineGraph,
  type DataPipelineEdge,
  type DataPipelineGraph,
  type DataPipelineNode as PipelineNode,
  type DataPipelineStepType,
} from '../api/data-pipelines'
import DataPipelineNode from './DataPipelineNode.vue'

const props = defineProps<{
  graph: DataPipelineGraph
  selectedNodeId: string
  nodeCatalog: Array<{ type: DataPipelineStepType, labelKey: string }>
}>()

const emit = defineEmits<{
  'update:graph': [graph: DataPipelineGraph]
  'select-node': [nodeId: string]
}>()

const flowId = `data-pipeline-flow-${Math.random().toString(36).slice(2, 8)}`
const { fitView } = useVueFlow(flowId)
const initialGraph = sanitizeDataPipelineGraph(props.graph)
const nodes = ref(clone(initialGraph.nodes))
const edges = ref(clone(initialGraph.edges))
const paletteDialogRef = ref<HTMLDialogElement | null>(null)
const paletteSearch = ref('')
const { t } = useI18n()

const autoLayoutColumnGap = 330
const autoLayoutRowGap = 120
const autoLayoutStartX = 60
const autoLayoutStartY = 80
const snapGridSize = 15
const stepOrder: Record<DataPipelineStepType, number> = {
  input: 0,
  profile: 10,
  clean: 20,
  normalize: 30,
  validate: 40,
  schema_mapping: 50,
  schema_completion: 60,
  enrich_join: 70,
  transform: 80,
  output: 1000,
}
const paletteCategories = [
  { id: 'input_output', labelKey: 'dataPipelines.paletteCategories.inputOutput' },
  { id: 'transform', labelKey: 'dataPipelines.paletteCategories.transform' },
  { id: 'quality', labelKey: 'dataPipelines.paletteCategories.quality' },
  { id: 'schema', labelKey: 'dataPipelines.paletteCategories.schema' },
] as const
type PaletteCategory = typeof paletteCategories[number]['id']
const stepCategory: Record<DataPipelineStepType, PaletteCategory> = {
  input: 'input_output',
  output: 'input_output',
  clean: 'quality',
  normalize: 'quality',
  validate: 'quality',
  profile: 'transform',
  enrich_join: 'transform',
  transform: 'transform',
  schema_mapping: 'schema',
  schema_completion: 'schema',
}
const categoryIcons: Record<PaletteCategory, typeof GitBranch> = {
  input_output: ArrowDownToLine,
  transform: SlidersHorizontal,
  quality: GitBranch,
  schema: GitBranch,
}

const paletteGroups = computed(() => {
  const query = paletteSearch.value.trim().toLowerCase()
  const catalog = props.nodeCatalog
    .map((node) => ({
      ...node,
      category: stepCategory[node.type],
      descriptionKey: `dataPipelines.stepDescriptions.${node.type}`,
      detailKey: `dataPipelines.stepDetails.${node.type}`,
    }))
    .filter((node) => {
      if (!query) {
        return true
      }
      return `${t(node.labelKey)} ${t(node.descriptionKey)} ${t(node.detailKey)}`.toLowerCase().includes(query)
    })

  return paletteCategories
    .map((category) => ({
      ...category,
      nodes: catalog.filter((node) => node.category === category.id),
    }))
    .filter((category) => category.nodes.length > 0)
})

watch(
  () => props.graph,
  (graph) => {
    const nextGraph = sanitizeDataPipelineGraph(graph)
    const nextNodes = clone(nextGraph.nodes)
    const nextEdges = clone(nextGraph.edges)
    const currentGraph = sanitizeDataPipelineGraph({
      nodes: clone(nodes.value),
      edges: clone(edges.value),
    })
    if (JSON.stringify(nextNodes) !== JSON.stringify(currentGraph.nodes)) {
      nodes.value = nextNodes
    }
    if (JSON.stringify(nextEdges) !== JSON.stringify(currentGraph.edges)) {
      edges.value = nextEdges
    }
  },
  { deep: true },
)

watch([nodes, edges], () => {
  emit('update:graph', sanitizeDataPipelineGraph({
    nodes: clone(nodes.value),
    edges: clone(edges.value),
  }))
}, { deep: true })

onBeforeUnmount(() => {
  if (paletteDialogRef.value?.open) {
    paletteDialogRef.value.close()
  }
})

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

function openPalette() {
  if (!paletteDialogRef.value?.open) {
    paletteSearch.value = ''
    paletteDialogRef.value?.showModal()
  }
}

function closePalette() {
  if (paletteDialogRef.value?.open) {
    paletteDialogRef.value.close()
  }
}

function addPaletteNode(stepType: DataPipelineStepType) {
  addNode(stepType)
  closePalette()
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

async function autoLayout() {
  const graph = sanitizeDataPipelineGraph({
    nodes: clone(nodes.value),
    edges: clone(edges.value),
  })
  const positions = autoLayoutPositions(graph.nodes, graph.edges)
  nodes.value = graph.nodes.map((node) => ({
    ...node,
    position: positions.get(node.id) ?? node.position,
  }))
  edges.value = graph.edges

  await nextTick()
  await fitView({ padding: 0.16 })
}

function autoLayoutPositions(graphNodes: PipelineNode[], graphEdges: DataPipelineEdge[]) {
  const nodeMap = new Map(graphNodes.map((node) => [node.id, node]))
  const nodeIndex = new Map(graphNodes.map((node, index) => [node.id, index]))
  const validEdges = graphEdges.filter((edge) => nodeMap.has(edge.source) && nodeMap.has(edge.target) && edge.source !== edge.target)
  const incoming = new Map<string, string[]>()
  const outgoing = new Map<string, string[]>()
  const indegree = new Map(graphNodes.map((node) => [node.id, 0]))

  for (const edge of validEdges) {
    outgoing.set(edge.source, [...(outgoing.get(edge.source) ?? []), edge.target])
    incoming.set(edge.target, [...(incoming.get(edge.target) ?? []), edge.source])
    indegree.set(edge.target, (indegree.get(edge.target) ?? 0) + 1)
  }

  const depth = new Map<string, number>()
  const queue = graphNodes
    .filter((node) => (indegree.get(node.id) ?? 0) === 0)
    .sort((a, b) => compareNodes(a, b, nodeIndex))
    .map((node) => node.id)

  if (queue.length === 0) {
    const input = graphNodes.find((node) => node.data.stepType === 'input') ?? graphNodes[0]
    if (input) {
      queue.push(input.id)
    }
  }

  for (const id of queue) {
    depth.set(id, 0)
  }

  const pendingIndegree = new Map(indegree)
  for (let cursor = 0; cursor < queue.length; cursor += 1) {
    const id = queue[cursor]
    const nextDepth = (depth.get(id) ?? 0) + 1
    const nextNodes = [...(outgoing.get(id) ?? [])]
      .map((target) => nodeMap.get(target))
      .filter((node): node is PipelineNode => Boolean(node))
      .sort((a, b) => compareNodes(a, b, nodeIndex))

    for (const target of nextNodes) {
      depth.set(target.id, Math.max(depth.get(target.id) ?? 0, nextDepth))
      pendingIndegree.set(target.id, Math.max((pendingIndegree.get(target.id) ?? 0) - 1, 0))
      if ((pendingIndegree.get(target.id) ?? 0) === 0 && !queue.includes(target.id)) {
        queue.push(target.id)
      }
    }
  }

  const fallbackDepth = Math.max(0, ...depth.values()) + 1
  for (const node of graphNodes) {
    if (!depth.has(node.id)) {
      const upstreamDepth = Math.max(
        -1,
        ...(incoming.get(node.id) ?? []).map((source) => depth.get(source) ?? -1),
      )
      depth.set(node.id, upstreamDepth >= 0 ? upstreamDepth + 1 : fallbackDepth)
    }
  }

  const outputDepth = Math.max(0, ...[...depth.entries()]
    .filter(([id]) => nodeMap.get(id)?.data.stepType !== 'output')
    .map(([, itemDepth]) => itemDepth)) + 1
  for (const node of graphNodes) {
    if (node.data.stepType === 'input') {
      depth.set(node.id, 0)
    } else if (node.data.stepType === 'output') {
      depth.set(node.id, Math.max(depth.get(node.id) ?? outputDepth, outputDepth))
    }
  }

  const layers = new Map<number, PipelineNode[]>()
  for (const node of graphNodes) {
    const itemDepth = depth.get(node.id) ?? 0
    layers.set(itemDepth, [...(layers.get(itemDepth) ?? []), node])
  }

  const maxLayerSize = Math.max(1, ...[...layers.values()].map((layer) => layer.length))
  const positions = new Map<string, { x: number, y: number }>()
  for (const [itemDepth, layerNodes] of [...layers.entries()].sort(([a], [b]) => a - b)) {
    const sortedLayerNodes = [...layerNodes].sort((a, b) => compareNodes(a, b, nodeIndex))
    const yOffset = ((maxLayerSize - sortedLayerNodes.length) * autoLayoutRowGap) / 2
    sortedLayerNodes.forEach((node, index) => {
      positions.set(node.id, {
        x: snapValue(autoLayoutStartX + itemDepth * autoLayoutColumnGap),
        y: snapValue(autoLayoutStartY + yOffset + index * autoLayoutRowGap),
      })
    })
  }

  return positions
}

function compareNodes(a: PipelineNode, b: PipelineNode, nodeIndex: Map<string, number>) {
  const orderDiff = (stepOrder[a.data.stepType] ?? 500) - (stepOrder[b.data.stepType] ?? 500)
  if (orderDiff !== 0) {
    return orderDiff
  }
  return (nodeIndex.get(a.id) ?? 0) - (nodeIndex.get(b.id) ?? 0)
}

function snapValue(value: number) {
  return Math.round(value / snapGridSize) * snapGridSize
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
      :id="flowId"
      v-model:nodes="nodes"
      v-model:edges="edges"
      fit-view-on-init
      snap-to-grid
      :snap-grid="[snapGridSize, snapGridSize]"
      :nodes-draggable="true"
      :nodes-connectable="true"
      :elements-selectable="true"
      @connect="onConnect"
      @node-click="onNodeClick"
    >
      <template #node-pipelineStep="nodeProps">
        <DataPipelineNode
          v-bind="nodeProps"
          :selected="nodeProps.id === selectedNodeId"
          :auto-preview-enabled="isDataPipelineAutoPreviewEnabled(nodeProps.data)"
        />
      </template>
      <Controls>
        <ControlButton
          :title="t('dataPipelines.openPalette')"
          :aria-label="t('dataPipelines.openPalette')"
          type="button"
          @click="openPalette"
        >
          <Plus :size="15" stroke-width="2.2" aria-hidden="true" />
        </ControlButton>
        <ControlButton
          :title="t('dataPipelines.autoLayout')"
          :aria-label="t('dataPipelines.autoLayout')"
          type="button"
          @click="autoLayout"
        >
          <AlignHorizontalSpaceBetween :size="14" stroke-width="2.1" aria-hidden="true" />
        </ControlButton>
      </Controls>
    </VueFlow>

    <dialog
      ref="paletteDialogRef"
      class="confirm-dialog data-pipeline-palette-dialog"
      @cancel.prevent="closePalette"
    >
      <div class="confirm-dialog-panel data-pipeline-palette-dialog-panel">
        <aside class="data-pipeline-palette-dialog-nav" :aria-label="t('dataPipelines.paletteCategoriesLabel')">
          <h2>{{ t('dataPipelines.blockLibrary') }}</h2>
          <label class="data-pipeline-palette-search">
            <Search :size="18" stroke-width="1.8" aria-hidden="true" />
            <input v-model="paletteSearch" type="search" :placeholder="t('common.search')" autocomplete="off">
          </label>
          <nav>
            <a v-for="category in paletteCategories" :key="category.id" :href="`#palette-${category.id}`">
              <component :is="categoryIcons[category.id]" :size="17" stroke-width="2" aria-hidden="true" />
              {{ t(category.labelKey) }}
            </a>
          </nav>
        </aside>

        <section class="data-pipeline-palette-dialog-content">
          <button
            class="data-pipeline-palette-close"
            type="button"
            :title="t('common.close')"
            :aria-label="t('common.close')"
            @click="closePalette"
          >
            <X :size="28" stroke-width="1.8" aria-hidden="true" />
          </button>
          <div v-for="group in paletteGroups" :id="`palette-${group.id}`" :key="group.id" class="data-pipeline-palette-group">
            <h3>{{ t(group.labelKey) }}</h3>
            <div class="data-pipeline-palette-card-grid">
              <button
                v-for="node in group.nodes"
                :key="node.type"
                class="data-pipeline-palette-card"
                type="button"
                @click="addPaletteNode(node.type)"
              >
                <strong>{{ t(node.labelKey) }}</strong>
                <span>{{ t(node.descriptionKey) }}</span>
                <span class="data-pipeline-palette-hover-card" role="tooltip">
                  {{ t(node.detailKey) }}
                </span>
              </button>
            </div>
          </div>
          <p v-if="paletteGroups.length === 0" class="data-pipeline-palette-empty">
            {{ t('dataPipelines.noPaletteResults') }}
          </p>
        </section>
      </div>
    </dialog>
  </div>
</template>
