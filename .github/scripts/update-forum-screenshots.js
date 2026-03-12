#!/usr/bin/env node
/**
 * Captures screenshots of all forums in the directory and uploads to R2.
 * Updates screenshot_url and reports health status via the Forumline API.
 *
 * Usage:
 *   node .github/scripts/update-forum-screenshots.js
 *
 * Environment variables:
 *   FORUMLINE_API_URL      - default: https://app.forumline.net
 *   FORUMLINE_SERVICE_KEY  - service role key for API auth
 *   R2_ACCOUNT_ID
 *   R2_ACCESS_KEY_ID
 *   R2_SECRET_ACCESS_KEY
 *   R2_BUCKET_NAME         - default: forumline-avatars
 *   R2_PUBLIC_URL
 */

const { chromium } = require('playwright')
const { S3Client, PutObjectCommand } = require('@aws-sdk/client-s3')

const API_URL = process.env.FORUMLINE_API_URL || 'https://app.forumline.net'
const SERVICE_KEY = process.env.FORUMLINE_SERVICE_KEY
const R2_ACCOUNT_ID = process.env.R2_ACCOUNT_ID
const R2_ACCESS_KEY_ID = process.env.R2_ACCESS_KEY_ID
const R2_SECRET_ACCESS_KEY = process.env.R2_SECRET_ACCESS_KEY
const R2_BUCKET_NAME = process.env.R2_BUCKET_NAME || 'forumline-avatars'
const R2_PUBLIC_URL = process.env.R2_PUBLIC_URL

const log = (msg) => process.stderr.write(msg + '\n')

const serviceHeaders = {
  'Content-Type': 'application/json',
  'Authorization': `Bearer ${SERVICE_KEY}`,
}

async function updateScreenshotViaAPI(domain, screenshotUrl) {
  const res = await fetch(`${API_URL}/api/forums/screenshot`, {
    method: 'PUT',
    headers: serviceHeaders,
    body: JSON.stringify({ domain, screenshot_url: screenshotUrl }),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(`API returned ${res.status}: ${text}`)
  }
}

async function updateIconViaAPI(domain, iconUrl) {
  const res = await fetch(`${API_URL}/api/forums/icon`, {
    method: 'PUT',
    headers: serviceHeaders,
    body: JSON.stringify({ domain, icon_url: iconUrl }),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(`API returned ${res.status}: ${text}`)
  }
}

async function syncIconFromManifest(domain) {
  try {
    const manifestRes = await fetch(`https://${domain}/.well-known/forumline-manifest.json`, {
      signal: AbortSignal.timeout(10000),
    })
    if (!manifestRes.ok) return
    const manifest = await manifestRes.json()
    if (manifest.icon_url) {
      await updateIconViaAPI(domain, manifest.icon_url)
      log(`  Synced icon for ${domain}: ${manifest.icon_url}`)
    }
  } catch { /* non-critical */ }
}

async function reportHealth(domain, healthy) {
  try {
    const res = await fetch(`${API_URL}/api/forums/health`, {
      method: 'PUT',
      headers: serviceHeaders,
      body: JSON.stringify({ domain, healthy }),
    })
    if (!res.ok) {
      log(`  Health report failed for ${domain}: ${res.status}`)
      return null
    }
    return await res.json()
  } catch (err) {
    log(`  Health report error for ${domain}: ${err.message}`)
    return null
  }
}

async function main() {
  if (!R2_ACCOUNT_ID || !R2_ACCESS_KEY_ID || !R2_SECRET_ACCESS_KEY || !R2_PUBLIC_URL) {
    log('Missing R2 environment variables')
    process.exit(1)
  }
  if (!SERVICE_KEY) {
    log('Missing FORUMLINE_SERVICE_KEY environment variable')
    process.exit(1)
  }

  // Fetch ALL forums (including unapproved) so we can health-check everything
  log(`Fetching all forums from ${API_URL}/api/forums/all...`)
  const res = await fetch(`${API_URL}/api/forums/all`, {
    headers: { 'Authorization': `Bearer ${SERVICE_KEY}` },
  })
  if (!res.ok) {
    log(`Failed to fetch forums: ${res.status}`)
    process.exit(1)
  }
  const allForums = await res.json()
  const screenshotForums = allForums.filter(f => f.capabilities && f.capabilities.length > 0)
  log(`Found ${allForums.length} total forums, ${screenshotForums.length} with capabilities to screenshot`)

  const s3 = new S3Client({
    region: 'auto',
    endpoint: `https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com`,
    credentials: {
      accessKeyId: R2_ACCESS_KEY_ID,
      secretAccessKey: R2_SECRET_ACCESS_KEY,
    },
  })

  const browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({
    viewport: { width: 1280, height: 720 },
    deviceScaleFactor: 1,
  })

  let updated = 0
  let healthy = 0
  let unhealthy = 0

  for (const forum of screenshotForums) {
    const domain = forum.domain
    const url = forum.web_base
    const key = `screenshots/${domain.replace(/\./g, '-')}.jpg`

    log(`Capturing ${domain}...`)
    try {
      const page = await context.newPage()
      await page.goto(url, { waitUntil: 'networkidle', timeout: 30000 })
      await page.waitForTimeout(2000)
      const buffer = await page.screenshot({ type: 'jpeg', quality: 80 })
      await page.close()

      log(`  Uploading to R2: ${key} (${(buffer.length / 1024).toFixed(0)} KB)`)
      await s3.send(new PutObjectCommand({
        Bucket: R2_BUCKET_NAME,
        Key: key,
        Body: buffer,
        ContentType: 'image/jpeg',
        CacheControl: 'public, max-age=86400',
      }))

      const screenshotUrl = `${R2_PUBLIC_URL}/${key}`
      log(`  Updating API: ${domain} -> ${screenshotUrl}`)
      await updateScreenshotViaAPI(domain, screenshotUrl)
      updated++

      // Sync icon from manifest and report healthy
      await syncIconFromManifest(domain)
      await reportHealth(domain, true)
      healthy++
    } catch (err) {
      log(`  Failed: ${err.message}`)

      // Report unhealthy
      const result = await reportHealth(domain, false)
      unhealthy++
      if (result?.action === 'delisted') {
        log(`  Forum ${domain} has been delisted after ${result.consecutive_failures} consecutive failures`)
      } else if (result?.action === 'auto_deleted') {
        log(`  Unowned forum ${domain} has been auto-deleted after ${result.consecutive_failures} consecutive failures`)
      }
    }
  }

  // Health-check forums without capabilities (no screenshot needed, just probe the manifest)
  const manifestOnlyForums = allForums.filter(f => !f.capabilities || f.capabilities.length === 0)
  for (const forum of manifestOnlyForums) {
    try {
      const manifestRes = await fetch(`https://${forum.domain}/.well-known/forumline-manifest.json`, {
        signal: AbortSignal.timeout(10000),
      })
      if (manifestRes.ok) {
        const manifest = await manifestRes.json()
        if (manifest.icon_url) {
          await updateIconViaAPI(forum.domain, manifest.icon_url)
          log(`  Synced icon for ${forum.domain}: ${manifest.icon_url}`)
        }
        await reportHealth(forum.domain, true)
        healthy++
      } else {
        await reportHealth(forum.domain, false)
        unhealthy++
      }
    } catch {
      await reportHealth(forum.domain, false)
      unhealthy++
    }
  }

  await browser.close()
  log(`Done! Screenshots: ${updated}/${screenshotForums.length}. Health: ${healthy} healthy, ${unhealthy} unhealthy.`)
}

main().catch(err => {
  log(err.message)
  process.exit(1)
})
