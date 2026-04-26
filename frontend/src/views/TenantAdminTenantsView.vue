<script setup lang="ts">
import { onMounted } from 'vue'

import type { TenantAdminTenantBody } from '../api/generated/types.gen'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import DataCard from '../components/DataCard.vue'
import EmptyState from '../components/EmptyState.vue'
import MetricTile from '../components/MetricTile.vue'
import PageHeader from '../components/PageHeader.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { useTenantAdminStore } from '../stores/tenant-admin'

const store = useTenantAdminStore()

onMounted(async () => {
  await store.loadList()
})

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

function formatMemberCount(item: TenantAdminTenantBody) {
  return new Intl.NumberFormat().format(item.activeMemberCount)
}
</script>

<template>
  <AdminAccessDenied
    v-if="store.status === 'forbidden'"
    title="Tenant admin role required"
    message="この画面を使うには global role tenant_admin が必要です。"
    role-label="tenant_admin"
  />

  <section v-else class="stack">
    <PageHeader
      eyebrow="Tenant Admin"
      title="Tenants"
      description="Tenant、member count、state を管理します。"
    >
      <template #actions>
      <div class="action-row">
        <button class="secondary-button" :disabled="store.status === 'loading'" type="button" @click="store.loadList()">
          {{ store.status === 'loading' ? 'Refreshing...' : 'Refresh' }}
        </button>
        <RouterLink class="primary-button link-button" to="/tenant-admin/new">
          New
        </RouterLink>
      </div>
      </template>
    </PageHeader>

    <div class="metric-grid">
      <MetricTile label="Tenants" :value="store.items.length" hint="Total records" />
      <MetricTile label="Active" :value="store.items.filter((item) => item.active).length" hint="Enabled tenants" />
      <MetricTile label="Inactive" :value="store.items.filter((item) => !item.active).length" hint="Disabled tenants" />
      <MetricTile label="Status" :value="store.status" hint="List loading state" />
    </div>

    <p v-if="store.errorMessage" class="error-message">
      {{ store.errorMessage }}
    </p>

    <p v-if="store.status === 'loading'">
      Loading tenants...
    </p>

    <DataCard v-else-if="store.items.length > 0" title="Tenant list">
    <div class="admin-table">
      <table>
        <thead>
          <tr>
            <th scope="col">Tenant</th>
            <th scope="col">Slug</th>
            <th scope="col">Members</th>
            <th scope="col">State</th>
            <th scope="col">Updated</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in store.items" :key="item.slug">
            <td>
              <RouterLink class="text-link" :to="{ name: 'tenant-admin-detail', params: { tenantSlug: item.slug } }">
                {{ item.displayName }}
              </RouterLink>
            </td>
            <td class="monospace-cell">{{ item.slug }}</td>
            <td class="tabular-cell">{{ formatMemberCount(item) }}</td>
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

    <EmptyState v-else-if="store.status === 'ready'" title="No tenants" message="Tenant はまだ登録されていません。">
      <template #actions>
        <RouterLink class="primary-button link-button" to="/tenant-admin/new">
          New Tenant
        </RouterLink>
      </template>
    </EmptyState>
  </section>
</template>
