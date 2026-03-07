import { test, expect } from '@playwright/test'

test.describe('Home Page', () => {
  test('renders layout with header, sidebar, and content', async ({ page }) => {
    await page.goto('/')

    // Header renders
    await expect(page.locator('header')).toBeVisible()
    await expect(page.locator('header >> text=Forumline')).toBeVisible()

    // Sign in / Sign up links (not logged in)
    await expect(page.locator('header >> a[href="/login"]')).toBeVisible()
    await expect(page.locator('header >> a[href="/register"]')).toBeVisible()

    // Hero banner
    await expect(page.locator('text=Welcome to Forumline')).toBeVisible()

    // Recent discussions heading
    await expect(page.locator('text=Recent Discussions')).toBeVisible()
  })

  test('sidebar shows categories', async ({ page }) => {
    await page.goto('/')

    // Wait for sidebar to load categories (use the desktop aside)
    await expect(page.locator('aside >> text=Categories')).toBeVisible()
    await expect(page.locator('aside >> a[href="/c/general"]')).toBeVisible()
    await expect(page.locator('aside >> a[href="/c/tech"]')).toBeVisible()
  })

  test('sidebar shows chat channels', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('aside >> text=Chat')).toBeVisible()
  })

  test('sidebar shows voice rooms', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('aside >> text=Voice')).toBeVisible()
    await expect(page.locator('aside >> text=Lounge')).toBeVisible()
  })
})

test.describe('Navigation', () => {
  test('navigates to login page', async ({ page }) => {
    await page.goto('/')
    await page.click('header >> a[href="/login"]')
    await expect(page.locator('h1 >> text=Sign In')).toBeVisible()
    await expect(page.locator('input[name="email"]')).toBeVisible()
    await expect(page.locator('input[name="password"]')).toBeVisible()
  })

  test('navigates to register page', async ({ page }) => {
    await page.goto('/')
    await page.click('header >> a[href="/register"]')
    await expect(page.locator('h1 >> text=Create Account')).toBeVisible()
    await expect(page.locator('input[name="username"]')).toBeVisible()
  })

  test('navigates to search page', async ({ page }) => {
    await page.goto('/')
    await page.click('header >> a[href="/search"]')
    await expect(page.locator('h1 >> text=Search')).toBeVisible()
    await expect(page.locator('#search-input')).toBeVisible()
  })

  test('navigates to category page', async ({ page }) => {
    await page.goto('/')
    await page.click('aside >> a[href="/c/general"]')
    await expect(page.locator('h1 >> text=General')).toBeVisible()
  })

  test('navigates to forgot password page', async ({ page }) => {
    await page.goto('/login')
    await page.click('a[href="/forgot-password"]')
    await expect(page.locator('h1 >> text=Reset Password')).toBeVisible()
  })

  test('shows 404 for unknown routes', async ({ page }) => {
    await page.goto('/nonexistent-page')
    await expect(page.locator('text=Page Not Found')).toBeVisible()
    await expect(page.locator('text=Go Home')).toBeVisible()
  })
})

test.describe('Auth Flow', () => {
  test('can register a new user', async ({ page }) => {
    const timestamp = Date.now()
    const username = `testuser${timestamp}`
    const email = `testuser${timestamp}@example.com`

    await page.goto('/register')
    await page.fill('input[name="username"]', username)
    await page.fill('input[name="email"]', email)
    await page.fill('input[name="password"]', 'testpass123')
    await page.click('button[type="submit"]')

    // Should redirect to home after successful registration
    await page.waitForURL('/', { timeout: 10000 })
    await expect(page.locator('header')).toBeVisible()

    // Should no longer show Sign In link (user is logged in)
    await expect(page.locator('header >> a[href="/login"]')).not.toBeVisible()
  })

  test('can sign in and sign out', async ({ page }) => {
    // First register
    const timestamp = Date.now()
    const email = `logintest${timestamp}@example.com`

    await page.goto('/register')
    await page.fill('input[name="username"]', `logintest${timestamp}`)
    await page.fill('input[name="email"]', email)
    await page.fill('input[name="password"]', 'testpass123')
    await page.click('button[type="submit"]')
    await page.waitForURL('/')

    // Sign out
    await page.click('#user-menu-btn')
    await page.click('#sign-out-btn')

    // Should show Sign In again
    await expect(page.locator('header >> a[href="/login"]')).toBeVisible()

    // Sign in
    await page.goto('/login')
    await page.fill('input[name="email"]', email)
    await page.fill('input[name="password"]', 'testpass123')
    await page.click('button[type="submit"]')
    await page.waitForURL('/')

    // Should be logged in
    await expect(page.locator('header >> a[href="/login"]')).not.toBeVisible()
  })

  test('shows error for invalid credentials', async ({ page }) => {
    await page.goto('/login')
    await page.fill('input[name="email"]', 'invalid@example.com')
    await page.fill('input[name="password"]', 'wrongpassword')
    await page.click('button[type="submit"]')
    await expect(page.locator('#login-error')).toBeVisible()
  })
})

test.describe('Category Page', () => {
  test('shows category name and description', async ({ page }) => {
    await page.goto('/c/general')
    await expect(page.locator('h1')).toContainText('General')
    // Category description appears below the heading
    await expect(page.locator('#page-content p')).toBeVisible()
  })

  test('shows empty state when no threads', async ({ page }) => {
    await page.goto('/c/tech')
    await expect(page.locator('text=No threads yet')).toBeVisible()
  })
})

test.describe('Search Page', () => {
  test('shows search input with placeholder', async ({ page }) => {
    await page.goto('/search')
    const input = page.locator('#search-input')
    await expect(input).toBeVisible()
    await expect(input).toHaveAttribute('placeholder', 'Search threads and posts...')
  })

  test('shows no results for nonsense query', async ({ page }) => {
    await page.goto('/search')
    await page.fill('#search-input', 'xyznonexistent12345')
    await expect(page.locator('text=No results for')).toBeVisible({ timeout: 5000 })
  })
})

test.describe('Protected Routes', () => {
  test('bookmarks requires auth', async ({ page }) => {
    await page.goto('/bookmarks')
    await expect(page.locator('#page-content >> text=Sign in')).toBeVisible()
  })

  test('settings requires auth', async ({ page }) => {
    await page.goto('/settings')
    await expect(page.locator('#page-content >> text=Sign in')).toBeVisible()
  })
})

test.describe('Thread Creation', () => {
  test('can create a thread and view it', async ({ page }) => {
    // Register
    const timestamp = Date.now()
    const email = `threadtest${timestamp}@example.com`
    await page.goto('/register')
    await page.fill('input[name="username"]', `threadtest${timestamp}`)
    await page.fill('input[name="email"]', email)
    await page.fill('input[name="password"]', 'testpass123')
    await page.click('button[type="submit"]')
    await page.waitForURL('/')

    // Navigate to category and create thread
    await page.goto('/c/general')
    await page.click('text=New Thread')
    await expect(page.locator('h1')).toContainText('New Thread')

    const title = `Test Thread ${timestamp}`
    await page.fill('input[name="title"]', title)
    await page.fill('textarea[name="content"]', 'This is a test thread created by Playwright.')
    await page.click('button[type="submit"]')

    // Should navigate to the new thread
    await page.waitForURL(/\/t\//, { timeout: 10000 })
    await expect(page.locator('h1')).toContainText(title)
    await expect(page.locator('text=This is a test thread created by Playwright')).toBeVisible()
  })
})
