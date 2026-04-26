<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { toApiErrorMessage } from '../api/client'
import { uploadFile } from '../api/files'
import { startSupportAccessSession } from '../api/support-access'
import type { TenantAdminMembershipBody, TenantAdminRoleBindingBody } from '../api/generated/types.gen'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import ConfirmActionDialog from '../components/ConfirmActionDialog.vue'
import { useTenantAdminStore } from '../stores/tenant-admin'
import { useTenantCommonStore } from '../stores/tenant-common'
import { useSessionStore } from '../stores/session'

type PendingAction =
  | { kind: 'deactivate' }
  | { kind: 'revoke', userPublicId: string, userLabel: string, roleCode: string }

const route = useRoute()
const store = useTenantAdminStore()
const commonStore = useTenantCommonStore()
const sessionStore = useSessionStore()

const displayName = ref('')
const active = ref(true)
const grantUserEmail = ref('')
const grantRoleCode = ref('customer_signal_user')
const invitationEmail = ref('')
const invitationRoleCode = ref('todo_user')
const fileQuotaBytes = ref(104857600)
const browserRateLimit = ref<number | null>(null)
const notificationsEnabled = ref(true)
const driveExternalSharingEnabled = ref(false)
const driveRequireApproval = ref(false)
const drivePublicLinksEnabled = ref(true)
const drivePasswordLinksEnabled = ref(false)
const driveRequireLinkPassword = ref(false)
const driveAllowedDomains = ref('')
const driveBlockedDomains = ref('')
const driveMaxLinkTTLHours = ref(168)
const driveViewerDownloadEnabled = ref(true)
const driveExternalDownloadEnabled = ref(false)
const webhookName = ref('')
const webhookUrl = ref('')
const webhookEvents = ref('customer_signal.created')
const importFile = ref<File | null>(null)
const supportUserPublicId = ref('')
const supportReason = ref('')
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
const drivePolicyRows = computed(() => [
  ['Public links', drivePublicLinksEnabled.value ? 'Enabled' : 'Disabled'],
  ['External sharing', driveExternalSharingEnabled.value ? 'Enabled' : 'Disabled'],
  ['External approval', driveRequireApproval.value ? 'Required' : 'Not required'],
  ['Password links', drivePasswordLinksEnabled.value ? 'Enabled' : 'Disabled'],
  ['Max link TTL', `${driveMaxLinkTTLHours.value} hours`],
])

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
  await store.loadDriveState(tenantSlug.value)
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
  const drive = (commonStore.settings.features?.drive ?? {}) as Record<string, unknown>
  driveExternalSharingEnabled.value = Boolean(drive.externalUserSharingEnabled)
  driveRequireApproval.value = Boolean(drive.requireExternalShareApproval)
  drivePublicLinksEnabled.value = drive.publicLinksEnabled !== false
  drivePasswordLinksEnabled.value = Boolean(drive.passwordProtectedLinksEnabled)
  driveRequireLinkPassword.value = Boolean(drive.requireShareLinkPassword)
  driveAllowedDomains.value = Array.isArray(drive.allowedExternalDomains) ? drive.allowedExternalDomains.join(', ') : ''
  driveBlockedDomains.value = Array.isArray(drive.blockedExternalDomains) ? drive.blockedExternalDomains.join(', ') : ''
  driveMaxLinkTTLHours.value = typeof drive.maxShareLinkTTLHours === 'number' ? drive.maxShareLinkTTLHours : 168
  driveViewerDownloadEnabled.value = drive.viewerDownloadEnabled !== false
  driveExternalDownloadEnabled.value = Boolean(drive.externalDownloadEnabled)
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

