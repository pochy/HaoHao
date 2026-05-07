import { expect, test } from '@playwright/test'

import { login, selectTenant } from './fixtures/auth'

test('tenant admin schema mapping evidence actions work with pointer clicks', async ({ page }) => {
  let sharedScope: 'private' | 'tenant' = 'private'
  const publicId = '019dfe97-f71a-74c2-87e1-e5f332429578'
  const localSearchJob = {
    attempts: 0,
    completedAt: undefined,
    createdAt: '2026-05-07T04:05:00Z',
    failedCount: 0,
    indexedCount: 0,
    lastError: undefined,
    publicId: '019dfe98-2f5f-7ac9-b57b-c0e6fc85f8a1',
    reason: 'admin_rebuild',
    resourceKind: undefined,
    skippedCount: 0,
    startedAt: undefined,
    status: 'queued',
    updatedAt: '2026-05-07T04:05:00Z',
  }
  let localSearchJobs: typeof localSearchJob[] = []

  await page.route('**/api/v1/admin/tenants/acme/data-pipelines/schema-mapping/examples**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback()
      return
    }
    await route.fulfill({
      json: {
        items: [
          {
            publicId,
            pipelinePublicId: '019dfe97-48aa-7879-a4c4-3c2f9b160b27',
            pipelineName: 'click smoke invoice mapping',
            sourceColumn: '請求No',
            targetColumn: 'invoice_number',
            decision: 'accepted',
            sharedScope,
            domain: 'invoice',
            schemaType: 'ap_invoice',
            sampleValues: ['INV-2026-0007', 'INV-2026-0008'],
            searchDocumentMaterialized: true,
            sharedAt: sharedScope === 'tenant' ? '2026-05-07T03:45:00Z' : null,
            updatedAt: '2026-05-07T03:45:00Z',
          },
        ],
      },
    })
  })

  await page.route('**/api/v1/admin/tenants/acme/data-pipelines/schema-mapping/examples/*/sharing', async (route) => {
    const body = route.request().postDataJSON() as { sharedScope: 'private' | 'tenant' }
    sharedScope = body.sharedScope
    await route.fulfill({
      json: {
        publicId,
        schemaColumnPublicId: '019dfe44-cc0c-7642-9d00-779d98bd1c0a',
        sourceColumn: '請求No',
        targetColumn: 'invoice_number',
        decision: 'accepted',
        sharedScope,
      },
    })
  })

  await page.route('**/api/v1/admin/tenants/acme/data-pipelines/schema-mapping/search-documents/rebuild', async (route) => {
    await route.fulfill({
      json: {
        indexed: 11,
        schemaColumnsIndexed: 10,
        mappingExamplesIndexed: 1,
      },
    })
  })

  await page.route('**/api/v1/admin/tenants/acme/drive/search/local-index/jobs', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback()
      return
    }
    await route.fulfill({ json: { items: localSearchJobs } })
  })

  await page.route('**/api/v1/admin/tenants/acme/drive/search/local-index/rebuilds', async (route) => {
    localSearchJobs = [localSearchJob]
    await route.fulfill({ json: localSearchJob })
  })

  await login(page)
  await selectTenant(page, 'acme')

  await page.goto('/tenant-admin/acme/data')
  await expect(page.getByRole('heading', { name: 'Local Search Index' })).toBeVisible()
  await page.getByRole('button', { name: 'Rebuild local search' }).click()
  await expect(page.getByText('Local search rebuild queued.')).toBeVisible()
  await expect(page.getByRole('row').filter({ hasText: 'admin_rebuild' })).toContainText('queued')

  await expect(page.getByRole('heading', { name: 'Schema Mapping Evidence' })).toBeVisible()

  const row = page.getByRole('row').filter({ hasText: '請求No' }).filter({ hasText: 'invoice_number' })
  await expect(row).toContainText(/private/i)

  await row.getByRole('button', { name: 'Share' }).click()
  await expect(row).toContainText(/tenant/i)
  await expect(row.getByRole('button', { name: 'Share' })).toBeDisabled()

  await row.getByRole('button', { name: 'Make private' }).click()
  await expect(row).toContainText(/private/i)
  await expect(row.getByRole('button', { name: 'Make private' })).toBeDisabled()

  await page.getByRole('button', { name: 'Rebuild search docs' }).click()
  await expect(page.getByText('Schema mapping search documents rebuilt.')).toBeVisible()
  await expect(page.getByText('Indexed 11: schema columns 10, mapping examples 1')).toBeVisible()
})
