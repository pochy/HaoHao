import { defineStore } from 'pinia'

import { connectRealtime, type RealtimeEvent, type RealtimeTransportStatus } from '../api/realtime'
import type { NotificationBody } from '../api/generated/types.gen'
import { useCustomerSignalStore } from './customer-signals'
import { useDatasetStore } from './datasets'
import { useDriveStore } from './drive'
import { useNotificationStore } from './notifications'
import { useSessionStore } from './session'
import { useTenantCommonStore } from './tenant-common'
import { useTenantStore } from './tenants'

type RealtimeConnectionHandle = ReturnType<typeof connectRealtime>
type DatasetSyncStatus = 'pending' | 'processing' | 'completed' | 'failed'
type GoldPublishRunStatus = 'pending' | 'processing' | 'completed' | 'failed'
type GoldPublicationStatus = 'pending' | 'active' | 'failed' | 'unpublished' | 'archived'

export const useRealtimeStore = defineStore('realtime', {
  state: () => ({
    status: 'idle' as RealtimeTransportStatus,
    cursor: typeof window !== 'undefined' ? window.localStorage.getItem('haohao.realtime.cursor') ?? '' : '',
    connectionKey: '',
    connection: null as RealtimeConnectionHandle | null,
  }),

  getters: {
    connected: (state) => state.status === 'connected' || state.status === 'polling',
  },

  actions: {
    start() {
      const sessionStore = useSessionStore()
      if (sessionStore.status !== 'authenticated') {
        this.stop()
        return
      }
      const tenantStore = useTenantStore()
      const key = `${sessionStore.user?.publicId ?? 'user'}:${tenantStore.activeTenant?.slug ?? 'global'}`
      if (this.connection && this.connectionKey === key) {
        return
      }
      this.stop()
      const storageKey = `haohao.realtime.cursor.${key}`
      this.cursor = window.localStorage.getItem(storageKey) ?? ''
      this.connectionKey = key
      this.connection = connectRealtime({
        cursor: this.cursor,
        storageKey,
        onCursor: (cursor) => {
          this.cursor = cursor
        },
        onStatus: (status) => {
          this.status = status
        },
        onEvent: (event) => {
          void this.handleEvent(event)
        },
      })
    },

    stop() {
      this.connection?.close()
      this.connection = null
      this.connectionKey = ''
      this.status = 'idle'
    },

    async handleEvent(event: RealtimeEvent) {
      if (event.type === 'notification.created') {
        const item = notificationFromPayload(event)
        if (item) {
          useNotificationStore().upsert(item)
        } else {
          await useNotificationStore().load()
        }
        return
      }
      if (event.type === 'notification.read') {
        const item = notificationFromPayload(event)
        if (item) {
          useNotificationStore().markReadFromRealtime(item)
        } else {
          await useNotificationStore().load()
        }
        return
      }
      if (event.type === 'job.updated') {
        await refreshForJobEvent(event)
      }
    },
  },
})

function notificationFromPayload(event: RealtimeEvent): NotificationBody | null {
  const raw = event.payload?.notification
  if (!raw || typeof raw !== 'object') {
    return null
  }
  const item = raw as Partial<NotificationBody>
  return typeof item.publicId === 'string' ? item as NotificationBody : null
}

