<script setup lang="ts">
import { computed, onMounted } from 'vue'

import { useTenantStore } from '../stores/tenants'

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
  <div class="tenant-selector">
    <label class="field-label" for="tenant-selector">Active tenant</label>
    <select
      id="tenant-selector"
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
</style>
