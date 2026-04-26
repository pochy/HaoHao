<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import type { DriveGroupBody } from '../api/generated/types.gen'
import { toApiErrorMessage } from '../api/client'
import { useDriveStore } from '../stores/drive'
import { useTenantStore } from '../stores/tenants'

const tenantStore = useTenantStore()
const driveStore = useDriveStore()

const name = ref('')
const description = ref('')
const memberPublicId = ref('')
const message = ref('')
const errorMessage = ref('')

const selectedGroup = computed(() => driveStore.currentGroup)
const canCreate = computed(() => name.value.trim() !== '' && !driveStore.isBusy && Boolean(tenantStore.activeTenant))

onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
})

watch(
  () => tenantStore.activeTenant?.slug,
  async (slug) => {
    driveStore.currentGroup = null
    message.value = ''
    errorMessage.value = ''
    if (slug) {
      try {
        await driveStore.loadGroups()
      } catch (error) {
        errorMessage.value = toApiErrorMessage(error)
      }
    }
  },
  { immediate: true },
)

async function createGroup() {
  if (!canCreate.value) {
    return
  }
  message.value = ''
  errorMessage.value = ''
  try {
    const group = await driveStore.createGroup(name.value, description.value)
    name.value = ''
    description.value = ''
    await driveStore.loadGroup(group.publicId)
    message.value = 'Drive group を作成しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function selectGroup(group: DriveGroupBody) {
  message.value = ''
  errorMessage.value = ''
  try {
    await driveStore.loadGroup(group.publicId)
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function addMember() {
  if (!selectedGroup.value || memberPublicId.value.trim() === '') {
    return
  }
  message.value = ''
  errorMessage.value = ''
  try {
    await driveStore.addGroupMember(selectedGroup.value, memberPublicId.value)
    memberPublicId.value = ''
    message.value = 'Group member を追加しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}

async function removeMember(userPublicId: string) {
  if (!selectedGroup.value) {
    return
  }
  message.value = ''
  errorMessage.value = ''
  try {
    await driveStore.removeGroupMember(selectedGroup.value, userPublicId)
    message.value = 'Group member を削除しました。'
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  }
}
</script>

<template>
  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">Drive</span>
        <h2>Drive Groups</h2>
      </div>
      <RouterLink class="secondary-button link-button" to="/drive">
        Back to Drive
      </RouterLink>
    </div>

    <p v-if="tenantStore.status === 'empty'" class="warning-message">
      Active tenant がありません。tenant selector で tenant を選択してください。
    </p>
    <p v-if="errorMessage || driveStore.errorMessage" class="error-message">
      {{ errorMessage || driveStore.errorMessage }}
    </p>
    <p v-if="message" class="notice-message">{{ message }}</p>

    <form class="admin-form" @submit.prevent="createGroup">
      <label class="field">
        <span class="field-label">Group name</span>
        <input v-model="name" class="field-input" maxlength="255" autocomplete="off" :disabled="driveStore.isBusy || !tenantStore.activeTenant">
      </label>
      <label class="field">
        <span class="field-label">Description</span>
        <input v-model="description" class="field-input" maxlength="2000" autocomplete="off" :disabled="driveStore.isBusy || !tenantStore.activeTenant">
      </label>
      <div class="action-row form-span">
        <button class="primary-button" type="submit" :disabled="!canCreate">
          Create group
        </button>
      </div>
    </form>

    <div class="split-grid">
      <div class="stack">
        <div class="section-header">
          <div>
            <span class="status-pill">Groups</span>
            <h2>App-managed groups</h2>
          </div>
        </div>
        <div v-if="driveStore.groups.length > 0" class="list-stack">
          <article v-for="group in driveStore.groups" :key="group.publicId" class="list-item">
            <div>
              <strong>{{ group.name }}</strong>
              <span class="cell-subtle">{{ group.description || 'No description' }}</span>
              <span class="cell-subtle monospace-cell">{{ group.publicId }}</span>
            </div>
            <button class="secondary-button compact-button" type="button" @click="selectGroup(group)">
              Members
            </button>
          </article>
        </div>
        <div v-else class="empty-state">
          <p>Drive group はまだありません。</p>
        </div>
      </div>

      <aside class="stack">
        <div class="section-header">
          <div>
            <span class="status-pill">Members</span>
            <h2>{{ selectedGroup?.name ?? 'Select a group' }}</h2>
          </div>
        </div>

        <form class="drive-inline-form" @submit.prevent="addMember">
          <label class="field drive-toolbar-field">
            <span class="field-label">User public ID</span>
            <input v-model="memberPublicId" class="field-input" autocomplete="off" :disabled="!selectedGroup || driveStore.isBusy">
          </label>
          <button class="primary-button compact-button" type="submit" :disabled="!selectedGroup || memberPublicId.trim() === '' || driveStore.isBusy">
            Add
          </button>
        </form>

        <div v-if="selectedGroup?.members?.length" class="list-stack">
          <article v-for="member in selectedGroup.members" :key="member" class="list-item">
            <div>
              <strong class="monospace-cell">{{ member }}</strong>
            </div>
            <button class="secondary-button compact-button danger-button" type="button" :disabled="driveStore.isBusy" @click="removeMember(member)">
              Remove
            </button>
          </article>
        </div>
        <p v-else class="cell-subtle">No members loaded.</p>
      </aside>
    </div>
  </section>
</template>
