<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import AdminAccessDenied from '../../components/AdminAccessDenied.vue'
import ConfirmActionDialog from '../../components/ConfirmActionDialog.vue'
import PageHeader from '../../components/PageHeader.vue'
import SectionSideNav from '../../components/SectionSideNav.vue'
import type { SectionSideNavItem } from '../../components/section-side-nav'
import {
  createTenantAdminDetailContext,
  provideTenantAdminDetailContext,
} from '../../tenant-admin/detail-context'
import { tenantAdminSections, tenantAdminSectionTo } from '../../tenant-admin/sections'

const { t } = useI18n()
const detail = createTenantAdminDetailContext()
provideTenantAdminDetailContext(detail)

const {
  cancelPendingAction,
  commonStore,
  confirmLabel,
  confirmMessage,
  confirmPendingAction,
  confirmTitle,
  errorMessage,
  formatDate,
  message,
  pendingAction,
  store,
  tenant,
  tenantSlug,
} = detail

const navItems = computed<SectionSideNavItem[]>(() => (
  tenantAdminSections.map((section) => ({
    key: section.key,
    label: t(section.labelKey),
    description: t(section.descriptionKey),
    to: tenantAdminSectionTo(section, tenantSlug.value),
    icon: section.icon,
  }))
))

const headerDescription = computed(() => {
  if (!tenant.value) {
    return t('tenantAdmin.descriptionFallback')
  }
  return t('tenantAdmin.description', {
    name: tenant.value.displayName,
    slug: tenant.value.slug,
  })
})
</script>

<template>
  <AdminAccessDenied
    v-if="store.status === 'forbidden'"
    :title="t('tenantAdmin.accessRequiredTitle')"
    :message="t('tenantAdmin.accessRequiredMessage')"
    role-label="tenant_admin"
  />

  <section v-else class="tenant-detail-shell">
    <PageHeader
      :eyebrow="t('tenantAdmin.eyebrow')"
      :title="t('routes.tenantDetail')"
      :description="headerDescription"
    >
      <template #actions>
        <RouterLink class="secondary-button link-button" to="/tenant-admin">
          {{ t('common.back') }}
        </RouterLink>
      </template>
    </PageHeader>

    <p v-if="store.status === 'loading'">
      {{ t('tenantAdmin.loading') }}
    </p>
    <p v-if="errorMessage || store.errorMessage || commonStore.errorMessage" class="error-message">
      {{ errorMessage || store.errorMessage || commonStore.errorMessage }}
    </p>
    <p v-if="message" class="notice-message">
      {{ message }}
    </p>

    <div v-if="tenant" class="tenant-detail-layout">
      <SectionSideNav
        :nav-label="t('tenantAdmin.navigation')"
        :title="tenant.displayName"
        :description="tenant.slug"
        :items="navItems"
      >
        <template #footer>
          <dl class="section-side-nav-footer">
            <div>
              <dt>{{ t('tenantAdmin.activeMembers') }}</dt>
              <dd>{{ tenant.activeMemberCount }}</dd>
            </div>
            <div>
              <dt>{{ t('common.updated') }}</dt>
              <dd>{{ formatDate(tenant.updatedAt) }}</dd>
            </div>
          </dl>
        </template>
      </SectionSideNav>

      <main class="tenant-detail-content">
        <RouterView />
      </main>
    </div>
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
