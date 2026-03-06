/**
 * useNotificationReporter — Forwards new notifications to the parent hub via postMessage.
 *
 * Only active when the forum is embedded in an iframe.
 * Subscribes to SSE on the notification stream and sends
 * each new notification to the parent frame as a `forumline:notification` message.
 */

import { useCallback } from 'react'
import type { ForumToHubMessage } from '@johnvondrashek/forumline-protocol'
import type { ForumNotification } from '@johnvondrashek/forumline-protocol'
import { useSSE } from '../lib/sse'
import { useAuth } from '../lib/auth'

export function useNotificationReporter(userId: string | null) {
  const isEmbedded = window.parent !== window
  const { getAccessToken } = useAuth()

  const sseUrl = isEmbedded && userId ? '/api/forumline/notifications/stream' : null

  const handleSSE = useCallback((data: unknown) => {
    const raw = data as {
      id: string
      type: string
      title: string
      body: string
      timestamp: string
      read: boolean
      link: string
      forum_domain: string
    }

    const notification: ForumNotification = {
      id: raw.id,
      type: raw.type as ForumNotification['type'],
      title: raw.title,
      body: raw.body,
      timestamp: raw.timestamp,
      read: raw.read,
      link: raw.link || '',
      forum_domain: raw.forum_domain || window.location.hostname,
    }

    const msg: ForumToHubMessage = { type: 'forumline:notification', notification }
    window.parent.postMessage(msg, '*')
  }, [])

  useSSE(sseUrl, handleSSE, getAccessToken)
}
