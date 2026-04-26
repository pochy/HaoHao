<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'

import type { DriveItemBody } from '../api/generated/types.gen'
import { driveItemName } from '../utils/driveItems'

const props = defineProps<{
  open: boolean
  item: DriveItemBody | null
  busy: boolean
  errorMessage: string
}>()

const emit = defineEmits<{
  close: []
  save: [description: string, tags: string[]]
}>()

const dialogRef = ref<HTMLDialogElement | null>(null)
const description = ref('')
const tagsText = ref('')

const title = computed(() => (props.item ? driveItemName(props.item) : 'Drive item'))

watch(
  () => props.item,
  (item) => {
    description.value = item?.file?.description ?? item?.folder?.description ?? ''
    tagsText.value = (item?.tags ?? []).join(', ')
  },
  { immediate: true },
)

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
      return
    }
    if (!open && dialog.open) {
      dialog.close()
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (dialogRef.value?.open) {
    dialogRef.value.close()
  }
})

function parsedTags() {
  return tagsText.value
    .split(/[\n,]+/)
    .map((tag) => tag.trim())
    .filter(Boolean)
}

function submit() {
  emit('save', description.value, parsedTags())
}

function handleClose() {
  if (props.open) {
    emit('close')
  }
}
</script>

<template>
  <dialog ref="dialogRef" class="drive-dialog" @close="handleClose" @cancel.prevent="emit('close')">
    <form class="drive-dialog-panel stack" @submit.prevent="submit">
      <div class="section-header">
        <div>
          <span class="status-pill">Metadata</span>
          <h2>{{ title }}</h2>
        </div>
        <button class="secondary-button compact-button" type="button" @click="emit('close')">
          Close
        </button>
      </div>

      <p v-if="errorMessage" class="error-message">{{ errorMessage }}</p>

      <label class="field">
        <span class="field-label">Description</span>
        <textarea v-model="description" class="field-input drive-textarea" maxlength="4000" :disabled="busy" />
      </label>

      <label class="field">
        <span class="field-label">Tags</span>
        <input v-model="tagsText" class="field-input" autocomplete="off" :disabled="busy" placeholder="finance, roadmap, draft">
      </label>

      <div class="action-row">
        <button class="primary-button compact-button" type="submit" :disabled="busy || !item">
          Save metadata
        </button>
      </div>
    </form>
  </dialog>
</template>
