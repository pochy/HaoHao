import { readCookie } from './client'
import {
  createCustomerSignalImport,
  getCustomerSignalImport,
  listCustomerSignalImports,
} from './generated/sdk.gen'
import type {
  CustomerSignalImportJobBody,
  CustomerSignalImportRequestBodyWritable,
} from './generated/types.gen'

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export async function fetchCustomerSignalImports(tenantSlug: string): Promise<CustomerSignalImportJobBody[]> {
  const data = await listCustomerSignalImports({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: CustomerSignalImportJobBody[] | null }

  return data.items ?? []
}

export async function createCustomerSignalImportItem(
  tenantSlug: string,
  body: CustomerSignalImportRequestBodyWritable,
): Promise<CustomerSignalImportJobBody> {
  return createCustomerSignalImport({
    headers: csrfHeaders(),
    path: { tenantSlug },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<CustomerSignalImportJobBody>
}

export async function fetchCustomerSignalImport(
  tenantSlug: string,
  importPublicId: string,
): Promise<CustomerSignalImportJobBody> {
  return getCustomerSignalImport({
    path: { tenantSlug, importPublicId },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<CustomerSignalImportJobBody>
}
