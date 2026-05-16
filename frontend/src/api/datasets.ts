import { apiErrorFromResponse, readCookie } from './client'
import {
  archiveDatasetGoldPublication,
  createDataset,
  createDatasetGoldPublication,
  createDatasetLineageChangeSet,
  createDatasetQueryJob,
  createDatasetScopedQueryJob,
  createDatasetSyncJob,
  createDatasetWorkTableExport,
  createDatasetWorkTableExportSchedule,
  deleteDataset,
  deleteDatasetWorkTable,
  disableDatasetWorkTableExportSchedule,
  getDatasetWorkTableExport,
  getDatasetWorkTable,
  getDatasetWorkTablePreview,
  getCsrf,
  getDataset,
  getDatasetGoldPublication,
  getDatasetLineage,
  getDatasetLineageChangeSet,
  getDatasetQueryJobLineage,
  getDatasetWorkTableLineage,
  getManagedDatasetWorkTable,
  getManagedDatasetWorkTablePreview,
  getManagedDatasetWorkTableScd2History,
  linkDatasetWorkTable,
  listDatasetLineageChangeSets,
  listDatasetGoldPublications,
  listDatasetGoldPublishRuns,
  listDatasetQueryJobs,
  listDatasetQueryJobLineageParseRuns,
  listDatasets,
  listDatasetScopedQueryJobs,
  listDatasetScopedWorkTables,
  listDatasetSourceFiles,
  listDatasetSyncJobs,
  listDatasetWorkTableExportSchedules,
  listDatasetWorkTableExports,
  listDatasetWorkTables,
  parseDatasetQueryJobLineage,
  previewDatasetGoldPublication,
  promoteDatasetWorkTable,
  publishDatasetLineageChangeSet,
  refreshDatasetGoldPublication,
  registerDatasetWorkTable,
  rejectDatasetLineageChangeSet,
  renameDatasetWorkTable,
  truncateDatasetWorkTable,
  unpublishDatasetGoldPublication,
  updateDatasetLineageChangeSetGraph,
  updateDatasetWorkTableExportSchedule,
} from './generated/sdk.gen'
import type {
  DatasetBody,
  DatasetCreateBodyWritable,
  DatasetGoldPublicationBody,
  DatasetGoldPublicationCreateBodyWritable,
  DatasetGoldPublicationPreviewBody,
  DatasetGoldPublishRunBody,
  DatasetLineageBody,
  DatasetLineageChangeSetBody,
  DatasetLineageChangeSetCreateBodyWritable,
  DatasetLineageChangeSetGraphBody,
  DatasetLineageGraphSaveBodyWritable,
  DatasetLineageParseRunBody,
  DatasetQueryCreateBodyWritable,
  DatasetQueryJobBody,
  DatasetSourceFileBody,
  DatasetSyncJobBody,
  DatasetSyncJobCreateBodyWritable,
  DatasetWorkTableBody,
  DatasetWorkTableExportBody,
  DatasetWorkTableExportScheduleBody,
  DatasetWorkTableExportScheduleCreateBodyWritable,
  DatasetWorkTableExportScheduleUpdateBodyWritable,
  DatasetWorkTableLinkBodyWritable,
  DatasetWorkTablePreviewBody,
  DatasetWorkTablePromoteBodyWritable,
  DatasetWorkTableRegisterBodyWritable,
  DatasetWorkTableScd2HistoryBody,
  DatasetWorkTableRenameBodyWritable,
} from './generated/types.gen'

export type DatasetWorkTableExportFormat = 'csv' | 'json' | 'parquet'
export type DatasetWorkTableExportFrequency = 'daily' | 'weekly' | 'monthly'
export type DatasetSyncMode = 'full_refresh'
export type DatasetLineageDirection = 'upstream' | 'downstream' | 'both'
export type DatasetLineageLevel = 'table' | 'column' | 'both'
export type DatasetLineageSource = 'metadata' | 'parser' | 'manual'
export type DatasetLineageChangeSetStatus = 'draft' | 'published' | 'rejected' | 'archived'

export interface DatasetRowsPageQuery {
  cursor?: number
  limit?: number
}

export interface DatasetRowsPageBody {
  columns: string[]
  rows: Array<Record<string, unknown>>
  nextCursor?: number | null
  hasMore: boolean
}

export interface DatasetLineageFetchOptions {
  level?: DatasetLineageLevel
  sources?: DatasetLineageSource[]
  includeDraft?: boolean
  changeSetPublicId?: string
}

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

