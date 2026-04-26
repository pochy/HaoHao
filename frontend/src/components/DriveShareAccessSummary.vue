<script setup lang="ts">
import { computed } from 'vue'

import type { DrivePermissionsBody } from '../api/generated/types.gen'

const props = defineProps<{
  permissions: DrivePermissionsBody | null
}>()

const directCount = computed(() => props.permissions?.direct?.length ?? 0)
const inheritedCount = computed(() => props.permissions?.inherited?.length ?? 0)
const linkCount = computed(() => props.permissions?.direct?.filter((permission) => permission.kind === 'share_link').length ?? 0)
</script>

<template>
  <div class="drive-share-summary" aria-label="Current access summary">
    <div>
      <strong>{{ directCount }}</strong>
      <span>direct</span>
    </div>
    <div>
      <strong>{{ inheritedCount }}</strong>
      <span>inherited</span>
    </div>
    <div>
      <strong>{{ linkCount }}</strong>
      <span>links</span>
    </div>
  </div>
</template>
