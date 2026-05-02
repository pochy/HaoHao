import { readCookie } from './client'
import {
  createCustomerSignalSavedFilter,
  deleteCustomerSignalSavedFilter,
  listCustomerSignalSavedFilters,
  updateCustomerSignalSavedFilter,
} from './generated/sdk.gen'
import type {
  CustomerSignalSavedFilterBody,
  CustomerSignalSavedFilterRequestBodyWritable,
} from './generated/types.gen'

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export type CustomerSignalSavedFilterListParams = {
  q?: string
  status?: string
  priority?: string
  source?: string
  cursor?: string
  limit?: number
}

export type CustomerSignalSavedFilterListResult = {
  items: CustomerSignalSavedFilterBody[]
  nextCursor: string
}

export async function fetchCustomerSignalSavedFilters(
  params: CustomerSignalSavedFilterListParams = {},
): Promise<CustomerSignalSavedFilterListResult> {
  const data = await listCustomerSignalSavedFilters({
    query: {
      q: params.q || undefined,
      status: params.status || undefined,
      priority: params.priority || undefined,
      source: params.source || undefined,
      cursor: params.cursor,
      limit: params.limit ?? 25,
    },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: CustomerSignalSavedFilterBody[] | null, nextCursor?: string }

  return {
    items: data.items ?? [],
    nextCursor: data.nextCursor ?? '',
  }
}

export async function createCustomerSignalSavedFilterItem(
  body: CustomerSignalSavedFilterRequestBodyWritable,
): Promise<CustomerSignalSavedFilterBody> {
  return createCustomerSignalSavedFilter({
    headers: csrfHeaders(),
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<CustomerSignalSavedFilterBody>
}

export async function updateCustomerSignalSavedFilterItem(
  filterPublicId: string,
  body: CustomerSignalSavedFilterRequestBodyWritable,
): Promise<CustomerSignalSavedFilterBody> {
  return updateCustomerSignalSavedFilter({
    headers: csrfHeaders(),
    path: { filterPublicId },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<CustomerSignalSavedFilterBody>
}

export async function deleteCustomerSignalSavedFilterItem(filterPublicId: string): Promise<void> {
  await deleteCustomerSignalSavedFilter({
    headers: csrfHeaders(),
    path: { filterPublicId },
    responseStyle: 'data',
    throwOnError: true,
  })
}
