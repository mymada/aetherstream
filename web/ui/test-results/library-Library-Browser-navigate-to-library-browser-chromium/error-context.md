# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: library.spec.ts >> Library Browser >> navigate to library browser
- Location: e2e/tests/library.spec.ts:28:3

# Error details

```
Test timeout of 30000ms exceeded while running "beforeEach" hook.
```

```
Error: page.fill: Test timeout of 30000ms exceeded.
Call log:
  - waiting for locator('input[type="text"]')

```

# Page snapshot

```yaml
- generic [ref=e2]: 404 page not found
```

# Test source

```ts
  1  | import { test, expect } from '@playwright/test'
  2  | 
  3  | const TEST_USER = { username: 'admin', password: 'admin' }
  4  | 
  5  | async function login(page: any) {
  6  |   await page.goto('/login')
> 7  |   await page.fill('input[type="text"]', TEST_USER.username)
     |              ^ Error: page.fill: Test timeout of 30000ms exceeded.
  8  |   await page.fill('input[type="password"]', TEST_USER.password)
  9  |   await page.click('button[type="submit"]')
  10 |   await page.waitForURL('**/')
  11 | }
  12 | 
  13 | test.describe('Library Browser', () => {
  14 |   test.beforeEach(async ({ page }) => {
  15 |     await login(page)
  16 |   })
  17 | 
  18 |   test.afterEach(async ({ page }) => {
  19 |     await page.evaluate(() => localStorage.clear())
  20 |   })
  21 | 
  22 |   test('dashboard shows libraries list', async ({ page }) => {
  23 |     await page.goto('/')
  24 |     await expect(page.locator('h2', { hasText: 'Dashboard' })).toBeVisible()
  25 |     await expect(page.locator('text=Libraries')).toBeVisible()
  26 |   })
  27 | 
  28 |   test('navigate to library browser', async ({ page }) => {
  29 |     await page.click('a:has-text("Libraries")')
  30 |     await page.waitForURL('**/libraries')
  31 |     await expect(page.locator('h2', { hasText: 'Library Browser' })).toBeVisible()
  32 |     await expect(page.locator('select')).toBeVisible()
  33 |   })
  34 | 
  35 |   test('select a library from dropdown', async ({ page }) => {
  36 |     await page.goto('/libraries')
  37 |     const select = page.locator('select')
  38 |     const options = await select.locator('option').count()
  39 |     if (options > 1) {
  40 |       await select.selectOption({ index: 1 })
  41 |       await page.waitForLoadState('networkidle')
  42 |       const url = page.url()
  43 |       expect(url).toMatch(/\/libraries\//)
  44 |     }
  45 |   })
  46 | 
  47 |   test('view items grid', async ({ page }) => {
  48 |     await page.goto('/libraries')
  49 |     await expect(page.locator('text=Loading...')).not.toBeVisible()
  50 |     const grid = page.locator('div[style*="grid"]')
  51 |     await expect(grid).toBeVisible()
  52 |   })
  53 | 
  54 |   test('click item navigates to player', async ({ page }) => {
  55 |     await page.goto('/libraries')
  56 |     await expect(page.locator('text=Loading...')).not.toBeVisible()
  57 |     const firstLink = page.locator('a[href^="/player/"]').first()
  58 |     if (await firstLink.count() > 0) {
  59 |       await firstLink.click()
  60 |       await page.waitForURL('**/player/**')
  61 |       await expect(page.locator('video')).toBeVisible()
  62 |     }
  63 |   })
  64 | })
  65 | 
```