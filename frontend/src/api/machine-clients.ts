import { readCookie } from './client'
import {
  createMachineClient,
  deleteMachineClient,
  getMachineClient,
  listMachineClients,
  updateMachineClient,
} from './generated/sdk.gen'
import type {
  MachineClientBody,
  MachineClientRequestBody,
} from './generated/types.gen'

export async function fetchMachineClients(): Promise<MachineClientBody[]> {
  const data = await listMachineClients({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items: MachineClientBody[] | null }

  return data.items ?? []
}

export async function fetchMachineClient(id: number): Promise<MachineClientBody> {
  return getMachineClient({
    path: { id },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<MachineClientBody>
}

export async function createMachineClientFromForm(
  body: MachineClientRequestBody,
): Promise<MachineClientBody> {
  return createMachineClient({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<MachineClientBody>
}

export async function updateMachineClientFromForm(
  id: number,
  body: MachineClientRequestBody,
): Promise<MachineClientBody> {
  return updateMachineClient({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    path: { id },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<MachineClientBody>
}

export async function disableMachineClient(id: number): Promise<void> {
  await deleteMachineClient({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    path: { id },
    responseStyle: 'data',
    throwOnError: true,
  })
}