function domainList(value: string) {
  return value.split(',').map((item) => item.trim()).filter(Boolean)
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
      features: {
        ...(commonStore.settings?.features ?? {}),
        drive: {
          linkSharingEnabled: true,
          publicLinksEnabled: drivePublicLinksEnabled.value,
          externalUserSharingEnabled: driveExternalSharingEnabled.value,
          passwordProtectedLinksEnabled: drivePasswordLinksEnabled.value,
          requireShareLinkPassword: driveRequireLinkPassword.value,
          requireExternalShareApproval: driveRequireApproval.value,
          allowedExternalDomains: domainList(driveAllowedDomains.value),
          blockedExternalDomains: domainList(driveBlockedDomains.value),
          maxShareLinkTTLHours: driveMaxLinkTTLHours.value,
          viewerDownloadEnabled: driveViewerDownloadEnabled.value,
          externalDownloadEnabled: driveExternalDownloadEnabled.value,
          editorCanReshare: false,
          editorCanDelete: false,
        },
      },
    })
    message.value = 'Tenant common settings を更新しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function approveDriveInvitation(publicId: string) {
  if (!tenant.value) {
    return
  }
  message.value = ''
  errorMessage.value = ''
  try {
    await store.approveDriveInvitation(tenant.value.slug, publicId)
    message.value = 'Drive invitation を承認しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function rejectDriveInvitation(publicId: string) {
  if (!tenant.value) {
    return
  }
  message.value = ''
  errorMessage.value = ''
  try {
    await store.rejectDriveInvitation(tenant.value.slug, publicId)
    message.value = 'Drive invitation を拒否しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function repairDriveSync() {
  if (!tenant.value) {
    return
  }
  message.value = ''
  errorMessage.value = ''
  try {
    await store.repairDriveSync(tenant.value.slug)
    message.value = 'Drive OpenFGA sync repair を実行しました。'
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

async function requestCSVExport() {
  if (!tenant.value) {
    return
  }
  message.value = ''
  errorMessage.value = ''
  try {
    await commonStore.requestCSVExport(tenant.value.slug)
    message.value = 'Customer Signals CSV export を request しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function saveEntitlements() {
  if (!tenant.value) {
    return
  }
  message.value = ''
  errorMessage.value = ''
  try {
    await commonStore.updateEntitlements(tenant.value.slug, commonStore.entitlements.map((item) => ({
      featureCode: item.featureCode,
      enabled: item.enabled,
      limitValue: item.limitValue,
    })))
    message.value = 'Entitlements を更新しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function createWebhook() {
  if (!tenant.value) {
    return
  }
  message.value = ''
  errorMessage.value = ''
  try {
    const created = await commonStore.createWebhook(tenant.value.slug, {
      name: webhookName.value.trim(),
      url: webhookUrl.value.trim(),
      eventTypes: webhookEvents.value.split(',').map((item) => item.trim()).filter(Boolean),
      active: true,
    })
    webhookName.value = ''
    webhookUrl.value = ''
    message.value = created.secret ? `Webhook secret: ${created.secret}` : 'Webhook を作成しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function uploadImportCSV() {
  if (!tenant.value || !importFile.value) {
    return
  }
  message.value = ''
  errorMessage.value = ''
  try {
    const form = new FormData()
    form.append('purpose', 'import')
    form.append('file', importFile.value)
    const file = await uploadFile(form)
    await commonStore.createImport(tenant.value.slug, file.publicId)
    importFile.value = null
    message.value = 'CSV import job を作成しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function startSupportAccess() {
  if (!tenant.value) {
    return
  }
  message.value = ''
  errorMessage.value = ''
  try {
    const result = await startSupportAccessSession({
      tenantSlug: tenant.value.slug,
      impersonatedUserPublicId: supportUserPublicId.value.trim(),
      reason: supportReason.value.trim(),
      durationMinutes: 30,
    })
    sessionStore.supportAccess = result.access ?? null
    sessionStore.status = 'idle'
    await sessionStore.bootstrap()
    message.value = 'Support access を開始しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

function onImportFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  importFile.value = target.files?.[0] ?? null
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
            <span class="status-pill">Drive Policy</span>
            <h2>Drive Authorization</h2>
          </div>
        </div>

        <form class="admin-form" @submit.prevent="saveCommonSettings">
          <label class="checkbox-field">
            <input v-model="drivePublicLinksEnabled" type="checkbox">
            <span>Public links enabled</span>
          </label>
          <label class="checkbox-field">
            <input v-model="driveExternalSharingEnabled" type="checkbox">
            <span>External user sharing enabled</span>
          </label>
          <label class="checkbox-field">
            <input v-model="driveRequireApproval" type="checkbox">
            <span>External share approval required</span>
          </label>
          <label class="checkbox-field">
            <input v-model="drivePasswordLinksEnabled" type="checkbox">
            <span>Password protected links enabled</span>
          </label>
          <label class="checkbox-field">
            <input v-model="driveRequireLinkPassword" type="checkbox">
            <span>Require share link password</span>
          </label>
          <label class="checkbox-field">
            <input v-model="driveViewerDownloadEnabled" type="checkbox">
            <span>Viewer download enabled</span>
          </label>
          <label class="checkbox-field">
            <input v-model="driveExternalDownloadEnabled" type="checkbox">
            <span>External download enabled</span>
          </label>
          <label class="field">
            <span class="field-label">Max link TTL hours</span>
            <input v-model.number="driveMaxLinkTTLHours" class="field-input" min="1" max="2160" type="number">
          </label>
          <label class="field">
            <span class="field-label">Allowed external domains</span>
            <input v-model="driveAllowedDomains" class="field-input" autocomplete="off" placeholder="example.com, partner.example">
          </label>
          <label class="field">
            <span class="field-label">Blocked external domains</span>
            <input v-model="driveBlockedDomains" class="field-input" autocomplete="off" placeholder="blocked.example">
          </label>
          <div class="action-row form-span">
            <button class="primary-button" :disabled="commonStore.saving" type="submit">
              Save Drive policy
            </button>
          </div>
        </form>

        <div class="admin-table">
          <table>
            <thead>
              <tr>
                <th scope="col">Policy</th>
                <th scope="col">Current Phase</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in drivePolicyRows" :key="row[0]">
                <td>{{ row[0] }}</td>
                <td>{{ row[1] }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <p class="cell-subtle">
          Audit は <code>drive.file.*</code>, <code>drive.folder.*</code>, <code>drive.share.*</code>, <code>drive.share_link.*</code>, <code>drive.authz.denied</code> を記録します。
          この画面には Drive file body の閲覧導線を置きません。
        </p>
      </div>

      <div class="stack">
        <div class="section-header">
          <div>
            <span class="status-pill">Drive Admin</span>
            <h2>Share State</h2>
          </div>
          <button class="secondary-button compact-button" type="button" :disabled="store.saving" @click="store.loadDriveState(tenant.slug)">
            Reload
          </button>
        </div>

        <div v-if="store.driveApprovals.length > 0" class="list-stack">
          <article v-for="item in store.driveApprovals" :key="item.publicId" class="list-item">
            <div>
              <strong>{{ item.resourceType }} / {{ item.role }} / {{ item.status }}</strong>
              <span class="cell-subtle">{{ item.maskedInviteeEmail || item.inviteeEmailDomain }}</span>
              <span class="cell-subtle">Expires {{ formatDate(item.expiresAt) }}</span>
            </div>
            <div class="action-row">
              <button class="primary-button compact-button" type="button" :disabled="store.saving" @click="approveDriveInvitation(item.publicId)">
                Approve
              </button>
              <button class="secondary-button danger-button compact-button" type="button" :disabled="store.saving" @click="rejectDriveInvitation(item.publicId)">
                Reject
              </button>
            </div>
          </article>
        </div>

        <div class="admin-table">
          <table>
            <thead>
              <tr>
                <th scope="col">Resource</th>
                <th scope="col">Subject</th>
                <th scope="col">Role</th>
                <th scope="col">Status</th>
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
                <td colspan="4">Drive share はありません。</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="admin-table">
          <table>
            <thead>
              <tr>
                <th scope="col">Link</th>
                <th scope="col">Resource</th>
                <th scope="col">Download</th>
                <th scope="col">Status</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in store.driveShareLinks" :key="item.publicId">
                <td>{{ item.publicId }}<span class="cell-subtle">Password {{ item.passwordRequired ? 'required' : 'not required' }}</span></td>
                <td>{{ item.resourceName || item.resourcePublicId }}</td>
                <td>{{ item.canDownload ? 'Allowed' : 'Blocked' }}</td>
                <td>{{ item.status }}<span class="cell-subtle">Expires {{ formatDate(item.expiresAt) }}</span></td>
              </tr>
              <tr v-if="store.driveShareLinks.length === 0">
                <td colspan="4">Drive share link はありません。</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="section-header">
          <div>
            <span class="status-pill">OpenFGA</span>
            <h2>Sync Status</h2>
          </div>
          <button class="secondary-button compact-button" type="button" :disabled="store.saving" @click="repairDriveSync">
            Repair
          </button>
        </div>
        <div class="list-stack">
          <article v-for="item in store.driveSync?.items ?? []" :key="`${item.kind}:${item.publicId}:${item.action}`" class="list-item">
            <div>
              <strong>{{ item.kind }} / {{ item.action }}</strong>
              <span class="cell-subtle">{{ item.publicId }} / {{ item.status }}</span>
              <span v-if="item.error" class="cell-subtle">{{ item.error }}</span>
            </div>
          </article>
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
      </div>

      <form class="admin-form" @submit.prevent="saveEntitlements">
        <div class="form-span">
          <span class="status-pill">Entitlements</span>
          <h2>Feature Gates</h2>
        </div>

        <label
          v-for="item in commonStore.entitlements"
          :key="item.featureCode"
          class="checkbox-field"
        >
          <input v-model="item.enabled" type="checkbox">
          <span>{{ item.featureCode }}</span>
        </label>

        <div class="action-row form-span">
          <button class="primary-button" :disabled="commonStore.saving" type="submit">
            Save entitlements
          </button>
        </div>
      </form>

      <form class="admin-form" @submit.prevent="startSupportAccess">
        <div class="form-span">
          <span class="status-pill">Support</span>
          <h2>Support Access</h2>
        </div>

        <label class="field">
          <span class="field-label">User public ID</span>
          <input v-model="supportUserPublicId" class="field-input" autocomplete="off">
        </label>

        <label class="field">
          <span class="field-label">Reason</span>
          <input v-model="supportReason" class="field-input" autocomplete="off">
        </label>

        <div class="action-row form-span">
          <button class="secondary-button" :disabled="supportUserPublicId.trim() === '' || supportReason.trim().length < 8" type="submit">
            Start support access
          </button>
        </div>
      </form>

      <form class="admin-form" @submit.prevent="createWebhook">
        <div class="form-span">
          <span class="status-pill">Webhooks</span>
          <h2>Outbound Webhooks</h2>
        </div>

        <label class="field">
          <span class="field-label">Name</span>
          <input v-model="webhookName" class="field-input" autocomplete="off" placeholder="Customer Signals">
        </label>

        <label class="field">
          <span class="field-label">URL</span>
          <input v-model="webhookUrl" class="field-input" autocomplete="url" placeholder="https://example.com/webhook">
        </label>

        <label class="field form-span">
          <span class="field-label">Events</span>
          <input v-model="webhookEvents" class="field-input" autocomplete="off">
        </label>

        <div class="action-row form-span">
          <button class="primary-button" :disabled="commonStore.saving || webhookName.trim() === '' || webhookUrl.trim() === ''" type="submit">
            Create webhook
          </button>
        </div>
      </form>

      <div v-if="commonStore.webhooks.length > 0" class="list-stack">
        <article v-for="item in commonStore.webhooks" :key="item.publicId" class="list-item">
          <div>
            <strong>{{ item.name }} / {{ item.active ? 'active' : 'inactive' }}</strong>
            <span class="cell-subtle">{{ item.url }}</span>
            <span class="cell-subtle">{{ item.eventTypes?.join(', ') }}</span>
          </div>
        </article>
      </div>

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
          <button
            class="secondary-button compact-button"
            type="button"
            :disabled="commonStore.saving"
            @click="requestCSVExport"
          >
            Request CSV
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

      <form class="admin-form" @submit.prevent="uploadImportCSV">
        <div class="form-span">
          <span class="status-pill">Import</span>
          <h2>Customer Signals CSV</h2>
        </div>

        <label class="field form-span">
          <span class="field-label">CSV file</span>
          <input class="field-input" accept=".csv,text/csv" type="file" @change="onImportFileChange">
        </label>

        <div class="action-row form-span">
          <button class="primary-button" :disabled="commonStore.saving || !importFile" type="submit">
            Upload and import
          </button>
        </div>
      </form>

      <div v-if="commonStore.imports.length > 0" class="list-stack">
        <article v-for="item in commonStore.imports" :key="item.publicId" class="list-item">
          <div>
            <strong>{{ item.status }}</strong>
            <span class="cell-subtle">{{ item.publicId }}</span>
            <span class="cell-subtle">
              rows {{ item.validRows }}/{{ item.totalRows }} valid, {{ item.invalidRows }} invalid
            </span>
          </div>
        </article>
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
