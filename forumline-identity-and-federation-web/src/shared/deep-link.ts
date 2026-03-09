/*
 * Deep link handler
 *
 * This file parses and listens for forumline:// deep links to open specific forums and pages from outside the app.
 *
 * It must:
 * - Parse forumline://forum/<domain>/<path> URLs into a domain and path
 * - Listen for deep link events in the Tauri desktop app via the event system
 * - Invoke a callback with the parsed target so the app can navigate to the correct forum and page
 * - Return a cleanup function to unsubscribe from deep link events
 * - No-op gracefully when running in a web browser (non-Tauri environment)
 */
import { isTauri } from './tauri.js'

export interface DeepLinkTarget {
  domain: string
  path: string
}

/** Parse a forumline:// URL into a domain and path */
export function parseDeepLink(url: string): DeepLinkTarget | null {
  try {
    const match = url.match(/^forumline:\/\/forum\/([^/]+)(.*)$/)
    if (!match) return null
    return {
      domain: match[1],
      path: match[2] || '/',
    }
  } catch {
    return null
  }
}

/**
 * Listen for deep link events and invoke the callback with the parsed target.
 * Only active in Tauri desktop context. Returns a cleanup function.
 */
export function setupDeepLinkListener(onDeepLink: (target: DeepLinkTarget) => void): () => void {
  if (!isTauri()) return () => {}

  let unlisten: (() => void) | undefined

  const setup = async () => {
    try {
      const { listen } = await import('@tauri-apps/api/event')
      unlisten = await listen<string[]>('deep-link://new-url', (event) => {
        const urls = event.payload
        for (const url of urls) {
          const target = parseDeepLink(url)
          if (target) {
            onDeepLink(target)
            break
          }
        }
      })
    } catch (err) {
      console.error('[FLD:DeepLink] Failed to set up deep link listener:', err)
    }
  }

  void setup()
  return () => unlisten?.()
}
