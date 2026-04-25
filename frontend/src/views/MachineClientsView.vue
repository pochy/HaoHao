<script setup lang="ts">
import { onMounted } from 'vue'

import AdminAccessDenied from '../components/AdminAccessDenied.vue'
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

  <section v-else class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">M2M</span>
        <h2>Machine Clients</h2>
      </div>
      <div class="action-row">
        <button class="secondary-button" :disabled="store.status === 'loading'" type="button" @click="store.loadList()">
          {{ store.status === 'loading' ? 'Refreshing...' : 'Refresh' }}
        </button>
        <RouterLink class="primary-button link-button" to="/machine-clients/new">
          New
        </RouterLink>
      </div>
    </div>

    <p v-if="store.errorMessage" class="error-message">
      {{ store.errorMessage }}
    </p>

    <p v-if="store.status === 'loading'">
      Loading machine clients...
    </p>

    <div v-else-if="store.items.length > 0" class="admin-table">
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
              <span :class="['status-pill', item.active ? '' : 'danger']">
                {{ item.active ? 'Active' : 'Inactive' }}
              </span>
            </td>
            <td>{{ formatDate(item.updatedAt) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else-if="store.status === 'ready'" class="empty-state">
      <p>Machine client はまだ登録されていません。</p>
      <RouterLink class="primary-button link-button" to="/machine-clients/new">
        New Machine Client
      </RouterLink>
    </div>
  </section>
</template>
