<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { toApiErrorMessage, toApiErrorStatus } from '../api/client'
import { acceptTenantInvitationItem, resolveTenantInvitationItem, setupTenantInvitationIdentityItem } from '../api/tenant-invitations'
import type { TenantInvitationBody } from '../api/generated/types.gen'
import { clearPendingTenantInvitationToken, readPendingTenantInvitationToken, savePendingTenantInvitationToken } from '../invitations/pending-token'
import { useSessionStore } from '../stores/session'

const route = useRoute()
const router = useRouter()
const sessionStore = useSessionStore()
const token = ref(String(route.query.token ?? ''))
const accepting = ref(false)
const setupLoading = ref(false)
const loading = ref(true)
const message = ref('')
const errorMessage = ref('')
const setupMessage = ref('')
const invitation = ref<TenantInvitationBody | null>(null)
const identitySetupInvitation = ref<TenantInvitationBody | null>(null)
const identityReady = ref(false)

const canAccept = computed(() => token.value.trim() !== '' && !accepting.value)
const isAuthenticated = computed(() => sessionStore.status === 'authenticated')
const tenantLabel = computed(() => invitation.value?.tenantDisplayName || invitation.value?.tenantSlug || String(invitation.value?.tenantId ?? ''))
const latestIdentitySetup = computed(() => identitySetupInvitation.value ?? invitation.value)
const hasIdentitySetupDetails = computed(() => Boolean(latestIdentitySetup.value?.identitySetupInviteCode || latestIdentitySetup.value?.identitySetupLoginUrl))

onMounted(async () => {
  if (!token.value.trim()) {
    token.value = readPendingTenantInvitationToken()
  }
  await sessionStore.bootstrap()
  await loadInvitation()
  if (isAuthenticated.value && canAccept.value && invitation.value?.status === 'pending') {
    await acceptInvitation()
  }
})

async function loadInvitation() {
  if (!token.value.trim()) {
    loading.value = false
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    invitation.value = await resolveTenantInvitationItem(token.value.trim())
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  } finally {
    loading.value = false
  }
}

function signInToAccept() {
  savePendingTenantInvitationToken(token.value)
  const returnTo = router.resolve({ name: 'invitation-accept', query: { token: token.value.trim() } }).fullPath
  const params = new URLSearchParams({ returnTo })
  if (invitation.value?.inviteeEmailNormalized) {
    params.set('loginHint', invitation.value.inviteeEmailNormalized)
  }
  window.location.href = `/api/v1/auth/login?${params.toString()}`
}

async function beginIdentitySetup() {
  if (!token.value.trim() || !invitation.value?.publicId) {
    return
  }
  setupLoading.value = true
  setupMessage.value = ''
  errorMessage.value = ''
  savePendingTenantInvitationToken(token.value)
  try {
    const updated = await setupTenantInvitationIdentityItem(invitation.value.publicId, token.value.trim())
    identitySetupInvitation.value = {
      ...invitation.value,
      ...updated,
      tenantSlug: invitation.value.tenantSlug || updated.tenantSlug,
      tenantDisplayName: invitation.value.tenantDisplayName || updated.tenantDisplayName,
    }
    if (updated.identitySetupInviteCode || updated.identitySetupLoginUrl) {
      setupMessage.value = '最新の Zitadel 初期設定コードを発行しました。Zitadel 画面では下の最新コードを入力してください。'
    } else {
      identityReady.value = true
      setupMessage.value = 'Zitadel アカウントは初期設定済みです。HaoHao にサインインして招待を承諾してください。'
    }
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  } finally {
    setupLoading.value = false
  }
}

