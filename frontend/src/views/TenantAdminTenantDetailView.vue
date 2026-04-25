<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { toApiErrorMessage } from '../api/client'
import type { TenantAdminMembershipBody, TenantAdminRoleBindingBody } from '../api/generated/types.gen'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import ConfirmActionDialog from '../components/ConfirmActionDialog.vue'
import { useTenantAdminStore } from '../stores/tenant-admin'
import { useTenantCommonStore } from '../stores/tenant-common'

type PendingAction =
  | { kind: 'deactivate' }
  | { kind: 'revoke', userPublicId: string, userLabel: string, roleCode: string }

const route = useRoute()
const store = useTenantAdminStore()
const commonStore = useTenantCommonStore()

const displayName = ref('')
const active = ref(true)
const grantUserEmail = ref('')
const grantRoleCode = ref('customer_signal_user')
const invitationEmail = ref('')
const invitationRoleCode = ref('todo_user')
const fileQuotaBytes = ref(104857600)
const browserRateLimit = ref<number | null>(null)
const notificationsEnabled = ref(true)
const message = ref('')
const errorMessage = ref('')
const pendingAction = ref<PendingAction | null>(null)

const tenantSlug = computed(() => {
  const raw = Array.isArray(route.params.tenantSlug)
    ? route.params.tenantSlug[0]
    : route.params.tenantSlug
  return raw ?? ''
})

const tenant = computed(() => store.current?.tenant ?? null)
const memberships = computed(() => store.current?.memberships ?? [])
const tenantRoleOptions = ['customer_signal_user', 'docs_reader', 'todo_user']

const canSaveSettings = computed(() => (
  Boolean(tenant.value) &&
  displayName.value.trim() !== '' &&
  !store.saving
))

const canGrantRole = computed(() => (
  Boolean(tenant.value) &&
  grantUserEmail.value.trim() !== '' &&
  grantRoleCode.value.trim() !== '' &&
  !store.saving
))

const canInvite = computed(() => (
  Boolean(tenant.value) &&
  invitationEmail.value.trim() !== '' &&
  invitationRoleCode.value.trim() !== '' &&
  !commonStore.saving
))

const canSaveCommonSettings = computed(() => (
  Boolean(tenant.value) &&
  fileQuotaBytes.value >= 0 &&
  !commonStore.saving
))

const confirmTitle = computed(() => {
  if (pendingAction.value?.kind === 'revoke') {
    return 'Revoke tenant role'
  }
  return 'Deactivate tenant'
})

const confirmMessage = computed(() => {
  if (pendingAction.value?.kind === 'revoke') {
    return `${pendingAction.value.userLabel} から ${pendingAction.value.roleCode} local role を無効化します。provider_claim / scim 由来の role は変更されません。`
  }
  return `${tenant.value?.slug ?? tenantSlug.value} を inactive にします。tenant selector からは外れますが、既存データと audit event は残ります。`
})

const confirmLabel = computed(() => (
  pendingAction.value?.kind === 'revoke' ? 'Revoke' : 'Deactivate'
))

onMounted(async () => {
  await loadCurrent()
})

watch(
  () => route.params.tenantSlug,
  async () => {
    await loadCurrent()
  },
)

watch(
  () => store.current?.tenant,
  () => syncForm(),
)

watch(
  () => commonStore.settings,
  () => syncCommonForm(),
)

async function loadCurrent() {
  message.value = ''
  errorMessage.value = ''
  if (!tenantSlug.value) {
    errorMessage.value = 'Invalid tenant slug.'
    return
  }
  await store.loadOne(tenantSlug.value)
  await commonStore.load(tenantSlug.value)
  syncForm()
  syncCommonForm()
}

function syncForm() {
  if (!store.current?.tenant) {
    displayName.value = ''
    active.value = true
    return
  }

  displayName.value = store.current.tenant.displayName
  active.value = store.current.tenant.active
}

function syncCommonForm() {
  if (!commonStore.settings) {
    fileQuotaBytes.value = 104857600
    browserRateLimit.value = null
    notificationsEnabled.value = true
    return
  }
  fileQuotaBytes.value = commonStore.settings.fileQuotaBytes
  browserRateLimit.value = commonStore.settings.rateLimitBrowserApiPerMinute ?? null
  notificationsEnabled.value = commonStore.settings.notificationsEnabled
}

