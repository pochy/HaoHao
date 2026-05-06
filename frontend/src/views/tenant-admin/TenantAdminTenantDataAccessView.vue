<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import { fetchDataPipelines, type DataPipelineBody } from '../../api/data-pipelines'
import { fetchDatasets, fetchDatasetWorkTables } from '../../api/datasets'
import type { DatasetBody, DatasetWorkTableBody, TenantAdminDataAccessGrantBody, TenantAdminDataAccessGroupBody } from '../../api/generated/types.gen'
import {
  addTenantAdminDataAccessGroupMemberItem,
  createTenantAdminDataAccessGroupItem,
  fetchTenantAdminDataAccessGroup,
  fetchTenantAdminDataAccessGroups,
  fetchTenantAdminDataAccessResourcePermissions,
  fetchTenantAdminDataAccessScopePermissions,
  putTenantAdminDataAccessResourcePermission,
  putTenantAdminDataAccessScopePermission,
  removeTenantAdminDataAccessGroupMemberItem,
  type DataAccessResourceType,
} from '../../api/tenant-admin'
import { toApiErrorMessage } from '../../api/client'
import { useTenantAdminDetailContext } from '../../tenant-admin/detail-context'

type ResourceKind = DataAccessResourceType | 'scope'
type SubjectType = 'user' | 'group'
type ResourceOption = {
  type: ResourceKind
  publicId: string
  label: string
  sublabel: string
}

const scopeActions = [
  { key: 'can_view', label: '全 Dataset / Work table / Pipeline を閲覧' },
  { key: 'can_create_dataset', label: 'Dataset 作成' },
  { key: 'can_create_work_table', label: 'Work table 作成' },
  { key: 'can_create_pipeline', label: 'Pipeline 作成' },
]

const resourceActions = [
  { key: 'can_view', label: '閲覧' },
  { key: 'can_preview', label: 'Preview' },
  { key: 'can_query', label: 'Query / input 利用' },
  { key: 'can_export', label: 'Export' },
  { key: 'can_update', label: '更新' },
  { key: 'can_delete', label: '削除' },
  { key: 'can_manage_permissions', label: '権限管理' },
]

const pipelineActions = [
  { key: 'can_view', label: '閲覧' },
  { key: 'can_preview', label: 'Preview' },
  { key: 'can_run', label: 'Run' },
  { key: 'can_update', label: '更新' },
  { key: 'can_save_version', label: 'Version 保存' },
  { key: 'can_publish_version', label: 'Publish' },
  { key: 'can_manage_schedule', label: 'Schedule 管理' },
  { key: 'can_delete', label: '削除' },
  { key: 'can_manage_permissions', label: '権限管理' },
]

const detail = useTenantAdminDetailContext()
const { formatDate, store, tenant } = detail

const groups = ref<TenantAdminDataAccessGroupBody[]>([])
const selectedGroupPublicId = ref('')
const selectedGroup = ref<TenantAdminDataAccessGroupBody | null>(null)
const datasets = ref<DatasetBody[]>([])
const workTables = ref<DatasetWorkTableBody[]>([])
const pipelines = ref<DataPipelineBody[]>([])
const selectedResourceKey = ref('scope:')
const grants = ref<TenantAdminDataAccessGrantBody[]>([])
const subjectType = ref<SubjectType>('group')
const selectedSubjectPublicId = ref('')
const selectedActions = ref<string[]>([])
const newGroupName = ref('')
const newGroupDescription = ref('')
const memberUserPublicId = ref('')
const loading = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const errorHint = ref('')
const message = ref('')

const memberships = computed(() => store.current?.memberships ?? [])

const subjectOptions = computed(() => (
  subjectType.value === 'group'
    ? groups.value.map((group) => ({
      publicId: group.publicId,
      label: group.name,
      sublabel: group.systemKey ? group.systemKey : group.publicId,
    }))
    : memberships.value.map((member) => ({
      publicId: member.userPublicId,
      label: member.displayName || member.email,
      sublabel: member.email,
    }))
))

const resourceOptions = computed<ResourceOption[]>(() => [
  {
    type: 'scope',
    publicId: '',
    label: 'Tenant-wide scope',
    sublabel: '全体の作成権限と全リソース閲覧',
  },
  ...datasets.value.map((item) => ({
    type: 'dataset' as const,
    publicId: item.publicId,
    label: item.name,
    sublabel: `Dataset / ${item.status}`,
  })),
  ...workTables.value.map((item) => ({
    type: 'work_table' as const,
    publicId: item.publicId ?? '',
    label: item.displayName || item.table,
    sublabel: `Work table / ${item.database}.${item.table}`,
  })).filter((item) => item.publicId),
  ...pipelines.value.map((item) => ({
    type: 'data_pipeline' as const,
    publicId: item.publicId,
    label: item.name,
    sublabel: `Pipeline / ${item.status}`,
  })),
])

