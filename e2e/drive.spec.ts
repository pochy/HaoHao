import { expect, test } from '@playwright/test'
import { writeFile } from 'node:fs/promises'

import { login, selectTenant } from './fixtures/auth'
import { runId } from './fixtures/run-id'

async function dropTextFiles(page: import('@playwright/test').Page, files: Array<{ name: string, text: string }>) {
  const dataTransfer = await page.evaluateHandle((items) => {
    const transfer = new DataTransfer()
    for (const item of items) {
      transfer.items.add(new File([item.text], item.name, { type: 'text/plain' }))
    }
    return transfer
  }, files)
  const target = page.locator('.drive-workspace-content')
  await target.dispatchEvent('dragenter', { dataTransfer })
  await target.dispatchEvent('dragover', { dataTransfer })
  await target.dispatchEvent('drop', { dataTransfer })
  await dataTransfer.dispose()
}

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

    await page.getByRole('button', { name: 'New folder' }).click()
    const folderDialog = page.locator('dialog').filter({ hasText: 'New folder' })
    await folderDialog.getByLabel('Folder name').fill(folderName)
    await folderDialog.getByRole('button', { name: 'Create folder' }).click()
    await expect(page.getByRole('button', { name: folderName })).toBeVisible()

    await page.getByRole('button', { name: folderName }).click()
    await expect(page.getByLabel('Drive breadcrumbs').getByText(folderName)).toBeVisible()

    await page.locator('input[type="file"]').first().setInputFiles(uploadPath)
    await expect(page.getByText(originalName)).toBeVisible()

    await page.getByRole('button', { name: 'List view' }).click()
    const row = page.getByRole('row').filter({ hasText: originalName })
    const downloadPromise = page.waitForEvent('download')
    await row.getByLabel(`Actions for ${originalName}`).click()
    await row.getByRole('menuitem', { name: 'Download' }).click()
    const download = await downloadPromise
    expect(download.suggestedFilename()).toBe(originalName)

    await row.getByLabel(`Actions for ${originalName}`).click()
    await row.getByRole('menuitem', { name: 'Rename' }).click()
    const renameDialog = page.locator('dialog').filter({ hasText: 'Rename item' })
    await renameDialog.getByLabel('New name').fill(renamedName)
    await renameDialog.getByRole('button', { name: 'Rename' }).click()
    await expect(page.getByText(renamedName)).toBeVisible()

    const renamedRow = page.getByRole('row').filter({ hasText: renamedName })
    await renamedRow.getByLabel(`Actions for ${renamedName}`).click()
    await renamedRow.getByRole('menuitem', { name: 'Share' }).click()
    await expect(page.getByRole('heading', { name: renamedName })).toBeVisible()
    await page.getByLabel('Allow download').uncheck()
    await page.getByRole('button', { name: 'Create link' }).click()
    const rawURL = await page.getByLabel('New link URL').inputValue()
    expect(rawURL).toContain('/public/drive/share-links/')
    await page.getByRole('button', { name: 'Copy link' }).click()
    await expect(page.getByRole('button', { name: 'Copied' })).toBeVisible()

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

  test('P16 completion flows cover upload queue, metadata, preview, archive, folder tree, and public folder browser', async ({ page, context }, testInfo) => {
    const id = runId(testInfo)
    const folderName = `P16 folder ${id}`
    const nestedFolderName = `P16 nested ${id}`
    const fileName = `p16-drag-${id}.txt`
    const secondFileName = `p16-second-${id}.txt`

    await login(page)
    await selectTenant(page, 'acme')

    await page.getByRole('link', { name: 'Drive' }).click()
    await expect(page.getByRole('heading', { name: 'Drive Browser' })).toBeVisible()

    await page.getByRole('button', { name: 'New folder' }).click()
    const folderDialog = page.locator('dialog').filter({ hasText: 'New folder' })
    await folderDialog.getByLabel('Folder name').fill(folderName)
    await folderDialog.getByRole('button', { name: 'Create folder' }).click()
    await expect(page.getByRole('button', { name: folderName })).toBeVisible()

    await page.getByRole('button', { name: folderName }).click()
    await expect(page.getByLabel('Drive breadcrumbs').getByText(folderName)).toBeVisible()

    await page.getByRole('button', { name: 'New folder' }).click()
    await folderDialog.getByLabel('Folder name').fill(nestedFolderName)
    await folderDialog.getByRole('button', { name: 'Create folder' }).click()
    await expect(page.getByRole('button', { name: nestedFolderName })).toBeVisible()

    await dropTextFiles(page, [
      { name: fileName, text: `drag upload ${id}\n` },
      { name: secondFileName, text: `second drag upload ${id}\n` },
    ])
    await expect(page.getByText(fileName)).toBeVisible()
    await expect(page.getByText(secondFileName)).toBeVisible()
    await expect(page.getByLabel('Upload queue')).toBeVisible()

    await page.getByRole('button', { name: 'List view' }).click()
    const row = page.getByRole('row').filter({ hasText: fileName })

    await row.getByLabel(`Actions for ${fileName}`).click()
    await row.getByRole('menuitem', { name: 'Metadata' }).click()
    const metadataDialog = page.locator('dialog').filter({ hasText: 'Metadata' })
    await metadataDialog.getByLabel('Description').fill(`P16 metadata ${id}`)
    await metadataDialog.getByLabel('Tags').fill(`p16, ${id}`)
    await metadataDialog.getByRole('button', { name: 'Save metadata' }).click()
    await expect(page.getByText('Metadata を更新しました。')).toBeVisible()

    await row.getByLabel(`Actions for ${fileName}`).click()
    await row.getByRole('menuitem', { name: 'Preview' }).click()
    const previewDialog = page.locator('dialog').filter({ hasText: 'Preview' })
    await expect(previewDialog.getByText(`drag upload ${id}`)).toBeVisible()
    await previewDialog.getByRole('button', { name: 'Close' }).click()

    await page.goto(`/drive/search?q=${encodeURIComponent('p16-drag')}&type=file&sort=name&direction=asc`)
    await expect(page.getByRole('heading', { name: 'Search Drive' })).toBeVisible()
    await expect(page.getByLabel('Search Drive')).toHaveValue('p16-drag')
    await expect(page.getByRole('row').filter({ hasText: fileName })).toBeVisible()
    await page.reload()
    await expect(page.getByLabel('Search Drive')).toHaveValue('p16-drag')
    await expect(page.getByRole('row').filter({ hasText: fileName })).toBeVisible()

    await page.getByLabel(`Select ${fileName} for archive download`).check()
    const zipDownloadPromise = page.waitForEvent('download')
    await page.locator('.drive-selection-bar').getByRole('button', { name: 'Download ZIP' }).click()
    const zipDownload = await zipDownloadPromise
    expect(zipDownload.suggestedFilename()).toMatch(/\.zip$/)

    await page.goto('/drive')
    const folderToggle = page.getByLabel(new RegExp(`(Expand|Collapse) ${folderName}`))
    await expect(folderToggle).toBeVisible()
    await folderToggle.click()
    await folderToggle.click()

    await page.getByLabel(`Actions for ${folderName}`).click()
    await page.getByRole('menuitem', { name: 'Share' }).click()
    const shareDialog = page.locator('dialog').filter({ hasText: 'Share' })
    await expect(shareDialog.getByRole('heading', { name: folderName })).toBeVisible()
    await expect(shareDialog.getByRole('heading', { name: 'Owner transfer' })).toBeVisible()
    await shareDialog.getByLabel('Search users', { exact: true }).fill('limited@example.com')
    await shareDialog.getByRole('button', { name: 'Search users' }).click()
    await shareDialog.locator('.drive-target-row').filter({ hasText: 'Limited User' }).first().click()
    page.once('dialog', async (dialog) => {
      expect(dialog.message()).toContain('Owner')
      await dialog.dismiss()
    })
    await shareDialog.getByRole('button', { name: 'Transfer owner' }).click()

    await shareDialog.getByRole('button', { name: 'Create link' }).click()
    const rawURL = await shareDialog.getByLabel('New link URL').inputValue()
    const publicPage = await context.newPage()
    await publicPage.goto(rawURL)
    await expect(publicPage.getByRole('heading', { name: folderName })).toBeVisible()
    await expect(publicPage.getByRole('heading', { name: 'Folder contents' })).toBeVisible()
    await expect(publicPage.getByText(fileName)).toBeVisible()
    await expect(publicPage.getByText(nestedFolderName)).toBeVisible()
    await publicPage.close()
  })
})
