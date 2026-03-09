/*
 * Real-Time Event Stream
 *
 * Delivers live updates to the user's browser so new posts, chat messages, and notifications appear instantly without refreshing.
 *
 * It must:
 * - Maintain a persistent server-sent events connection to receive real-time data from the backend
 * - Automatically reconnect after network interruptions so users never miss updates
 * - Support authenticated streams by attaching the user's access token to the connection
 * - Return a cleanup function so pages can disconnect when navigating away
 */

import { getAccessToken } from './auth.js'

export function connectSSE(url, onMessage, requireAuth = false) {
  let es = null
  let reconnectTimer = null
  let cancelled = false

  async function connect() {
    if (cancelled) return

    let fullUrl = url
    if (requireAuth) {
      const token = await getAccessToken()
      if (!token) {
        reconnectTimer = setTimeout(connect, 3000)
        return
      }
      const sep = fullUrl.includes('?') ? '&' : '?'
      fullUrl = `${fullUrl}${sep}access_token=${encodeURIComponent(token)}`
    }

    es = new EventSource(fullUrl)

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        onMessage(data)
      } catch {
        // Ignore heartbeats
      }
    }

    es.onerror = () => {
      if (cancelled) return
      es?.close()
      es = null
      reconnectTimer = setTimeout(connect, 3000)
    }
  }

  connect()

  return () => {
    cancelled = true
    es?.close()
    if (reconnectTimer) clearTimeout(reconnectTimer)
  }
}