const selectedResource = computed(() => (
  resourceOptions.value.find((item) => resourceKey(item) === selectedResourceKey.value) ?? resourceOptions.value[0]
))

const availableActions = computed(() => {
  if (selectedResource.value.type === 'scope') {
    return scopeActions
  }
  if (selectedResource.value.type === 'data_pipeline') {
    return pipelineActions
  }
  return resourceActions
})

const grantsForSelectedSubject = computed(() => {
  const subjectId = selectedSubjectPublicId.value
  return grants.value.filter((grant) => (
    subjectType.value === 'group'
      ? grant.subjectGroupPublicId === subjectId
      : grant.subjectUserPublicId === subjectId
  ))
})

watch(selectedResourceKey, () => {
  void loadPermissions()
})

watch([subjectType, selectedSubjectPublicId, grants], () => {
  selectedActions.value = grantsForSelectedSubject.value.map((grant) => grant.action)
})

watch(subjectOptions, (options) => {
  if (!options.some((item) => item.publicId === selectedSubjectPublicId.value)) {
    selectedSubjectPublicId.value = options[0]?.publicId ?? ''
  }
}, { immediate: true })

watch(selectedGroupPublicId, () => {
  void loadSelectedGroup()
})

onMounted(() => {
  void loadAll()
})

function resourceKey(resource: ResourceOption) {
  return `${resource.type}:${resource.publicId}`
}

function actionSelected(action: string) {
  return selectedActions.value.includes(action)
}

function toggleAction(action: string, checked: boolean) {
  selectedActions.value = checked
    ? Array.from(new Set([...selectedActions.value, action]))
    : selectedActions.value.filter((item) => item !== action)
}

function handleActionChange(action: string, event: Event) {
  toggleAction(action, Boolean((event.target as HTMLInputElement | null)?.checked))
}

async function loadAll() {
  const slug = tenant.value?.slug
  if (!slug) {
    return
  }
  loading.value = true
  errorMessage.value = ''
  errorHint.value = ''
  try {
    const [loadedGroups, loadedDatasets, loadedWorkTables, loadedPipelines] = await Promise.all([
      fetchTenantAdminDataAccessGroups(slug),
      fetchDatasets(),
      fetchDatasetWorkTables(),
      fetchDataPipelines({ limit: 100 }),
    ])
    groups.value = loadedGroups
    datasets.value = loadedDatasets
    workTables.value = loadedWorkTables
    pipelines.value = loadedPipelines.items
    selectedGroupPublicId.value = selectedGroupPublicId.value || loadedGroups[0]?.publicId || ''
    await Promise.all([
      loadSelectedGroup(),
      loadPermissions(),
    ])
  } catch (error) {
    setErrorMessage(error)
  } finally {
    loading.value = false
  }
}

async function loadSelectedGroup() {
  const slug = tenant.value?.slug
  if (!slug || !selectedGroupPublicId.value) {
    selectedGroup.value = null
    return
  }
  try {
    selectedGroup.value = await fetchTenantAdminDataAccessGroup(slug, selectedGroupPublicId.value)
  } catch (error) {
    setErrorMessage(error)
  }
}

async function loadPermissions() {
  const slug = tenant.value?.slug
  const resource = selectedResource.value
  if (!slug || !resource) {
    return
  }
  try {
    grants.value = resource.type === 'scope'
      ? await fetchTenantAdminDataAccessScopePermissions(slug)
      : await fetchTenantAdminDataAccessResourcePermissions(slug, resource.type, resource.publicId)
  } catch (error) {
    setErrorMessage(error)
  }
}

async function createGroup() {
  const slug = tenant.value?.slug
  if (!slug || !newGroupName.value.trim()) {
    return
  }
  saving.value = true
  errorMessage.value = ''
  errorHint.value = ''
  try {
    const group = await createTenantAdminDataAccessGroupItem(slug, {
      name: newGroupName.value.trim(),
      description: newGroupDescription.value.trim(),
    })
    groups.value = [group, ...groups.value.filter((item) => item.publicId !== group.publicId)]
    selectedGroupPublicId.value = group.publicId
    newGroupName.value = ''
    newGroupDescription.value = ''
    message.value = 'Group を作成しました。'
  } catch (error) {
    setErrorMessage(error)
  } finally {
    saving.value = false
  }
}

