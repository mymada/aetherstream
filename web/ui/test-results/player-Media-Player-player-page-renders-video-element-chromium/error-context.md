# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: player.spec.ts >> Media Player >> player page renders video element
- Location: e2e/tests/player.spec.ts:22:3

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
  1   | import { test, expect } from '@playwright/test'
  2   | 
  3   | const TEST_USER = { username: 'admin', password: 'admin' }
  4   | 
  5   | async function login(page: any) {
  6   |   await page.goto('/login')
> 7   |   await page.fill('input[type="text"]', TEST_USER.username)
      |              ^ Error: page.fill: Test timeout of 30000ms exceeded.
  8   |   await page.fill('input[type="password"]', TEST_USER.password)
  9   |   await page.click('button[type="submit"]')
  10  |   await page.waitForURL('**/')
  11  | }
  12  | 
  13  | test.describe('Media Player', () => {
  14  |   test.beforeEach(async ({ page }) => {
  15  |     await login(page)
  16  |   })
  17  | 
  18  |   test.afterEach(async ({ page }) => {
  19  |     await page.evaluate(() => localStorage.clear())
  20  |   })
  21  | 
  22  |   test('player page renders video element', async ({ page }) => {
  23  |     // Navigate to a library and click first item
  24  |     await page.goto('/libraries')
  25  |     await expect(page.locator('text=Loading...')).not.toBeVisible()
  26  |     const firstLink = page.locator('a[href^="/player/"]').first()
  27  |     const count = await firstLink.count()
  28  |     if (count === 0) {
  29  |       test.skip(true, 'No media items available')
  30  |       return
  31  |     }
  32  |     await firstLink.click()
  33  |     await page.waitForURL('**/player/**')
  34  |     const video = page.locator('video')
  35  |     await expect(video).toBeVisible()
  36  |   })
  37  | 
  38  |   test('video can play', async ({ page }) => {
  39  |     await page.goto('/libraries')
  40  |     await expect(page.locator('text=Loading...')).not.toBeVisible()
  41  |     const firstLink = page.locator('a[href^="/player/"]').first()
  42  |     const count = await firstLink.count()
  43  |     if (count === 0) {
  44  |       test.skip(true, 'No media items available')
  45  |       return
  46  |     }
  47  |     await firstLink.click()
  48  |     await page.waitForURL('**/player/**')
  49  |     const video = page.locator('video')
  50  |     await video.evaluate((el: HTMLVideoElement) => el.play())
  51  |     await page.waitForTimeout(500)
  52  |     const paused = await video.evaluate((el: HTMLVideoElement) => el.paused)
  53  |     expect(paused).toBe(false)
  54  |   })
  55  | 
  56  |   test('video can pause', async ({ page }) => {
  57  |     await page.goto('/libraries')
  58  |     await expect(page.locator('text=Loading...')).not.toBeVisible()
  59  |     const firstLink = page.locator('a[href^="/player/"]').first()
  60  |     const count = await firstLink.count()
  61  |     if (count === 0) {
  62  |       test.skip(true, 'No media items available')
  63  |       return
  64  |     }
  65  |     await firstLink.click()
  66  |     await page.waitForURL('**/player/**')
  67  |     const video = page.locator('video')
  68  |     await video.evaluate((el: HTMLVideoElement) => { el.play() })
  69  |     await page.waitForTimeout(500)
  70  |     await video.evaluate((el: HTMLVideoElement) => { el.pause() })
  71  |     await page.waitForTimeout(200)
  72  |     const paused = await video.evaluate((el: HTMLVideoElement) => el.paused)
  73  |     expect(paused).toBe(true)
  74  |   })
  75  | 
  76  |   test('fullscreen toggle works', async ({ page }) => {
  77  |     await page.goto('/libraries')
  78  |     await expect(page.locator('text=Loading...')).not.toBeVisible()
  79  |     const firstLink = page.locator('a[href^="/player/"]').first()
  80  |     const count = await firstLink.count()
  81  |     if (count === 0) {
  82  |       test.skip(true, 'No media items available')
  83  |       return
  84  |     }
  85  |     await firstLink.click()
  86  |     await page.waitForURL('**/player/**')
  87  |     const video = page.locator('video')
  88  | 
  89  |     // Request fullscreen via video element API
  90  |     await video.evaluate((el: HTMLVideoElement) => {
  91  |       if (el.requestFullscreen) el.requestFullscreen()
  92  |     })
  93  |     await page.waitForTimeout(300)
  94  |     const isFullscreen = await page.evaluate(() => !!document.fullscreenElement)
  95  |     expect(isFullscreen).toBe(true)
  96  | 
  97  |     // Exit fullscreen
  98  |     await page.evaluate(() => {
  99  |       if (document.exitFullscreen) document.exitFullscreen()
  100 |     })
  101 |     await page.waitForTimeout(300)
  102 |     const notFullscreen = await page.evaluate(() => !document.fullscreenElement)
  103 |     expect(notFullscreen).toBe(true)
  104 |   })
  105 | })
  106 | 
```