import { test, expect } from '@playwright/test'

const TEST_USER = { username: 'admin', password: 'admin' }

test.describe('Auth Flow', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login')
  })

  test.afterEach(async ({ page }) => {
    await page.evaluate(() => localStorage.clear())
  })

  test('login page renders', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('AetherStream')
    await expect(page.locator('input[type="text"]')).toBeVisible()
    await expect(page.locator('input[type="password"]')).toBeVisible()
    await expect(page.locator('button[type="submit"]')).toBeVisible()
  })

  test('successful login stores JWT and redirects to dashboard', async ({ page }) => {
    await page.fill('input[type="text"]', TEST_USER.username)
    await page.fill('input[type="password"]', TEST_USER.password)
    await page.click('button[type="submit"]')

    await page.waitForURL('**/')
    await expect(page).toHaveURL('/')

    const token = await page.evaluate(() => localStorage.getItem('aetherstream_token'))
    expect(token).toBeTruthy()
    expect(token).toMatch(/^eyJ/) // JWT prefix
  })

  test('failed login shows error', async ({ page }) => {
    await page.fill('input[type="text"]', 'baduser')
    await page.fill('input[type="password"]', 'badpass')
    await page.click('button[type="submit"]')

    const error = page.locator('div', { hasText: /Login failed|Unauthorized|Invalid/ })
    await expect(error.first()).toBeVisible()
  })

  test('logout clears token and redirects to login', async ({ page }) => {
    // Login first
    await page.fill('input[type="text"]', TEST_USER.username)
    await page.fill('input[type="password"]', TEST_USER.password)
    await page.click('button[type="submit"]')
    await page.waitForURL('**/')

    // Logout via sidebar
    await page.click('button:has-text("Logout")')
    await page.waitForURL('**/login')

    const token = await page.evaluate(() => localStorage.getItem('aetherstream_token'))
    expect(token).toBeNull()
  })

  test('unauthenticated user is redirected to login', async ({ page }) => {
    await page.goto('/libraries')
    await expect(page).toHaveURL('/login')
  })
})
