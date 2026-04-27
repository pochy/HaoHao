<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { useTenantAdminDetailContext } from '../../tenant-admin/detail-context'

const { t } = useI18n()
const {
  approveDriveInvitation,
  formatDate,
  rejectDriveInvitation,
  repairDriveSync,
  store,
  tenant,
} = useTenantAdminDetailContext()
</script>

<template>
  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.labels.driveAdmin') }}</span>
        <h2>{{ t('tenantAdmin.headings.operationsHealth') }}</h2>
      </div>
      <button
        v-if="tenant"
        class="secondary-button compact-button"
        type="button"
        :disabled="store.saving"
        @click="store.loadDriveState(tenant.slug)"
      >
        {{ t('tenantAdmin.actions.reload') }}
      </button>
    </div>

    <div v-if="store.driveHealth" class="metadata-grid">
      <div>
        <dt>{{ t('tenantAdmin.fields.workspaces') }}</dt>
        <dd>{{ store.driveHealth.workspaceCount }}</dd>
      </div>
      <div>
        <dt>{{ t('tenantAdmin.fields.missingWorkspaceBindings') }}</dt>
        <dd>{{ store.driveHealth.missingWorkspaceCount }}</dd>
      </div>
      <div>
        <dt>{{ t('tenantAdmin.fields.openFgaDrift') }}</dt>
        <dd>{{ store.driveHealth.openfgaDriftCount }}</dd>
      </div>
      <div>
        <dt>{{ t('tenantAdmin.fields.storageMissingSample') }}</dt>
        <dd>{{ store.driveHealth.storageMissingCount }}</dd>
      </div>
    </div>

    <div v-else class="empty-state">
      <p>{{ t('tenantAdmin.empty.driveHealth') }}</p>
    </div>
  </section>

  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.labels.driveAdmin') }}</span>
        <h2>{{ t('tenantAdmin.headings.shareApprovals') }}</h2>
      </div>
    </div>

    <div v-if="store.driveApprovals.length > 0" class="list-stack">
      <article v-for="item in store.driveApprovals" :key="item.publicId" class="list-item">
        <div>
          <strong>{{ item.resourceType }} / {{ item.role }} / {{ item.status }}</strong>
          <span class="cell-subtle">{{ item.maskedInviteeEmail || item.inviteeEmailDomain }}</span>
          <span class="cell-subtle">{{ t('tenantAdmin.labels.expiresAt', { date: formatDate(item.expiresAt) }) }}</span>
        </div>
        <div class="action-row">
          <button class="primary-button compact-button" type="button" :disabled="store.saving" @click="approveDriveInvitation(item.publicId)">
            {{ t('tenantAdmin.actions.approve') }}
          </button>
          <button class="secondary-button danger-button compact-button" type="button" :disabled="store.saving" @click="rejectDriveInvitation(item.publicId)">
            {{ t('tenantAdmin.actions.reject') }}
          </button>
        </div>
      </article>
    </div>

    <div v-else class="empty-state">
      <p>{{ t('tenantAdmin.empty.driveApprovals') }}</p>
    </div>
  </section>

  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.labels.driveAdmin') }}</span>
        <h2>{{ t('tenantAdmin.headings.shareState') }}</h2>
      </div>
    </div>

    <div class="admin-table">
      <table>
        <thead>
          <tr>
            <th scope="col">{{ t('tenantAdmin.fields.resource') }}</th>
            <th scope="col">{{ t('tenantAdmin.fields.subject') }}</th>
            <th scope="col">{{ t('tenantAdmin.fields.role') }}</th>
            <th scope="col">{{ t('common.status') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in store.driveShares" :key="item.publicId">
            <td>{{ item.resourceName || item.resourcePublicId }}<span class="cell-subtle">{{ item.resourceType }}</span></td>
            <td>{{ item.subjectPublicId }}<span class="cell-subtle">{{ item.subjectType }}</span></td>
            <td>{{ item.role }}</td>
            <td>{{ item.status }}</td>
          </tr>
          <tr v-if="store.driveShares.length === 0">
            <td colspan="4">{{ t('tenantAdmin.empty.driveShares') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.labels.driveAdmin') }}</span>
        <h2>{{ t('tenantAdmin.headings.shareLinks') }}</h2>
      </div>
    </div>

    <div class="admin-table">
      <table>
        <thead>
          <tr>
            <th scope="col">{{ t('tenantAdmin.fields.link') }}</th>
            <th scope="col">{{ t('tenantAdmin.fields.resource') }}</th>
            <th scope="col">{{ t('common.download') }}</th>
            <th scope="col">{{ t('common.status') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in store.driveShareLinks" :key="item.publicId">
            <td>{{ item.publicId }}<span class="cell-subtle">{{ item.passwordRequired ? t('tenantAdmin.status.passwordRequired') : t('tenantAdmin.status.passwordNotRequired') }}</span></td>
            <td>{{ item.resourceName || item.resourcePublicId }}</td>
            <td>{{ item.canDownload ? t('tenantAdmin.status.allowed') : t('tenantAdmin.status.blocked') }}</td>
            <td>{{ item.status }}<span class="cell-subtle">{{ t('tenantAdmin.labels.expiresAt', { date: formatDate(item.expiresAt) }) }}</span></td>
          </tr>
          <tr v-if="store.driveShareLinks.length === 0">
            <td colspan="4">{{ t('tenantAdmin.empty.driveShareLinks') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">OpenFGA</span>
        <h2>{{ t('tenantAdmin.headings.syncStatus') }}</h2>
      </div>
      <button class="secondary-button compact-button" type="button" :disabled="store.saving" @click="repairDriveSync">
        {{ t('tenantAdmin.actions.repair') }}
      </button>
    </div>

    <div v-if="(store.driveSync?.items ?? []).length > 0" class="list-stack">
      <article v-for="item in store.driveSync?.items ?? []" :key="`${item.kind}:${item.publicId}:${item.action}`" class="list-item">
        <div>
          <strong>{{ item.kind }} / {{ item.action }}</strong>
          <span class="cell-subtle">{{ item.publicId }} / {{ item.status }}</span>
          <span v-if="item.error" class="cell-subtle">{{ item.error }}</span>
        </div>
      </article>
    </div>

    <div v-else class="empty-state">
      <p>{{ t('tenantAdmin.empty.openFgaDrift') }}</p>
    </div>
  </section>

  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.labels.driveAudit') }}</span>
        <h2>{{ t('tenantAdmin.headings.recentAuditEvents') }}</h2>
      </div>
    </div>

    <div v-if="store.driveAuditEvents.length > 0" class="list-stack">
      <article v-for="item in store.driveAuditEvents" :key="item.publicId" class="list-item">
        <div>
          <strong>{{ item.action }}</strong>
          <span class="cell-subtle">{{ item.targetType }} / {{ item.targetId }}</span>
          <span class="cell-subtle">{{ formatDate(item.occurredAt) }}</span>
        </div>
      </article>
    </div>

    <div v-else class="empty-state">
      <p>{{ t('tenantAdmin.empty.driveAuditEvents') }}</p>
    </div>
  </section>
</template>
