<script setup lang="ts">
import {
  Archive,
  File,
  FileImage,
  FileText,
  Folder,
} from 'lucide-vue-next'

withDefaults(defineProps<{
  kind: string
  size?: number
}>(), {
  size: 18,
})
</script>

<template>
  <span class="drive-file-type-icon" :class="kind">
    <Folder v-if="kind === 'folder'" :size="size" stroke-width="1.9" aria-hidden="true" />
    <FileImage v-else-if="kind === 'image'" :size="size" stroke-width="1.9" aria-hidden="true" />
    <Archive v-else-if="kind === 'archive'" :size="size" stroke-width="1.9" aria-hidden="true" />
    <FileText v-else-if="kind === 'document'" :size="size" stroke-width="1.9" aria-hidden="true" />
    <File v-else :size="size" stroke-width="1.9" aria-hidden="true" />
  </span>
</template>
