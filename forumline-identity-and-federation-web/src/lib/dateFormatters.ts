/*
 * Date and time formatting utilities
 *
 * This file provides human-friendly date formatting for timestamps shown in the DM UI.
 *
 * It must:
 * - Format recent timestamps as relative shorthand (now, 5m, 3h, 2d) for conversation list previews
 * - Fall back to a localized date string for timestamps older than 7 days
 * - Format message timestamps as localized clock times (e.g., "2:30 PM") for the message thread
 */
export function formatShortTimeAgo(date: Date): string {
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const minutes = Math.floor(diff / 60000)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  if (minutes < 1) return 'now'
  if (minutes < 60) return `${minutes}m`
  if (hours < 24) return `${hours}h`
  if (days < 7) return `${days}d`
  return date.toLocaleDateString()
}

export function formatMessageTime(date: Date): string {
  return date.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
  })
}
