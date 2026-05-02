<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'

import type { DriveItemBody } from '../api/generated/types.gen'
import { driveItemContentType, driveItemIsCsv, driveItemName } from '../utils/driveItems'

const props = defineProps<{
  open: boolean
  item: DriveItemBody | null
}>()

const emit = defineEmits<{
  close: []
}>()

const dialogRef = ref<HTMLDialogElement | null>(null)
const textPreview = ref('')
const errorMessage = ref('')
const loading = ref(false)

const file = computed(() => props.item?.file ?? null)
const title = computed(() => (props.item ? driveItemName(props.item) : 'Preview'))
const contentType = computed(() => (props.item ? driveItemContentType(props.item) : ''))
const previewUrl = computed(() => (file.value ? `/api/v1/drive/files/${encodeURIComponent(file.value.publicId)}/preview` : ''))
const previewKind = computed(() => {
  const type = contentType.value
  const name = title.value.toLowerCase()
  if (props.item && driveItemIsCsv(props.item)) {
    return 'unsupported'
  }
  if (type.startsWith('image/')) {
    return 'image'
  }
  if (type === 'application/pdf' || name.endsWith('.pdf')) {
    return 'pdf'
  }
  if (type.startsWith('text/') || type.includes('json') || type.includes('xml') || name.endsWith('.md') || name.endsWith('.txt')) {
    return 'text'
  }
  return 'unsupported'
})

watch(
  () => props.open,
  async (open) => {
    await nextTick()
    const dialog = dialogRef.value
    if (!dialog) {
      return
    }
    if (open && !dialog.open) {
      dialog.showModal()
      await loadTextPreview()
      return
    }
    if (!open && dialog.open) {
      dialog.close()
    }
  },
  { immediate: true },
)

watch(
  () => props.item?.file?.publicId,
  async () => {
    if (props.open) {
      await loadTextPreview()
    }
  },
)

onBeforeUnmount(() => {
  if (dialogRef.value?.open) {
    dialogRef.value.close()
  }
})

async function loadTextPreview() {
  textPreview.value = ''
  errorMessage.value = ''
  if (!props.open || previewKind.value !== 'text' || !previewUrl.value) {
    return
  }
  loading.value = true
  try {
    const response = await fetch(previewUrl.value, {
      credentials: 'include',
      headers: { Accept: 'text/plain, application/json, text/*' },
    })
    if (!response.ok) {
      throw new Error(response.statusText || 'Preview failed')
    }
    textPreview.value = await response.text()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Preview failed'
  } finally {
    loading.value = false
  }
}

function handleClose() {
  if (props.open) {
    emit('close')
  }
}
</script>

<template>
  <dialog ref="dialogRef" class="drive-dialog drive-preview-dialog" @close="handleClose" @cancel.prevent="emit('close')">
    <div class="drive-dialog-panel stack">
      <div class="section-header">
        <div>
          <span class="status-pill">Preview</span>
          <h2>{{ title }}</h2>
        </div>
        <button class="secondary-button compact-button" type="button" @click="emit('close')">
          Close
        </button>
      </div>

      <p v-if="errorMessage" class="error-message">{{ errorMessage }}</p>
      <img v-if="previewKind === 'image'" class="drive-preview-image" :src="previewUrl" :alt="title">
      <iframe v-else-if="previewKind === 'pdf'" class="drive-preview-frame" :src="previewUrl" :title="title" />
      <pre v-else-if="previewKind === 'text'" class="drive-preview-text">{{ loading ? 'Loading preview...' : textPreview }}</pre>
      <p v-else class="cell-subtle">
        Preview is not available for this file type.
      </p>
    </div>
  </dialog>
</template>
