import { expect, test } from '@playwright/test'
import { writeFile } from 'node:fs/promises'

import { login, selectTenant } from './fixtures/auth'
import { runId } from './fixtures/run-id'

test.describe('P5 Drive UI', () => {
  test.skip(process.env.E2E_OPENFGA_ENABLED !== 'true', 'Set E2E_OPENFGA_ENABLED=true with OPENFGA_* env to run Drive UI E2E.')

  test('owner can manage files and create a no-download public link', async ({ page, context }, testInfo) => {
    const id = runId(testInfo)
    const folderName = `P5 folder ${id}`
    const originalName = `drive-${id}.txt`
    const renamedName = `drive-${id}-renamed.txt`
    const uploadPath = testInfo.outputPath(originalName)

    await writeFile(uploadPath, `hello from drive ${id}\n`)

    await login(page)
    await selectTenant(page, 'acme')

    await page.getByRole('link', { name: 'Drive' }).click()
    await expect(page.getByRole('heading', { name: 'Drive Browser' })).toBeVisible()

    await page.getByLabel('New folder').fill(folderName)
    await page.getByRole('button', { name: 'Create' }).click()
    await expect(page.getByRole('button', { name: folderName })).toBeVisible()

    await page.getByRole('button', { name: folderName }).click()
    await expect(page.getByLabel('Drive breadcrumbs').getByText(folderName)).toBeVisible()

    await page.locator('input[type="file"]').first().setInputFiles(uploadPath)
    await expect(page.getByText(originalName)).toBeVisible()

    const row = page.getByRole('row').filter({ hasText: originalName })
    const downloadPromise = page.waitForEvent('download')
    await row.getByRole('button', { name: 'Download' }).click()
    const download = await downloadPromise
    expect(download.suggestedFilename()).toBe(originalName)

    await row.getByRole('button', { name: 'Rename' }).click()
    const renameDialog = page.locator('dialog').filter({ hasText: 'Rename item' })
    await renameDialog.getByLabel('New name').fill(renamedName)
    await renameDialog.getByRole('button', { name: 'Rename' }).click()
    await expect(page.getByText(renamedName)).toBeVisible()

    await page.getByRole('row').filter({ hasText: renamedName }).getByRole('button', { name: 'Share' }).click()
    await expect(page.getByRole('heading', { name: renamedName })).toBeVisible()
    await page.getByLabel('Allow download').uncheck()
    await page.getByRole('button', { name: 'Create link' }).click()
    const rawURL = await page.getByLabel('New link URL').inputValue()
    expect(rawURL).toContain('/public/drive/share-links/')

    const publicPage = await context.newPage()
    await publicPage.goto(rawURL)
    await expect(publicPage.getByRole('heading', { name: renamedName })).toBeVisible()
    await expect(publicPage.getByText('Download blocked')).toBeVisible()
    await expect(publicPage.getByRole('link', { name: 'Download' })).toHaveCount(0)
    await publicPage.close()

    await page.getByRole('button', { name: 'Disable' }).last().click()
    const deniedPage = await context.newPage()
    await deniedPage.goto(rawURL)
    await expect(deniedPage.locator('.error-message')).toContainText(/not found|permission denied/i)
    await deniedPage.close()
  })
})
