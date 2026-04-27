<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { useTenantAdminDetailContext } from '../../tenant-admin/detail-context'

const { t } = useI18n()
const {
  askRevoke,
  canGrantRole,
  grantRole,
  grantRoleCode,
  grantUserEmail,
  memberships,
  roleSourceClass,
  store,
  tenantRoleOptions,
  userLabel,
} = useTenantAdminDetailContext()
</script>

<template>
  <section class="panel stack">
    <form class="admin-form" @submit.prevent="grantRole">
      <div class="form-span">
        <span class="status-pill">{{ t('tenantAdmin.sections.members') }}</span>
        <h2>{{ t('tenantAdmin.headings.grantTenantRole') }}</h2>
      </div>

      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.userEmail') }}</span>
        <input v-model="grantUserEmail" class="field-input" autocomplete="email" type="email" required>
      </label>

      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.role') }}</span>
        <select v-model="grantRoleCode" class="field-input">
          <option v-for="role in tenantRoleOptions" :key="role" :value="role">
            {{ role }}
          </option>
        </select>
      </label>

      <div class="action-row form-span">
        <button class="primary-button" :disabled="!canGrantRole" type="submit">
          {{ store.saving ? t('common.saving') : t('tenantAdmin.actions.grant') }}
        </button>
      </div>
    </form>
  </section>

  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.labels.roles') }}</span>
        <h2>{{ t('tenantAdmin.sections.members') }}</h2>
      </div>
    </div>

    <div v-if="memberships.length > 0" class="admin-table">
      <table>
        <thead>
          <tr>
            <th scope="col">{{ t('common.user') }}</th>
            <th scope="col">{{ t('tenantAdmin.fields.role') }}</th>
            <th scope="col">{{ t('tenantAdmin.fields.source') }}</th>
            <th scope="col">{{ t('tenantAdmin.fields.state') }}</th>
            <th scope="col">{{ t('tenantAdmin.fields.action') }}</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="member in memberships" :key="member.userPublicId">
            <tr v-for="role in member.roles ?? []" :key="`${member.userPublicId}:${role.roleCode}:${role.source}`">
              <td>
                {{ userLabel(member) }}
                <span class="cell-subtle">{{ member.userPublicId }}</span>
                <span v-if="member.deactivated" class="cell-subtle">{{ t('tenantAdmin.status.userInactive') }}</span>
              </td>
              <td class="monospace-cell">{{ role.roleCode }}</td>
              <td>
                <span :class="roleSourceClass(role)">
                  {{ role.source }}
                </span>
                <span v-if="role.source !== 'local_override'" class="cell-subtle">
                  {{ t('tenantAdmin.status.managedBy', { source: role.source }) }}
                </span>
              </td>
              <td>
                <span :class="['status-pill', role.active ? '' : 'danger']">
                  {{ role.active ? t('common.active') : t('tenantAdmin.status.inactive') }}
                </span>
              </td>
              <td>
                <button
                  class="secondary-button danger-button compact-button"
                  type="button"
                  :disabled="role.source !== 'local_override' || !role.active || store.saving"
                  @click="askRevoke(member, role)"
                >
                  {{ t('tenantAdmin.actions.revoke') }}
                </button>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <div v-else class="empty-state">
      <p>{{ t('tenantAdmin.empty.memberships') }}</p>
    </div>
  </section>
</template>
