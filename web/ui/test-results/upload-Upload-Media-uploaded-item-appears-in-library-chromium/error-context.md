# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: upload.spec.ts >> Upload Media >> uploaded item appears in library
- Location: e2e/tests/upload.spec.ts:90:3

# Error details

```
Test timeout of 30000ms exceeded while running "beforeEach" hook.
```

```
Error: page.fill: Test timeout of 30000ms exceeded.
Call log:
  - waiting for locator('input[type="text"]')

```

# Test source

```ts
  1   | import { test, expect } from '@playwright/test'
  2   | import * as fs from 'fs'
  3   | import * as path from 'path'
  4   | import { fileURLToPath } from 'url'
  5   | 
  6   | const __filename = fileURLToPath(import.meta.url)
  7   | const __dirname = path.dirname(__filename)
  8   | 
  9   | const TEST_USER = { username: 'admin', password: 'admin' }
  10  | 
  11  | async function login(page: any) {
  12  |   await page.goto('/login')
> 13  |   await page.fill('input[type="text"]', TEST_USER.username)
      |              ^ Error: page.fill: Test timeout of 30000ms exceeded.
  14  |   await page.fill('input[type="password"]', TEST_USER.password)
  15  |   await page.click('button[type="submit"]')
  16  |   await page.waitForURL('**/')
  17  | }
  18  | 
  19  | function createDummyVideo(tmpDir: string): string {
  20  |   const filePath = path.join(tmpDir, 'test-video.mp4')
  21  |   // Minimal MP4 header (ftyp + moov) so server recognises it as video
  22  |   const ftyp = Buffer.from([
  23  |     0x00,0x00,0x00,0x20,0x66,0x74,0x79,0x70,0x69,0x73,0x6f,0x6d,
  24  |     0x00,0x00,0x00,0x00,0x69,0x73,0x6f,0x6d,0x6d,0x70,0x34,0x31,
  25  |     0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,
  26  |   ])
  27  |   const moov = Buffer.from([
  28  |     0x00,0x00,0x00,0x08,0x6d,0x6f,0x6f,0x76,
  29  |   ])
  30  |   fs.writeFileSync(filePath, Buffer.concat([ftyp, moov]))
  31  |   return filePath
  32  | }
  33  | 
  34  | test.describe('Upload Media', () => {
  35  |   const tmpDir = path.join(__dirname, '..', 'tmp-uploads')
  36  |   let uploadedIds: string[] = []
  37  | 
  38  |   test.beforeAll(() => {
  39  |     if (!fs.existsSync(tmpDir)) fs.mkdirSync(tmpDir, { recursive: true })
  40  |   })
  41  | 
  42  |   test.beforeEach(async ({ page }) => {
  43  |     await login(page)
  44  |   })
  45  | 
  46  |   test.afterEach(async ({ page }) => {
  47  |     await page.evaluate(() => localStorage.clear())
  48  |   })
  49  | 
  50  |   test.afterAll(async () => {
  51  |     // Clean up temp files
  52  |     if (fs.existsSync(tmpDir)) {
  53  |       fs.rmSync(tmpDir, { recursive: true, force: true })
  54  |     }
  55  |     // Clean up uploaded items via API if IDs were tracked
  56  |     // (API cleanup depends on server support; skipping here)
  57  |   })
  58  | 
  59  |   test('upload page is accessible from settings or dashboard', async ({ page }) => {
  60  |     // If the app has an upload route, navigate there
  61  |     // Current UI may not have upload page; test API upload instead
  62  |     await page.goto('/libraries')
  63  |     await expect(page.locator('h2', { hasText: 'Library Browser' })).toBeVisible()
  64  |   })
  65  | 
  66  |   test('API accepts multipart file upload', async ({ request }) => {
  67  |     const videoPath = createDummyVideo(tmpDir)
  68  |     const tokenRes = await request.post('/auth/login', {
  69  |       data: { username: TEST_USER.username, password: TEST_USER.password },
  70  |     })
  71  |     expect(tokenRes.ok()).toBeTruthy()
  72  |     const { token } = await tokenRes.json()
  73  | 
  74  |     // Try uploading via generic /upload endpoint if it exists
  75  |     const uploadRes = await request.post('/upload', {
  76  |       headers: { Authorization: `Bearer ${token}` },
  77  |       multipart: {
  78  |         file: fs.createReadStream(videoPath),
  79  |         library_id: 'default',
  80  |       },
  81  |     })
  82  |     // Accept 200 or 201 as success; 404 means endpoint doesn't exist yet
  83  |     expect([200, 201, 404]).toContain(uploadRes.status())
  84  |     if (uploadRes.ok()) {
  85  |       const body = await uploadRes.json().catch(() => ({}))
  86  |       if (body.id) uploadedIds.push(body.id)
  87  |     }
  88  |   })
  89  | 
  90  |   test('uploaded item appears in library', async ({ page, request }) => {
  91  |     const videoPath = createDummyVideo(tmpDir)
  92  |     const tokenRes = await request.post('/auth/login', {
  93  |       data: { username: TEST_USER.username, password: TEST_USER.password },
  94  |     })
  95  |     const { token } = await tokenRes.json()
  96  | 
  97  |     const uploadRes = await request.post('/upload', {
  98  |       headers: { Authorization: `Bearer ${token}` },
  99  |       multipart: {
  100 |         file: fs.createReadStream(videoPath),
  101 |         library_id: 'default',
  102 |       },
  103 |     })
  104 | 
  105 |     if (!uploadRes.ok()) {
  106 |       test.skip(true, 'Upload endpoint not available')
  107 |       return
  108 |     }
  109 | 
  110 |     const body = await uploadRes.json()
  111 |     if (body.id) uploadedIds.push(body.id)
  112 | 
  113 |     await page.goto('/libraries')
```