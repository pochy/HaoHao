<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { DriveOcrPageBody } from '../api/generated/types.gen'

const props = defineProps<{
  pages: DriveOcrPageBody[]
  loading?: boolean
}>()

const { t } = useI18n()
const dialogRef = ref<HTMLDialogElement | null>(null)
const pageIndex = ref(0)

const pageCount = computed(() => props.pages.length)
const currentPage = computed(() => props.pages[pageIndex.value] ?? null)
const currentPageLabel = computed(() => (currentPage.value ? t('drive.ocrPage', { page: currentPage.value.pageNumber }) : t('drive.ocrText')))
const currentText = computed(() => currentPage.value?.rawText?.trim() || t('drive.ocrNoText'))

watch(
  () => props.pages.length,
  (length) => {
    if (length === 0) {
      pageIndex.value = 0
      return
    }
    pageIndex.value = Math.min(pageIndex.value, length - 1)
  },
)

onBeforeUnmount(() => {
  if (dialogRef.value?.open) {
    dialogRef.value.close()
  }
})

function previousPage() {
  pageIndex.value = Math.max(0, pageIndex.value - 1)
}

function nextPage() {
  pageIndex.value = Math.min(pageCount.value - 1, pageIndex.value + 1)
}

async function openDialog() {
  await nextTick()
  const dialog = dialogRef.value
  if (dialog && !dialog.open) {
    dialog.showModal()
  }
}

function closeDialog() {
  dialogRef.value?.close()
}
</script>

<template>
  <div class="drive-ocr-text-viewer">
    <p v-if="loading" class="cell-subtle">{{ t('common.loading') }}</p>
    <p v-else-if="pageCount === 0" class="cell-subtle">{{ t('drive.ocrNoText') }}</p>

    <template v-else>
      <div class="drive-ocr-text-toolbar">
        <div>
          <strong>{{ currentPageLabel }}</strong>
          <span class="tabular-cell">{{ t('drive.ocrPageCounter', { current: pageIndex + 1, total: pageCount }) }}</span>
        </div>
        <div class="drive-ocr-text-controls">
          <button class="secondary-button compact-button" type="button" :disabled="pageIndex === 0" @click="previousPage">
            {{ t('drive.ocrPreviousPage') }}
          </button>
          <label class="drive-ocr-page-select">
            <span class="sr-only">{{ t('drive.ocrPageSelector') }}</span>
            <select v-model.number="pageIndex" class="field-input">
              <option v-for="(page, index) in pages" :key="page.pageNumber" :value="index">
                {{ t('drive.ocrPage', { page: page.pageNumber }) }}
              </option>
            </select>
          </label>
          <button class="secondary-button compact-button" type="button" :disabled="pageIndex >= pageCount - 1" @click="nextPage">
            {{ t('drive.ocrNextPage') }}
          </button>
          <button class="primary-button compact-button" type="button" @click="openDialog">
            {{ t('drive.ocrOpenFullText') }}
          </button>
        </div>
      </div>

      <pre class="drive-ocr-text-page">{{ currentText }}</pre>

      <dialog ref="dialogRef" class="drive-dialog drive-ocr-text-dialog" @cancel.prevent="closeDialog">
        <div class="drive-dialog-panel drive-ocr-text-dialog-panel">
          <div class="section-header">
            <div>
              <span class="status-pill">{{ t('drive.ocrText') }}</span>
              <h2>{{ currentPageLabel }}</h2>
              <p class="cell-subtle tabular-cell">{{ t('drive.ocrPageCounter', { current: pageIndex + 1, total: pageCount }) }}</p>
            </div>
            <button class="secondary-button compact-button" type="button" @click="closeDialog">
              {{ t('drive.ocrCloseFullText') }}
            </button>
          </div>

          <div class="drive-ocr-text-controls">
            <button class="secondary-button compact-button" type="button" :disabled="pageIndex === 0" @click="previousPage">
              {{ t('drive.ocrPreviousPage') }}
            </button>
            <label class="drive-ocr-page-select">
              <span class="sr-only">{{ t('drive.ocrPageSelector') }}</span>
              <select v-model.number="pageIndex" class="field-input">
                <option v-for="(page, index) in pages" :key="page.pageNumber" :value="index">
                  {{ t('drive.ocrPage', { page: page.pageNumber }) }}
                </option>
              </select>
            </label>
            <button class="secondary-button compact-button" type="button" :disabled="pageIndex >= pageCount - 1" @click="nextPage">
              {{ t('drive.ocrNextPage') }}
            </button>
          </div>

          <pre class="drive-ocr-text-page full">{{ currentText }}</pre>
        </div>
      </dialog>
    </template>
  </div>
</template>
