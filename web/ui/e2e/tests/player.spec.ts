import { test, expect } from '@playwright/test'

const TEST_USER = { username: 'admin', password: 'admin' }

async function login(page: any) {
  await page.goto('/login')
  await page.fill('input[type="text"]', TEST_USER.username)
  await page.fill('input[type="password"]', TEST_USER.password)
  await page.click('button[type="submit"]')
  await page.waitForURL('**/')
}

test.describe('Media Player', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test.afterEach(async ({ page }) => {
    await page.evaluate(() => localStorage.clear())
  })

  test('player page renders video element', async ({ page }) => {
    // Navigate to a library and click first item
    await page.goto('/libraries')
    await expect(page.locator('text=Loading...')).not.toBeVisible()
    const firstLink = page.locator('a[href^="/player/"]').first()
    const count = await firstLink.count()
    if (count === 0) {
      test.skip(true, 'No media items available')
      return
    }
    await firstLink.click()
    await page.waitForURL('**/player/**')
    const video = page.locator('video')
    await expect(video).toBeVisible()
  })

  test('video can play', async ({ page }) => {
    await page.goto('/libraries')
    await expect(page.locator('text=Loading...')).not.toBeVisible()
    const firstLink = page.locator('a[href^="/player/"]').first()
    const count = await firstLink.count()
    if (count === 0) {
      test.skip(true, 'No media items available')
      return
    }
    await firstLink.click()
    await page.waitForURL('**/player/**')
    const video = page.locator('video')
    await video.evaluate((el: HTMLVideoElement) => el.play())
    await page.waitForTimeout(500)
    const paused = await video.evaluate((el: HTMLVideoElement) => el.paused)
    expect(paused).toBe(false)
  })

  test('video can pause', async ({ page }) => {
    await page.goto('/libraries')
    await expect(page.locator('text=Loading...')).not.toBeVisible()
    const firstLink = page.locator('a[href^="/player/"]').first()
    const count = await firstLink.count()
    if (count === 0) {
      test.skip(true, 'No media items available')
      return
    }
    await firstLink.click()
    await page.waitForURL('**/player/**')
    const video = page.locator('video')
    await video.evaluate((el: HTMLVideoElement) => { el.play() })
    await page.waitForTimeout(500)
    await video.evaluate((el: HTMLVideoElement) => { el.pause() })
    await page.waitForTimeout(200)
    const paused = await video.evaluate((el: HTMLVideoElement) => el.paused)
    expect(paused).toBe(true)
  })

  test('fullscreen toggle works', async ({ page }) => {
    await page.goto('/libraries')
    await expect(page.locator('text=Loading...')).not.toBeVisible()
    const firstLink = page.locator('a[href^="/player/"]').first()
    const count = await firstLink.count()
    if (count === 0) {
      test.skip(true, 'No media items available')
      return
    }
    await firstLink.click()
    await page.waitForURL('**/player/**')
    const video = page.locator('video')

    // Request fullscreen via video element API
    await video.evaluate((el: HTMLVideoElement) => {
      if (el.requestFullscreen) el.requestFullscreen()
    })
    await page.waitForTimeout(300)
    const isFullscreen = await page.evaluate(() => !!document.fullscreenElement)
    expect(isFullscreen).toBe(true)

    // Exit fullscreen
    await page.evaluate(() => {
      if (document.exitFullscreen) document.exitFullscreen()
    })
    await page.waitForTimeout(300)
    const notFullscreen = await page.evaluate(() => !document.fullscreenElement)
    expect(notFullscreen).toBe(true)
  })
})
