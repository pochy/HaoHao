<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  open: boolean
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
}>(), {
  confirmLabel: 'Confirm',
  cancelLabel: 'Cancel',
})

const emit = defineEmits<{
  cancel: []
  confirm: []
}>()

const dialogRef = ref<HTMLDialogElement | null>(null)

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

function cancel() {
  emit('cancel')
}

function confirm() {
  emit('confirm')
}

function handleClose() {
  if (props.open) {
    emit('cancel')
  }
}
</script>

<template>
  <dialog ref="dialogRef" class="confirm-dialog" @close="handleClose" @cancel.prevent="cancel">
    <div class="confirm-dialog-panel">
      <div class="stack">
        <span class="status-pill danger">Confirm</span>
        <h2>{{ title }}</h2>
        <p>{{ message }}</p>
      </div>

      <div class="action-row">
        <button class="secondary-button" type="button" autofocus @click="cancel">
          {{ cancelLabel }}
        </button>
        <button class="secondary-button danger-button" type="button" @click="confirm">
          {{ confirmLabel }}
        </button>
      </div>
    </div>
  </dialog>
</template>
