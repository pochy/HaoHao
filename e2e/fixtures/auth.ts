import { expect, type Page } from '@playwright/test'

export const demoUser = {
  email: 'demo@example.com',
  password: 'changeme123',
}

export const limitedUser = {
  email: 'limited@example.com',
  password: 'changeme123',
}

export async function login(page: Page, user = demoUser) {
  await page.goto('/login')
  await page.getByTestId('login-email').fill(user.email)
  await page.getByTestId('login-password').fill(user.password)
  await page.getByRole('button', { name: 'Sign in' }).click()

  await expect(page.getByTestId('identity-status')).toHaveText('Authenticated')
}

export async function selectTenant(page: Page, slug: string) {
  const selector = page.getByTestId('tenant-selector')

  await expect(selector).toBeEnabled()
  await selector.selectOption(slug)
  await expect(selector).toHaveValue(slug)
  await expect(selector).toBeEnabled()
}
