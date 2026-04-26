<script setup lang="ts">
import type { DrivePermissionBody, DrivePermissionsBody } from '../api/generated/types.gen'

defineProps<{
  permissions: DrivePermissionsBody | null
  busy: boolean
}>()

const emit = defineEmits<{
  revokeShare: [permission: DrivePermissionBody]
  disableLink: [permission: DrivePermissionBody]
}>()

function formatDate(value?: string) {
  if (!value) {
    return '-'
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

function subjectLabel(permission: DrivePermissionBody) {
  if (permission.kind === 'share_link') {
    return 'Share link'
  }
  if (permission.kind === 'owner') {
    return 'Owner'
  }
  if (!permission.subjectType && !permission.subjectId) {
    return permission.kind
  }
  return `${permission.subjectType ?? 'subject'}:${permission.subjectId ?? '-'}`
}
</script>

<template>
  <div class="drive-permissions-panel">
    <section>
      <h3>Direct</h3>
      <div v-if="permissions?.direct?.length" class="drive-permission-list">
        <article v-for="permission in permissions.direct" :key="`${permission.kind}:${permission.publicId}:${permission.subjectId}`" class="drive-permission-row">
          <div>
            <strong>{{ subjectLabel(permission) }}</strong>
            <span class="cell-subtle">
              {{ permission.role }} / {{ permission.status || 'active' }}
            </span>
            <span v-if="permission.expiresAt" class="cell-subtle">
              Expires {{ formatDate(permission.expiresAt) }}
            </span>
            <span v-if="permission.canDownload !== undefined" class="cell-subtle">
              Download {{ permission.canDownload ? 'allowed' : 'blocked' }}
            </span>
          </div>
          <div class="drive-row-actions">
            <button
              v-if="permission.kind === 'share' && permission.publicId"
              class="secondary-button compact-button danger-button"
              type="button"
              :disabled="busy"
              @click="emit('revokeShare', permission)"
            >
              Revoke
            </button>
            <button
              v-if="permission.kind === 'share_link' && permission.publicId"
              class="secondary-button compact-button danger-button"
              type="button"
              :disabled="busy"
              @click="emit('disableLink', permission)"
            >
              Disable
            </button>
          </div>
        </article>
      </div>
      <p v-else class="cell-subtle">No direct permissions.</p>
    </section>

    <section>
      <h3>Inherited</h3>
      <div v-if="permissions?.inherited?.length" class="drive-permission-list">
        <article v-for="permission in permissions.inherited" :key="`${permission.kind}:${permission.inheritedFromId}:${permission.subjectId}`" class="drive-permission-row">
          <div>
            <strong>{{ subjectLabel(permission) }}</strong>
            <span class="cell-subtle">
              {{ permission.role }} inherited from {{ permission.inheritedFromId || '-' }}
            </span>
          </div>
        </article>
      </div>
      <p v-else class="cell-subtle">No inherited permissions.</p>
    </section>
  </div>
</template>
