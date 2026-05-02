import { createRouter, createWebHistory } from 'vue-router'

import { useSessionStore } from '../stores/session'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    title?: string
    group?: string
    titleKey?: string
    groupKey?: string
  }
}

const HomeView = () => import('../views/HomeView.vue')
const LoginView = () => import('../views/LoginView.vue')
const IntegrationsView = () => import('../views/IntegrationsView.vue')
const NotificationsView = () => import('../views/NotificationsView.vue')
const InvitationAcceptView = () => import('../views/InvitationAcceptView.vue')
const TodosView = () => import('../views/TodosView.vue')
const CustomerSignalsView = () => import('../views/CustomerSignalsView.vue')
const CustomerSignalDetailView = () => import('../views/CustomerSignalDetailView.vue')
const DatasetsView = () => import('../views/DatasetsView.vue')
const DatasetDetailView = () => import('../views/DatasetDetailView.vue')
const DatasetGoldDetailView = () => import('../views/DatasetGoldDetailView.vue')
const DriveView = () => import('../views/DriveView.vue')
const DriveGroupsView = () => import('../views/DriveGroupsView.vue')
const PublicDriveShareView = () => import('../views/PublicDriveShareView.vue')
const TenantAdminTenantsView = () => import('../views/TenantAdminTenantsView.vue')
const TenantAdminTenantFormView = () => import('../views/TenantAdminTenantFormView.vue')
const TenantAdminTenantShellView = () => import('../views/tenant-admin/TenantAdminTenantShellView.vue')
const TenantAdminTenantOverviewView = () => import('../views/tenant-admin/TenantAdminTenantOverviewView.vue')
const TenantAdminTenantMembersView = () => import('../views/tenant-admin/TenantAdminTenantMembersView.vue')
const TenantAdminTenantInvitationsView = () => import('../views/tenant-admin/TenantAdminTenantInvitationsView.vue')
const TenantAdminTenantSettingsView = () => import('../views/tenant-admin/TenantAdminTenantSettingsView.vue')
const TenantAdminTenantDrivePolicyView = () => import('../views/tenant-admin/TenantAdminTenantDrivePolicyView.vue')
const TenantAdminTenantDriveOperationsView = () => import('../views/tenant-admin/TenantAdminTenantDriveOperationsView.vue')
const TenantAdminTenantEntitlementsView = () => import('../views/tenant-admin/TenantAdminTenantEntitlementsView.vue')
const TenantAdminTenantSupportView = () => import('../views/tenant-admin/TenantAdminTenantSupportView.vue')
const TenantAdminTenantWebhooksView = () => import('../views/tenant-admin/TenantAdminTenantWebhooksView.vue')
const TenantAdminTenantDataView = () => import('../views/tenant-admin/TenantAdminTenantDataView.vue')
const MachineClientsView = () => import('../views/MachineClientsView.vue')
const MachineClientFormView = () => import('../views/MachineClientFormView.vue')
const MachineClientDetailView = () => import('../views/MachineClientDetailView.vue')

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
      path: '/datasets',
      name: 'datasets',
      component: DatasetsView,
      meta: {
        requiresAuth: true,
        title: 'Datasets',
        group: 'Work',
        titleKey: 'nav.items.datasets',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/datasets/gold/:goldPublicId',
      name: 'dataset-gold-detail',
      component: DatasetGoldDetailView,
      meta: {
        requiresAuth: true,
        title: 'Gold Publication',
        group: 'Work',
        titleKey: 'routes.goldPublicationDetail',
        groupKey: 'nav.groups.work',
      },
    },
    {
      path: '/datasets/:datasetPublicId',
      name: 'dataset-detail',
      component: DatasetDetailView,
      meta: {
        requiresAuth: true,
        title: 'Dataset Detail',
        group: 'Work',
        titleKey: 'routes.datasetDetail',
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
