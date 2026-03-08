#!/usr/bin/env node
/**
 * Captures screenshots of all forums in the directory and uploads to R2.
 * Outputs SQL to update screenshot_url in forumline_forums.
 *
 * Usage:
 *   node scripts/update-forum-screenshots.js
 *
 * Pipe the SQL output to update the database:
 *   node scripts/update-forum-screenshots.js | ssh root@host "pct exec 101 -- docker exec -i forumline-postgres-1 psql -U postgres"
 *
 * Environment variables:
 *   FORUMLINE_API_URL      - default: https://app.forumline.net
 *   R2_ACCOUNT_ID
 *   R2_ACCESS_KEY_ID
 *   R2_SECRET_ACCESS_KEY
 *   R2_BUCKET_NAME         - default: forumline-avatars
 *   R2_PUBLIC_URL
 */

const { chromium } = require('playwright')
const { S3Client, PutObjectCommand } = require('@aws-sdk/client-s3')

const API_URL = process.env.FORUMLINE_API_URL || 'https://app.forumline.net'
const R2_ACCOUNT_ID = process.env.R2_ACCOUNT_ID
const R2_ACCESS_KEY_ID = process.env.R2_ACCESS_KEY_ID
const R2_SECRET_ACCESS_KEY = process.env.R2_SECRET_ACCESS_KEY
const R2_BUCKET_NAME = process.env.R2_BUCKET_NAME || 'forumline-avatars'
const R2_PUBLIC_URL = process.env.R2_PUBLIC_URL

const log = (msg) => process.stderr.write(msg + '\n')

async function main() {
  if (!R2_ACCOUNT_ID || !R2_ACCESS_KEY_ID || !R2_SECRET_ACCESS_KEY || !R2_PUBLIC_URL) {
    log('Missing R2 environment variables')
    process.exit(1)
  }

  log(`Fetching forums from ${API_URL}/api/forums...`)
  const res = await fetch(`${API_URL}/api/forums`)
  const forums = await res.json()
  const realForums = forums.filter(f => f.capabilities && f.capabilities.length > 0)
  log(`Found ${realForums.length} forums to screenshot`)

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

  const sqlStatements = []

  for (const forum of realForums) {
    const domain = forum.domain
    const url = forum.web_base
    const key = `screenshots/${domain.replace(/\./g, '-')}.png`

    log(`Capturing ${domain}...`)
    try {
      const page = await context.newPage()
      await page.goto(url, { waitUntil: 'networkidle', timeout: 30000 })
      await page.waitForTimeout(2000)
      const buffer = await page.screenshot({ type: 'png' })
      await page.close()

      log(`  Uploading to R2: ${key}`)
      await s3.send(new PutObjectCommand({
        Bucket: R2_BUCKET_NAME,
        Key: key,
        Body: buffer,
        ContentType: 'image/png',
        CacheControl: 'public, max-age=3600',
      }))

      const screenshotUrl = `${R2_PUBLIC_URL}/${key}`
      log(`  URL: ${screenshotUrl}`)

      const escapedUrl = screenshotUrl.replace(/'/g, "''")
      const escapedDomain = domain.replace(/'/g, "''")
      sqlStatements.push(
        `UPDATE forumline_forums SET screenshot_url = '${escapedUrl}', updated_at = now() WHERE domain = '${escapedDomain}';`
      )
    } catch (err) {
      log(`  Failed: ${err.message}`)
    }
  }

  await browser.close()

  // Output SQL to stdout (can be piped to psql)
  if (sqlStatements.length) {
    console.log(sqlStatements.join('\n'))
  }

  log('Done!')
}

main().catch(err => {
  log(err.message)
  process.exit(1)
})
