<script setup lang="ts">
import { computed } from 'vue'
import { GitBranch, Layers3 } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import type { MedallionAssetBody, MedallionCatalogBody, MedallionPipelineRunBody } from '../api/generated/types.gen'
import { formatDriveSize } from '../utils/driveItems'

const props = defineProps<{
  catalog: MedallionCatalogBody | null
  loading?: boolean
  title?: string
}>()

const { d, n, t } = useI18n()

const asset = computed(() => props.catalog?.asset ?? null)
const pipelineRuns = computed(() => props.catalog?.pipelineRuns ?? [])
const latestRuns = computed(() => pipelineRuns.value.slice(0, 5))
const upstream = computed(() => props.catalog?.upstream ?? [])
const downstream = computed(() => props.catalog?.downstream ?? [])
const schemaColumnCount = computed(() => {
  const value = asset.value?.schemaSummary?.columns
  return typeof value === 'number' ? value : 0
})
const titleText = computed(() => props.title || t('medallion.catalog'))

function statusClass(status = '') {
  if (status === 'active' || status === 'completed') {
    return 'success'
  }
  if (status === 'failed') {
    return 'danger'
  }
  if (status === 'skipped' || status === 'archived') {
    return ''
  }
  return 'warning'
}

function formatDate(value?: string) {
  return value ? d(new Date(value), 'short') : '-'
}

function layerLabel(item: MedallionAssetBody) {
  return t(`medallion.layer.${item.layer}`)
}

function resourceKindLabel(kind = '') {
  return t(`medallion.resourceKind.${kind}`)
}

function pipelineTypeLabel(run: MedallionPipelineRunBody) {
  return t(`medallion.pipelineType.${run.pipelineType}`)
}
</script>

<template>
  <section class="medallion-catalog-panel">
    <div class="section-header compact-section-header">
      <div>
        <span class="status-pill">{{ t('medallion.badge') }}</span>
        <h3>{{ titleText }}</h3>
      </div>
      <span v-if="asset" class="status-pill" :class="statusClass(asset.status)">
        <Layers3 :size="14" aria-hidden="true" />
        {{ layerLabel(asset) }}
      </span>
    </div>

    <p v-if="loading" class="cell-subtle">{{ t('medallion.loading') }}</p>
    <div v-else-if="!asset" class="empty-state compact-empty-state">
      <p>{{ t('medallion.empty') }}</p>
    </div>
    <template v-else>
      <div class="medallion-badge-row">
        <span class="status-pill" :class="statusClass(asset.status)">{{ asset.status }}</span>
        <span class="status-pill">{{ resourceKindLabel(asset.resourceKind) }}</span>
        <span class="status-pill">{{ t('medallion.relatedCount', { count: upstream.length + downstream.length }) }}</span>
      </div>

      <dl class="metadata-grid compact medallion-metadata-grid">
        <div>
          <dt>{{ t('common.publicId') }}</dt>
          <dd class="monospace-cell">{{ asset.publicId }}</dd>
        </div>
        <div>
          <dt>{{ t('medallion.resource') }}</dt>
          <dd>{{ resourceKindLabel(asset.resourceKind) }}</dd>
        </div>
        <div>
          <dt>{{ t('datasets.rows') }}</dt>
          <dd class="tabular-cell">{{ asset.rowCount !== undefined ? n(asset.rowCount) : '-' }}</dd>
        </div>
        <div>
          <dt>{{ t('datasets.totalBytes') }}</dt>
          <dd class="tabular-cell">{{ asset.byteSize !== undefined ? formatDriveSize(asset.byteSize) : '-' }}</dd>
        </div>
        <div>
          <dt>{{ t('datasets.columns') }}</dt>
          <dd class="tabular-cell">{{ schemaColumnCount > 0 ? n(schemaColumnCount) : '-' }}</dd>
        </div>
        <div>
          <dt>{{ t('common.updated') }}</dt>
          <dd>{{ formatDate(asset.updatedAt) }}</dd>
        </div>
      </dl>

      <div class="medallion-run-list">
        <div class="section-header compact-section-header">
          <div>
            <span class="status-pill">{{ t('medallion.pipelineHistory') }}</span>
            <h3>{{ t('medallion.latestRuns') }}</h3>
          </div>
        </div>
        <article v-for="run in latestRuns" :key="run.publicId" class="medallion-run-row">
          <GitBranch :size="16" aria-hidden="true" />
          <span>
            <strong>{{ pipelineTypeLabel(run) }}</strong>
            <small>{{ run.runtime || run.triggerKind }} · {{ formatDate(run.completedAt || run.updatedAt) }}</small>
            <small v-if="run.errorSummary" class="dataset-query-error-text">{{ run.errorSummary }}</small>
          </span>
          <span class="status-pill" :class="statusClass(run.status)">{{ run.status }}</span>
        </article>
        <p v-if="latestRuns.length === 0" class="cell-subtle">{{ t('medallion.noRuns') }}</p>
      </div>
    </template>
  </section>
</template>
