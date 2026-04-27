<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { toApiErrorMessage } from '../api/client'
import { refreshCurrentSession } from '../api/session'
import DataCard from '../components/DataCard.vue'
import DocsLink from '../components/DocsLink.vue'
import MetricTile from '../components/MetricTile.vue'
import PageHeader from '../components/PageHeader.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { useSessionStore } from '../stores/session'

const router = useRouter()
const sessionStore = useSessionStore()
const { t } = useI18n()

const userJson = computed(() => JSON.stringify(sessionStore.user, null, 2))
const userEmail = computed(() => sessionStore.user?.email ?? t('common.noEmail'))
const userId = computed(() => sessionStore.user?.publicId ?? t('auth.status.anonymous'))
const refreshing = ref(false)
const refreshMessage = ref('')
const refreshErrorMessage = ref('')

async function signOut() {
  try {
    const postLogoutURL = await sessionStore.logout()
    if (postLogoutURL) {
      window.location.assign(postLogoutURL)
      return
    }
    await router.push({ name: 'login' })
  } catch {
    // The store exposes the error message for the current view.
  }
}

async function rotateSession() {
  refreshing.value = true
  refreshMessage.value = ''
  refreshErrorMessage.value = ''

  try {
    await refreshCurrentSession()
    refreshMessage.value = t('home.refreshSuccess')
  } catch (error) {
    refreshErrorMessage.value = toApiErrorMessage(error)
  } finally {
    refreshing.value = false
  }
}
</script>

<template>
  <section class="stack">
    <PageHeader
      :eyebrow="t('home.eyebrow')"
      :title="t('home.title')"
      :description="t('home.description')"
    >
      <template #actions>
        <button class="secondary-button" :disabled="refreshing" type="button" @click="rotateSession">
          {{ refreshing ? t('common.refreshing') : t('home.refreshSession') }}
        </button>
        <button class="secondary-button" type="button" @click="signOut">
          {{ t('home.logout') }}
        </button>
      </template>
    </PageHeader>

    <div class="metric-grid">
      <MetricTile :label="t('common.status')" :value="sessionStore.status" :hint="t('home.statusHint')" />
      <MetricTile :label="t('common.user')" :value="sessionStore.user?.displayName ?? t('auth.guest')" :hint="userEmail" />
      <MetricTile :label="t('common.userPublicId')" :value="userId" :hint="t('home.userPublicIdHint')" />
      <MetricTile :label="t('home.supportAccess')" :value="sessionStore.supportAccess ? t('common.active') : t('common.off')" :hint="t('home.supportAccessHint')" />
    </div>

    <div class="split-grid">
      <DataCard :title="t('home.currentSession')" :subtitle="t('home.currentSessionSubtitle')">
        <StatusBadge tone="success">{{ t('auth.status.authenticated') }}</StatusBadge>
        <pre class="json-card">{{ userJson }}</pre>

        <div class="action-row">
          <RouterLink class="secondary-button link-button" to="/todos">
            {{ t('home.openTodo') }}
          </RouterLink>
          <DocsLink />
        </div>

        <p v-if="refreshMessage">{{ refreshMessage }}</p>
        <p v-if="refreshErrorMessage" class="error-message">
          {{ refreshErrorMessage }}
        </p>
        <p v-if="sessionStore.errorMessage" class="error-message">
          {{ sessionStore.errorMessage }}
        </p>
      </DataCard>

      <DataCard :title="t('home.verification')" :subtitle="t('home.verificationSubtitle')">
        <ul class="check-list">
          <li>{{ t('home.verificationItems.cookie') }}</li>
          <li>{{ t('home.verificationItems.session') }}</li>
          <li>{{ t('home.verificationItems.refresh') }}</li>
          <li>{{ t('home.verificationItems.docs') }}</li>
        </ul>
      </DataCard>
    </div>
  </section>
</template>

<style scoped>
.check-list {
  margin: 0;
  padding-left: 1.2rem;
}
</style>