async function ensureCSRFCookie() {
  if (readCookie('XSRF-TOKEN')) {
    return
  }
  await getCsrf()
}

export async function fetchDatasets(): Promise<DatasetBody[]> {
  const data = await listDatasets({
    query: { limit: 100 },
  }) as unknown as { items?: DatasetBody[] | null }
  return data.items ?? []
}

export async function fetchDataset(datasetPublicId: string): Promise<DatasetBody> {
  return getDataset({
    path: { datasetPublicId },
  }) as unknown as Promise<DatasetBody>
}

function lineageQuery(direction: DatasetLineageDirection, options: DatasetLineageFetchOptions = {}) {
  return {
    direction,
    depth: 2,
    includeHistory: true,
    limit: 50,
    level: options.level ?? 'table',
    sources: (options.sources?.length ? options.sources : ['metadata', 'parser', 'manual']).join(','),
    includeDraft: options.includeDraft ?? false,
    ...(options.changeSetPublicId ? { changeSetPublicId: options.changeSetPublicId } : {}),
  }
}

export async function fetchDatasetLineage(datasetPublicId: string, direction: DatasetLineageDirection = 'both', options: DatasetLineageFetchOptions = {}): Promise<DatasetLineageBody> {
  return getDatasetLineage({
    path: { datasetPublicId },
    query: lineageQuery(direction, options),
  }) as unknown as Promise<DatasetLineageBody>
}

export async function fetchDatasetSourceFiles(query = ''): Promise<DatasetSourceFileBody[]> {
  const data = await listDatasetSourceFiles({
    query: {
      ...(query.trim() ? { q: query.trim() } : {}),
      limit: 100,
    },
  }) as unknown as { items?: DatasetSourceFileBody[] | null }
  return data.items ?? []
}

export async function fetchDatasetRows(datasetPublicId: string, query: DatasetRowsPageQuery = {}): Promise<DatasetRowsPageBody> {
  const cursor = query.cursor ?? 0
  const limit = query.limit ?? 250
  const response = await fetch(`/api/v1/datasets/${datasetPublicId}/rows?cursor=${encodeURIComponent(String(cursor))}&limit=${encodeURIComponent(String(limit))}`, {
    credentials: 'include',
    headers: {
      Accept: 'application/json',
    },
  })
  if (!response.ok) {
    throw await apiErrorFromResponse(response, 'データ行の読み込みに失敗しました')
  }
  return response.json() as Promise<DatasetRowsPageBody>
}

export async function fetchDatasetWorkTables(): Promise<DatasetWorkTableBody[]> {
  const data = await listDatasetWorkTables({
    query: { limit: 100 },
  }) as unknown as { items?: DatasetWorkTableBody[] | null }
  return data.items ?? []
}

export async function fetchGoldPublications(): Promise<DatasetGoldPublicationBody[]> {
  const data = await listDatasetGoldPublications({
    query: { limit: 100 },
  }) as unknown as { items?: DatasetGoldPublicationBody[] | null }
  return data.items ?? []
}

export async function fetchGoldPublication(goldPublicId: string): Promise<DatasetGoldPublicationBody> {
  return getDatasetGoldPublication({
    path: { goldPublicId },
  }) as unknown as Promise<DatasetGoldPublicationBody>
}

export async function fetchGoldPublicationPreview(goldPublicId: string): Promise<DatasetGoldPublicationPreviewBody> {
  return previewDatasetGoldPublication({
    path: { goldPublicId },
    query: { limit: 100 },
  }) as unknown as Promise<DatasetGoldPublicationPreviewBody>
}

export async function fetchGoldPublishRuns(goldPublicId: string): Promise<DatasetGoldPublishRunBody[]> {
  const data = await listDatasetGoldPublishRuns({
    path: { goldPublicId },
    query: { limit: 25 },
  }) as unknown as { items?: DatasetGoldPublishRunBody[] | null }
  return data.items ?? []
}

export async function createGoldPublication(workTablePublicId: string, body: DatasetGoldPublicationCreateBodyWritable): Promise<DatasetGoldPublicationBody> {
  await ensureCSRFCookie()
  return createDatasetGoldPublication({
    headers: csrfHeaders(),
    path: { workTablePublicId },
    body,
  }) as unknown as Promise<DatasetGoldPublicationBody>
}

export async function refreshGoldPublication(goldPublicId: string): Promise<DatasetGoldPublishRunBody> {
  await ensureCSRFCookie()
  return refreshDatasetGoldPublication({
    headers: csrfHeaders(),
    path: { goldPublicId },
  }) as unknown as Promise<DatasetGoldPublishRunBody>
}

