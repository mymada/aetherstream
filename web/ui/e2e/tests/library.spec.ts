import { test, expect } from '@playwright/test'

const TEST_USER = { username: 'admin', password: 'admin' }

async function login(page: any) {
  await page.goto('/login')
  await page.fill('input[type="text"]', TEST_USER.username)
  await page.fill('input[type="password"]', TEST_USER.password)
  await page.click('button[type="submit"]')
  await page.waitForURL('**/')
}

test.describe('Library Browser', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test.afterEach(async ({ page }) => {
    await page.evaluate(() => localStorage.clear())
  })

  test('dashboard shows libraries list', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('h2', { hasText: 'Dashboard' })).toBeVisible()
    await expect(page.locator('text=Libraries')).toBeVisible()
  })

  test('navigate to library browser', async ({ page }) => {
    await page.click('a:has-text("Libraries")')
    await page.waitForURL('**/libraries')
    await expect(page.locator('h2', { hasText: 'Library Browser' })).toBeVisible()
    await expect(page.locator('select')).toBeVisible()
  })

  test('select a library from dropdown', async ({ page }) => {
    await page.goto('/libraries')
    const select = page.locator('select')
    const options = await select.locator('option').count()
    if (options > 1) {
      await select.selectOption({ index: 1 })
      await page.waitForLoadState('networkidle')
      const url = page.url()
      expect(url).toMatch(/\/libraries\//)
    }
  })

  test('view items grid', async ({ page }) => {
    await page.goto('/libraries')
    await expect(page.locator('text=Loading...')).not.toBeVisible()
    const grid = page.locator('div[style*="grid"]')
    await expect(grid).toBeVisible()
  })

  test('click item navigates to player', async ({ page }) => {
    await page.goto('/libraries')
    await expect(page.locator('text=Loading...')).not.toBeVisible()
    const firstLink = page.locator('a[href^="/player/"]').first()
    if (await firstLink.count() > 0) {
      await firstLink.click()
      await page.waitForURL('**/player/**')
      await expect(page.locator('video')).toBeVisible()
    }
  })
})
