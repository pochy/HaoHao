import { createRouter, createWebHistory } from 'vue-router'

import { useSessionStore } from '../stores/session'
import HomeView from '../views/HomeView.vue'
import IntegrationsView from '../views/IntegrationsView.vue'
import LoginView from '../views/LoginView.vue'
import MachineClientDetailView from '../views/MachineClientDetailView.vue'
import MachineClientFormView from '../views/MachineClientFormView.vue'
import MachineClientsView from '../views/MachineClientsView.vue'

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