export async function unpublishGoldPublication(goldPublicId: string): Promise<DatasetGoldPublicationBody> {
  await ensureCSRFCookie()
  return unpublishDatasetGoldPublication({
    headers: csrfHeaders(),
    path: { goldPublicId },
  }) as unknown as Promise<DatasetGoldPublicationBody>
}

export async function archiveGoldPublication(goldPublicId: string): Promise<DatasetGoldPublicationBody> {
  await ensureCSRFCookie()
  return archiveDatasetGoldPublication({
    headers: csrfHeaders(),
    path: { goldPublicId },
  }) as unknown as Promise<DatasetGoldPublicationBody>
}

export async function fetchDatasetWorkTable(database: string, table: string): Promise<DatasetWorkTableBody> {
  return getDatasetWorkTable({
    path: { database, table },
  }) as unknown as Promise<DatasetWorkTableBody>
}

export async function fetchDatasetWorkTablePreview(database: string, table: string): Promise<DatasetWorkTablePreviewBody> {
  return getDatasetWorkTablePreview({
    path: { database, table },
    query: { limit: 100 },
  }) as unknown as Promise<DatasetWorkTablePreviewBody>
}

export async function registerWorkTable(body: DatasetWorkTableRegisterBodyWritable): Promise<DatasetWorkTableBody> {
  await ensureCSRFCookie()
  return registerDatasetWorkTable({
    headers: csrfHeaders(),
    body,
  }) as unknown as Promise<DatasetWorkTableBody>
}

export async function fetchManagedDatasetWorkTable(workTablePublicId: string): Promise<DatasetWorkTableBody> {
  return getManagedDatasetWorkTable({
    path: { workTablePublicId },
  }) as unknown as Promise<DatasetWorkTableBody>
}

export async function fetchManagedDatasetWorkTablePreview(workTablePublicId: string): Promise<DatasetWorkTablePreviewBody> {
  return getManagedDatasetWorkTablePreview({
    path: { workTablePublicId },
    query: { limit: 100 },
  }) as unknown as Promise<DatasetWorkTablePreviewBody>
}

export async function fetchManagedDatasetWorkTableSCD2History(workTablePublicId: string, key: string | string[], keyColumns: string[] = []): Promise<DatasetWorkTableScd2HistoryBody> {
  const keyValues = Array.isArray(key) ? key : [key]
  return getManagedDatasetWorkTableScd2History({
    path: { workTablePublicId },
    query: keyColumns.length > 1
      ? { keyColumns: keyColumns.join(','), keyValues: keyValues.join(','), limit: 100 }
      : { key: keyValues[0] ?? '', limit: 100 },
  }) as unknown as Promise<DatasetWorkTableScd2HistoryBody>
}

export async function fetchDatasetWorkTableLineage(workTablePublicId: string, direction: DatasetLineageDirection = 'both', options: DatasetLineageFetchOptions = {}): Promise<DatasetLineageBody> {
  return getDatasetWorkTableLineage({
    path: { workTablePublicId },
    query: lineageQuery(direction, options),
  }) as unknown as Promise<DatasetLineageBody>
}

export async function fetchDatasetQueryJobLineage(queryJobPublicId: string, direction: DatasetLineageDirection = 'both', options: DatasetLineageFetchOptions = {}): Promise<DatasetLineageBody> {
  return getDatasetQueryJobLineage({
    path: { queryJobPublicId },
    query: lineageQuery(direction, options),
  }) as unknown as Promise<DatasetLineageBody>
}

export async function requestDatasetQueryJobLineageParse(queryJobPublicId: string): Promise<DatasetLineageChangeSetGraphBody> {
  await ensureCSRFCookie()
  return parseDatasetQueryJobLineage({
    headers: csrfHeaders(),
    path: { queryJobPublicId },
  }) as unknown as Promise<DatasetLineageChangeSetGraphBody>
}

export async function fetchDatasetQueryJobLineageParseRuns(queryJobPublicId: string): Promise<DatasetLineageParseRunBody[]> {
  const data = await listDatasetQueryJobLineageParseRuns({
    path: { queryJobPublicId },
    query: { limit: 25 },
  }) as unknown as { items?: DatasetLineageParseRunBody[] | null }
  return data.items ?? []
}

export async function fetchDatasetLineageChangeSets(status: DatasetLineageChangeSetStatus = 'draft'): Promise<DatasetLineageChangeSetBody[]> {
  const data = await listDatasetLineageChangeSets({
    query: { status, limit: 50 },
  }) as unknown as { items?: DatasetLineageChangeSetBody[] | null }
  return data.items ?? []
}

