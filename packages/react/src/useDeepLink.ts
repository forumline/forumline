/**
 * useDeepLink — Handles forumline:// deep links in the Tauri desktop app.
 *
 * URL scheme: forumline://forum/{domain}/t/{threadId}
 *             forumline://forum/{domain}/chat/{channel}
 *             forumline://forum/{domain}/{path}
 *
 * When a deep link is received, it switches to the specified forum
 * and optionally navigates within it.
 */

import { useEffect } from 'react'
import { isTauri } from './tauri'

export interface DeepLinkTarget {
  domain: string
  path: string
}

/** Parse a forumline:// URL into a domain and path */
export function parseDeepLink(url: string): DeepLinkTarget | null {
  try {
    // forumline://forum/{domain}/t/{threadId}
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
 * Only active in Tauri desktop context.
 */
export function useDeepLink(onDeepLink: (target: DeepLinkTarget) => void) {
  useEffect(() => {
    if (!isTauri()) return

    let unlisten: (() => void) | undefined

    const setup = async () => {
      try {
        // Tauri deep-link plugin emits 'deep-link://new-url' events
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

    setup()
    return () => unlisten?.()
  }, [onDeepLink])
}
