<script setup lang="ts">
import type { DriveFolderBody } from '../api/generated/types.gen'

defineProps<{
  currentFolder: DriveFolderBody
}>()
</script>

<template>
  <nav class="drive-breadcrumbs" aria-label="Drive breadcrumbs">
    <RouterLink to="/drive">Drive</RouterLink>
    <span aria-hidden="true">/</span>
    <span>{{ currentFolder.publicId === 'root' ? 'Root' : currentFolder.name }}</span>
  </nav>
</template>
