import { expect, test } from '@playwright/test'

import { limitedUser, login } from './fixtures/auth'

test('limited user sees role-specific access denied UI', async ({ page }) => {
  await login(page, limitedUser)

  await page.goto('/tenant-admin')
  await expect(page.getByText('Tenant admin role required')).toBeVisible()
  await expect(page.getByText('tenant_admin')).toBeVisible()

  await page.goto('/customer-signals')
  await expect(page.getByText('Customer Signal role required')).toBeVisible()
  await expect(page.getByText('customer_signal_user')).toBeVisible()

  await page.goto('/todos')
  await expect(page.getByRole('heading', { name: 'TODO' })).toBeVisible()
})

test('single binary keeps SPA fallback separate from API and assets', async ({ page, request }) => {
  await page.goto('/customer-signals')
  await expect(page.getByRole('heading', { name: 'Login' })).toBeVisible()

  const missingAsset = await request.get('/assets/definitely-missing-p9.js')
  expect(missingAsset.status()).toBe(404)

  const missingAPI = await request.get('/api/definitely-missing-p9')
  expect(missingAPI.status()).toBe(404)
})
