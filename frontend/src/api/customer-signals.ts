import { readCookie } from './client'
import {
  createCustomerSignal,
  deleteCustomerSignal,
  getCustomerSignal,
  listCustomerSignals,
  updateCustomerSignal,
} from './generated/sdk.gen'
import type {
  CreateCustomerSignalBodyWritable,
  CustomerSignalBody,
  CustomerSignalListBody,
  UpdateCustomerSignalBodyWritable,
} from './generated/types.gen'

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export type CustomerSignalListParams = {
  q?: string
  status?: string
  priority?: string
  source?: string
  cursor?: string
  limit?: number
}

export async function fetchCustomerSignals(params: CustomerSignalListParams = {}): Promise<CustomerSignalListBody> {
  const data = await listCustomerSignals({
    query: params,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as CustomerSignalListBody

  return {
    items: data.items ?? [],
    nextCursor: data.nextCursor,
  }
}

export async function fetchCustomerSignal(signalPublicId: string): Promise<CustomerSignalBody> {
  return getCustomerSignal({
    path: { signalPublicId },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<CustomerSignalBody>
}

export async function createCustomerSignalItem(
  body: CreateCustomerSignalBodyWritable,
): Promise<CustomerSignalBody> {
  return createCustomerSignal({
    headers: csrfHeaders(),
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<CustomerSignalBody>
}

export async function updateCustomerSignalItem(
  signalPublicId: string,
  body: UpdateCustomerSignalBodyWritable,
): Promise<CustomerSignalBody> {
  return updateCustomerSignal({
    headers: csrfHeaders(),
    path: { signalPublicId },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<CustomerSignalBody>
}

export async function deleteCustomerSignalItem(signalPublicId: string): Promise<void> {
  await deleteCustomerSignal({
    headers: csrfHeaders(),
    path: { signalPublicId },
    responseStyle: 'data',
    throwOnError: true,
  })
}