async function acceptInvitation() {
  if (!canAccept.value) {
    return
  }
  accepting.value = true
  message.value = ''
  errorMessage.value = ''
  try {
    const acceptedInvitation = await acceptTenantInvitationItem(token.value.trim())
    invitation.value = {
      ...invitation.value,
      ...acceptedInvitation,
      tenantSlug: invitation.value?.tenantSlug || acceptedInvitation.tenantSlug,
      tenantDisplayName: invitation.value?.tenantDisplayName || acceptedInvitation.tenantDisplayName,
    }
    message.value = `Invitation accepted for ${invitation.value.tenantDisplayName || invitation.value.tenantSlug || `tenant ${invitation.value.tenantId}`}.`
    clearPendingTenantInvitationToken()
    await sessionStore.bootstrap()
  } catch (error) {
    if (toApiErrorStatus(error) === 401) {
      signInToAccept()
      return
    }
    errorMessage.value = toApiErrorMessage(error)
  } finally {
    accepting.value = false
  }
}
</script>

<template>
  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">Invitation</span>
        <h2>Accept Tenant Invitation</h2>
      </div>
    </div>

    <p v-if="loading">
      Loading invitation...
    </p>

    <p v-if="errorMessage" class="error-message">
      {{ errorMessage }}
    </p>
    <p v-if="message" class="notice-message">
      {{ message }}
    </p>
    <p v-if="setupMessage" class="notice-message">
      {{ setupMessage }}
    </p>

    <div v-if="invitation" class="detail-list">
      <div>
        <strong>Tenant</strong>
        <p>{{ tenantLabel }}</p>
      </div>
      <div>
        <strong>Invitee</strong>
        <p>{{ invitation.inviteeEmailNormalized }}</p>
      </div>
      <div>
        <strong>Roles</strong>
        <p>{{ invitation.roleCodes?.join(', ') || 'None' }}</p>
      </div>
      <div>
        <strong>Status</strong>
        <p>{{ invitation.status }}</p>
      </div>
    </div>

    <form v-if="!message" class="admin-form" @submit.prevent="acceptInvitation">
      <p v-if="!isAuthenticated && invitation?.status === 'pending'" class="cell-subtle form-span">
        初回は HaoHao から Zitadel アカウント初期設定を開始してください。Zitadel 側の Resend code は使わず、HaoHao が表示する最新コードを Zitadel の画面で入力します。
      </p>

      <div v-if="!isAuthenticated && invitation?.status === 'pending'" class="form-span stack">
        <p v-if="latestIdentitySetup?.identitySetupInviteCode" class="cell-subtle">
          Step 1 Zitadel アカウント初期設定コード: <strong>{{ latestIdentitySetup.identitySetupInviteCode }}</strong>
        </p>
        <a
          v-if="latestIdentitySetup?.identitySetupLoginUrl"
          class="primary-button"
          :href="latestIdentitySetup.identitySetupLoginUrl"
          @click="savePendingTenantInvitationToken(token)"
        >
          Zitadel アカウント初期設定を開く
        </a>
        <button
          v-else-if="!identityReady"
          class="primary-button"
          type="button"
          :disabled="setupLoading || !canAccept"
          @click="beginIdentitySetup"
        >
          {{ setupLoading ? '準備中...' : 'Zitadel アカウント初期設定を開始' }}
        </button>
        <p v-if="hasIdentitySetupDetails" class="cell-subtle">
          Step 2 初期設定が完了したら HaoHao に戻り、サインインしてこの招待を承諾します。
        </p>
      </div>

      <details v-if="token" class="form-span token-details">
        <summary>Invitation token</summary>
        <input :value="token" class="field-input" autocomplete="off" readonly>
      </details>

      <label v-else class="field form-span">
        <span class="field-label">Invitation token</span>
        <input v-model="token" class="field-input" autocomplete="off" required>
      </label>

      <div class="action-row form-span">
        <button v-if="isAuthenticated" class="primary-button" type="submit" :disabled="!canAccept">
          {{ accepting ? 'Accepting...' : 'Accept' }}
        </button>
        <button v-else-if="identityReady || hasIdentitySetupDetails" class="primary-button" type="button" :disabled="!canAccept" @click="signInToAccept">
          Sign in to accept
        </button>
      </div>
    </form>
  </section>
</template>
