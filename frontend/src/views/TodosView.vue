<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import type { TodoBody } from '../api/generated/types.gen'
import { toApiErrorMessage } from '../api/client'
import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import { useTenantStore } from '../stores/tenants'
import { useTodoStore } from '../stores/todos'

const tenantStore = useTenantStore()
const todoStore = useTodoStore()

const newTitle = ref('')
const drafts = ref<Record<string, string>>({})
const actionErrorMessage = ref('')

const activeTenantLabel = computed(() => (
  tenantStore.activeTenant
    ? `${tenantStore.activeTenant.displayName} / ${tenantStore.activeTenant.slug}`
    : 'None'
))

const canCreate = computed(() => (
  newTitle.value.trim() !== '' &&
  todoStore.status !== 'loading' &&
  !todoStore.creating
))

onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
})

watch(
  () => tenantStore.activeTenant?.slug,
  async (slug) => {
    actionErrorMessage.value = ''
    drafts.value = {}
    todoStore.reset()

    if (slug) {
      await todoStore.load()
    }
  },
  { immediate: true },
)

watch(
  () => todoStore.items,
  (items) => {
    const nextDrafts: Record<string, string> = {}
    for (const item of items) {
      nextDrafts[item.publicId] = drafts.value[item.publicId] ?? item.title
    }
    drafts.value = nextDrafts
  },
  { deep: true, immediate: true },
)

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

async function createTodo() {
  if (!canCreate.value) {
    return
  }

  actionErrorMessage.value = ''

  try {
    await todoStore.create(newTitle.value)
    newTitle.value = ''
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function toggleTodo(item: TodoBody) {
  actionErrorMessage.value = ''

  try {
    await todoStore.toggle(item.publicId, !item.completed)
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function renameTodo(item: TodoBody) {
  const draft = drafts.value[item.publicId]?.trim() ?? ''
  if (!draft || draft === item.title) {
    drafts.value[item.publicId] = item.title
    return
  }

  actionErrorMessage.value = ''

  try {
    await todoStore.rename(item.publicId, draft)
  } catch (error) {
    drafts.value[item.publicId] = item.title
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}

async function removeTodo(item: TodoBody) {
  actionErrorMessage.value = ''

  try {
    await todoStore.remove(item.publicId)
  } catch (error) {
    actionErrorMessage.value = toApiErrorMessage(error)
  }
}
</script>

<template>
  <AdminAccessDenied
    v-if="todoStore.status === 'forbidden'"
    title="TODO role required"
    message="この画面を使うには active tenant の todo_user role が必要です。"
    role-label="todo_user"
  />

  <section v-else class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">Tenant TODO</span>
        <h2>TODO</h2>
      </div>
      <button
        class="secondary-button"
        :disabled="todoStore.status === 'loading' || !tenantStore.activeTenant"
        type="button"
        @click="todoStore.load()"
      >
        {{ todoStore.status === 'loading' ? 'Refreshing...' : 'Refresh' }}
      </button>
    </div>

    <dl class="metadata-grid">
      <div>
        <dt>Active tenant</dt>
        <dd>{{ activeTenantLabel }}</dd>
      </div>
      <div>
        <dt>Items</dt>
        <dd>{{ todoStore.items.length }}</dd>
      </div>
    </dl>

    <p v-if="tenantStore.status === 'empty'" class="warning-message">
      Active tenant がありません。tenant membership を seed してから再ログインしてください。
    </p>
    <p v-if="tenantStore.status === 'error'" class="error-message">
      {{ tenantStore.errorMessage }}
    </p>
    <p v-if="actionErrorMessage || todoStore.errorMessage" class="error-message">
      {{ actionErrorMessage || todoStore.errorMessage }}
    </p>

    <form class="todo-form" @submit.prevent="createTodo">
      <label class="field todo-title-field">
        <span class="field-label">New TODO</span>
        <input
          v-model="newTitle"
          class="field-input"
          autocomplete="off"
          maxlength="200"
          placeholder="Follow up with customer"
          :disabled="!tenantStore.activeTenant || todoStore.creating"
        >
      </label>
      <button class="primary-button" :disabled="!canCreate" type="submit">
        {{ todoStore.creating ? 'Adding...' : 'Add' }}
      </button>
    </form>

    <p v-if="todoStore.status === 'loading'" class="todo-loading">
      Loading TODOs...
    </p>

    <div v-else-if="todoStore.items.length > 0" class="todo-list">
      <article v-for="item in todoStore.items" :key="item.publicId" class="todo-item">
        <label class="todo-check">
          <input
            :checked="item.completed"
            :disabled="todoStore.updatingPublicId === item.publicId"
            type="checkbox"
            @change="toggleTodo(item)"
          >
          <span>{{ item.completed ? 'Done' : 'Open' }}</span>
        </label>

        <form class="inline-edit-form" @submit.prevent="renameTodo(item)">
          <input
            v-model="drafts[item.publicId]"
            class="field-input"
            maxlength="200"
            autocomplete="off"
            :class="{ completed: item.completed }"
            :disabled="todoStore.updatingPublicId === item.publicId"
          >
          <button
            class="secondary-button"
            :disabled="todoStore.updatingPublicId === item.publicId"
            type="submit"
          >
            Save
          </button>
        </form>

        <div class="todo-meta">
          Created {{ formatDate(item.createdAt) }}
        </div>

        <button
          class="secondary-button danger-button"
          :disabled="todoStore.deletingPublicId === item.publicId"
          type="button"
          @click="removeTodo(item)"
        >
          {{ todoStore.deletingPublicId === item.publicId ? 'Deleting...' : 'Delete' }}
        </button>
      </article>
    </div>

    <div v-else-if="todoStore.status === 'empty'" class="empty-state">
      <p>この tenant の TODO はまだありません。</p>
    </div>
  </section>
</template>
