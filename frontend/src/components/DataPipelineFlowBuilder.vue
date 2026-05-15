<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { VueFlow, useVueFlow, type Connection } from '@vue-flow/core'
import { ControlButton, Controls } from '@vue-flow/controls'
import type { ELK, ElkExtendedEdge, ElkNode } from 'elkjs/lib/elk-api'
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
  type DataPipelineGraphValidationBody,
  type DataPipelineNodeWarningBody,
  type DataPipelineNode as PipelineNode,
  type DataPipelineStepType,
} from '../api/data-pipelines'
import DataPipelineNode from './DataPipelineNode.vue'

const props = defineProps<{
  graph: DataPipelineGraph
  selectedNodeId: string
  nodeCatalog: Array<{ type: DataPipelineStepType, labelKey: string }>
  validation?: DataPipelineGraphValidationBody | null
}>()

const emit = defineEmits<{
  'update:graph': [graph: DataPipelineGraph, options?: GraphUpdateOptions]
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
let elkPromise: Promise<ELK> | null = null
let autoLayouting = false
let nodeDragging = false
let nodeDragStartGraph: DataPipelineGraph | null = null

type GraphUpdateOptions = {
  transient?: boolean
  commit?: boolean
}

const autoLayoutLayerGap = 72
const autoLayoutRowGap = 99
const autoLayoutStartX = 60
const autoLayoutStartY = 80
const autoLayoutNodeMinWidth = 190
const autoLayoutNodeFallbackWidth = 220
const autoLayoutNodeHeight = 78
const autoLayoutRowSpacing = autoLayoutRowGap - autoLayoutNodeHeight
const snapGridSize = 15
const stepOrder: Record<DataPipelineStepType, number> = {
  input: 0,
  profile: 10,
  clean: 20,
  normalize: 30,
  validate: 40,
  schema_mapping: 50,
  schema_completion: 60,
  join: 68,
  enrich_join: 70,
  transform: 80,
  extract_text: 12,
  json_extract: 13,
  excel_extract: 14,
  classify_document: 15,
  extract_fields: 16,
  extract_table: 18,
  product_extraction: 19,
  detect_language_encoding: 19,
  canonicalize: 22,
  redact_pii: 24,
  deduplicate: 26,
  schema_inference: 62,
  entity_resolution: 72,
  unit_conversion: 74,
  relationship_extraction: 76,
  human_review: 92,
  sample_compare: 94,
  quality_report: 96,
  confidence_gate: 90,
  quarantine: 91,
  route_by_condition: 93,
  output: 1000,
}
const paletteCategories = [
  { id: 'input_output', labelKey: 'dataPipelines.paletteCategories.inputOutput' },
  { id: 'extraction', labelKey: 'dataPipelines.paletteCategories.extraction' },
  { id: 'transform', labelKey: 'dataPipelines.paletteCategories.transform' },
  { id: 'quality', labelKey: 'dataPipelines.paletteCategories.quality' },
  { id: 'schema', labelKey: 'dataPipelines.paletteCategories.schema' },
] as const
type PaletteCategory = typeof paletteCategories[number]['id']
const stepCategory: Record<DataPipelineStepType, PaletteCategory> = {
  input: 'input_output',
  output: 'input_output',
  extract_text: 'extraction',
  json_extract: 'extraction',
  excel_extract: 'extraction',
  classify_document: 'extraction',
  extract_fields: 'extraction',
  extract_table: 'extraction',
  product_extraction: 'extraction',
  confidence_gate: 'quality',
  quarantine: 'quality',
  route_by_condition: 'quality',
  deduplicate: 'quality',
  canonicalize: 'quality',
  redact_pii: 'quality',
  detect_language_encoding: 'quality',
  schema_inference: 'schema',
  entity_resolution: 'transform',
  unit_conversion: 'transform',
  relationship_extraction: 'transform',
  human_review: 'quality',
  sample_compare: 'quality',
  quality_report: 'quality',
  clean: 'quality',
  normalize: 'quality',
  validate: 'quality',
  profile: 'transform',
  join: 'transform',
  enrich_join: 'transform',
  transform: 'transform',
  schema_mapping: 'schema',
  schema_completion: 'schema',
}
const categoryIcons: Record<PaletteCategory, typeof GitBranch> = {
  input_output: ArrowDownToLine,
  extraction: GitBranch,
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
const validationWarningsByNode = computed(() => {
  const warnings = new Map<string, DataPipelineNodeWarningBody[]>()
  for (const warning of props.validation?.nodeWarnings ?? []) {
    warnings.set(warning.nodeId, [...(warnings.get(warning.nodeId) ?? []), warning])
  }
  return warnings
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
  emitCurrentGraph(nodeDragging ? { transient: true } : undefined)
}, { deep: true })

onBeforeUnmount(() => {
  if (paletteDialogRef.value?.open) {
    paletteDialogRef.value.close()
  }
})

function addNode(stepType: DataPipelineStepType) {
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

function onConnect(connection: Connection) {
  if (autoLayouting) {
    return
  }
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

function onNodeDragStart() {
  nodeDragging = true
  nodeDragStartGraph = currentGraph()
}

function onNodeDragStop() {
  const startGraph = nodeDragStartGraph
  const endGraph = currentGraph()
  nodeDragging = false
  nodeDragStartGraph = null
  if (startGraph && !graphsEqual(startGraph, endGraph)) {
    emit('update:graph', endGraph, { commit: true })
  }
}

function emitCurrentGraph(options?: GraphUpdateOptions) {
  emit('update:graph', currentGraph(), options)
}

function currentGraph() {
  return sanitizeDataPipelineGraph({
    nodes: clone(nodes.value),
    edges: clone(edges.value),
  })
}

async function autoLayout() {
  const layoutEdges = clone(edges.value)
  const graph = sanitizeDataPipelineGraph({
    nodes: clone(nodes.value),
    edges: layoutEdges,
  })
  autoLayouting = true
  try {
    await nextTick()
    const positions = await autoLayoutPositions(graph.nodes, graph.edges)
    nodes.value = graph.nodes.map((node) => ({
      ...node,
      position: positions.get(node.id) ?? node.position,
    }))
    edges.value = layoutEdges

    await nextTick()
    edges.value = layoutEdges
    await fitView({ padding: 0.16 })
  } finally {
    autoLayouting = false
  }
}

async function autoLayoutPositions(graphNodes: PipelineNode[], graphEdges: DataPipelineEdge[]) {
  if (graphNodes.length === 0) {
    return new Map<string, { x: number, y: number }>()
  }

  const nodeMap = new Map(graphNodes.map((node) => [node.id, node]))
  const nodeIndex = new Map(graphNodes.map((node, index) => [node.id, index]))
  const validEdges = graphEdges.filter((edge) => nodeMap.has(edge.source) && nodeMap.has(edge.target) && edge.source !== edge.target)
  const sortedNodes = [...graphNodes].sort((a, b) => compareLayoutOrder(a, b, nodeIndex))
  const layoutSizes = layoutNodeSizes(graphNodes)
  const layoutGraph: ElkNode = {
    id: 'data-pipeline-layout-root',
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': 'RIGHT',
      'elk.edgeRouting': 'ORTHOGONAL',
      'elk.separateConnectedComponents': 'false',
      'elk.spacing.nodeNode': String(autoLayoutRowSpacing),
      'org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers': String(autoLayoutLayerGap),
      'org.eclipse.elk.layered.spacing.edgeNodeBetweenLayers': '48',
      'org.eclipse.elk.layered.crossingMinimization.forceNodeModelOrder': 'true',
      'org.eclipse.elk.layered.considerModelOrder.strategy': 'NODES_AND_EDGES',
      'org.eclipse.elk.layered.nodePlacement.strategy': 'NETWORK_SIMPLEX',
      'org.eclipse.elk.layered.nodePlacement.favorStraightEdges': 'true',
      'org.eclipse.elk.layered.cycleBreaking.strategy': 'GREEDY',
    },
    children: sortedNodes.map((node, index) => ({
      id: node.id,
      width: layoutSizes.get(node.id)?.width ?? autoLayoutNodeFallbackWidth,
      height: layoutSizes.get(node.id)?.height ?? autoLayoutNodeHeight,
      layoutOptions: {
        'org.eclipse.elk.layered.crossingMinimization.positionId': String(index),
        ...nodeLayerConstraint(node),
      },
    })),
    edges: autoLayoutEdges(validEdges),
  }

  let laidOutGraph: ElkNode
  try {
    laidOutGraph = await (await getElk()).layout(layoutGraph)
  } catch {
    return fallbackAutoLayoutPositions(graphNodes, graphEdges)
  }

  const laidOutNodes = laidOutGraph.children?.filter((node) => (
    typeof node.x === 'number' && typeof node.y === 'number'
  )) ?? []
  if (laidOutNodes.length !== graphNodes.length) {
    return fallbackAutoLayoutPositions(graphNodes, graphEdges)
  }

  const minX = Math.min(...laidOutNodes.map((node) => node.x ?? 0))
  const minY = Math.min(...laidOutNodes.map((node) => node.y ?? 0))
  const positions = new Map<string, { x: number, y: number }>()
  for (const node of laidOutNodes) {
    positions.set(node.id, {
      x: snapValue(autoLayoutStartX + (node.x ?? 0) - minX),
      y: snapValue(autoLayoutStartY + (node.y ?? 0) - minY),
    })
  }

  return positions
}

function layoutNodeSizes(graphNodes: PipelineNode[]) {
  const sizes = new Map<string, { width: number, height: number }>()
  for (const node of graphNodes) {
    sizes.set(node.id, measureRenderedNode(node) ?? estimateNodeSize(node))
  }
  return sizes
}

function measureRenderedNode(node: PipelineNode) {
  if (typeof document === 'undefined') {
    return null
  }

  const renderedNode = document.querySelector(
    `.vue-flow__node[data-id="${cssEscape(node.id)}"] .data-pipeline-node`,
  )
  if (!(renderedNode instanceof HTMLElement)) {
    return null
  }

  const width = renderedNode.offsetWidth
  const height = renderedNode.offsetHeight
  if (!Number.isFinite(width) || width <= 0) {
    return null
  }

  return {
    width: clampNodeWidth(width),
    height: Math.max(autoLayoutNodeHeight, Math.ceil(height)),
  }
}

function estimateNodeSize(node: PipelineNode) {
  const labelWidth = estimateTextWidth(displayLabelForLayout(node), '700 13.76px "Avenir Next", "Segoe UI", sans-serif')
  const stepWidth = estimateTextWidth(t(`dataPipelines.step.${node.data.stepType}`), '400 11.68px "Avenir Next", "Segoe UI", sans-serif')
  const copyWidth = Math.max(labelWidth, stepWidth + 23)
  const chromeWidth = 24 + 30 + 16 + 15

  return {
    width: clampNodeWidth(Math.ceil(chromeWidth + copyWidth + 28)),
    height: autoLayoutNodeHeight,
  }
}

function estimateTextWidth(text: string, font: string) {
  if (typeof document === 'undefined') {
    return text.length * 8
  }

  const canvas = document.createElement('canvas')
  const context = canvas.getContext('2d')
  if (!context) {
    return text.length * 8
  }
  context.font = font
  return context.measureText(text).width
}

function displayLabelForLayout(node: PipelineNode) {
  const text = node.data.label?.trim()
  if (!text || text === englishLabelForStep(node.data.stepType)) {
    return t(`dataPipelines.step.${node.data.stepType}`)
  }
  return text
}

function clampNodeWidth(width: number) {
  return Math.max(autoLayoutNodeMinWidth, Math.ceil(width))
}

function cssEscape(value: string) {
  if (typeof CSS !== 'undefined' && typeof CSS.escape === 'function') {
    return CSS.escape(value)
  }
  return value.replace(/["\\]/g, '\\$&')
}

function getElk() {
  elkPromise ??= Promise.all([
    import('elkjs/lib/elk-api'),
    import('elkjs/lib/elk-worker.min.js?worker'),
  ]).then(([{ default: Elk }, { default: ElkWorker }]) => new Elk({ workerFactory: () => new ElkWorker() }))
  return elkPromise
}

function nodeLayerConstraint(node: PipelineNode): Record<string, string> {
  if (node.data.stepType === 'input') {
    return { 'org.eclipse.elk.layered.layering.layerConstraint': 'FIRST' }
  }
  if (node.data.stepType === 'output') {
    return { 'org.eclipse.elk.layered.layering.layerConstraint': 'LAST' }
  }
  return {}
}

function autoLayoutEdges(validEdges: DataPipelineEdge[]) {
  const edgeKeys = new Set<string>()
  const layoutEdges: ElkExtendedEdge[] = []
  const appendEdge = (id: string, source: string, target: string) => {
    const edgeKey = `${source}:${target}`
    if (edgeKeys.has(edgeKey)) {
      return
    }
    edgeKeys.add(edgeKey)
    layoutEdges.push({ id, sources: [source], targets: [target] })
  }

  for (const edge of validEdges) {
    appendEdge(edge.id || `layout-edge-${edge.source}-${edge.target}`, edge.source, edge.target)
  }

  return layoutEdges
}

function compareLayoutOrder(a: PipelineNode, b: PipelineNode, nodeIndex: Map<string, number>) {
  const yDiff = a.position.y - b.position.y
  if (Math.abs(yDiff) >= snapGridSize) {
    return yDiff
  }
  const indexDiff = (nodeIndex.get(a.id) ?? 0) - (nodeIndex.get(b.id) ?? 0)
  if (indexDiff !== 0) {
    return indexDiff
  }
  return compareNodes(a, b, nodeIndex)
}

function fallbackAutoLayoutPositions(graphNodes: PipelineNode[], graphEdges: DataPipelineEdge[]) {
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

  const layoutSizes = layoutNodeSizes(graphNodes)
  const layerWidths = new Map<number, number>()
  for (const [itemDepth, layerNodes] of layers.entries()) {
    layerWidths.set(itemDepth, Math.max(
      autoLayoutNodeFallbackWidth,
      ...layerNodes.map((node) => layoutSizes.get(node.id)?.width ?? autoLayoutNodeFallbackWidth),
    ))
  }

  const sortedLayerDepths = [...layers.keys()].sort((a, b) => a - b)
  const layerX = new Map<number, number>()
  let nextX = autoLayoutStartX
  for (const itemDepth of sortedLayerDepths) {
    layerX.set(itemDepth, nextX)
    nextX += (layerWidths.get(itemDepth) ?? autoLayoutNodeFallbackWidth) + autoLayoutLayerGap
  }

  const maxLayerSize = Math.max(1, ...[...layers.values()].map((layer) => layer.length))
  const positions = new Map<string, { x: number, y: number }>()
  for (const [itemDepth, layerNodes] of [...layers.entries()].sort(([a], [b]) => a - b)) {
    const sortedLayerNodes = [...layerNodes].sort((a, b) => compareNodes(a, b, nodeIndex))
    const yOffset = ((maxLayerSize - sortedLayerNodes.length) * autoLayoutRowGap) / 2
    sortedLayerNodes.forEach((node, index) => {
      positions.set(node.id, {
        x: snapValue(layerX.get(itemDepth) ?? autoLayoutStartX),
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

function englishLabelForStep(type: DataPipelineStepType) {
  return type
    .split('_')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

function defaultConfig(type: DataPipelineStepType): Record<string, unknown> {
  switch (type) {
  case 'input':
    return { sourceKind: 'dataset', datasetPublicId: '' }
  case 'extract_text':
    return { chunkMode: 'page', includeBoxes: true }
  case 'json_extract':
    return { sourceColumn: 'raw_record_json', recordPath: '$', fields: [], includeSourceColumns: true, includeRawRecord: false, maxRows: 100000 }
  case 'excel_extract':
    return { sourceFileColumn: 'file_public_id', sheetIndex: 0, headerRow: 1, columns: [], includeSourceColumns: true, includeSourceMetadataColumns: true, maxRows: 100000 }
  case 'classify_document':
    return { classes: [{ label: 'invoice', keywords: ['invoice', '請求書'], priority: 10 }], outputColumn: 'document_type', confidenceColumn: 'document_type_confidence' }
  case 'extract_fields':
    return { provider: 'rules', outputMode: 'columns_and_json', fields: [{ name: 'document_date', type: 'date', required: false, patterns: ['(\\\\d{4}[-/]\\\\d{1,2}[-/]\\\\d{1,2})'] }] }
  case 'extract_table':
    return { source: 'text_delimited', delimiter: ',', headerRow: false }
  case 'product_extraction':
    return { sourceFileColumn: 'file_public_id', includeSourceColumns: true, confidenceThreshold: 0.8, maxItems: 1000 }
  case 'confidence_gate':
    return { threshold: 0.8, mode: 'annotate', statusColumn: 'gate_status' }
  case 'quarantine':
    return { mode: 'filter', statusColumn: 'gate_status', matchValues: ['needs_review'], outputMode: 'quarantine_only' }
  case 'route_by_condition':
    return { mode: 'annotate', routeColumn: 'route_key', defaultRoute: 'default', route: 'needs_review', rules: [{ column: 'gate_status', operator: '=', value: 'needs_review', route: 'needs_review' }] }
  case 'deduplicate':
    return { keyColumns: [], mode: 'annotate', statusColumn: 'duplicate_status', groupColumn: 'duplicate_group_id' }
  case 'canonicalize':
    return { rules: [{ column: '', outputColumn: '', operations: ['trim', 'normalize_spaces'], mappings: {} }] }
  case 'redact_pii':
    return { columns: ['text'], types: ['email', 'phone', 'postal_code', 'api_key_like'], mode: 'mask', outputSuffix: '_redacted' }
  case 'detect_language_encoding':
    return { textColumn: 'text', outputTextColumn: 'normalized_text', languageColumn: 'language', mojibakeScoreColumn: 'mojibake_score' }
  case 'schema_inference':
    return { columns: [], sampleLimit: 1000 }
  case 'entity_resolution':
    return { column: 'vendor', outputPrefix: 'vendor', dictionary: [] }
  case 'unit_conversion':
    return { rules: [{ valueColumn: '', unitColumn: '', outputUnit: '', conversions: [] }] }
  case 'relationship_extraction':
    return { textColumn: 'text', patterns: [{ relationType: 'related_to', pattern: '' }] }
  case 'human_review':
    return { reasonColumns: ['gate_status'], statusColumn: 'review_status', queueColumn: 'review_queue', queue: 'default', createReviewItems: false, reviewItemLimit: 1000, mode: 'annotate' }
  case 'sample_compare':
    return { pairs: [{ field: '', beforeColumn: '', afterColumn: '' }] }
  case 'quality_report':
    return { columns: [], outputMode: 'row_summary' }
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
  case 'join':
    return { joinType: 'left', joinStrictness: 'all', leftKeys: [], rightKeys: [], selectColumns: [] }
  case 'enrich_join':
    return { rightSourceKind: 'dataset', rightDatasetPublicId: '', joinType: 'left', joinStrictness: 'all', leftKeys: [], rightKeys: [], selectColumns: [] }
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

function graphsEqual(a: DataPipelineGraph, b: DataPipelineGraph) {
  return JSON.stringify(sanitizeDataPipelineGraph(a)) === JSON.stringify(sanitizeDataPipelineGraph(b))
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
      @node-drag-start="onNodeDragStart"
      @node-drag-stop="onNodeDragStop"
    >
      <template #node-pipelineStep="nodeProps">
        <DataPipelineNode
          v-bind="nodeProps"
          :selected="nodeProps.id === selectedNodeId"
          :auto-preview-enabled="isDataPipelineAutoPreviewEnabled(nodeProps.data)"
          :validation-warnings="validationWarningsByNode.get(nodeProps.id) ?? []"
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
