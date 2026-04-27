<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { DrivePermissionBody, DrivePermissionsBody } from '../api/generated/types.gen'

defineProps<{
  permissions: DrivePermissionsBody | null
  busy: boolean
}>()

const emit = defineEmits<{
  revokeShare: [permission: DrivePermissionBody]
  disableLink: [permission: DrivePermissionBody]
  updateShareRole: [permission: DrivePermissionBody, role: string]
}>()

const { d, t } = useI18n()

function formatDate(value?: string) {
  if (!value) {
    return '-'
  }
  return d(new Date(value), 'long')
}

function subjectLabel(permission: DrivePermissionBody) {
  if (permission.kind === 'share_link') {
    return t('drivePermissions.shareLink')
  }
  if (permission.kind === 'owner') {
    return t('driveShare.owner')
  }
  if (!permission.subjectType && !permission.subjectId) {
    return permission.kind
  }
  return `${permission.subjectType ?? t('drivePermissions.subject')}:${permission.subjectId ?? '-'}`
}
</script>

<template>
  <div class="drive-permissions-panel">
    <section>
      <h3>{{ t('drivePermissions.direct') }}</h3>
      <div v-if="permissions?.direct?.length" class="drive-permission-list">
        <article v-for="permission in permissions.direct" :key="`${permission.kind}:${permission.publicId}:${permission.subjectId}`" class="drive-permission-row">
          <div>
            <strong>{{ subjectLabel(permission) }}</strong>
            <span class="cell-subtle">
              {{ permission.role }} / {{ permission.status || t('drivePermissions.active') }}
            </span>
            <span v-if="permission.expiresAt" class="cell-subtle">
              {{ t('drivePermissions.expires', { date: formatDate(permission.expiresAt) }) }}
            </span>
            <span v-if="permission.canDownload !== undefined" class="cell-subtle">
              {{ t('drivePermissions.download') }} {{ permission.canDownload ? t('drivePermissions.allowed') : t('drivePermissions.blocked') }}
            </span>
          </div>
          <div class="drive-row-actions">
            <select
              v-if="permission.kind === 'share' && permission.publicId"
              class="field-input compact-select"
              :value="permission.role"
              :disabled="busy"
              :aria-label="t('drivePermissions.updateShareRole')"
              @change="emit('updateShareRole', permission, ($event.target as HTMLSelectElement).value)"
            >
              <option value="viewer">{{ t('driveShare.viewer') }}</option>
              <option value="editor">{{ t('driveShare.editor') }}</option>
            </select>
            <button
              v-if="permission.kind === 'share' && permission.publicId"
              class="secondary-button compact-button danger-button"
              type="button"
              :disabled="busy"
              @click="emit('revokeShare', permission)"
            >
              {{ t('drivePermissions.revoke') }}
            </button>
            <button
              v-if="permission.kind === 'share_link' && permission.publicId"
              class="secondary-button compact-button danger-button"
              type="button"
              :disabled="busy"
              @click="emit('disableLink', permission)"
            >
              {{ t('drivePermissions.disable') }}
            </button>
          </div>
        </article>
      </div>
      <p v-else class="cell-subtle">{{ t('drivePermissions.noDirect') }}</p>
    </section>

    <section>
      <h3>{{ t('drivePermissions.inherited') }}</h3>
      <div v-if="permissions?.inherited?.length" class="drive-permission-list">
        <article v-for="permission in permissions.inherited" :key="`${permission.kind}:${permission.inheritedFromId}:${permission.subjectId}`" class="drive-permission-row">
          <div>
            <strong>{{ subjectLabel(permission) }}</strong>
            <span class="cell-subtle">
              {{ t('drivePermissions.inheritedFrom', { role: permission.role, source: permission.inheritedFromId || '-' }) }}
            </span>
          </div>
        </article>
      </div>
      <p v-else class="cell-subtle">{{ t('drivePermissions.noInherited') }}</p>
    </section>
  </div>
</template>
