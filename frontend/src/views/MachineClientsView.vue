<script setup lang="ts">
import { onMounted } from 'vue'

import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import DataCard from '../components/DataCard.vue'
import EmptyState from '../components/EmptyState.vue'
import MetricTile from '../components/MetricTile.vue'
import PageHeader from '../components/PageHeader.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { useMachineClientStore } from '../stores/machine-clients'
import type { MachineClientBody } from '../api/generated/types.gen'

const store = useMachineClientStore()

onMounted(async () => {
  await store.loadList()
})

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

function formatScopes(item: MachineClientBody) {
  return item.allowedScopes?.join(' ') || 'None'
}
</script>

<template>
  <AdminAccessDenied v-if="store.status === 'forbidden'" />

  <section v-else class="stack">
    <PageHeader
      eyebrow="M2M"
      title="Machine Clients"
      description="Provider client、default tenant、allowed scopes を管理します。"
    >
      <template #actions>
      <div class="action-row">
        <button class="secondary-button" :disabled="store.status === 'loading'" type="button" @click="store.loadList()">
          {{ store.status === 'loading' ? 'Refreshing...' : 'Refresh' }}
        </button>
        <RouterLink class="primary-button link-button" to="/machine-clients/new">
          New
        </RouterLink>
      </div>
      </template>
    </PageHeader>

    <div class="metric-grid">
      <MetricTile label="Clients" :value="store.items.length" hint="Total records" />
      <MetricTile label="Active" :value="store.items.filter((item) => item.active).length" hint="Enabled clients" />
      <MetricTile label="Inactive" :value="store.items.filter((item) => !item.active).length" hint="Disabled clients" />
      <MetricTile label="Status" :value="store.status" hint="List loading state" />
    </div>

    <p v-if="store.errorMessage" class="error-message">
      {{ store.errorMessage }}
    </p>

    <p v-if="store.status === 'loading'">
      Loading machine clients...
    </p>

    <DataCard v-else-if="store.items.length > 0" title="Machine client list">
    <div class="admin-table">
      <table>
        <thead>
          <tr>
            <th scope="col">Client</th>
            <th scope="col">Provider ID</th>
            <th scope="col">Default tenant</th>
            <th scope="col">Scopes</th>
            <th scope="col">State</th>
            <th scope="col">Updated</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in store.items" :key="item.id">
            <td>
              <RouterLink class="text-link" :to="{ name: 'machine-client-detail', params: { id: item.id } }">
                {{ item.displayName }}
              </RouterLink>
              <span class="cell-subtle">{{ item.provider }}</span>
            </td>
            <td class="monospace-cell">{{ item.providerClientId }}</td>
            <td>{{ item.defaultTenant?.displayName ?? 'None' }}</td>
            <td>{{ formatScopes(item) }}</td>
            <td>
              <StatusBadge :tone="item.active ? 'success' : 'danger'">
                {{ item.active ? 'Active' : 'Inactive' }}
              </StatusBadge>
            </td>
            <td>{{ formatDate(item.updatedAt) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    </DataCard>

    <EmptyState v-else-if="store.status === 'ready'" title="No machine clients" message="Machine client はまだ登録されていません。">
      <template #actions>
        <RouterLink class="primary-button link-button" to="/machine-clients/new">
          New Machine Client
        </RouterLink>
      </template>
    </EmptyState>
  </section>
</template>
