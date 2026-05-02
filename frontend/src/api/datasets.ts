import { readCookie } from './client'
import {
  createDataset,
  createDatasetQueryJob,
  deleteDataset,
  getCsrf,
  getDataset,
  listDatasetQueryJobs,
  listDatasets,
  listDatasetSourceFiles,
} from './generated/sdk.gen'
import type {
  DatasetBody,
  DatasetCreateBodyWritable,
  DatasetQueryCreateBodyWritable,
  DatasetQueryJobBody,
  DatasetSourceFileBody,
} from './generated/types.gen'

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

export async function fetchDatasetSourceFiles(query = ''): Promise<DatasetSourceFileBody[]> {
  const data = await listDatasetSourceFiles({
    query: {
      ...(query.trim() ? { q: query.trim() } : {}),
      limit: 100,
    },
  }) as unknown as { items?: DatasetSourceFileBody[] | null }
  return data.items ?? []
}

export async function createDatasetFromDriveFile(body: DatasetCreateBodyWritable): Promise<DatasetBody> {
  await ensureCSRFCookie()
  return createDataset({
    headers: csrfHeaders(),
    body,
  }) as unknown as Promise<DatasetBody>
}

export async function deleteDatasetItem(datasetPublicId: string): Promise<void> {
  await deleteDataset({
    headers: csrfHeaders(),
    path: { datasetPublicId },
  })
}

export async function createDatasetQuery(body: DatasetQueryCreateBodyWritable): Promise<DatasetQueryJobBody> {
  return createDatasetQueryJob({
    headers: csrfHeaders(),
    body,
  }) as unknown as Promise<DatasetQueryJobBody>
}

export async function fetchDatasetQueryJobs(): Promise<DatasetQueryJobBody[]> {
  const data = await listDatasetQueryJobs({
    query: { limit: 25 },
  }) as unknown as { items?: DatasetQueryJobBody[] | null }
  return data.items ?? []
}
