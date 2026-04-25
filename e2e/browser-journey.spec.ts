import { expect, test } from '@playwright/test'
import { writeFile } from 'node:fs/promises'

import { login, selectTenant } from './fixtures/auth'
import { runId } from './fixtures/run-id'

test.describe.serial('P9 browser journey', () => {
  test('login, tenant, signals, files, settings, export, notifications', async ({ page }, testInfo) => {
    const id = runId(testInfo)
    const signalTitle = `P9 signal ${id}`
    const uploadPath = testInfo.outputPath(`attachment-${id}.txt`)

    await login(page)
    await selectTenant(page, 'beta')
    await selectTenant(page, 'acme')

    await page.getByRole('link', { name: 'Signals' }).click()
    await expect(page.getByRole('heading', { name: 'Signals' })).toBeVisible()

    await page.getByRole('textbox', { name: 'Customer', exact: true }).fill('Acme')
    await page.getByRole('textbox', { name: 'Title', exact: true }).fill(signalTitle)
    await page.getByRole('textbox', { name: 'Details', exact: true }).fill(`Created by Playwright ${id}`)
    await page.getByRole('button', { name: 'Add Signal' }).click()

    await expect(page.getByRole('link', { name: signalTitle })).toBeVisible()
    await page.getByRole('link', { name: signalTitle }).click()
    await expect(page.getByRole('heading', { name: 'Signal Detail' })).toBeVisible()

    await writeFile(uploadPath, `hello from ${id}\n`)
    await page.getByLabel('File').setInputFiles(uploadPath)
    await page.getByRole('button', { name: 'Upload' }).click()
    await expect(page.getByText(`attachment-${id}.txt`)).toBeVisible()

    await page.goto('/tenant-admin/acme')
    await expect(page.getByRole('heading', { name: 'Tenant Detail' })).toBeVisible()

    await page.getByTestId('tenant-file-quota').fill('104857600')
    await page.getByTestId('tenant-browser-rate-limit').fill('120')
    await page.getByRole('button', { name: 'Save common settings' }).click()
    await expect(page.getByText('Tenant common settings を更新しました。')).toBeVisible()

    await page.getByTestId('tenant-invitation-email').fill('demo@example.com')
    await page.getByTestId('tenant-invitation-role').selectOption('todo_user')
    await page.getByRole('button', { name: 'Invite' }).click()
    await expect(page.getByText('Invitation を作成しました')).toBeVisible()

    await page.getByTestId('tenant-request-export').click()
    await expect(page.getByText('Tenant data export を request しました。')).toBeVisible()
    await expect(page.getByText(/json \/ (processing|ready)/i).first()).toBeVisible()

    await page.getByRole('link', { name: 'Notifications' }).click()
    await expect(page.getByRole('heading', { name: 'Notification Center' })).toBeVisible()
    await expect(page.getByText('Tenant invitation').first()).toBeVisible()
  })
})