function formatDate(value?: string) {
  if (!value) {
    return 'Never'
  }

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

function userLabel(member: TenantAdminMembershipBody) {
  return member.displayName ? `${member.displayName} / ${member.email}` : member.email
}

function roleSourceClass(role: TenantAdminRoleBindingBody) {
  return ['source-chip', role.source === 'local_override' ? 'local' : '', role.active ? '' : 'inactive']
}

async function saveSettings() {
  if (!tenant.value || !canSaveSettings.value) {
    return
  }

  message.value = ''
  errorMessage.value = ''

  try {
    await store.update(tenant.value.slug, {
      displayName: displayName.value.trim(),
      active: active.value,
    })
    message.value = 'Tenant settings を更新しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function grantRole() {
  if (!tenant.value || !canGrantRole.value) {
    return
  }

  message.value = ''
  errorMessage.value = ''

  try {
    await store.grantRole(tenant.value.slug, {
      userEmail: grantUserEmail.value.trim(),
      roleCode: grantRoleCode.value,
    })
    grantUserEmail.value = ''
    message.value = 'Tenant role を付与しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function createInvitation() {
  if (!tenant.value || !canInvite.value) {
    return
  }

  message.value = ''
  errorMessage.value = ''

  try {
    const created = await commonStore.createInvitation(tenant.value.slug, {
      email: invitationEmail.value.trim(),
      roleCodes: [invitationRoleCode.value],
    })
    invitationEmail.value = ''
    message.value = created.acceptUrl
      ? `Invitation を作成しました: ${created.acceptUrl}`
      : 'Invitation を作成しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function revokeInvitation(publicId: string) {
  if (!tenant.value) {
    return
  }

  message.value = ''
  errorMessage.value = ''

  try {
    await commonStore.revokeInvitation(tenant.value.slug, publicId)
    message.value = 'Invitation を revoke しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function saveCommonSettings() {
  if (!tenant.value || !canSaveCommonSettings.value) {
    return
  }

  message.value = ''
  errorMessage.value = ''

  try {
    await commonStore.updateSettings(tenant.value.slug, {
      fileQuotaBytes: fileQuotaBytes.value,
      rateLimitBrowserApiPerMinute: browserRateLimit.value ?? undefined,
      notificationsEnabled: notificationsEnabled.value,
      features: commonStore.settings?.features ?? {},
    })
    message.value = 'Tenant common settings を更新しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function requestExport() {
  if (!tenant.value) {
    return
  }

  message.value = ''
  errorMessage.value = ''

  try {
    await commonStore.requestExport(tenant.value.slug)
    message.value = 'Tenant data export を request しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

function askDeactivate() {
  pendingAction.value = { kind: 'deactivate' }
}

function askRevoke(member: TenantAdminMembershipBody, role: TenantAdminRoleBindingBody) {
  pendingAction.value = {
    kind: 'revoke',
    userPublicId: member.userPublicId,
    userLabel: userLabel(member),
    roleCode: role.roleCode,
  }
}

function cancelPendingAction() {
  pendingAction.value = null
}

async function confirmPendingAction() {
  if (!tenant.value || !pendingAction.value) {
    return
  }

  const action = pendingAction.value
  pendingAction.value = null
  message.value = ''
  errorMessage.value = ''

  try {
    if (action.kind === 'deactivate') {
      await store.deactivate(tenant.value.slug)
      active.value = false
      message.value = 'Tenant を inactive にしました。'
      return
    }

    await store.revokeRole(tenant.value.slug, action.userPublicId, action.roleCode)
    message.value = 'Tenant local role を無効化しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
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
        <h2>Tenant Detail</h2>
      </div>
      <RouterLink class="secondary-button link-button" to="/tenant-admin">
        Back
      </RouterLink>
    </div>

    <p v-if="store.status === 'loading'">
      Loading tenant...
    </p>
    <p v-if="errorMessage || store.errorMessage || commonStore.errorMessage" class="error-message">
      {{ errorMessage || store.errorMessage || commonStore.errorMessage }}
    </p>
    <p v-if="message" class="notice-message">
      {{ message }}
    </p>

    <template v-if="tenant">
      <form class="admin-form" @submit.prevent="saveSettings">
        <label class="field">
          <span class="field-label">Slug</span>
          <input :value="tenant.slug" class="field-input" autocomplete="off" disabled>
        </label>

        <label class="field">
          <span class="field-label">Display name</span>
          <input v-model="displayName" class="field-input" autocomplete="off" required>
        </label>

        <label class="checkbox-field form-span">
          <input v-model="active" type="checkbox">
          <span>Active</span>
        </label>

        <dl class="metadata-grid form-span">
          <div>
            <dt>Active members</dt>
            <dd>{{ tenant.activeMemberCount }}</dd>
          </div>
          <div>
            <dt>Updated</dt>
            <dd>{{ formatDate(tenant.updatedAt) }}</dd>
          </div>
        </dl>

        <div class="action-row form-span">
          <button class="primary-button" :disabled="!canSaveSettings" type="submit">
            {{ store.saving ? 'Saving...' : 'Save' }}
          </button>
          <button
            class="secondary-button danger-button"
            :disabled="store.saving || !tenant.active"
            type="button"
            @click="askDeactivate"
          >
            Deactivate
          </button>
        </div>
      </form>

      <form class="admin-form" @submit.prevent="grantRole">
        <div class="form-span">
          <span class="status-pill">Membership</span>
          <h2>Grant Tenant Role</h2>
        </div>

        <label class="field">
          <span class="field-label">User email</span>
          <input v-model="grantUserEmail" class="field-input" autocomplete="email" type="email" required>
        </label>

        <label class="field">
          <span class="field-label">Role</span>
          <select v-model="grantRoleCode" class="field-input">
            <option v-for="role in tenantRoleOptions" :key="role" :value="role">
              {{ role }}
            </option>
          </select>
        </label>

        <div class="action-row form-span">
          <button class="primary-button" :disabled="!canGrantRole" type="submit">
            {{ store.saving ? 'Saving...' : 'Grant' }}
          </button>
        </div>
      </form>

      <div class="stack">
        <div class="section-header">
          <div>
            <span class="status-pill">Roles</span>
            <h2>Memberships</h2>
          </div>
        </div>

        <div v-if="memberships.length > 0" class="admin-table">
          <table>
            <thead>
              <tr>
                <th scope="col">User</th>
                <th scope="col">Role</th>
                <th scope="col">Source</th>
                <th scope="col">State</th>
                <th scope="col">Action</th>
              </tr>
            </thead>
            <tbody>
              <template v-for="member in memberships" :key="member.userPublicId">
                <tr v-for="role in member.roles ?? []" :key="`${member.userPublicId}:${role.roleCode}:${role.source}`">
                  <td>
                    {{ userLabel(member) }}
                    <span class="cell-subtle">{{ member.userPublicId }}</span>
                    <span v-if="member.deactivated" class="cell-subtle">User inactive</span>
                  </td>
                  <td class="monospace-cell">{{ role.roleCode }}</td>
                  <td>
                    <span :class="roleSourceClass(role)">
                      {{ role.source }}
                    </span>
                    <span v-if="role.source !== 'local_override'" class="cell-subtle">
                      Managed by {{ role.source }}
                    </span>
                  </td>
                  <td>
                    <span :class="['status-pill', role.active ? '' : 'danger']">
                      {{ role.active ? 'Active' : 'Inactive' }}
                    </span>
                  </td>
                  <td>
                    <button
                      class="secondary-button danger-button compact-button"
                      type="button"
                      :disabled="role.source !== 'local_override' || !role.active || store.saving"
                      @click="askRevoke(member, role)"
                    >
                      Revoke
                    </button>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>

        <div v-else class="empty-state">
          <p>この tenant には membership がありません。</p>
        </div>
      </div>

      <form class="admin-form" @submit.prevent="createInvitation">
        <div class="form-span">
          <span class="status-pill">Invitation</span>
          <h2>Invite User</h2>
        </div>

        <label class="field">
          <span class="field-label">Email</span>
          <input
            v-model="invitationEmail"
            data-testid="tenant-invitation-email"
            class="field-input"
            autocomplete="email"
            type="email"
            required
          >
        </label>

        <label class="field">
          <span class="field-label">Role</span>
          <select v-model="invitationRoleCode" data-testid="tenant-invitation-role" class="field-input">
            <option v-for="role in tenantRoleOptions" :key="role" :value="role">
              {{ role }}
            </option>
          </select>
        </label>

        <div class="action-row form-span">
          <button class="primary-button" :disabled="!canInvite" type="submit">
            {{ commonStore.saving ? 'Saving...' : 'Invite' }}
          </button>
        </div>
      </form>

      <div class="stack">
        <div class="section-header">
          <div>
            <span class="status-pill">Invitations</span>
            <h2>Pending Invites</h2>
          </div>
        </div>

        <div v-if="commonStore.invitations.length > 0" class="list-stack">
          <article v-for="invitation in commonStore.invitations" :key="invitation.publicId" class="list-item">
            <div>
              <strong>{{ invitation.inviteeEmailNormalized }}</strong>
              <span class="cell-subtle">{{ invitation.roleCodes?.join(', ') }} / {{ invitation.status }}</span>
              <span class="cell-subtle">Expires {{ formatDate(invitation.expiresAt) }}</span>
            </div>
            <button
              class="secondary-button danger-button compact-button"
              type="button"
              :disabled="invitation.status !== 'pending' || commonStore.saving"
              @click="revokeInvitation(invitation.publicId)"
            >
              Revoke
            </button>
          </article>
        </div>

        <div v-else class="empty-state">
          <p>Invitation はありません。</p>
        </div>
      </div>

      <form class="admin-form" @submit.prevent="saveCommonSettings">
        <div class="form-span">
          <span class="status-pill">Common</span>
          <h2>Settings and Quota</h2>
        </div>

        <label class="field">
          <span class="field-label">File quota bytes</span>
          <input v-model.number="fileQuotaBytes" data-testid="tenant-file-quota" class="field-input" min="0" type="number">
        </label>

        <label class="field">
          <span class="field-label">Browser API limit / minute</span>
          <input v-model.number="browserRateLimit" data-testid="tenant-browser-rate-limit" class="field-input" min="1" type="number">
        </label>

        <label class="checkbox-field form-span">
          <input v-model="notificationsEnabled" type="checkbox">
          <span>Notifications enabled</span>
        </label>

        <div class="action-row form-span">
          <button class="primary-button" :disabled="!canSaveCommonSettings" type="submit">
            {{ commonStore.saving ? 'Saving...' : 'Save common settings' }}
          </button>
        </div>
      </form>

      <div class="stack">
        <div class="section-header">
          <div>
            <span class="status-pill">Export</span>
            <h2>Tenant Data Exports</h2>
          </div>
          <button
            data-testid="tenant-request-export"
            class="primary-button compact-button"
            type="button"
            :disabled="commonStore.saving"
            @click="requestExport"
          >
            Request export
          </button>
        </div>

        <div v-if="commonStore.exports.length > 0" class="list-stack">
          <article v-for="item in commonStore.exports" :key="item.publicId" class="list-item">
            <div>
              <strong>{{ item.format }} / {{ item.status }}</strong>
              <span class="cell-subtle">{{ item.publicId }}</span>
              <span class="cell-subtle">Expires {{ formatDate(item.expiresAt) }}</span>
            </div>
            <a
              v-if="item.status === 'ready'"
              class="secondary-button compact-button link-button"
              :href="`/api/v1/admin/tenants/${tenant.slug}/exports/${item.publicId}/download`"
            >
              Download
            </a>
          </article>
        </div>

        <div v-else class="empty-state">
          <p>Export request はありません。</p>
        </div>
      </div>
    </template>
  </section>

  <ConfirmActionDialog
    :open="pendingAction !== null"
    :title="confirmTitle"
    :message="confirmMessage"
    :confirm-label="confirmLabel"
    @cancel="cancelPendingAction"
    @confirm="confirmPendingAction"
  />
</template>
