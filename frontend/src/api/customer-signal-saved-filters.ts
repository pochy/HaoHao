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

export async function fetchCustomerSignalSavedFilters(): Promise<CustomerSignalSavedFilterBody[]> {
  const data = await listCustomerSignalSavedFilters({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: CustomerSignalSavedFilterBody[] | null }

  return data.items ?? []
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
