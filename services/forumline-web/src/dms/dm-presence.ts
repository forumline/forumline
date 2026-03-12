/*
 * DM presence tracker
 *
 * Tracks which users are online by sending heartbeats and polling presence status.
 *
 * It must:
 * - Send a heartbeat every 30s while active to report the current user as online
 * - Poll presence status for a set of tracked user IDs every 30s
 * - Expose a reactive map of userId -> online boolean
 * - Use reference counting so heartbeat/poll only runs when components need it
 */
import type { ForumlineStore } from '../shared/forumline-store.js'
import { state } from '../shared/dom.js'

// Reactive state: map of userId -> online status
const onlineUsers = state<Record<string, boolean>>({})

let forumlineStoreRef: ForumlineStore | null = null
let heartbeatInterval: ReturnType<typeof setInterval> | null = null
let pollInterval: ReturnType<typeof setInterval> | null = null
let refCount = 0
let trackedUserIds = new Set<string>()

async function sendHeartbeat() {
  const client = forumlineStoreRef?.get().forumlineClient
  if (!client) return
  try {
    await client.presenceHeartbeat()
  } catch {
    // Silently ignore heartbeat failures
  }
}

async function pollPresence() {
  const client = forumlineStoreRef?.get().forumlineClient
  if (!client || trackedUserIds.size === 0) return
  try {
    const status = await client.getPresenceStatus([...trackedUserIds])
    // Only update if something actually changed to avoid unnecessary re-renders
    const prev = onlineUsers.val
    const changed = Object.keys(status).some(id => status[id] !== prev[id]) ||
                    Object.keys(prev).some(id => !(id in status))
    if (changed) onlineUsers.val = status
  } catch {
    // Silently ignore poll failures
  }
}

/** Update the set of user IDs to track presence for (only polls if set changed) */
function setTrackedUsers(userIds: string[]) {
  const newSet = new Set(userIds)
  // Only re-poll if the tracked set actually changed
  if (newSet.size === trackedUserIds.size && userIds.every(id => trackedUserIds.has(id))) return
  trackedUserIds = newSet
  void pollPresence()
}

/** Check if a user is online */
function isUserOnline(userId: string): boolean {
  return onlineUsers.val[userId] ?? false
}

/** Start presence tracking. Ref-counted. */
function startPresence(forumlineStore: ForumlineStore): () => void {
  forumlineStoreRef = forumlineStore
  refCount++

  if (refCount === 1) {
    // Send initial heartbeat immediately
    void sendHeartbeat()
    heartbeatInterval = setInterval(sendHeartbeat, 30_000)
    pollInterval = setInterval(pollPresence, 30_000)
  }

  return () => {
    refCount--
    if (refCount === 0) {
      if (heartbeatInterval) { clearInterval(heartbeatInterval); heartbeatInterval = null }
      if (pollInterval) { clearInterval(pollInterval); pollInterval = null }
      trackedUserIds.clear()
      onlineUsers.val = {}
    }
  }
}

export {
  onlineUsers,
  isUserOnline,
  setTrackedUsers,
  startPresence,
}
