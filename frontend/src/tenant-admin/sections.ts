import {
  Activity,
  Building2,
  Database,
  KeySquare,
  KeyRound,
  LifeBuoy,
  MailPlus,
  ShieldCheck,
  SlidersHorizontal,
  UsersRound,
  Webhook,
} from 'lucide-vue-next'
import type { Component } from 'vue'
import type { RouteLocationRaw } from 'vue-router'

export type TenantAdminRouteName =
  | 'tenant-admin-detail-overview'
  | 'tenant-admin-detail-members'
  | 'tenant-admin-detail-invitations'
  | 'tenant-admin-detail-settings'
  | 'tenant-admin-detail-drive-policy'
  | 'tenant-admin-detail-drive-operations'
  | 'tenant-admin-detail-entitlements'
  | 'tenant-admin-detail-support'
  | 'tenant-admin-detail-webhooks'
  | 'tenant-admin-detail-data'
  | 'tenant-admin-detail-data-access'

export type TenantAdminSection = {
  key: string
  labelKey: string
  descriptionKey: string
  routeName: TenantAdminRouteName
  icon: Component
}

export const tenantAdminSections: TenantAdminSection[] = [
  {
    key: 'overview',
    labelKey: 'tenantAdmin.sections.overview',
    descriptionKey: 'tenantAdmin.sectionDescriptions.overview',
    routeName: 'tenant-admin-detail-overview',
    icon: Building2,
  },
  {
    key: 'members',
    labelKey: 'tenantAdmin.sections.members',
    descriptionKey: 'tenantAdmin.sectionDescriptions.members',
    routeName: 'tenant-admin-detail-members',
    icon: UsersRound,
  },
  {
    key: 'invitations',
    labelKey: 'tenantAdmin.sections.invitations',
    descriptionKey: 'tenantAdmin.sectionDescriptions.invitations',
    routeName: 'tenant-admin-detail-invitations',
    icon: MailPlus,
  },
  {
    key: 'settings',
    labelKey: 'tenantAdmin.sections.settings',
    descriptionKey: 'tenantAdmin.sectionDescriptions.settings',
    routeName: 'tenant-admin-detail-settings',
    icon: SlidersHorizontal,
  },
  {
    key: 'drive-policy',
    labelKey: 'tenantAdmin.sections.drivePolicy',
    descriptionKey: 'tenantAdmin.sectionDescriptions.drivePolicy',
    routeName: 'tenant-admin-detail-drive-policy',
    icon: ShieldCheck,
  },
  {
    key: 'drive-operations',
    labelKey: 'tenantAdmin.sections.driveOperations',
    descriptionKey: 'tenantAdmin.sectionDescriptions.driveOperations',
    routeName: 'tenant-admin-detail-drive-operations',
    icon: Activity,
  },
  {
    key: 'entitlements',
    labelKey: 'tenantAdmin.sections.entitlements',
    descriptionKey: 'tenantAdmin.sectionDescriptions.entitlements',
    routeName: 'tenant-admin-detail-entitlements',
    icon: KeyRound,
  },
  {
    key: 'support',
    labelKey: 'tenantAdmin.sections.support',
    descriptionKey: 'tenantAdmin.sectionDescriptions.support',
    routeName: 'tenant-admin-detail-support',
    icon: LifeBuoy,
  },
  {
    key: 'webhooks',
    labelKey: 'tenantAdmin.sections.webhooks',
    descriptionKey: 'tenantAdmin.sectionDescriptions.webhooks',
    routeName: 'tenant-admin-detail-webhooks',
    icon: Webhook,
  },
  {
    key: 'data',
    labelKey: 'tenantAdmin.sections.data',
    descriptionKey: 'tenantAdmin.sectionDescriptions.data',
    routeName: 'tenant-admin-detail-data',
    icon: Database,
  },
  {
    key: 'data-access',
    labelKey: 'tenantAdmin.sections.dataAccess',
    descriptionKey: 'tenantAdmin.sectionDescriptions.dataAccess',
    routeName: 'tenant-admin-detail-data-access',
    icon: KeySquare,
  },
]

export function tenantAdminSectionTo(section: TenantAdminSection, tenantSlug: string): RouteLocationRaw {
  return {
    name: section.routeName,
    params: { tenantSlug },
  }
}
