<script setup lang="ts">
import { ref } from 'vue'

defineProps<{
  busy: boolean
  disabled: boolean
}>()

const emit = defineEmits<{
  createFolder: [name: string]
  uploadFile: [file: File]
  search: [query: string]
  refresh: []
}>()

const folderName = ref('')
const searchQuery = ref('')

function submitFolder() {
  const name = folderName.value.trim()
  if (!name) {
    return
  }
  emit('createFolder', name)
  folderName.value = ''
}

function submitSearch() {
  emit('search', searchQuery.value.trim())
}

function onFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (file) {
    emit('uploadFile', file)
  }
  target.value = ''
}
</script>

<template>
  <div class="drive-toolbar">
    <form class="drive-inline-form" @submit.prevent="submitFolder">
      <label class="field drive-toolbar-field">
        <span class="field-label">New folder</span>
        <input
          v-model="folderName"
          class="field-input"
          maxlength="255"
          autocomplete="off"
          placeholder="Project files"
          :disabled="disabled || busy"
        >
      </label>
      <button class="primary-button compact-button" type="submit" :disabled="disabled || busy || folderName.trim() === ''">
        Create
      </button>
    </form>

    <label class="secondary-button compact-button drive-upload-button">
      <span>Upload</span>
      <input class="drive-hidden-input" type="file" :disabled="disabled || busy" @change="onFileChange">
    </label>

    <form class="drive-inline-form drive-search-form" @submit.prevent="submitSearch">
      <label class="field drive-toolbar-field">
        <span class="field-label">Search</span>
        <input
          v-model="searchQuery"
          class="field-input"
          autocomplete="off"
          placeholder="Filename"
          :disabled="disabled || busy"
        >
      </label>
      <button class="secondary-button compact-button" type="submit" :disabled="disabled || busy">
        Search
      </button>
    </form>

    <button class="secondary-button compact-button" type="button" :disabled="disabled || busy" @click="emit('refresh')">
      Refresh
    </button>
  </div>
</template>