async function addMember() {
  const slug = tenant.value?.slug
  if (!slug || !selectedGroupPublicId.value || !memberUserPublicId.value) {
    return
  }
  saving.value = true
  errorMessage.value = ''
  errorHint.value = ''
  try {
    await addTenantAdminDataAccessGroupMemberItem(slug, selectedGroupPublicId.value, {
      userPublicId: memberUserPublicId.value,
    })
    await loadSelectedGroup()
    message.value = 'Member を追加しました。'
  } catch (error) {
    setErrorMessage(error)
  } finally {
    saving.value = false
  }
}

async function removeMember(userPublicId: string) {
  const slug = tenant.value?.slug
  if (!slug || !selectedGroupPublicId.value) {
    return
  }
  saving.value = true
  errorMessage.value = ''
  errorHint.value = ''
  try {
    await removeTenantAdminDataAccessGroupMemberItem(slug, selectedGroupPublicId.value, userPublicId)
    await loadSelectedGroup()
    message.value = 'Member を削除しました。'
  } catch (error) {
    setErrorMessage(error)
  } finally {
    saving.value = false
  }
}

async function savePermission() {
  const slug = tenant.value?.slug
  const resource = selectedResource.value
  if (!slug || !resource || !selectedSubjectPublicId.value) {
    return
  }
  saving.value = true
  errorMessage.value = ''
  errorHint.value = ''
  try {
    const body = {
      subjectType: subjectType.value,
      subjectPublicId: selectedSubjectPublicId.value,
      actions: selectedActions.value,
    }
    if (resource.type === 'scope') {
      await putTenantAdminDataAccessScopePermission(slug, body)
    } else {
      await putTenantAdminDataAccessResourcePermission(slug, resource.type, resource.publicId, body)
    }
    await loadPermissions()
    message.value = 'Permission を保存しました。'
  } catch (error) {
    setErrorMessage(error)
  } finally {
    saving.value = false
  }
}

function setErrorMessage(error: unknown) {
  const message = toApiErrorMessage(error)
  const lowerMessage = message.toLowerCase()
  if (
    lowerMessage.includes('data resource authorization unavailable')
    || lowerMessage.includes('dataset_group')
    || lowerMessage.includes('data_scope')
    || lowerMessage.includes('authorization model')
    || lowerMessage.includes('openfga')
  ) {
    errorMessage.value = 'Data Access の権限モデルが未更新です。'
    errorHint.value = 'OpenFGA に最新の openfga/drive.fga を登録し、.env の OPENFGA_AUTHORIZATION_MODEL_ID を更新して backend を再起動してください。'
    return
  }
  errorMessage.value = message
  errorHint.value = ''
}

function subjectLabel(grant: TenantAdminDataAccessGrantBody) {
  if (grant.subjectType === 'group') {
    return grant.subjectGroupName || grant.subjectGroupPublicId
  }
  return grant.subjectUserName || grant.subjectUserEmail || grant.subjectUserPublicId
}
</script>