export async function createLineageChangeSet(body: DatasetLineageChangeSetCreateBodyWritable): Promise<DatasetLineageChangeSetBody> {
  await ensureCSRFCookie()
  return createDatasetLineageChangeSet({
    headers: csrfHeaders(),
    body,
  }) as unknown as Promise<DatasetLineageChangeSetBody>
}

export async function fetchLineageChangeSet(changeSetPublicId: string): Promise<DatasetLineageChangeSetGraphBody> {
  return getDatasetLineageChangeSet({
    path: { changeSetPublicId },
  }) as unknown as Promise<DatasetLineageChangeSetGraphBody>
}

export async function saveLineageChangeSetGraph(changeSetPublicId: string, body: DatasetLineageGraphSaveBodyWritable): Promise<DatasetLineageChangeSetGraphBody> {
  await ensureCSRFCookie()
  return updateDatasetLineageChangeSetGraph({
    headers: csrfHeaders(),
    path: { changeSetPublicId },
    body,
  }) as unknown as Promise<DatasetLineageChangeSetGraphBody>
}

export async function publishLineageChangeSet(changeSetPublicId: string): Promise<DatasetLineageChangeSetBody> {
  await ensureCSRFCookie()
  return publishDatasetLineageChangeSet({
    headers: csrfHeaders(),
    path: { changeSetPublicId },
  }) as unknown as Promise<DatasetLineageChangeSetBody>
}

export async function rejectLineageChangeSet(changeSetPublicId: string): Promise<DatasetLineageChangeSetBody> {
  await ensureCSRFCookie()
  return rejectDatasetLineageChangeSet({
    headers: csrfHeaders(),
    path: { changeSetPublicId },
  }) as unknown as Promise<DatasetLineageChangeSetBody>
}

export async function fetchDatasetLinkedWorkTables(datasetPublicId: string): Promise<DatasetWorkTableBody[]> {
  const data = await listDatasetScopedWorkTables({
    path: { datasetPublicId },
    query: { limit: 100 },
  }) as unknown as { items?: DatasetWorkTableBody[] | null }
  return data.items ?? []
}

export async function fetchDatasetSyncJobs(datasetPublicId: string): Promise<DatasetSyncJobBody[]> {
  const data = await listDatasetSyncJobs({
    path: { datasetPublicId },
    query: { limit: 25 },
  }) as unknown as { items?: DatasetSyncJobBody[] | null }
  return data.items ?? []
}

export async function requestDatasetSync(datasetPublicId: string, body: DatasetSyncJobCreateBodyWritable = { mode: 'full_refresh' }): Promise<DatasetSyncJobBody> {
  await ensureCSRFCookie()
  return createDatasetSyncJob({
    headers: csrfHeaders(),
    path: { datasetPublicId },
    body,
  }) as unknown as Promise<DatasetSyncJobBody>
}

export async function linkWorkTable(workTablePublicId: string, body: DatasetWorkTableLinkBodyWritable): Promise<DatasetWorkTableBody> {
  await ensureCSRFCookie()
  return linkDatasetWorkTable({
    headers: csrfHeaders(),
    path: { workTablePublicId },
    body,
  }) as unknown as Promise<DatasetWorkTableBody>
}

export async function renameWorkTable(workTablePublicId: string, body: DatasetWorkTableRenameBodyWritable): Promise<DatasetWorkTableBody> {
  await ensureCSRFCookie()
  return renameDatasetWorkTable({
    headers: csrfHeaders(),
    path: { workTablePublicId },
    body,
  }) as unknown as Promise<DatasetWorkTableBody>
}

export async function truncateWorkTable(workTablePublicId: string): Promise<DatasetWorkTableBody> {
  await ensureCSRFCookie()
  return truncateDatasetWorkTable({
    headers: csrfHeaders(),
    path: { workTablePublicId },
  }) as unknown as Promise<DatasetWorkTableBody>
}

export async function dropWorkTable(workTablePublicId: string): Promise<void> {
  await ensureCSRFCookie()
  await deleteDatasetWorkTable({
    headers: csrfHeaders(),
    path: { workTablePublicId },
  })
}

export async function promoteWorkTable(workTablePublicId: string, body: DatasetWorkTablePromoteBodyWritable): Promise<DatasetBody> {
  await ensureCSRFCookie()
  return promoteDatasetWorkTable({
    headers: csrfHeaders(),
    path: { workTablePublicId },
    body,
  }) as unknown as Promise<DatasetBody>
}