async function refreshForJobEvent(event: RealtimeEvent) {
  const datasetStore = useDatasetStore()
  const driveStore = useDriveStore()
  const tenantStore = useTenantStore()
  const commonStore = useTenantCommonStore()
  const payload = event.payload ?? {}

  if (event.resourceType === 'dataset_import' || event.resourceType === 'dataset') {
    if (datasetStore.status !== 'idle') {
      await datasetStore.load()
    }
    const datasetPublicId = stringPayload(payload.datasetPublicId) || event.resourcePublicId || datasetStore.selectedPublicId
    if (datasetPublicId && datasetStore.selectedPublicId === datasetPublicId) {
      await datasetStore.refreshSelected()
      await datasetStore.loadQueryJobs(datasetPublicId).catch(() => undefined)
      await datasetStore.loadLinkedWorkTables(datasetPublicId).catch(() => undefined)
    }
    return
  }

  if (event.resourceType === 'dataset_sync') {
    const syncJobPublicId = stringPayload(payload.syncJobPublicId) || event.resourcePublicId
    const datasetPublicId = stringPayload(payload.datasetPublicId) || datasetStore.selectedPublicId
    const status = datasetSyncStatusPayload(payload.status)
    const errorSummary = stringPayload(payload.errorSummary)
    const rowCount = numberPayload(payload.rowCount)
    const updatedAt = event.createdAt
    const completedAt = updatedAt && status && ['completed', 'failed'].includes(status) ? updatedAt : undefined
    if (syncJobPublicId) {
      datasetStore.applyDatasetSyncUpdate({
        publicId: syncJobPublicId,
        ...(status ? { status } : {}),
        ...(errorSummary ? { errorSummary } : {}),
        ...(rowCount !== undefined ? { rowCount } : {}),
        ...(updatedAt ? { updatedAt } : {}),
        ...(completedAt ? { completedAt } : {}),
      })
    }
    if (datasetStore.status !== 'idle') {
      await datasetStore.load().catch(() => undefined)
    }
    if (datasetPublicId && datasetStore.selectedPublicId === datasetPublicId) {
      await datasetStore.refreshSelected()
      await datasetStore.loadDatasetSyncJobs(datasetPublicId).catch(() => undefined)
      await datasetStore.loadQueryJobs(datasetPublicId).catch(() => undefined)
      await datasetStore.loadLinkedWorkTables(datasetPublicId).catch(() => undefined)
    }
    return
  }

  if (event.resourceType === 'dataset_work_table_export') {
    const exportPublicId = stringPayload(payload.exportPublicId) || event.resourcePublicId
    const workTablePublicId = stringPayload(payload.workTablePublicId)
    const schedulePublicId = stringPayload(payload.schedulePublicId)
    const status = stringPayload(payload.status)
    const errorSummary = stringPayload(payload.errorSummary)
    const format = stringPayload(payload.format)
    const source = exportSourcePayload(payload.source)
    const expiresAt = stringPayload(payload.expiresAt)
    const scheduledFor = stringPayload(payload.scheduledFor)
    const updatedAt = event.createdAt
    const completedAt = updatedAt && ['ready', 'failed'].includes(status) ? updatedAt : undefined
    const matched = exportPublicId
      ? datasetStore.applyWorkTableExportUpdate({
          publicId: exportPublicId,
          ...(status ? { status } : {}),
          ...(format ? { format } : {}),
          ...(source ? { source } : {}),
          ...(schedulePublicId ? { schedulePublicId } : {}),
          ...(scheduledFor ? { scheduledFor } : {}),
          ...(expiresAt ? { expiresAt } : {}),
          ...(errorSummary ? { errorSummary } : {}),
          ...(updatedAt ? { updatedAt } : {}),
          ...(completedAt ? { completedAt } : {}),
        })
      : false
    const selectedWorkTableMatched = Boolean(workTablePublicId && datasetStore.selectedWorkTable?.publicId === workTablePublicId)
    if (matched || selectedWorkTableMatched || datasetStore.hasActiveWorkTableExports) {
      await datasetStore.refreshSelectedWorkTableExports()
    }
    if (schedulePublicId && (selectedWorkTableMatched || matched)) {
      await datasetStore.refreshSelectedWorkTableExportSchedules()
    }
    return
  }

  if (event.resourceType === 'dataset_gold_publish') {
    const publishRunPublicId = stringPayload(payload.publishRunPublicId) || event.resourcePublicId
    const goldPublicationPublicId = stringPayload(payload.goldPublicationPublicId)
    const status = goldPublishRunStatusPayload(payload.status)
    const errorSummary = stringPayload(payload.errorSummary)
    const rowCount = numberPayload(payload.rowCount)
    const updatedAt = event.createdAt
    const completedAt = updatedAt && status && ['completed', 'failed'].includes(status) ? updatedAt : undefined
    const matched = publishRunPublicId
      ? datasetStore.applyGoldPublishRunUpdate({
          publicId: publishRunPublicId,
          ...(status ? { status } : {}),
          ...(errorSummary ? { errorSummary } : {}),
          ...(rowCount !== undefined ? { rowCount } : {}),
          ...(updatedAt ? { updatedAt } : {}),
          ...(completedAt ? { completedAt } : {}),
        })
      : false
    const selectedMatched = Boolean(goldPublicationPublicId && datasetStore.selectedGoldPublication?.publicId === goldPublicationPublicId)
    if (goldPublicationPublicId && (selectedMatched || matched)) {
      await datasetStore.loadGoldPublication(goldPublicationPublicId).catch(() => undefined)
    }
    if (matched || selectedMatched || datasetStore.goldPublications.length > 0) {
      await datasetStore.loadGoldPublications().catch(() => undefined)
    }
    return
  }

  if (event.resourceType === 'dataset_gold_publication') {
    const goldPublicationPublicId = stringPayload(payload.goldPublicationPublicId) || event.resourcePublicId
    const status = goldPublicationStatusPayload(payload.status)
    const rowCount = numberPayload(payload.rowCount)
    const updatedAt = event.createdAt
    const matched = goldPublicationPublicId
      ? datasetStore.applyGoldPublicationUpdate({
          publicId: goldPublicationPublicId,
          ...(status ? { status } : {}),
          ...(rowCount !== undefined ? { rowCount } : {}),
          ...(updatedAt ? { updatedAt } : {}),
        })
      : false
    const selectedMatched = Boolean(goldPublicationPublicId && datasetStore.selectedGoldPublication?.publicId === goldPublicationPublicId)
    if (goldPublicationPublicId && selectedMatched) {
      await datasetStore.loadGoldPublication(goldPublicationPublicId).catch(() => undefined)
    }
    if (matched || selectedMatched || datasetStore.goldPublications.length > 0) {
      await datasetStore.loadGoldPublications().catch(() => undefined)
    }
    return
  }

  if (event.resourceType === 'drive_ocr_run') {
    const filePublicId = stringPayload(payload.filePublicId)
    if (filePublicId && driveStore.selectedResource?.type === 'file' && driveStore.selectedResource.publicId === filePublicId) {
      await driveStore.loadOCR({ type: 'file', publicId: filePublicId }, { showLoading: false })
      await driveStore.loadMedallion({ type: 'file', publicId: filePublicId })
      driveStore.syncOCRPollingForResource(driveStore.selectedResource)
    }
    return
  }

  if (event.resourceType === 'tenant_data_export') {
    const slug = tenantStore.activeTenant?.slug
    const exportPublicId = stringPayload(payload.exportPublicId) || event.resourcePublicId
    const status = stringPayload(payload.status)
    const errorSummary = stringPayload(payload.errorSummary)
    const format = stringPayload(payload.format)
    const updatedAt = event.createdAt
    const completedAt = updatedAt && ['ready', 'failed'].includes(status) ? updatedAt : undefined
    const matched = exportPublicId
      ? commonStore.applyTenantDataExportUpdate({
          publicId: exportPublicId,
          ...(status ? { status } : {}),
          ...(format ? { format } : {}),
          ...(errorSummary ? { errorSummary } : {}),
          ...(updatedAt ? { updatedAt } : {}),
          ...(completedAt ? { completedAt } : {}),
        })
      : false
    if (slug && (matched || commonStore.hasActiveDataJobs)) {
      await commonStore.refreshTenantDataExports(slug)
    }
    return
  }

  if (event.resourceType === 'customer_signal_import') {
    const slug = tenantStore.activeTenant?.slug
    const importPublicId = stringPayload(payload.importPublicId) || event.resourcePublicId
    const status = stringPayload(payload.status)
    const errorSummary = stringPayload(payload.errorSummary)
    const updatedAt = event.createdAt
    const completedAt = updatedAt && ['completed', 'failed'].includes(status) ? updatedAt : undefined
    const totalRows = numberPayload(payload.totalRows)
    const validRows = numberPayload(payload.validRows)
    const invalidRows = numberPayload(payload.invalidRows)
    const insertedRows = numberPayload(payload.insertedRows)
    const validateOnly = booleanPayload(payload.validateOnly)
    const matched = importPublicId
      ? commonStore.applyCustomerSignalImportUpdate({
          publicId: importPublicId,
          ...(status ? { status } : {}),
          ...(errorSummary ? { errorSummary } : {}),
          ...(updatedAt ? { updatedAt } : {}),
          ...(completedAt ? { completedAt } : {}),
          ...(totalRows !== undefined ? { totalRows } : {}),
          ...(validRows !== undefined ? { validRows } : {}),
          ...(invalidRows !== undefined ? { invalidRows } : {}),
          ...(insertedRows !== undefined ? { insertedRows } : {}),
          ...(validateOnly !== undefined ? { validateOnly } : {}),
        })
      : false
    if (slug && (matched || commonStore.hasActiveDataJobs)) {
      await commonStore.refreshCustomerSignalImports(slug)
    }
    if (status === 'completed') {
      const signalStore = useCustomerSignalStore()
      if (signalStore.status !== 'idle') {
        await signalStore.load().catch(() => undefined)
      }
    }
  }
}

function stringPayload(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function numberPayload(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}

function booleanPayload(value: unknown): boolean | undefined {
  return typeof value === 'boolean' ? value : undefined
}

function exportSourcePayload(value: unknown): 'manual' | 'scheduled' | undefined {
  return value === 'manual' || value === 'scheduled' ? value : undefined
}

function datasetSyncStatusPayload(value: unknown): DatasetSyncStatus | undefined {
  return value === 'pending' || value === 'processing' || value === 'completed' || value === 'failed' ? value : undefined
}

function goldPublishRunStatusPayload(value: unknown): GoldPublishRunStatus | undefined {
  return value === 'pending' || value === 'processing' || value === 'completed' || value === 'failed' ? value : undefined
}

function goldPublicationStatusPayload(value: unknown): GoldPublicationStatus | undefined {
  return value === 'pending' || value === 'active' || value === 'failed' || value === 'unpublished' || value === 'archived' ? value : undefined
}
