import { test } from '@playwright/test'

test.use({ screenshot: 'on' })

test('walkthrough - home page', async ({ page }) => {
  await page.goto('/')
  await page.waitForTimeout(500)
  await page.screenshot({ path: 'screenshots/01-home.png', fullPage: true })
})

test('walkthrough - login page', async ({ page }) => {
  await page.goto('/login')
  await page.screenshot({ path: 'screenshots/02-login.png' })
})

test('walkthrough - register page', async ({ page }) => {
  await page.goto('/register')
  await page.screenshot({ path: 'screenshots/03-register.png' })
})

test('walkthrough - category page', async ({ page }) => {
  await page.goto('/c/general')
  await page.waitForTimeout(300)
  await page.screenshot({ path: 'screenshots/04-category.png', fullPage: true })
})

test('walkthrough - search page', async ({ page }) => {
  await page.goto('/search')
  await page.screenshot({ path: 'screenshots/05-search.png' })
})

test('walkthrough - 404 page', async ({ page }) => {
  await page.goto('/nonexistent')
  await page.screenshot({ path: 'screenshots/06-404.png' })
})

test('walkthrough - register, create thread, view it', async ({ page }) => {
  const ts = Date.now()
  // Register
  await page.goto('/register')
  await page.fill('input[name="username"]', `screenshotuser${ts}`)
  await page.fill('input[name="email"]', `screenshotuser${ts}@example.com`)
  await page.fill('input[name="password"]', 'testpass123')
  await page.click('button[type="submit"]')
  await page.waitForURL('/')
  await page.waitForTimeout(300)
  await page.screenshot({ path: 'screenshots/07-logged-in-home.png', fullPage: true })

  // Settings
  await page.goto('/settings')
  await page.waitForTimeout(300)
  await page.screenshot({ path: 'screenshots/08-settings.png' })

  // Bookmarks
  await page.goto('/bookmarks')
  await page.waitForTimeout(300)
  await page.screenshot({ path: 'screenshots/09-bookmarks.png' })

  // Create thread
  await page.goto('/c/general')
  await page.click('text=New Thread')
  await page.waitForTimeout(200)
  await page.fill('input[name="title"]', `Screenshot Test Thread ${ts}`)
  await page.fill('textarea[name="content"]', 'This thread was created during the screenshot walkthrough. It demonstrates the full thread creation flow.')
  await page.screenshot({ path: 'screenshots/10-new-thread.png' })

  await page.click('button[type="submit"]')
  await page.waitForURL(/\/t\//, { timeout: 10000 })
  await page.waitForTimeout(500)
  await page.screenshot({ path: 'screenshots/11-thread-view.png', fullPage: true })

  // Home should now show the thread
  await page.goto('/')
  await page.waitForTimeout(500)
  await page.screenshot({ path: 'screenshots/12-home-with-thread.png', fullPage: true })

  // Chat
  await page.goto('/chat')
  await page.waitForTimeout(500)
  await page.screenshot({ path: 'screenshots/13-chat.png' })

  // Voice
  await page.goto('/voice')
  await page.waitForTimeout(300)
  await page.screenshot({ path: 'screenshots/14-voice.png' })

  // Profile
  await page.goto(`/u/screenshotuser${ts}`)
  await page.waitForTimeout(500)
  await page.screenshot({ path: 'screenshots/15-profile.png', fullPage: true })
})
