import { readCookie } from './client'
import {
  createWebhook,
  deleteWebhook,
  listWebhookDeliveries,
  listWebhooks,
  retryWebhookDelivery,
  rotateWebhookSecret,
  updateWebhook,
} from './generated/sdk.gen'
import type {
  WebhookDeliveryBody,
  WebhookEndpointBody,
  WebhookEndpointRequestBodyWritable,
} from './generated/types.gen'

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export async function fetchWebhooks(tenantSlug: string): Promise<WebhookEndpointBody[]> {
  const data = await listWebhooks({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: WebhookEndpointBody[] | null }

  return data.items ?? []
}

export async function createWebhookItem(
  tenantSlug: string,
  body: WebhookEndpointRequestBodyWritable,
): Promise<WebhookEndpointBody> {
  return createWebhook({
    headers: csrfHeaders(),
    path: { tenantSlug },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<WebhookEndpointBody>
}

export async function updateWebhookItem(
  tenantSlug: string,
  webhookPublicId: string,
  body: WebhookEndpointRequestBodyWritable,
): Promise<WebhookEndpointBody> {
  return updateWebhook({
    headers: csrfHeaders(),
    path: { tenantSlug, webhookPublicId },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<WebhookEndpointBody>
}

export async function deleteWebhookItem(tenantSlug: string, webhookPublicId: string): Promise<void> {
  await deleteWebhook({
    headers: csrfHeaders(),
    path: { tenantSlug, webhookPublicId },
    responseStyle: 'data',
    throwOnError: true,
  })
}

export async function rotateWebhookSecretItem(
  tenantSlug: string,
  webhookPublicId: string,
): Promise<WebhookEndpointBody> {
  return rotateWebhookSecret({
    headers: csrfHeaders(),
    path: { tenantSlug, webhookPublicId },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<WebhookEndpointBody>
}

export async function fetchWebhookDeliveries(
  tenantSlug: string,
  webhookPublicId: string,
): Promise<WebhookDeliveryBody[]> {
  const data = await listWebhookDeliveries({
    path: { tenantSlug, webhookPublicId },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: WebhookDeliveryBody[] | null }

  return data.items ?? []
}

export async function retryWebhookDeliveryItem(
  tenantSlug: string,
  webhookPublicId: string,
  deliveryPublicId: string,
): Promise<WebhookDeliveryBody> {
  return retryWebhookDelivery({
    headers: csrfHeaders(),
    path: { tenantSlug, webhookPublicId, deliveryPublicId },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<WebhookDeliveryBody>
}
