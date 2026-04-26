<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  open: boolean
  title: string
  label: string
  message?: string
  initialValue?: string
  placeholder?: string
  confirmLabel?: string
  cancelLabel?: string
  allowEmpty?: boolean
}>(), {
  message: '',
  initialValue: '',
  placeholder: '',
  confirmLabel: 'Save',
  cancelLabel: 'Cancel',
  allowEmpty: false,
})

const emit = defineEmits<{
  cancel: []
  confirm: [value: string]
}>()

const dialogRef = ref<HTMLDialogElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)
const value = ref('')

watch(
  () => props.open,
  async (open) => {
    await nextTick()
    const dialog = dialogRef.value
    if (!dialog) {
      return
    }

    if (open && !dialog.open) {
      value.value = props.initialValue
      dialog.showModal()
      await nextTick()
      inputRef.value?.focus()
      inputRef.value?.select()
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
  if (!props.allowEmpty && value.value.trim() === '') {
    return
  }
  emit('confirm', value.value)
}

function handleClose() {
  if (props.open) {
    emit('cancel')
  }
}
</script>

<template>
  <dialog ref="dialogRef" class="confirm-dialog" @close="handleClose" @cancel.prevent="cancel">
    <form class="confirm-dialog-panel" @submit.prevent="confirm">
      <div class="stack">
        <span class="status-pill">Input</span>
        <h2>{{ title }}</h2>
        <p v-if="message">{{ message }}</p>
        <label class="field">
          <span class="field-label">{{ label }}</span>
          <input
            ref="inputRef"
            v-model="value"
            class="field-input"
            autocomplete="off"
            :placeholder="placeholder"
          >
        </label>
      </div>

      <div class="action-row">
        <button class="secondary-button" type="button" @click="cancel">
          {{ cancelLabel }}
        </button>
        <button class="primary-button" type="submit" :disabled="!allowEmpty && value.trim() === ''">
          {{ confirmLabel }}
        </button>
      </div>
    </form>
  </dialog>
</template>
