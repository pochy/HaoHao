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
import TenantAdminTenantDetailView from '../views/TenantAdminTenantDetailView.vue'
import TenantAdminTenantFormView from '../views/TenantAdminTenantFormView.vue'
import TenantAdminTenantsView from '../views/TenantAdminTenantsView.vue'
import TodosView from '../views/TodosView.vue'
import PublicDriveShareView from '../views/PublicDriveShareView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
      meta: { requiresAuth: true },
    },
    {
      path: '/login',
      name: 'login',
      component: LoginView,
    },
    {
      path: '/integrations',
      name: 'integrations',
      component: IntegrationsView,
      meta: { requiresAuth: true },
    },
    {
      path: '/notifications',
      name: 'notifications',
      component: NotificationsView,
      meta: { requiresAuth: true },
    },
    {
      path: '/invitations/accept',
      name: 'invitation-accept',
      component: InvitationAcceptView,
      meta: { requiresAuth: true },
    },
    {
      path: '/todos',
      name: 'todos',
      component: TodosView,
      meta: { requiresAuth: true },
    },
    {
      path: '/customer-signals',
      name: 'customer-signals',
      component: CustomerSignalsView,
      meta: { requiresAuth: true },
    },
    {
      path: '/customer-signals/:signalPublicId',
      name: 'customer-signal-detail',
      component: CustomerSignalDetailView,
      meta: { requiresAuth: true },
    },
    {
      path: '/drive',
      name: 'drive',
      component: DriveView,
      meta: { requiresAuth: true },
    },
    {
      path: '/drive/folders/:folderPublicId',
      name: 'drive-folder',
      component: DriveView,
      meta: { requiresAuth: true },
    },
    {
      path: '/drive/search',
      name: 'drive-search',
      component: DriveView,
      meta: { requiresAuth: true },
    },
    {
      path: '/drive/groups',
      name: 'drive-groups',
      component: DriveGroupsView,
      meta: { requiresAuth: true },
    },
    {
      path: '/public/drive/share-links/:token',
      name: 'public-drive-share-link',
      component: PublicDriveShareView,
    },
    {
      path: '/tenant-admin',
      name: 'tenant-admin',
      component: TenantAdminTenantsView,
      meta: { requiresAuth: true },
    },
    {
      path: '/tenant-admin/new',
      name: 'tenant-admin-new',
      component: TenantAdminTenantFormView,
      meta: { requiresAuth: true },
    },
    {
      path: '/tenant-admin/:tenantSlug',
      name: 'tenant-admin-detail',
      component: TenantAdminTenantDetailView,
      meta: { requiresAuth: true },
    },
    {
      path: '/machine-clients',
      name: 'machine-clients',
      component: MachineClientsView,
      meta: { requiresAuth: true },
    },
    {
      path: '/machine-clients/new',
      name: 'machine-client-new',
      component: MachineClientFormView,
      meta: { requiresAuth: true },
    },
    {
      path: '/machine-clients/:id',
      name: 'machine-client-detail',
      component: MachineClientDetailView,
      meta: { requiresAuth: true },
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
