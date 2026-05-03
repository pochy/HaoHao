<script setup lang="ts">
import { computed } from 'vue'
import { Handle, Position } from '@vue-flow/core'
import { CheckCircle2, CircleAlert, Database, FileOutput, GitBranch, SlidersHorizontal } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import type { DataPipelineStepType } from '../api/data-pipelines'

const props = defineProps<{
  data?: {
    label?: string
    stepType?: DataPipelineStepType
    config?: Record<string, unknown>
  }
  selected?: boolean
}>()

const { t } = useI18n()
const stepType = computed(() => props.data?.stepType ?? 'transform')
const label = computed(() => displayNodeLabel(props.data?.label, stepType.value))
const status = computed(() => {
  if (stepType.value === 'input' && !props.data?.config?.datasetPublicId && !props.data?.config?.workTablePublicId) {
    return 'warning'
  }
  return 'ready'
})
const icon = computed(() => {
  if (stepType.value === 'input') return Database
  if (stepType.value === 'output') return FileOutput
  if (stepType.value === 'enrich_join') return GitBranch
  return SlidersHorizontal
})

function displayNodeLabel(value: string | undefined, type: DataPipelineStepType) {
  const text = value?.trim()
  if (!text || text === englishLabelForStep(type)) {
    return stepLabel(type)
  }
  return text
}

function stepLabel(type: DataPipelineStepType) {
  return t(`dataPipelines.step.${type}`)
}

function englishLabelForStep(type: DataPipelineStepType) {
  return type
    .split('_')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}
</script>

<template>
  <div class="data-pipeline-node" :class="{ selected, warning: status === 'warning' }">
    <Handle type="target" :position="Position.Left" />
    <div class="data-pipeline-node-icon">
      <component :is="icon" :size="16" stroke-width="1.9" aria-hidden="true" />
    </div>
    <div class="data-pipeline-node-copy">
      <strong>{{ label }}</strong>
      <span>{{ stepLabel(stepType) }}</span>
    </div>
    <CircleAlert v-if="status === 'warning'" class="data-pipeline-node-status" :size="15" stroke-width="1.9" aria-hidden="true" />
    <CheckCircle2 v-else class="data-pipeline-node-status ready" :size="15" stroke-width="1.9" aria-hidden="true" />
    <Handle type="source" :position="Position.Right" />
  </div>
</template>
