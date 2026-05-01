import { apiErrorFromResponse, readCookie } from './client'
import {
  createDatasetQueryJob,
  deleteDataset,
  getCsrf,
  getDataset,
  listDatasetQueryJobs,
  listDatasets,
} from './generated/sdk.gen'
import type {
  DatasetBody,
  DatasetQueryCreateBodyWritable,
  DatasetQueryJobBody,
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

export async function uploadDatasetFile(file: File, name: string): Promise<DatasetBody> {
  await ensureCSRFCookie()
  const body = new FormData()
  body.append('file', file)
  if (name.trim()) {
    body.append('name', name.trim())
  }
  const headers = new Headers({
    Accept: 'application/json',
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    'Idempotency-Key': crypto.randomUUID(),
  })
  const response = await fetch('/api/v1/datasets', {
    method: 'POST',
    credentials: 'include',
    headers,
    body,
  })
  if (!response.ok) {
    throw await apiErrorFromResponse(response, `Dataset upload failed (${response.status})`)
  }
  return await response.json() as DatasetBody
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
