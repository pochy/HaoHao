<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

import type { TenantAdminTenantBody } from '../api/generated/types.gen'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import DataCard from '../components/DataCard.vue'
import EmptyState from '../components/EmptyState.vue'
import MetricTile from '../components/MetricTile.vue'
import PageHeader from '../components/PageHeader.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { useTenantAdminStore } from '../stores/tenant-admin'

const store = useTenantAdminStore()
const { d, n, t } = useI18n()

onMounted(async () => {
  await store.loadList()
})

function formatDate(value: string) {
  return d(new Date(value), 'long')
}

function formatMemberCount(item: TenantAdminTenantBody) {
  return n(item.activeMemberCount, 'integer')
}
</script>

<template>
  <AdminAccessDenied
    v-if="store.status === 'forbidden'"
    :title="t('tenantAdmin.accessRequiredTitle')"
    :message="t('tenantAdmin.accessRequiredMessage')"
    role-label="tenant_admin"
  />

  <section v-else class="stack">
    <PageHeader
      :eyebrow="t('tenantAdmin.eyebrow')"
      :title="t('tenantAdmin.list.title')"
      :description="t('tenantAdmin.list.description')"
    >
      <template #actions>
        <div class="action-row">
          <button class="secondary-button" :disabled="store.status === 'loading'" type="button" @click="store.loadList()">
            {{ store.status === 'loading' ? t('common.refreshing') : t('common.refresh') }}
          </button>
          <RouterLink class="primary-button link-button" to="/tenant-admin/new">
            {{ t('tenantAdmin.actions.new') }}
          </RouterLink>
        </div>
      </template>
    </PageHeader>

    <div class="metric-grid">
      <MetricTile :label="t('tenantAdmin.metrics.tenants')" :value="store.items.length" :hint="t('tenantAdmin.metrics.totalRecords')" />
      <MetricTile :label="t('common.active')" :value="store.items.filter((item) => item.active).length" :hint="t('tenantAdmin.metrics.enabledTenants')" />
      <MetricTile :label="t('tenantAdmin.status.inactive')" :value="store.items.filter((item) => !item.active).length" :hint="t('tenantAdmin.metrics.disabledTenants')" />
      <MetricTile :label="t('common.status')" :value="store.status" :hint="t('tenantAdmin.metrics.listLoadingState')" />
    </div>

    <p v-if="store.errorMessage" class="error-message">
      {{ store.errorMessage }}
    </p>

    <p v-if="store.status === 'loading'">
      {{ t('tenantAdmin.list.loading') }}
    </p>

    <DataCard v-else-if="store.items.length > 0" :title="t('tenantAdmin.list.cardTitle')">
      <div class="admin-table">
        <table>
          <thead>
            <tr>
              <th scope="col">{{ t('tenantAdmin.fields.tenant') }}</th>
              <th scope="col">{{ t('tenantAdmin.fields.slug') }}</th>
              <th scope="col">{{ t('tenantAdmin.fields.members') }}</th>
              <th scope="col">{{ t('tenantAdmin.fields.state') }}</th>
              <th scope="col">{{ t('common.updated') }}</th>
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
                  {{ item.active ? t('common.active') : t('tenantAdmin.status.inactive') }}
                </StatusBadge>
              </td>
              <td>{{ formatDate(item.updatedAt) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </DataCard>

    <EmptyState v-else-if="store.status === 'ready'" :title="t('tenantAdmin.list.emptyTitle')" :message="t('tenantAdmin.list.emptyMessage')">
      <template #actions>
        <RouterLink class="primary-button link-button" to="/tenant-admin/new">
          {{ t('tenantAdmin.actions.newTenant') }}
        </RouterLink>
      </template>
    </EmptyState>
  </section>
</template>