<template>
  <div class="stack">
    <section class="panel stack">
      <div class="section-header">
        <div>
          <span class="status-pill">Data Access</span>
          <h2>Dataset / Work table / Pipeline permissions</h2>
          <p class="cell-subtle">
            Tenant-wide scope と個別リソース単位で user / group に action を付与します。
          </p>
        </div>
        <button class="secondary-button compact-button" type="button" :disabled="loading" @click="loadAll">
          Refresh
        </button>
      </div>

      <div v-if="errorMessage" class="data-access-error" role="alert">
        <strong>{{ errorMessage }}</strong>
        <p v-if="errorHint">{{ errorHint }}</p>
      </div>
      <p v-if="message" class="notice-message">{{ message }}</p>
      <p v-if="loading">{{ 'Loading...' }}</p>
    </section>

    <section class="panel stack">
      <div class="section-header">
        <div>
          <span class="status-pill">Groups</span>
          <h2>Permission groups</h2>
        </div>
      </div>

      <form class="admin-form" @submit.prevent="createGroup">
        <label class="field">
          <span class="field-label">Group name</span>
          <input v-model="newGroupName" class="field-input" type="text" placeholder="Analysts">
        </label>
        <label class="field">
          <span class="field-label">Description</span>
          <input v-model="newGroupDescription" class="field-input" type="text" placeholder="Dataset viewers">
        </label>
        <div class="action-row form-span">
          <button class="primary-button" type="submit" :disabled="saving || !newGroupName.trim()">
            Create group
          </button>
        </div>
      </form>

      <div class="data-access-grid">
        <div class="list-stack">
          <button
            v-for="group in groups"
            :key="group.publicId"
            class="data-access-list-button"
            :class="{ active: group.publicId === selectedGroupPublicId }"
            type="button"
            @click="selectedGroupPublicId = group.publicId"
          >
            <strong>{{ group.name }}</strong>
            <span>{{ group.systemKey || group.publicId }}</span>
          </button>
        </div>

        <div class="stack">
          <div v-if="selectedGroup" class="admin-table">
            <div class="section-header">
              <div>
                <h3>{{ selectedGroup.name }}</h3>
                <p class="cell-subtle">{{ selectedGroup.description || selectedGroup.publicId }}</p>
              </div>
            </div>

            <form class="data-access-inline-form" @submit.prevent="addMember">
              <label class="field">
                <span class="field-label">Add member</span>
                <select v-model="memberUserPublicId" class="field-input">
                  <option value="">Select tenant member</option>
                  <option v-for="member in memberships" :key="member.userPublicId" :value="member.userPublicId">
                    {{ member.displayName || member.email }} / {{ member.email }}
                  </option>
                </select>
              </label>
              <button class="secondary-button" type="submit" :disabled="saving || !memberUserPublicId">
                Add
              </button>
            </form>

            <table>
              <thead>
                <tr>
                  <th>Member</th>
                  <th>Added</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="member in selectedGroup.members ?? []" :key="member.userPublicId">
                  <td>
                    <strong>{{ member.displayName || member.email }}</strong>
                    <span class="cell-subtle">{{ member.email }}</span>
                  </td>
                  <td>{{ formatDate(member.createdAt) }}</td>
                  <td>
                    <button class="secondary-button compact-button" type="button" :disabled="saving" @click="removeMember(member.userPublicId)">
                      Remove
                    </button>
                  </td>
                </tr>
                <tr v-if="(selectedGroup.members ?? []).length === 0">
                  <td colspan="3">No members.</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </section>

    <section class="panel stack">
      <div class="section-header">
        <div>
          <span class="status-pill">Permissions</span>
          <h2>Scope and resource grants</h2>
        </div>
      </div>

      <div class="data-access-permission-layout">
        <label class="field">
          <span class="field-label">Resource</span>
          <select v-model="selectedResourceKey" class="field-input">
            <option v-for="resource in resourceOptions" :key="resourceKey(resource)" :value="resourceKey(resource)">
              {{ resource.label }} - {{ resource.sublabel }}
            </option>
          </select>
        </label>

        <label class="field">
          <span class="field-label">Subject type</span>
          <select v-model="subjectType" class="field-input">
            <option value="group">Group</option>
            <option value="user">User</option>
          </select>
        </label>

        <label class="field">
          <span class="field-label">Subject</span>
          <select v-model="selectedSubjectPublicId" class="field-input">
            <option v-for="subject in subjectOptions" :key="subject.publicId" :value="subject.publicId">
              {{ subject.label }} - {{ subject.sublabel }}
            </option>
          </select>
        </label>
      </div>

      <div class="data-access-action-grid">
        <label v-for="action in availableActions" :key="action.key" class="checkbox-field">
          <input
            type="checkbox"
            :checked="actionSelected(action.key)"
            @change="handleActionChange(action.key, $event)"
          >
          <span>{{ action.label }}</span>
        </label>
      </div>

      <div class="action-row">
        <button class="primary-button" type="button" :disabled="saving || !selectedSubjectPublicId" @click="savePermission">
          Save permission
        </button>
      </div>

      <div class="admin-table">
        <table>
          <thead>
            <tr>
              <th>Subject</th>
              <th>Action</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="grant in grants" :key="`${grant.subjectType}:${subjectLabel(grant)}:${grant.action}`">
              <td>
                <strong>{{ subjectLabel(grant) }}</strong>
                <span class="cell-subtle">{{ grant.subjectType }}</span>
              </td>
              <td><code>{{ grant.action }}</code></td>
              <td>{{ formatDate(grant.createdAt) }}</td>
            </tr>
            <tr v-if="grants.length === 0">
              <td colspan="3">No permissions configured for this target.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
