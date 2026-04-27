import { expect, test } from '@playwright/test'

import { login } from './fixtures/auth'

test('user can switch locale and keep it after reload', async ({ page }) => {
  await login(page)

  await page.getByTestId('locale-switcher').selectOption('ja')
  await expect(page.getByRole('heading', { name: 'セッション', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible()
  await expect(page.getByRole('link', { name: '通知', exact: true })).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('lang', 'ja')

  await page.getByRole('link', { name: 'シグナル', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'シグナル', exact: true })).toBeVisible()
  await expect(page.getByRole('textbox', { name: '顧客', exact: true })).toBeVisible()

  await page.reload()
  await expect(page.getByTestId('locale-switcher')).toHaveValue('ja')
  await expect(page.getByRole('link', { name: '通知', exact: true })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'シグナル', exact: true })).toBeVisible()

  await page.getByTestId('locale-switcher').selectOption('en')
  await expect(page.getByRole('link', { name: 'Notifications', exact: true })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Signals', exact: true })).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('lang', 'en')
})
