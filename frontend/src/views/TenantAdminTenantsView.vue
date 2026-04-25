<script setup lang="ts">
import { onMounted } from 'vue'

import type { TenantAdminTenantBody } from '../api/generated/types.gen'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
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

  <section v-else class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">Tenant Admin</span>
        <h2>Tenants</h2>
      </div>
      <div class="action-row">
        <button class="secondary-button" :disabled="store.status === 'loading'" type="button" @click="store.loadList()">
          {{ store.status === 'loading' ? 'Refreshing...' : 'Refresh' }}
        </button>
        <RouterLink class="primary-button link-button" to="/tenant-admin/new">
          New
        </RouterLink>
      </div>
    </div>

    <p v-if="store.errorMessage" class="error-message">
      {{ store.errorMessage }}
    </p>

    <p v-if="store.status === 'loading'">
      Loading tenants...
    </p>

    <div v-else-if="store.items.length > 0" class="admin-table">
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
      <p>Tenant はまだ登録されていません。</p>
      <RouterLink class="primary-button link-button" to="/tenant-admin/new">
        New Tenant
      </RouterLink>
    </div>
  </section>
</template>
