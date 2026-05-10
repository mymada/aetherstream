# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: auth.spec.ts >> Auth Flow >> logout clears token and redirects to login
- Location: e2e/tests/auth.spec.ts:43:3

# Error details

```
Test timeout of 30000ms exceeded.
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
  5  | test.describe('Auth Flow', () => {
  6  |   test.beforeEach(async ({ page }) => {
  7  |     await page.goto('/login')
  8  |   })
  9  | 
  10 |   test.afterEach(async ({ page }) => {
  11 |     await page.evaluate(() => localStorage.clear())
  12 |   })
  13 | 
  14 |   test('login page renders', async ({ page }) => {
  15 |     await expect(page.locator('h1')).toContainText('AetherStream')
  16 |     await expect(page.locator('input[type="text"]')).toBeVisible()
  17 |     await expect(page.locator('input[type="password"]')).toBeVisible()
  18 |     await expect(page.locator('button[type="submit"]')).toBeVisible()
  19 |   })
  20 | 
  21 |   test('successful login stores JWT and redirects to dashboard', async ({ page }) => {
  22 |     await page.fill('input[type="text"]', TEST_USER.username)
  23 |     await page.fill('input[type="password"]', TEST_USER.password)
  24 |     await page.click('button[type="submit"]')
  25 | 
  26 |     await page.waitForURL('**/')
  27 |     await expect(page).toHaveURL('/')
  28 | 
  29 |     const token = await page.evaluate(() => localStorage.getItem('aetherstream_token'))
  30 |     expect(token).toBeTruthy()
  31 |     expect(token).toMatch(/^eyJ/) // JWT prefix
  32 |   })
  33 | 
  34 |   test('failed login shows error', async ({ page }) => {
  35 |     await page.fill('input[type="text"]', 'baduser')
  36 |     await page.fill('input[type="password"]', 'badpass')
  37 |     await page.click('button[type="submit"]')
  38 | 
  39 |     const error = page.locator('div', { hasText: /Login failed|Unauthorized|Invalid/ })
  40 |     await expect(error.first()).toBeVisible()
  41 |   })
  42 | 
  43 |   test('logout clears token and redirects to login', async ({ page }) => {
  44 |     // Login first
> 45 |     await page.fill('input[type="text"]', TEST_USER.username)
     |                ^ Error: page.fill: Test timeout of 30000ms exceeded.
  46 |     await page.fill('input[type="password"]', TEST_USER.password)
  47 |     await page.click('button[type="submit"]')
  48 |     await page.waitForURL('**/')
  49 | 
  50 |     // Logout via sidebar
  51 |     await page.click('button:has-text("Logout")')
  52 |     await page.waitForURL('**/login')
  53 | 
  54 |     const token = await page.evaluate(() => localStorage.getItem('aetherstream_token'))
  55 |     expect(token).toBeNull()
  56 |   })
  57 | 
  58 |   test('unauthenticated user is redirected to login', async ({ page }) => {
  59 |     await page.goto('/libraries')
  60 |     await expect(page).toHaveURL('/login')
  61 |   })
  62 | })
  63 | 
```