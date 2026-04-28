<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { DriveProductExtractionItemBody } from '../api/generated/types.gen'

const props = defineProps<{
  items: DriveProductExtractionItemBody[]
}>()

const { t } = useI18n()

const rows = computed(() => props.items.map((item, index) => ({
  index: index + 1,
  name: item.name,
  model: item.model || item.sku || '',
  janCode: item.janCode || '',
  brand: item.brand || item.manufacturer || '',
  category: item.category || item.itemType || '',
  price: formatPrice(item.price),
  confidence: typeof item.confidence === 'number' ? `${Math.round(item.confidence * 100)}%` : '',
  sourceText: compactText(item.sourceText),
  publicId: item.publicId,
})))

function recordValue(record: Record<string, unknown>, key: string) {
  const value = record[key]
  return typeof value === 'string' || typeof value === 'number' ? value : ''
}

function formatPrice(price: Record<string, unknown>) {
  const amount = recordValue(price, 'amount')
  const currency = recordValue(price, 'currency')
  if (amount === '') {
    return ''
  }
  if (typeof amount === 'number' && typeof currency === 'string' && currency) {
    try {
      return new Intl.NumberFormat(undefined, {
        style: 'currency',
        currency,
      }).format(amount)
    } catch {
      return `${currency} ${amount}`
    }
  }
  return currency ? `${currency} ${amount}` : String(amount)
}

function compactText(value: string) {
  const text = value.replace(/\s+/g, ' ').trim()
  return text.length > 120 ? `${text.slice(0, 120)}...` : text
}
</script>

<template>
  <div class="drive-product-table" role="region" :aria-label="t('drive.productExtractions')">
    <table>
      <thead>
        <tr>
          <th scope="col" class="tabular-cell">#</th>
          <th scope="col">{{ t('common.title') }}</th>
          <th scope="col">{{ t('drive.model') }}</th>
          <th scope="col">{{ t('drive.janCode') }}</th>
          <th scope="col">{{ t('drive.brand') }}</th>
          <th scope="col">{{ t('drive.category') }}</th>
          <th scope="col">{{ t('drive.price') }}</th>
          <th scope="col">{{ t('drive.confidence') }}</th>
          <th scope="col">{{ t('drive.sourceText') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in rows" :key="row.publicId">
          <td class="tabular-cell">{{ row.index }}</td>
          <td class="drive-product-table-name">{{ row.name }}</td>
          <td>{{ row.model || '-' }}</td>
          <td class="tabular-cell">{{ row.janCode || '-' }}</td>
          <td>{{ row.brand || '-' }}</td>
          <td>{{ row.category || '-' }}</td>
          <td class="tabular-cell">{{ row.price || '-' }}</td>
          <td class="tabular-cell">{{ row.confidence || '-' }}</td>
          <td class="drive-product-table-source">{{ row.sourceText || '-' }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
