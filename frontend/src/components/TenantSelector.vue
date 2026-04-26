<script setup lang="ts">
import { computed, onMounted } from 'vue'

import { useTenantStore } from '../stores/tenants'

withDefaults(defineProps<{
  compact?: boolean
}>(), {
  compact: false,
})

const tenantStore = useTenantStore()

const selectedSlug = computed(() => tenantStore.activeTenant?.slug ?? '')
const disabled = computed(() => (
  tenantStore.status === 'loading' ||
  tenantStore.status === 'empty' ||
  Boolean(tenantStore.switchingSlug)
))

async function onChange(event: Event) {
  const target = event.target as HTMLSelectElement
  await tenantStore.select(target.value)
}

onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
})
</script>

<template>
  <div class="tenant-selector" :class="{ compact }">
    <label class="field-label" for="tenant-selector">Active tenant</label>
    <select
      id="tenant-selector"
      data-testid="tenant-selector"
      class="field-input tenant-select"
      :disabled="disabled"
      :value="selectedSlug"
      @change="onChange"
    >
      <option v-if="tenantStore.status === 'loading'" value="">
        Loading tenants
      </option>
      <option v-else-if="tenantStore.status === 'empty'" value="">
        No tenant
      </option>
      <option
        v-for="tenant in tenantStore.items"
        :key="tenant.slug"
        :value="tenant.slug"
      >
        {{ tenant.displayName }} / {{ tenant.slug }}
      </option>
    </select>
    <p v-if="tenantStore.errorMessage" class="error-message">
      {{ tenantStore.errorMessage }}
    </p>
  </div>
</template>

<style scoped>
.tenant-selector {
  display: grid;
  gap: 8px;
  min-width: 240px;
}

.tenant-select {
  min-height: 48px;
}

.tenant-selector.compact {
  position: relative;
  min-width: min(260px, 28vw);
  gap: 4px;
}

.tenant-selector.compact .field-label {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
  clip-path: inset(50%);
}

.tenant-selector.compact .tenant-select {
  min-height: 38px;
  padding: 8px 32px 8px 12px;
}

@media (max-width: 860px) {
  .tenant-selector.compact {
    min-width: 100%;
  }
}
</style>