export async function requestWorkTableExport(workTablePublicId: string, format: DatasetWorkTableExportFormat = 'csv'): Promise<DatasetWorkTableExportBody> {
  await ensureCSRFCookie()
  return createDatasetWorkTableExport({
    headers: csrfHeaders(),
    path: { workTablePublicId },
    body: { format },
  }) as unknown as Promise<DatasetWorkTableExportBody>
}

export async function fetchWorkTableExports(workTablePublicId: string): Promise<DatasetWorkTableExportBody[]> {
  const data = await listDatasetWorkTableExports({
    path: { workTablePublicId },
    query: { limit: 25 },
  }) as unknown as { items?: DatasetWorkTableExportBody[] | null }
  return data.items ?? []
}

export async function fetchWorkTableExportSchedules(workTablePublicId: string): Promise<DatasetWorkTableExportScheduleBody[]> {
  const data = await listDatasetWorkTableExportSchedules({
    path: { workTablePublicId },
  }) as unknown as { items?: DatasetWorkTableExportScheduleBody[] | null }
  return data.items ?? []
}

export async function createWorkTableExportSchedule(workTablePublicId: string, body: DatasetWorkTableExportScheduleCreateBodyWritable): Promise<DatasetWorkTableExportScheduleBody> {
  await ensureCSRFCookie()
  return createDatasetWorkTableExportSchedule({
    headers: csrfHeaders(),
    path: { workTablePublicId },
    body,
  }) as unknown as Promise<DatasetWorkTableExportScheduleBody>
}

export async function updateWorkTableExportSchedule(schedulePublicId: string, body: DatasetWorkTableExportScheduleUpdateBodyWritable): Promise<DatasetWorkTableExportScheduleBody> {
  await ensureCSRFCookie()
  return updateDatasetWorkTableExportSchedule({
    headers: csrfHeaders(),
    path: { schedulePublicId },
    body,
  }) as unknown as Promise<DatasetWorkTableExportScheduleBody>
}

export async function disableWorkTableExportSchedule(schedulePublicId: string): Promise<DatasetWorkTableExportScheduleBody> {
  await ensureCSRFCookie()
  return disableDatasetWorkTableExportSchedule({
    headers: csrfHeaders(),
    path: { schedulePublicId },
  }) as unknown as Promise<DatasetWorkTableExportScheduleBody>
}

export async function fetchWorkTableExport(exportPublicId: string): Promise<DatasetWorkTableExportBody> {
  return getDatasetWorkTableExport({
    path: { exportPublicId },
  }) as unknown as Promise<DatasetWorkTableExportBody>
}

export function workTableExportDownloadUrl(exportPublicId: string): string {
  return `/api/v1/dataset-work-table-exports/${encodeURIComponent(exportPublicId)}/download`
}

export async function createDatasetFromDriveFile(body: DatasetCreateBodyWritable): Promise<DatasetBody> {
  await ensureCSRFCookie()
  return createDataset({
    headers: csrfHeaders(),
    body,
  }) as unknown as Promise<DatasetBody>
}

export async function deleteDatasetItem(datasetPublicId: string): Promise<void> {
  await ensureCSRFCookie()
  await deleteDataset({
    headers: csrfHeaders(),
    path: { datasetPublicId },
  })
}

export async function createDatasetQuery(body: DatasetQueryCreateBodyWritable): Promise<DatasetQueryJobBody> {
  await ensureCSRFCookie()
  return createDatasetQueryJob({
    headers: csrfHeaders(),
    body,
  }) as unknown as Promise<DatasetQueryJobBody>
}

export async function createDatasetScopedQuery(datasetPublicId: string, body: DatasetQueryCreateBodyWritable): Promise<DatasetQueryJobBody> {
  await ensureCSRFCookie()
  return createDatasetScopedQueryJob({
    headers: csrfHeaders(),
    path: { datasetPublicId },
    body,
  }) as unknown as Promise<DatasetQueryJobBody>
}

export async function fetchDatasetQueryJobs(): Promise<DatasetQueryJobBody[]> {
  const data = await listDatasetQueryJobs({
    query: { limit: 25 },
  }) as unknown as { items?: DatasetQueryJobBody[] | null }
  return data.items ?? []
}

export async function fetchDatasetScopedQueryJobs(datasetPublicId: string): Promise<DatasetQueryJobBody[]> {
  const data = await listDatasetScopedQueryJobs({
    path: { datasetPublicId },
    query: { limit: 25 },
  }) as unknown as { items?: DatasetQueryJobBody[] | null }
  return data.items ?? []
}
