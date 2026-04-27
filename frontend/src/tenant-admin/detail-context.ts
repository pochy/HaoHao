import { computed, inject, onMounted, provide, ref, watch, type InjectionKey } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { toApiErrorMessage } from '../api/client'
import { uploadFile } from '../api/files'
import { startSupportAccessSession } from '../api/support-access'
import type { TenantAdminMembershipBody, TenantAdminRoleBindingBody } from '../api/generated/types.gen'
import { useSessionStore } from '../stores/session'
import { useTenantAdminStore } from '../stores/tenant-admin'
import { useTenantCommonStore } from '../stores/tenant-common'

type PendingAction =
  | { kind: 'deactivate' }
  | { kind: 'revoke', userPublicId: string, userLabel: string, roleCode: string }

export function createTenantAdminDetailContext() {
  const route = useRoute()
  const store = useTenantAdminStore()
  const commonStore = useTenantCommonStore()
  const sessionStore = useSessionStore()
  const { d, t } = useI18n()

  const displayName = ref('')
  const active = ref(true)
  const grantUserEmail = ref('')
  const grantRoleCode = ref('customer_signal_user')
  const invitationEmail = ref('')
  const invitationRoleCode = ref('todo_user')
  const fileQuotaBytes = ref(104857600)
  const browserRateLimit = ref<number | null>(null)
  const notificationsEnabled = ref(true)
  const driveExternalSharingEnabled = ref(false)
  const driveRequireApproval = ref(false)
  const drivePublicLinksEnabled = ref(true)
  const drivePasswordLinksEnabled = ref(false)
  const driveRequireLinkPassword = ref(false)
  const driveAllowedDomains = ref('')
  const driveBlockedDomains = ref('')
  const driveMaxLinkTTLHours = ref(168)
  const driveViewerDownloadEnabled = ref(true)
  const driveExternalDownloadEnabled = ref(false)
  const driveAdminContentAccessMode = ref('disabled')
  const driveAnonymousEditorLinksEnabled = ref(false)
  const driveAnonymousEditorLinksRequirePassword = ref(true)
  const driveAnonymousEditorLinkMaxTTLMinutes = ref(60)
  const driveContentScanEnabled = ref(false)
  const driveBlockDownloadUntilScanComplete = ref(true)
  const driveBlockShareUntilScanComplete = ref(true)
  const driveDlpEnabled = ref(false)
  const drivePlanCode = ref('standard')
  const driveMaxFileSizeBytes = ref(10485760)
  const driveMaxWorkspaceCount = ref(25)
  const driveMaxPublicLinkCount = ref(1000)
  const driveM2MApiEnabled = ref(false)
  const driveSearchEnabled = ref(true)
  const driveCollaborationEnabled = ref(false)
  const driveSyncEnabled = ref(false)
  const driveMobileOfflineEnabled = ref(false)
  const driveOfflineCacheAllowed = ref(false)
  const driveCmkEnabled = ref(false)
  const driveDataResidencyEnabled = ref(false)
  const driveLegalDiscoveryEnabled = ref(false)
  const driveCleanRoomEnabled = ref(false)
  const driveCleanRoomRawExportEnabled = ref(false)
  const driveOfficeCoauthoringEnabled = ref(false)
  const driveEDiscoveryProviderExportEnabled = ref(false)
  const driveHsmEnabled = ref(false)
  const driveOnPremGatewayEnabled = ref(false)
  const driveE2eeEnabled = ref(false)
  const driveE2eeZeroKnowledgeRequired = ref(true)
  const driveAiEnabled = ref(false)
  const driveAiTrainingOptOut = ref(true)
  const driveMarketplaceEnabled = ref(false)
  const driveEncryptionMode = ref('service_managed')
  const drivePrimaryRegion = ref('global')
  const driveAllowedRegions = ref('global')
  const webhookName = ref('')
  const webhookUrl = ref('')
  const webhookEvents = ref('customer_signal.created')
  const importFile = ref<File | null>(null)
  const supportUserPublicId = ref('')
  const supportReason = ref('')
  const message = ref('')
  const errorMessage = ref('')
  const pendingAction = ref<PendingAction | null>(null)

  const tenantSlug = computed(() => {
    const raw = Array.isArray(route.params.tenantSlug)
      ? route.params.tenantSlug[0]
      : route.params.tenantSlug
    return raw ?? ''
  })

  const tenant = computed(() => store.current?.tenant ?? null)
  const memberships = computed(() => store.current?.memberships ?? [])
  const tenantRoleOptions = ['customer_signal_user', 'docs_reader', 'todo_user']
  const drivePolicyRows = computed(() => [
    [t('tenantAdmin.policy.publicLinks'), enabledLabel(drivePublicLinksEnabled.value)],
    [t('tenantAdmin.policy.externalSharing'), enabledLabel(driveExternalSharingEnabled.value)],
    [t('tenantAdmin.policy.externalApproval'), driveRequireApproval.value ? t('tenantAdmin.status.required') : t('tenantAdmin.status.notRequired')],
    [t('tenantAdmin.policy.passwordLinks'), enabledLabel(drivePasswordLinksEnabled.value)],
    [t('tenantAdmin.policy.adminContentAccess'), driveAdminContentAccessModeLabel.value],
    [t('tenantAdmin.policy.anonymousEditorLinks'), enabledLabel(driveAnonymousEditorLinksEnabled.value)],
    [t('tenantAdmin.policy.scanDlp'), t('tenantAdmin.policy.scanDlpValue', {
      scan: driveContentScanEnabled.value ? t('tenantAdmin.status.scanOn') : t('tenantAdmin.status.scanOff'),
      dlp: driveDlpEnabled.value ? t('tenantAdmin.status.dlpOn') : t('tenantAdmin.status.dlpOff'),
    })],
    [t('tenantAdmin.policy.plan'), drivePlanCodeLabel.value],
    [t('tenantAdmin.policy.searchCollab'), t('tenantAdmin.policy.searchCollabValue', {
      search: driveSearchEnabled.value ? t('tenantAdmin.status.searchOn') : t('tenantAdmin.status.searchOff'),
      collab: driveCollaborationEnabled.value ? t('tenantAdmin.status.collabOn') : t('tenantAdmin.status.collabOff'),
    })],
    [t('tenantAdmin.policy.syncMobile'), t('tenantAdmin.policy.syncMobileValue', {
      sync: driveSyncEnabled.value ? t('tenantAdmin.status.syncOn') : t('tenantAdmin.status.syncOff'),
      mobile: driveMobileOfflineEnabled.value ? t('tenantAdmin.status.mobileOfflineOn') : t('tenantAdmin.status.mobileOfflineOff'),
    })],
    [t('tenantAdmin.policy.cmkResidency'), t('tenantAdmin.policy.cmkResidencyValue', {
      cmk: driveCmkEnabled.value ? driveEncryptionModeLabel.value : t('tenantAdmin.status.cmkOff'),
      residency: driveDataResidencyEnabled.value ? drivePrimaryRegion.value : t('tenantAdmin.status.residencyOff'),
    })],
    [t('tenantAdmin.policy.legalCleanRoom'), t('tenantAdmin.policy.legalCleanRoomValue', {
      legal: driveLegalDiscoveryEnabled.value ? t('tenantAdmin.status.legalOn') : t('tenantAdmin.status.legalOff'),
      cleanRoom: driveCleanRoomEnabled.value ? t('tenantAdmin.status.cleanRoomOn') : t('tenantAdmin.status.cleanRoomOff'),
    })],
    [t('tenantAdmin.policy.officeEDiscovery'), t('tenantAdmin.policy.officeEDiscoveryValue', {
      office: driveOfficeCoauthoringEnabled.value ? t('tenantAdmin.status.officeOn') : t('tenantAdmin.status.officeOff'),
      providerExport: driveEDiscoveryProviderExportEnabled.value ? t('tenantAdmin.status.providerExportOn') : t('tenantAdmin.status.providerExportOff'),
    })],
    [t('tenantAdmin.policy.hsmGateway'), t('tenantAdmin.policy.hsmGatewayValue', {
      hsm: driveHsmEnabled.value ? t('tenantAdmin.status.hsmOn') : t('tenantAdmin.status.hsmOff'),
      gateway: driveOnPremGatewayEnabled.value ? t('tenantAdmin.status.gatewayOn') : t('tenantAdmin.status.gatewayOff'),
    })],
    [t('tenantAdmin.policy.e2eeAiApps'), t('tenantAdmin.policy.e2eeAiAppsValue', {
      e2ee: driveE2eeEnabled.value ? t('tenantAdmin.status.e2eeOn') : t('tenantAdmin.status.e2eeOff'),
      ai: driveAiEnabled.value ? t('tenantAdmin.status.aiOn') : t('tenantAdmin.status.aiOff'),
      apps: driveMarketplaceEnabled.value ? t('tenantAdmin.status.appsOn') : t('tenantAdmin.status.appsOff'),
    })],
    [t('tenantAdmin.policy.maxLinkTtl'), t('tenantAdmin.policy.hours', { count: driveMaxLinkTTLHours.value })],
  ])
  const driveAdminContentAccessModeLabel = computed(() => (
    driveAdminContentAccessMode.value === 'break_glass'
      ? t('tenantAdmin.options.breakGlass')
      : t('common.disabled')
  ))
  const driveEncryptionModeLabel = computed(() => (
    driveEncryptionMode.value === 'tenant_managed'
      ? t('tenantAdmin.options.tenantManaged')
      : t('tenantAdmin.options.serviceManaged')
  ))
  const drivePlanCodeLabel = computed(() => {
    if (drivePlanCode.value === 'free') {
      return t('tenantAdmin.options.free')
    }
    if (drivePlanCode.value === 'enterprise') {
      return t('tenantAdmin.options.enterprise')
    }
    return t('tenantAdmin.options.standard')
  })

  const canSaveSettings = computed(() => (
    Boolean(tenant.value) &&
    displayName.value.trim() !== '' &&
    !store.saving
  ))

  const canGrantRole = computed(() => (
    Boolean(tenant.value) &&
    grantUserEmail.value.trim() !== '' &&
    grantRoleCode.value.trim() !== '' &&
    !store.saving
  ))

  const canInvite = computed(() => (
    Boolean(tenant.value) &&
    invitationEmail.value.trim() !== '' &&
    invitationRoleCode.value.trim() !== '' &&
    !commonStore.saving
  ))

  const canSaveCommonSettings = computed(() => (
    Boolean(tenant.value) &&
    fileQuotaBytes.value >= 0 &&
    !commonStore.saving
  ))

  const confirmTitle = computed(() => {
    if (pendingAction.value?.kind === 'revoke') {
      return t('tenantAdmin.confirm.revokeTitle')
    }
    return t('tenantAdmin.confirm.deactivateTitle')
  })

  const confirmMessage = computed(() => {
    if (pendingAction.value?.kind === 'revoke') {
      return t('tenantAdmin.confirm.revokeMessage', {
        userLabel: pendingAction.value.userLabel,
        roleCode: pendingAction.value.roleCode,
      })
    }
    return t('tenantAdmin.confirm.deactivateMessage', {
      slug: tenant.value?.slug ?? tenantSlug.value,
    })
  })

  const confirmLabel = computed(() => (
    pendingAction.value?.kind === 'revoke' ? t('tenantAdmin.actions.revoke') : t('tenantAdmin.actions.deactivate')
  ))

  onMounted(async () => {
    await loadCurrent()
  })

  watch(
    () => route.params.tenantSlug,
    async () => {
      await loadCurrent()
    },
  )

  watch(
    () => store.current?.tenant,
    () => syncForm(),
  )

  watch(
    () => commonStore.settings,
    () => syncCommonForm(),
  )

  async function loadCurrent() {
    message.value = ''
    errorMessage.value = ''
    if (!tenantSlug.value) {
      errorMessage.value = t('tenantAdmin.errors.invalidSlug')
      return
    }
    await store.loadOne(tenantSlug.value)
    await store.loadDriveState(tenantSlug.value)
    await commonStore.load(tenantSlug.value)
    syncForm()
    syncCommonForm()
  }

  function syncForm() {
    if (!store.current?.tenant) {
      displayName.value = ''
      active.value = true
      return
    }

    displayName.value = store.current.tenant.displayName
    active.value = store.current.tenant.active
  }

  function syncCommonForm() {
    if (!commonStore.settings) {
      fileQuotaBytes.value = 104857600
      browserRateLimit.value = null
      notificationsEnabled.value = true
      return
    }
    fileQuotaBytes.value = commonStore.settings.fileQuotaBytes
    browserRateLimit.value = commonStore.settings.rateLimitBrowserApiPerMinute ?? null
    notificationsEnabled.value = commonStore.settings.notificationsEnabled
    const drive = (commonStore.settings.features?.drive ?? {}) as Record<string, unknown>
    driveExternalSharingEnabled.value = Boolean(drive.externalUserSharingEnabled)
    driveRequireApproval.value = Boolean(drive.requireExternalShareApproval)
    drivePublicLinksEnabled.value = drive.publicLinksEnabled !== false
    drivePasswordLinksEnabled.value = Boolean(drive.passwordProtectedLinksEnabled)
    driveRequireLinkPassword.value = Boolean(drive.requireShareLinkPassword)
    driveAllowedDomains.value = Array.isArray(drive.allowedExternalDomains) ? drive.allowedExternalDomains.join(', ') : ''
    driveBlockedDomains.value = Array.isArray(drive.blockedExternalDomains) ? drive.blockedExternalDomains.join(', ') : ''
    driveMaxLinkTTLHours.value = typeof drive.maxShareLinkTTLHours === 'number' ? drive.maxShareLinkTTLHours : 168
    driveViewerDownloadEnabled.value = drive.viewerDownloadEnabled !== false
    driveExternalDownloadEnabled.value = Boolean(drive.externalDownloadEnabled)
    driveAdminContentAccessMode.value = typeof drive.adminContentAccessMode === 'string' ? drive.adminContentAccessMode : 'disabled'
    driveAnonymousEditorLinksEnabled.value = Boolean(drive.anonymousEditorLinksEnabled)
    driveAnonymousEditorLinksRequirePassword.value = drive.anonymousEditorLinksRequirePassword !== false
    driveAnonymousEditorLinkMaxTTLMinutes.value = typeof drive.anonymousEditorLinkMaxTTLMinutes === 'number' ? drive.anonymousEditorLinkMaxTTLMinutes : 60
    driveContentScanEnabled.value = Boolean(drive.contentScanEnabled)
    driveBlockDownloadUntilScanComplete.value = drive.blockDownloadUntilScanComplete !== false
    driveBlockShareUntilScanComplete.value = drive.blockShareUntilScanComplete !== false
    driveDlpEnabled.value = Boolean(drive.dlpEnabled)
    drivePlanCode.value = typeof drive.planCode === 'string' ? drive.planCode : 'standard'
    driveMaxFileSizeBytes.value = typeof drive.maxFileSizeBytes === 'number' ? drive.maxFileSizeBytes : 10485760
    driveMaxWorkspaceCount.value = typeof drive.maxWorkspaceCount === 'number' ? drive.maxWorkspaceCount : 25
    driveMaxPublicLinkCount.value = typeof drive.maxPublicLinkCount === 'number' ? drive.maxPublicLinkCount : 1000
    driveM2MApiEnabled.value = Boolean(drive.m2mDriveAPIEnabled)
    driveSearchEnabled.value = drive.searchEnabled !== false
    driveCollaborationEnabled.value = Boolean(drive.collaborationEnabled)
    driveSyncEnabled.value = Boolean(drive.syncEnabled)
    driveMobileOfflineEnabled.value = Boolean(drive.mobileOfflineEnabled)
    driveOfflineCacheAllowed.value = Boolean(drive.offlineCacheAllowed)
    driveCmkEnabled.value = Boolean(drive.cmkEnabled)
    driveDataResidencyEnabled.value = Boolean(drive.dataResidencyEnabled)
    driveLegalDiscoveryEnabled.value = Boolean(drive.legalDiscoveryEnabled)
    driveCleanRoomEnabled.value = Boolean(drive.cleanRoomEnabled)
    driveCleanRoomRawExportEnabled.value = Boolean(drive.cleanRoomRawExportEnabled)
    driveOfficeCoauthoringEnabled.value = Boolean(drive.officeCoauthoringEnabled)
    driveEDiscoveryProviderExportEnabled.value = Boolean(drive.eDiscoveryProviderExportEnabled)
    driveHsmEnabled.value = Boolean(drive.hsmEnabled)
    driveOnPremGatewayEnabled.value = Boolean(drive.onPremGatewayEnabled)
    driveE2eeEnabled.value = Boolean(drive.e2eeEnabled)
    driveE2eeZeroKnowledgeRequired.value = drive.e2eeZeroKnowledgeRequired !== false
    driveAiEnabled.value = Boolean(drive.aiEnabled)
    driveAiTrainingOptOut.value = drive.aiTrainingOptOut !== false
    driveMarketplaceEnabled.value = Boolean(drive.marketplaceEnabled)
    driveEncryptionMode.value = typeof drive.encryptionMode === 'string' ? drive.encryptionMode : 'service_managed'
    drivePrimaryRegion.value = typeof drive.primaryRegion === 'string' ? drive.primaryRegion : 'global'
    driveAllowedRegions.value = Array.isArray(drive.allowedRegions) ? drive.allowedRegions.join(', ') : 'global'
  }

  function formatDate(value?: string) {
    if (!value) {
      return t('common.never')
    }

    return d(new Date(value), 'long')
  }

  function userLabel(member: TenantAdminMembershipBody) {
    return member.displayName ? `${member.displayName} / ${member.email}` : member.email
  }

  function roleSourceClass(role: TenantAdminRoleBindingBody) {
    return ['source-chip', role.source === 'local_override' ? 'local' : '', role.active ? '' : 'inactive']
  }

  function domainList(value: string) {
    return value.split(',').map((item) => item.trim()).filter(Boolean)
  }

  function enabledLabel(enabled: boolean) {
    return enabled ? t('common.enabled') : t('common.disabled')
  }

  async function saveSettings() {
    if (!tenant.value || !canSaveSettings.value) {
      return
    }

    message.value = ''
    errorMessage.value = ''

    try {
      await store.update(tenant.value.slug, {
        displayName: displayName.value.trim(),
        active: active.value,
      })
      message.value = t('tenantAdmin.messages.settingsSaved')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  async function grantRole() {
    if (!tenant.value || !canGrantRole.value) {
      return
    }

    message.value = ''
    errorMessage.value = ''

    try {
      await store.grantRole(tenant.value.slug, {
        userEmail: grantUserEmail.value.trim(),
        roleCode: grantRoleCode.value,
      })
      grantUserEmail.value = ''
      message.value = t('tenantAdmin.messages.roleGranted')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  async function createInvitation() {
    if (!tenant.value || !canInvite.value) {
      return
    }

    message.value = ''
    errorMessage.value = ''

    try {
      const created = await commonStore.createInvitation(tenant.value.slug, {
        email: invitationEmail.value.trim(),
        roleCodes: [invitationRoleCode.value],
      })
      invitationEmail.value = ''
      message.value = created.acceptUrl
        ? t('tenantAdmin.messages.invitationCreatedWithUrl', { url: created.acceptUrl })
        : t('tenantAdmin.messages.invitationCreated')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  async function revokeInvitation(publicId: string) {
    if (!tenant.value) {
      return
    }

    message.value = ''
    errorMessage.value = ''

    try {
      await commonStore.revokeInvitation(tenant.value.slug, publicId)
      message.value = t('tenantAdmin.messages.invitationRevoked')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  async function saveCommonSettings() {
    if (!tenant.value || !canSaveCommonSettings.value) {
      return
    }

    message.value = ''
    errorMessage.value = ''

    try {
      await commonStore.updateSettings(tenant.value.slug, {
        fileQuotaBytes: fileQuotaBytes.value,
        rateLimitBrowserApiPerMinute: browserRateLimit.value ?? undefined,
        notificationsEnabled: notificationsEnabled.value,
        features: {
          ...(commonStore.settings?.features ?? {}),
          drive: {
            linkSharingEnabled: true,
            publicLinksEnabled: drivePublicLinksEnabled.value,
            externalUserSharingEnabled: driveExternalSharingEnabled.value,
            passwordProtectedLinksEnabled: drivePasswordLinksEnabled.value,
            requireShareLinkPassword: driveRequireLinkPassword.value,
            requireExternalShareApproval: driveRequireApproval.value,
            allowedExternalDomains: domainList(driveAllowedDomains.value),
            blockedExternalDomains: domainList(driveBlockedDomains.value),
            maxShareLinkTTLHours: driveMaxLinkTTLHours.value,
            viewerDownloadEnabled: driveViewerDownloadEnabled.value,
            externalDownloadEnabled: driveExternalDownloadEnabled.value,
            editorCanReshare: false,
            editorCanDelete: false,
            adminContentAccessMode: driveAdminContentAccessMode.value,
            anonymousEditorLinksEnabled: driveAnonymousEditorLinksEnabled.value,
            anonymousEditorLinksRequirePassword: driveAnonymousEditorLinksRequirePassword.value,
            anonymousEditorLinkMaxTTLMinutes: driveAnonymousEditorLinkMaxTTLMinutes.value,
            contentScanEnabled: driveContentScanEnabled.value,
            blockDownloadUntilScanComplete: driveBlockDownloadUntilScanComplete.value,
            blockShareUntilScanComplete: driveBlockShareUntilScanComplete.value,
            dlpEnabled: driveDlpEnabled.value,
            planCode: drivePlanCode.value,
            maxFileSizeBytes: driveMaxFileSizeBytes.value,
            maxWorkspaceCount: driveMaxWorkspaceCount.value,
            maxPublicLinkCount: driveMaxPublicLinkCount.value,
            passwordLinksPlanEnabled: true,
            dlpPlanEnabled: true,
            m2mDriveAPIEnabled: driveM2MApiEnabled.value,
            searchEnabled: driveSearchEnabled.value,
            collaborationEnabled: driveCollaborationEnabled.value,
            syncEnabled: driveSyncEnabled.value,
            mobileOfflineEnabled: driveMobileOfflineEnabled.value,
            offlineCacheAllowed: driveOfflineCacheAllowed.value,
            cmkEnabled: driveCmkEnabled.value,
            dataResidencyEnabled: driveDataResidencyEnabled.value,
            legalDiscoveryEnabled: driveLegalDiscoveryEnabled.value,
            cleanRoomEnabled: driveCleanRoomEnabled.value,
            cleanRoomRawExportEnabled: driveCleanRoomRawExportEnabled.value,
            officeCoauthoringEnabled: driveOfficeCoauthoringEnabled.value,
            eDiscoveryProviderExportEnabled: driveEDiscoveryProviderExportEnabled.value,
            hsmEnabled: driveHsmEnabled.value,
            onPremGatewayEnabled: driveOnPremGatewayEnabled.value,
            e2eeEnabled: driveE2eeEnabled.value,
            e2eeZeroKnowledgeRequired: driveE2eeZeroKnowledgeRequired.value,
            aiEnabled: driveAiEnabled.value,
            aiTrainingOptOut: driveAiTrainingOptOut.value,
            marketplaceEnabled: driveMarketplaceEnabled.value,
            encryptionMode: driveEncryptionMode.value,
            primaryRegion: drivePrimaryRegion.value,
            allowedRegions: domainList(driveAllowedRegions.value),
          },
        },
      })
      message.value = t('tenantAdmin.messages.commonSettingsSaved')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  async function approveDriveInvitation(publicId: string) {
    if (!tenant.value) {
      return
    }
    message.value = ''
    errorMessage.value = ''
    try {
      await store.approveDriveInvitation(tenant.value.slug, publicId)
      message.value = t('tenantAdmin.messages.driveInvitationApproved')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  async function rejectDriveInvitation(publicId: string) {
    if (!tenant.value) {
      return
    }
    message.value = ''
    errorMessage.value = ''
    try {
      await store.rejectDriveInvitation(tenant.value.slug, publicId)
      message.value = t('tenantAdmin.messages.driveInvitationRejected')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  async function repairDriveSync() {
    if (!tenant.value) {
      return
    }
    message.value = ''
    errorMessage.value = ''
    try {
      await store.repairDriveSync(tenant.value.slug)
      message.value = t('tenantAdmin.messages.driveSyncRepairStarted')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  async function requestExport() {
    if (!tenant.value) {
      return
    }

    message.value = ''
    errorMessage.value = ''

    try {
      await commonStore.requestExport(tenant.value.slug)
      message.value = t('tenantAdmin.messages.exportRequested')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  async function requestCSVExport() {
    if (!tenant.value) {
      return
    }
    message.value = ''
    errorMessage.value = ''
    try {
      await commonStore.requestCSVExport(tenant.value.slug)
      message.value = t('tenantAdmin.messages.csvExportRequested')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  async function saveEntitlements() {
    if (!tenant.value) {
      return
    }
    message.value = ''
    errorMessage.value = ''
    try {
      await commonStore.updateEntitlements(tenant.value.slug, commonStore.entitlements.map((item) => ({
        featureCode: item.featureCode,
        enabled: item.enabled,
        limitValue: item.limitValue,
      })))
      message.value = t('tenantAdmin.messages.entitlementsSaved')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  async function createWebhook() {
    if (!tenant.value) {
      return
    }
    message.value = ''
    errorMessage.value = ''
    try {
      const created = await commonStore.createWebhook(tenant.value.slug, {
        name: webhookName.value.trim(),
        url: webhookUrl.value.trim(),
        eventTypes: webhookEvents.value.split(',').map((item) => item.trim()).filter(Boolean),
        active: true,
      })
      webhookName.value = ''
      webhookUrl.value = ''
      message.value = created.secret
        ? t('tenantAdmin.messages.webhookSecret', { secret: created.secret })
        : t('tenantAdmin.messages.webhookCreated')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  async function uploadImportCSV() {
    if (!tenant.value || !importFile.value) {
      return
    }
    message.value = ''
    errorMessage.value = ''
    try {
      const form = new FormData()
      form.append('purpose', 'import')
      form.append('file', importFile.value)
      const file = await uploadFile(form)
      await commonStore.createImport(tenant.value.slug, file.publicId)
      importFile.value = null
      message.value = t('tenantAdmin.messages.importJobCreated')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  async function startSupportAccess() {
    if (!tenant.value) {
      return
    }
    message.value = ''
    errorMessage.value = ''
    try {
      const result = await startSupportAccessSession({
        tenantSlug: tenant.value.slug,
        impersonatedUserPublicId: supportUserPublicId.value.trim(),
        reason: supportReason.value.trim(),
        durationMinutes: 30,
      })
      sessionStore.supportAccess = result.access ?? null
      sessionStore.status = 'idle'
      await sessionStore.bootstrap()
      message.value = t('tenantAdmin.messages.supportAccessStarted')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  function onImportFileChange(event: Event) {
    const target = event.target as HTMLInputElement
    importFile.value = target.files?.[0] ?? null
  }

  function askDeactivate() {
    pendingAction.value = { kind: 'deactivate' }
  }

  function askRevoke(member: TenantAdminMembershipBody, role: TenantAdminRoleBindingBody) {
    pendingAction.value = {
      kind: 'revoke',
      userPublicId: member.userPublicId,
      userLabel: userLabel(member),
      roleCode: role.roleCode,
    }
  }

  function cancelPendingAction() {
    pendingAction.value = null
  }

  async function confirmPendingAction() {
    if (!tenant.value || !pendingAction.value) {
      return
    }

    const action = pendingAction.value
    pendingAction.value = null
    message.value = ''
    errorMessage.value = ''

    try {
      if (action.kind === 'deactivate') {
        await store.deactivate(tenant.value.slug)
        active.value = false
        message.value = t('tenantAdmin.messages.deactivated')
        return
      }

      await store.revokeRole(tenant.value.slug, action.userPublicId, action.roleCode)
      message.value = t('tenantAdmin.messages.localRoleRevoked')
    } catch (error) {
      errorMessage.value = toApiErrorMessage(error)
    }
  }

  return {
    active,
    approveDriveInvitation,
    askDeactivate,
    askRevoke,
    browserRateLimit,
    canGrantRole,
    canInvite,
    canSaveCommonSettings,
    canSaveSettings,
    cancelPendingAction,
    commonStore,
    confirmLabel,
    confirmMessage,
    confirmPendingAction,
    confirmTitle,
    createInvitation,
    createWebhook,
    displayName,
    driveAdminContentAccessMode,
    driveAiEnabled,
    driveAiTrainingOptOut,
    driveAllowedDomains,
    driveAllowedRegions,
    driveAnonymousEditorLinkMaxTTLMinutes,
    driveAnonymousEditorLinksEnabled,
    driveAnonymousEditorLinksRequirePassword,
    driveBlockDownloadUntilScanComplete,
    driveBlockShareUntilScanComplete,
    driveBlockedDomains,
    driveCleanRoomEnabled,
    driveCleanRoomRawExportEnabled,
    driveCmkEnabled,
    driveCollaborationEnabled,
    driveContentScanEnabled,
    driveDataResidencyEnabled,
    driveDlpEnabled,
    driveE2eeEnabled,
    driveE2eeZeroKnowledgeRequired,
    driveEDiscoveryProviderExportEnabled,
    driveEncryptionMode,
    driveExternalDownloadEnabled,
    driveExternalSharingEnabled,
    driveHsmEnabled,
    driveLegalDiscoveryEnabled,
    driveM2MApiEnabled,
    driveMarketplaceEnabled,
    driveMaxFileSizeBytes,
    driveMaxLinkTTLHours,
    driveMaxPublicLinkCount,
    driveMaxWorkspaceCount,
    driveMobileOfflineEnabled,
    driveOfficeCoauthoringEnabled,
    driveOfflineCacheAllowed,
    driveOnPremGatewayEnabled,
    drivePasswordLinksEnabled,
    drivePlanCode,
    drivePolicyRows,
    drivePrimaryRegion,
    drivePublicLinksEnabled,
    driveRequireApproval,
    driveRequireLinkPassword,
    driveSearchEnabled,
    driveSyncEnabled,
    driveViewerDownloadEnabled,
    errorMessage,
    fileQuotaBytes,
    formatDate,
    grantRole,
    grantRoleCode,
    grantUserEmail,
    importFile,
    invitationEmail,
    invitationRoleCode,
    memberships,
    message,
    notificationsEnabled,
    onImportFileChange,
    pendingAction,
    rejectDriveInvitation,
    repairDriveSync,
    requestCSVExport,
    requestExport,
    revokeInvitation,
    roleSourceClass,
    saveCommonSettings,
    saveEntitlements,
    saveSettings,
    startSupportAccess,
    store,
    supportReason,
    supportUserPublicId,
    tenant,
    tenantRoleOptions,
    tenantSlug,
    uploadImportCSV,
    userLabel,
    webhookEvents,
    webhookName,
    webhookUrl,
  }
}

export type TenantAdminDetailContext = ReturnType<typeof createTenantAdminDetailContext>

export const tenantAdminDetailContextKey: InjectionKey<TenantAdminDetailContext> = Symbol('tenant-admin-detail-context')

export function provideTenantAdminDetailContext(context: TenantAdminDetailContext) {
  provide(tenantAdminDetailContextKey, context)
}

export function useTenantAdminDetailContext() {
  const context = inject(tenantAdminDetailContextKey)
  if (!context) {
    throw new Error('Tenant admin detail context is not available.')
  }
  return context
}
