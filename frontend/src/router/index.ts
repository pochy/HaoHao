import { createRouter, createWebHistory } from 'vue-router'

import { useSessionStore } from '../stores/session'
import HomeView from '../views/HomeView.vue'
import IntegrationsView from '../views/IntegrationsView.vue'
import LoginView from '../views/LoginView.vue'
import MachineClientDetailView from '../views/MachineClientDetailView.vue'
import MachineClientFormView from '../views/MachineClientFormView.vue'
import MachineClientsView from '../views/MachineClientsView.vue'
import InvitationAcceptView from '../views/InvitationAcceptView.vue'
import NotificationsView from '../views/NotificationsView.vue'
import CustomerSignalDetailView from '../views/CustomerSignalDetailView.vue'
import CustomerSignalsView from '../views/CustomerSignalsView.vue'
import DriveGroupsView from '../views/DriveGroupsView.vue'
import DriveView from '../views/DriveView.vue'
import TenantAdminTenantFormView from '../views/TenantAdminTenantFormView.vue'
import TenantAdminTenantsView from '../views/TenantAdminTenantsView.vue'
import TenantAdminTenantDataView from '../views/tenant-admin/TenantAdminTenantDataView.vue'
import TenantAdminTenantDriveOperationsView from '../views/tenant-admin/TenantAdminTenantDriveOperationsView.vue'
import TenantAdminTenantDrivePolicyView from '../views/tenant-admin/TenantAdminTenantDrivePolicyView.vue'
import TenantAdminTenantEntitlementsView from '../views/tenant-admin/TenantAdminTenantEntitlementsView.vue'
import TenantAdminTenantInvitationsView from '../views/tenant-admin/TenantAdminTenantInvitationsView.vue'
import TenantAdminTenantMembersView from '../views/tenant-admin/TenantAdminTenantMembersView.vue'
import TenantAdminTenantOverviewView from '../views/tenant-admin/TenantAdminTenantOverviewView.vue'
import TenantAdminTenantSettingsView from '../views/tenant-admin/TenantAdminTenantSettingsView.vue'
import TenantAdminTenantShellView from '../views/tenant-admin/TenantAdminTenantShellView.vue'
import TenantAdminTenantSupportView from '../views/tenant-admin/TenantAdminTenantSupportView.vue'
import TenantAdminTenantWebhooksView from '../views/tenant-admin/TenantAdminTenantWebhooksView.vue'
import TodosView from '../views/TodosView.vue'
import PublicDriveShareView from '../views/PublicDriveShareView.vue'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    title?: string
    group?: string
    titleKey?: string
    groupKey?: string
  }
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
      meta: {
        requiresAuth: true,
        title: 'Session',
        group: 'Workspace',
        titleKey: 'nav.items.session',
        groupKey: 'nav.groups.workspace',
      },
    },
    {
      path: '/login',
      name: 'login',
      component: LoginView,
      meta: {
        title: 'Login',
        group: 'Authentication',
        titleKey: 'routes.login',
        groupKey: 'nav.groups.authentication',
      },
    },
    {
      path: '/integrations',
      name: 'integrations',
      component: IntegrationsView,
      meta: {
        requiresAuth: true,
        title: 'Integrations',
        group: 'Admin',
        titleKey: 'nav.items.integrations',
        groupKey: 'nav.groups.admin',
      },
    },
    {
      path: '/notifications',
      name: 'notifications',
      component: NotificationsView,
      meta: {
        requiresAuth: true,
        title: 'Notifications',
        group: 'Workspace',
        titleKey: 'nav.items.notifications',
        groupKey: 'nav.groups.workspace',
      },
    },
    {
      path: '/invitations/accept',
      name: 'invitation-accept',
      component: InvitationAcceptView,
      meta: {
        requiresAuth: true,
        title: 'Invitation',
        group: 'Workspace',
        titleKey: 'routes.invitation',
        groupKey: 'nav.groups.workspace',
      },
    },
    {
      path: '/todos',
      name: 'todos',
      component: TodosView,
      meta: {
        requiresAuth: true,
        title: 'TODO',
        group: 'Work',
        titleKey: 'nav.items.todos',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/customer-signals',
      name: 'customer-signals',
      component: CustomerSignalsView,
      meta: {
        requiresAuth: true,
        title: 'Signals',
        group: 'Work',
        titleKey: 'nav.items.signals',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/customer-signals/:signalPublicId',
      name: 'customer-signal-detail',
      component: CustomerSignalDetailView,
      meta: {
        requiresAuth: true,
        title: 'Signal Detail',
        group: 'Work',
        titleKey: 'routes.signalDetail',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/drive',
      name: 'drive',
      component: DriveView,
      meta: {
        requiresAuth: true,
        title: 'Drive',
        group: 'Work',
        titleKey: 'nav.items.drive',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/drive/folders/:folderPublicId',
      name: 'drive-folder',
      component: DriveView,
      meta: {
        requiresAuth: true,
        title: 'Drive Folder',
        group: 'Work',
        titleKey: 'routes.driveFolder',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/drive/files/:filePublicId',
      name: 'drive-file-detail',
      component: DriveView,
      meta: {
        requiresAuth: true,
        title: 'Drive File',
        group: 'Work',
        titleKey: 'routes.driveFile',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/drive/search',
      name: 'drive-search',
      component: DriveView,
      meta: {
        requiresAuth: true,
        title: 'Drive Search',
        group: 'Work',
        titleKey: 'routes.driveSearch',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/drive/shared',
      name: 'drive-shared',
      component: DriveView,
      meta: {
        requiresAuth: true,
        title: 'Shared with me',
        group: 'Work',
        titleKey: 'routes.driveShared',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/drive/starred',
      name: 'drive-starred',
      component: DriveView,
      meta: {
        requiresAuth: true,
        title: 'Starred',
        group: 'Work',
        titleKey: 'routes.driveStarred',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/drive/recent',
      name: 'drive-recent',
      component: DriveView,
      meta: {
        requiresAuth: true,
        title: 'Recent Drive',
        group: 'Work',
        titleKey: 'routes.driveRecent',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/drive/storage',
      name: 'drive-storage',
      component: DriveView,
      meta: {
        requiresAuth: true,
        title: 'Drive Storage',
        group: 'Work',
        titleKey: 'routes.driveStorage',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/drive/trash',
      name: 'drive-trash',
      component: DriveView,
      meta: {
        requiresAuth: true,
        title: 'Drive Trash',
        group: 'Work',
        titleKey: 'routes.driveTrash',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/drive/groups',
      name: 'drive-groups',
      component: DriveGroupsView,
      meta: {
        requiresAuth: true,
        title: 'Drive Groups',
        group: 'Work',
        titleKey: 'nav.items.driveGroups',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/public/drive/share-links/:token',
      name: 'public-drive-share-link',
      component: PublicDriveShareView,
      meta: {
        title: 'Public Drive Link',
        group: 'Public',
        titleKey: 'routes.publicDriveLink',
        groupKey: 'nav.groups.public',
      },
    },
    {
      path: '/tenant-admin',
      name: 'tenant-admin',
      component: TenantAdminTenantsView,
      meta: {
        requiresAuth: true,
        title: 'Tenants',
        group: 'Admin',
        titleKey: 'nav.items.tenants',
        groupKey: 'nav.groups.admin',
      },
    },
    {
      path: '/tenant-admin/new',
      name: 'tenant-admin-new',
      component: TenantAdminTenantFormView,
      meta: {
        requiresAuth: true,
        title: 'New Tenant',
        group: 'Admin',
        titleKey: 'routes.newTenant',
        groupKey: 'nav.groups.admin',
      },
    },
    {
      path: '/tenant-admin/:tenantSlug',
      component: TenantAdminTenantShellView,
      meta: {
        requiresAuth: true,
        title: 'Tenant Detail',
        group: 'Admin',
        titleKey: 'routes.tenantDetail',
        groupKey: 'nav.groups.admin',
      },
      children: [
        {
          path: '',
          name: 'tenant-admin-detail',
          redirect: (to) => ({
            name: 'tenant-admin-detail-overview',
            params: to.params,
          }),
        },
        {
          path: 'overview',
          name: 'tenant-admin-detail-overview',
          component: TenantAdminTenantOverviewView,
        },
        {
          path: 'members',
          name: 'tenant-admin-detail-members',
          component: TenantAdminTenantMembersView,
        },
        {
          path: 'invitations',
          name: 'tenant-admin-detail-invitations',
          component: TenantAdminTenantInvitationsView,
        },
        {
          path: 'settings',
          name: 'tenant-admin-detail-settings',
          component: TenantAdminTenantSettingsView,
        },
        {
          path: 'drive-policy',
          name: 'tenant-admin-detail-drive-policy',
          component: TenantAdminTenantDrivePolicyView,
        },
        {
          path: 'drive-operations',
          name: 'tenant-admin-detail-drive-operations',
          component: TenantAdminTenantDriveOperationsView,
        },
        {
          path: 'entitlements',
          name: 'tenant-admin-detail-entitlements',
          component: TenantAdminTenantEntitlementsView,
        },
        {
          path: 'support',
          name: 'tenant-admin-detail-support',
          component: TenantAdminTenantSupportView,
        },
        {
          path: 'webhooks',
          name: 'tenant-admin-detail-webhooks',
          component: TenantAdminTenantWebhooksView,
        },
        {
          path: 'data',
          name: 'tenant-admin-detail-data',
          component: TenantAdminTenantDataView,
        },
      ],
    },
    {
      path: '/machine-clients',
      name: 'machine-clients',
      component: MachineClientsView,
      meta: {
        requiresAuth: true,
        title: 'Machine Clients',
        group: 'Admin',
        titleKey: 'nav.items.machineClients',
        groupKey: 'nav.groups.admin',
      },
    },
    {
      path: '/machine-clients/new',
      name: 'machine-client-new',
      component: MachineClientFormView,
      meta: {
        requiresAuth: true,
        title: 'New Machine Client',
        group: 'Admin',
        titleKey: 'routes.newMachineClient',
        groupKey: 'nav.groups.admin',
      },
    },
    {
      path: '/machine-clients/:id',
      name: 'machine-client-detail',
      component: MachineClientDetailView,
      meta: {
        requiresAuth: true,
        title: 'Machine Client Detail',
        group: 'Admin',
        titleKey: 'routes.machineClientDetail',
        groupKey: 'nav.groups.admin',
      },
    },
  ],
})

router.beforeEach(async (to) => {
  const sessionStore = useSessionStore()
  await sessionStore.bootstrap()

  if (to.meta.requiresAuth && sessionStore.status !== 'authenticated') {
    return { name: 'login' }
  }

  if (to.name === 'login' && sessionStore.status === 'authenticated') {
    return { name: 'home' }
  }

  return true
})

export default router
