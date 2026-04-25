import { readCookie } from './client'
import { deleteFile, listFiles } from './generated/sdk.gen'
import type { FileObjectBody } from './generated/types.gen'

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export async function fetchFiles(attachedToType: string, attachedToId: string): Promise<FileObjectBody[]> {
  const data = await listFiles({
    query: { attachedToType, attachedToId },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: FileObjectBody[] | null }

  return data.items ?? []
}

export async function uploadFile(form: FormData): Promise<FileObjectBody> {
  const response = await fetch('/api/v1/files', {
    method: 'POST',
    credentials: 'include',
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
      'Idempotency-Key': crypto.randomUUID(),
    },
    body: form,
  })

  if (!response.ok) {
    throw new Error((await response.json().catch(() => null))?.title ?? 'file upload failed')
  }

  return response.json() as Promise<FileObjectBody>
}

export async function deleteFileItem(filePublicId: string): Promise<void> {
  await deleteFile({
    headers: csrfHeaders(),
    path: { filePublicId },
    responseStyle: 'data',
    throwOnError: true,
  })
}
