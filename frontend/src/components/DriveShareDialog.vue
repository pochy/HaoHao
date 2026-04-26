<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'

import type {
  DriveGroupBody,
  DrivePermissionBody,
  DrivePermissionsBody,
  DriveShareLinkBody,
  DriveShareTargetBody,
} from '../api/generated/types.gen'
import { fetchDriveShareTargets, type DriveResourceRef } from '../api/drive'
import { toApiErrorMessage } from '../api/client'
import DriveShareAccessSummary from './DriveShareAccessSummary.vue'
import DrivePermissionsPanel from './DrivePermissionsPanel.vue'

const props = defineProps<{
  open: boolean
  resource: DriveResourceRef | null
  label: string
  groups: DriveGroupBody[]
  permissions: DrivePermissionsBody | null
  lastRawShareLink: DriveShareLinkBody | null
  busy: boolean
  errorMessage: string
}>()

const emit = defineEmits<{
  close: []
  createUserShare: [subjectPublicId: string, role: string]
  createGroupShare: [groupPublicId: string, role: string]
  createExternalInvitation: [inviteeEmail: string, role: string]
  createShareLink: [expiresAt: string, canDownload: boolean, password: string, role: string]
  revokeShare: [permission: DrivePermissionBody]
  disableLink: [permission: DrivePermissionBody]
  updateShareRole: [permission: DrivePermissionBody, role: string]
  transferOwner: [newOwnerUserPublicId: string, revokePreviousOwnerAccess: boolean]
  reloadPermissions: []
}>()

const dialogRef = ref<HTMLDialogElement | null>(null)
const userPublicId = ref('')
const targetQuery = ref('')
const targetResults = ref<DriveShareTargetBody[]>([])
const targetSearchError = ref('')
const ownerTargetQuery = ref('')
const ownerTargetResults = ref<DriveShareTargetBody[]>([])
const ownerTargetSearchError = ref('')
const ownerTransferUserPublicId = ref('')
const revokePreviousOwnerAccess = ref(false)
const externalEmail = ref('')
const groupPublicId = ref('')
const shareRole = ref('viewer')
const linkExpiresAt = ref('')
const linkCanDownload = ref(true)
const linkPassword = ref('')
const linkRole = ref('viewer')
const copied = ref(false)

const canCreateUserShare = computed(() => Boolean(props.resource) && userPublicId.value.trim() !== '')
const canCreateExternalInvitation = computed(() => Boolean(props.resource) && externalEmail.value.trim() !== '')
const canCreateGroupShare = computed(() => Boolean(props.resource) && groupPublicId.value !== '')
const canCreateShareLink = computed(() => Boolean(props.resource) && linkExpiresAt.value !== '')
const canTransferOwner = computed(() => Boolean(props.resource) && ownerTransferUserPublicId.value.trim() !== '')
const rawLinkURL = computed(() => (
  props.lastRawShareLink?.token ? `${window.location.origin}/public/drive/share-links/${props.lastRawShareLink.token}` : ''
))

watch(
  () => props.open,
  async (open) => {
    await nextTick()
    const dialog = dialogRef.value
    if (!dialog) {
      return
    }
    if (open && !dialog.open) {
      if (!linkExpiresAt.value) {
        const date = new Date(Date.now() + 60 * 60 * 1000)
        linkExpiresAt.value = toDatetimeLocalValue(date)
      }
      dialog.showModal()
      return
    }
    if (!open && dialog.open) {
      dialog.close()
    }
  },
  { immediate: true },
)

watch(
  () => props.groups,
  (groups) => {
    if (!groupPublicId.value && groups[0]) {
      groupPublicId.value = groups[0].publicId
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (dialogRef.value?.open) {
    dialogRef.value.close()
  }
})

function toDatetimeLocalValue(value: Date) {
  const offsetMs = value.getTimezoneOffset() * 60_000
  return new Date(value.getTime() - offsetMs).toISOString().slice(0, 16)
}

function close() {
  emit('close')
}

function handleClose() {
  if (props.open) {
    emit('close')
  }
}

function createUserShare() {
  if (!canCreateUserShare.value) {
    return
  }
  emit('createUserShare', userPublicId.value.trim(), shareRole.value)
  userPublicId.value = ''
}

async function searchTargets() {
  targetSearchError.value = ''
  try {
    targetResults.value = await fetchDriveShareTargets(targetQuery.value)
  } catch (error) {
    targetResults.value = []
    targetSearchError.value = toApiErrorMessage(error)
  }
}

function shareWithTarget(target: DriveShareTargetBody) {
  if (target.type === 'group') {
    emit('createGroupShare', target.publicId, shareRole.value)
  } else {
    emit('createUserShare', target.publicId, shareRole.value)
  }
}

async function searchOwnerTargets() {
  ownerTargetSearchError.value = ''
  try {
    ownerTargetResults.value = (await fetchDriveShareTargets(ownerTargetQuery.value))
      .filter((target) => target.type === 'user')
  } catch (error) {
    ownerTargetResults.value = []
    ownerTargetSearchError.value = toApiErrorMessage(error)
  }
}

function selectOwnerTarget(target: DriveShareTargetBody) {
  ownerTransferUserPublicId.value = target.publicId
  ownerTargetQuery.value = target.displayName || target.publicId
}

function transferOwner() {
  if (!canTransferOwner.value) {
    return
  }
  emit('transferOwner', ownerTransferUserPublicId.value.trim(), revokePreviousOwnerAccess.value)
}

function createGroupShare() {
  if (!canCreateGroupShare.value) {
    return
  }
  emit('createGroupShare', groupPublicId.value, shareRole.value)
}

function createExternalInvitation() {
  if (!canCreateExternalInvitation.value) {
    return
  }
  emit('createExternalInvitation', externalEmail.value.trim(), shareRole.value)
  externalEmail.value = ''
}

function createShareLink() {
  if (!canCreateShareLink.value) {
    return
  }
  emit('createShareLink', linkExpiresAt.value, linkCanDownload.value, linkPassword.value, linkRole.value)
  linkPassword.value = ''
  copied.value = false
}

async function copyRawLink() {
  if (!rawLinkURL.value) {
    return
  }
  try {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(rawLinkURL.value)
    }
  } catch {
    // The URL remains selected in the readonly field; copying can be retried manually.
  }
  copied.value = true
}
</script>

