<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { fetchPublicDriveShareLink, fetchPublicDriveShareLinkChildren, verifyPublicDriveShareLinkPassword } from '../api/drive'
import { toApiErrorMessage } from '../api/client'
import type { DriveItemBody, PublicDriveShareLinkOutputBody } from '../api/generated/types.gen'
import DriveFileTypeIcon from '../components/DriveFileTypeIcon.vue'
import {
  driveItemKind,
  driveItemName,
  driveItemPublicId,
  driveItemUpdatedAt,
  formatDriveDate as formatDriveItemDate,
  formatDriveSize,
} from '../utils/driveItems'

const route = useRoute()

const data = ref<PublicDriveShareLinkOutputBody | null>(null)
const status = ref<'idle' | 'loading' | 'ready' | 'error'>('idle')
const errorMessage = ref('')
const password = ref('')
const verifying = ref(false)
const passwordVerified = ref(false)
const children = ref<DriveItemBody[]>([])
const childrenStatus = ref<'idle' | 'loading' | 'ready' | 'error'>('idle')
const childrenErrorMessage = ref('')

const token = computed(() => {
  const raw = route.params.token
  return Array.isArray(raw) ? raw[0] : raw
})

const itemLabel = computed(() => (
  data.value?.file?.originalFilename ?? data.value?.folder?.name ?? 'Shared item'
))

const contentURL = computed(() => (
  token.value ? `/api/public/drive/share-links/${encodeURIComponent(token.value)}/content` : ''
))

onMounted(load)

watch(
  () => route.params.token,
  () => load(),
)

async function load() {
  if (!token.value) {
    return
  }
  status.value = 'loading'
  errorMessage.value = ''
  try {
    data.value = await fetchPublicDriveShareLink(token.value)
    passwordVerified.value = !data.value.link.passwordRequired
    status.value = 'ready'
    children.value = []
    if (data.value.folder && passwordVerified.value) {
      await loadChildren()
    }
  } catch (error) {
    data.value = null
    status.value = 'error'
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function verifyPassword() {
  if (!token.value || password.value.trim() === '') {
    return
  }
  verifying.value = true
  errorMessage.value = ''
  try {
    await verifyPublicDriveShareLinkPassword(token.value, password.value)
    password.value = ''
    passwordVerified.value = true
    if (data.value?.folder) {
      await loadChildren()
    }
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  } finally {
    verifying.value = false
  }
}

async function loadChildren() {
  if (!token.value || !data.value?.folder) {
    return
  }
  childrenStatus.value = 'loading'
  childrenErrorMessage.value = ''
  try {
    children.value = await fetchPublicDriveShareLinkChildren(token.value)
    childrenStatus.value = 'ready'
  } catch (error) {
    children.value = []
    childrenStatus.value = 'error'
    childrenErrorMessage.value = toApiErrorMessage(error)
  }
}

function formatDate(value?: string) {
  if (!value) {
    return '-'
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}
</script>

<template>
  <section class="panel stack drive-public-panel">
    <div class="section-header">
      <div>
        <span class="status-pill">Public Drive Link</span>
        <h2>{{ itemLabel }}</h2>
      </div>
    </div>

    <p v-if="status === 'loading'">Loading shared item...</p>
    <p v-if="errorMessage" class="error-message">{{ errorMessage }}</p>

    <template v-if="data">
      <dl class="metadata-grid">
        <div>
          <dt>Resource</dt>
          <dd>{{ data.file ? 'File' : 'Folder' }}</dd>
        </div>
        <div>
          <dt>Expires</dt>
          <dd>{{ formatDate(data.link.expiresAt) }}</dd>
        </div>
        <div v-if="data.file">
          <dt>Content type</dt>
          <dd>{{ data.file.contentType }}</dd>
        </div>
        <div>
          <dt>Password</dt>
          <dd>{{ data.link.passwordRequired ? (passwordVerified ? 'Verified' : 'Required') : 'Not required' }}</dd>
        </div>
        <div>
          <dt>Download</dt>
          <dd>{{ data.link.canDownload ? 'Allowed' : 'Blocked' }}</dd>
        </div>
      </dl>

      <form v-if="data.link.passwordRequired && !passwordVerified" class="admin-form" @submit.prevent="verifyPassword">
        <label class="field">
          <span class="field-label">Password</span>
          <input v-model="password" class="field-input" type="password" autocomplete="current-password">
        </label>
        <div class="action-row">
          <button class="primary-button" type="submit" :disabled="verifying || password.trim() === ''">
            {{ verifying ? 'Verifying...' : 'Unlock' }}
          </button>
        </div>
      </form>

      <div class="action-row">
        <a
          v-if="data.file && data.link.canDownload && passwordVerified"
          class="primary-button link-button"
          :href="contentURL"
        >
          Download
        </a>
        <span v-else-if="data.file" class="status-pill danger">
          {{ data.link.passwordRequired && !passwordVerified ? 'Password required' : 'Download blocked' }}
        </span>
      </div>
      <p v-if="data.file && !data.link.canDownload" class="warning-message">
        この link では content download は許可されていません。
      </p>
      <p v-if="data.folder" class="cell-subtle">
        This public folder link shows the folder contents allowed by the link policy.
      </p>

      <section v-if="data.folder && passwordVerified" class="drive-public-browser">
        <div class="section-header">
          <div>
            <h3>Folder contents</h3>
            <p class="cell-subtle">{{ children.length }} items visible through this public link.</p>
          </div>
          <button class="secondary-button compact-button" type="button" :disabled="childrenStatus === 'loading'" @click="loadChildren">
            Refresh
          </button>
        </div>

        <p v-if="childrenErrorMessage" class="error-message">{{ childrenErrorMessage }}</p>
        <p v-if="childrenStatus === 'loading'" class="cell-subtle">Loading folder contents...</p>
        <div v-else-if="children.length > 0" class="drive-public-grid">
          <article v-for="item in children" :key="driveItemPublicId(item)" class="drive-public-item">
            <DriveFileTypeIcon :kind="driveItemKind(item)" :size="18" />
            <div>
              <strong>{{ driveItemName(item) }}</strong>
              <span>{{ item.folder ? 'Folder' : formatDriveSize(item.file?.byteSize) }} · {{ formatDriveItemDate(driveItemUpdatedAt(item)) }}</span>
            </div>
          </article>
        </div>
        <p v-else class="cell-subtle">This folder has no visible children.</p>
      </section>
    </template>
  </section>
</template>