<template>
  <dialog ref="dialogRef" class="drive-dialog" @close="handleClose" @cancel.prevent="close">
    <div class="drive-dialog-panel stack">
      <div class="section-header">
        <div>
          <span class="status-pill">Share</span>
          <h2>{{ label }}</h2>
          <p class="cell-subtle">Current access, people, groups, and link sharing for this Drive item.</p>
        </div>
        <button class="secondary-button compact-button" type="button" @click="close">
          Close
        </button>
      </div>

      <p v-if="errorMessage" class="error-message">{{ errorMessage }}</p>

      <DriveShareAccessSummary :permissions="permissions" />

      <section class="drive-share-section">
        <div>
          <h3>Add people and groups</h3>
          <p class="cell-subtle">Search tenant users and Drive groups, then choose the role to grant.</p>
        </div>

        <form class="admin-form" @submit.prevent="searchTargets">
          <label class="field form-span">
            <span class="field-label">Search users and groups</span>
            <input v-model="targetQuery" class="field-input" autocomplete="off" :disabled="busy" placeholder="Name, email, or public ID">
          </label>
          <label class="field">
            <span class="field-label">Role</span>
            <select v-model="shareRole" class="field-input" :disabled="busy">
              <option value="viewer">Viewer</option>
              <option value="editor">Editor</option>
            </select>
          </label>
          <div class="action-row form-span">
            <button class="secondary-button compact-button" type="submit" :disabled="busy">
              Search targets
            </button>
          </div>
          <p v-if="targetSearchError" class="error-message form-span">{{ targetSearchError }}</p>
          <div v-if="targetResults.length > 0" class="drive-target-results form-span">
            <button
              v-for="target in targetResults"
              :key="`${target.type}:${target.publicId}`"
              class="drive-target-row"
              type="button"
              :disabled="busy"
              @click="shareWithTarget(target)"
            >
              <span>
                <strong>{{ target.displayName }}</strong>
                <small>{{ target.type }} · {{ target.secondary || target.publicId }}</small>
              </span>
              <span class="status-pill">{{ shareRole }}</span>
            </button>
          </div>
        </form>

        <form class="admin-form" @submit.prevent="createUserShare">
          <div class="form-span">
            <h4>User share</h4>
          </div>
          <label class="field">
            <span class="field-label">User public ID</span>
            <input v-model="userPublicId" class="field-input" autocomplete="off" :disabled="busy">
          </label>
          <label class="field">
            <span class="field-label">Role</span>
            <select v-model="shareRole" class="field-input" :disabled="busy">
              <option value="viewer">Viewer</option>
              <option value="editor">Editor</option>
            </select>
          </label>
          <div class="action-row form-span">
            <button class="primary-button compact-button" type="submit" :disabled="busy || !canCreateUserShare">
              Share with user
            </button>
          </div>
        </form>

        <form class="admin-form" @submit.prevent="createGroupShare">
          <div class="form-span">
            <h4>Group share</h4>
          </div>
          <label class="field">
            <span class="field-label">Drive group</span>
            <select v-model="groupPublicId" class="field-input" :disabled="busy || groups.length === 0">
              <option v-for="group in groups" :key="group.publicId" :value="group.publicId">
                {{ group.name }}
              </option>
            </select>
          </label>
          <label class="field">
            <span class="field-label">Role</span>
            <select v-model="shareRole" class="field-input" :disabled="busy">
              <option value="viewer">Viewer</option>
              <option value="editor">Editor</option>
            </select>
          </label>
          <div class="action-row form-span">
            <button class="primary-button compact-button" type="submit" :disabled="busy || !canCreateGroupShare">
              Share with group
            </button>
            <RouterLink class="secondary-button compact-button link-button" to="/drive/groups">
              Manage groups
            </RouterLink>
          </div>
        </form>

        <form class="admin-form" @submit.prevent="createExternalInvitation">
          <div class="form-span">
            <h4>External invitation</h4>
          </div>
          <label class="field">
            <span class="field-label">Invitee email</span>
            <input v-model="externalEmail" class="field-input" autocomplete="email" type="email" :disabled="busy">
          </label>
          <label class="field">
            <span class="field-label">Role</span>
            <select v-model="shareRole" class="field-input" :disabled="busy">
              <option value="viewer">Viewer</option>
              <option value="editor">Editor</option>
            </select>
          </label>
          <div class="action-row form-span">
            <button class="primary-button compact-button" type="submit" :disabled="busy || !canCreateExternalInvitation">
              Invite external user
            </button>
          </div>
        </form>
      </section>

      <section class="drive-share-section">
        <div>
          <h3>Owner transfer</h3>
          <p class="cell-subtle">Transfer ownership to another tenant user. This action requires owner access and confirmation.</p>
        </div>

        <form class="admin-form" @submit.prevent="searchOwnerTargets">
          <label class="field form-span">
            <span class="field-label">Search users</span>
            <input v-model="ownerTargetQuery" class="field-input" autocomplete="off" :disabled="busy" placeholder="Name, email, or public ID">
          </label>
          <div class="action-row form-span">
            <button class="secondary-button compact-button" type="submit" :disabled="busy">
              Search users
            </button>
          </div>
          <p v-if="ownerTargetSearchError" class="error-message form-span">{{ ownerTargetSearchError }}</p>
          <div v-if="ownerTargetResults.length > 0" class="drive-target-results form-span">
            <button
              v-for="target in ownerTargetResults"
              :key="`owner:${target.publicId}`"
              class="drive-target-row"
              type="button"
              :disabled="busy"
              @click="selectOwnerTarget(target)"
            >
              <span>
                <strong>{{ target.displayName }}</strong>
                <small>{{ target.secondary || target.publicId }}</small>
              </span>
              <span class="status-pill">Owner</span>
            </button>
          </div>
        </form>

        <form class="admin-form" @submit.prevent="transferOwner">
          <label class="field">
            <span class="field-label">New owner user public ID</span>
            <input v-model="ownerTransferUserPublicId" class="field-input" autocomplete="off" :disabled="busy">
          </label>
          <label class="checkbox-field">
            <input v-model="revokePreviousOwnerAccess" type="checkbox" :disabled="busy">
            <span>Revoke previous owner access after transfer</span>
          </label>
          <div class="action-row form-span">
            <button class="secondary-button compact-button" type="submit" :disabled="busy || !canTransferOwner">
              Transfer owner
            </button>
          </div>
        </form>
      </section>

      <section class="drive-share-section">
        <form class="admin-form" @submit.prevent="createShareLink">
          <div class="form-span">
            <h3>Share link</h3>
            <p class="cell-subtle">
              Download 禁止は操作上の制限であり、スクリーンショット等を完全には防止できません。
            </p>
          </div>
          <label class="field">
            <span class="field-label">Expires at</span>
            <input v-model="linkExpiresAt" class="field-input" type="datetime-local" :disabled="busy">
          </label>
          <label class="field">
            <span class="field-label">Role</span>
            <select v-model="linkRole" class="field-input" :disabled="busy">
              <option value="viewer">Viewer</option>
              <option value="editor">Editor</option>
            </select>
          </label>
          <label class="checkbox-field">
            <input v-model="linkCanDownload" type="checkbox" :disabled="busy || linkRole === 'editor'">
            <span>Allow download</span>
          </label>
          <label class="field form-span">
            <span class="field-label">Password</span>
            <input v-model="linkPassword" class="field-input" autocomplete="new-password" type="password" :disabled="busy">
          </label>
          <div class="action-row form-span">
            <button class="primary-button compact-button" type="submit" :disabled="busy || !canCreateShareLink">
              Create link
            </button>
          </div>
        </form>

        <div v-if="rawLinkURL" class="drive-raw-link">
          <label class="field">
            <span class="field-label">New link URL</span>
            <input :value="rawLinkURL" class="field-input monospace-cell" readonly>
          </label>
          <div class="action-row">
            <button class="secondary-button compact-button" type="button" @click="copyRawLink">
              {{ copied ? 'Copied' : 'Copy link' }}
            </button>
            <p class="cell-subtle">この URL は作成直後だけ表示されます。</p>
          </div>
        </div>
      </section>

      <div class="section-header">
        <div>
          <span class="status-pill">Permissions</span>
          <h3>Current permissions</h3>
        </div>
        <button class="secondary-button compact-button" type="button" :disabled="busy" @click="emit('reloadPermissions')">
          Reload
        </button>
      </div>

      <DrivePermissionsPanel
        :permissions="permissions"
        :busy="busy"
        @revoke-share="emit('revokeShare', $event)"
        @disable-link="emit('disableLink', $event)"
        @update-share-role="(permission, role) => emit('updateShareRole', permission, role)"
      />
    </div>
  </dialog>
</template>
