/**
 * @module presence
 *
 * Ref-counted presence system for DM online/offline status.
 * Sends periodic heartbeats to mark the current user as online and polls
 * the status of tracked users (1:1 DM conversation partners).
 *
 * @example
 * ```ts
 * PresenceTracker.setTrackedUsers(['user-123', 'user-456']);
 * const stopTracking = PresenceTracker.start();
 * PresenceTracker.onUpdate((online) => {
 *   console.log('user-123 is', online['user-123'] ? 'online' : 'offline');
 * });
 * // later: stopTracking() when the DM view unmounts
 * ```
 */

import { ForumlineAPI } from './client.js';

const HEARTBEAT_MS = 30000;
const POLL_MS = 30000;

type OnlineMap = Record<string, boolean>;
type UpdateListener = (onlineUsers: OnlineMap) => void;
type Unsubscribe = () => void;

let onlineUsers: OnlineMap = {};
let trackedUserIds: string[] = [];
let heartbeatTimer: ReturnType<typeof setInterval> | null = null;
let pollTimer: ReturnType<typeof setInterval> | null = null;
let refCount = 0;
const updateListeners = new Set<UpdateListener>();

function _notify(): void {
  for (const fn of updateListeners) {
    try {
      fn(onlineUsers);
    } catch (e) {
      console.error('[PresenceTracker]', e);
    }
  }
}

function _sendHeartbeat(): void {
  ForumlineAPI.presenceHeartbeat().catch(() => {});
}

async function _pollPresence(): Promise<void> {
  if (!trackedUserIds.length) return;
  try {
    const result = await ForumlineAPI.getPresenceStatus(trackedUserIds);
    if (!result) return;
    let changed = false;
    const newOnline: OnlineMap = {};
    for (const uid of trackedUserIds) {
      const val = result[uid];
      const isOn =
        typeof val === 'boolean' ? val : (val && (val as { online: boolean }).online) || false;
      newOnline[uid] = isOn;
      if (onlineUsers[uid] !== isOn) changed = true;
    }
    onlineUsers = newOnline;
    if (changed) _notify();
  } catch {
    // silently ignore polling errors
  }
}

/**
 * Start presence tracking. Begins sending heartbeats and polling tracked users.
 * Ref-counted — safe to call from multiple components.
 * @returns Unsubscribe function that decrements the ref count and stops tracking when it hits zero.
 */
function start(): Unsubscribe {
  refCount++;
  if (refCount === 1) {
    _sendHeartbeat();
    void _pollPresence();
    heartbeatTimer = setInterval(_sendHeartbeat, HEARTBEAT_MS);
    pollTimer = setInterval(_pollPresence, POLL_MS);
  }
  return () => stop();
}

/**
 * Decrement the ref count. When it reaches zero, stops all timers
 * and clears tracked state.
 */
function stop(): void {
  refCount--;
  if (refCount <= 0) {
    refCount = 0;
    if (heartbeatTimer) {
      clearInterval(heartbeatTimer);
      heartbeatTimer = null;
    }
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
    onlineUsers = {};
    trackedUserIds = [];
  }
}

/**
 * Set which user IDs to poll for online status.
 * Triggers an immediate poll if tracking is active.
 * @param userIds - Array of user IDs (typically 1:1 DM conversation partners).
 */
function setTrackedUsers(userIds: string[]): void {
  trackedUserIds = userIds || [];
  if (refCount > 0 && trackedUserIds.length) void _pollPresence();
}

/**
 * Check if a specific user is currently online.
 * @param userId - User ID to check.
 * @returns `true` if the user was reported online in the last poll.
 */
function isOnline(userId: string): boolean {
  return !!onlineUsers[userId];
}

/**
 * Register a callback that fires whenever online/offline status changes.
 * Receives the full `{ [userId]: boolean }` map.
 * @returns Unsubscribe function.
 */
function onUpdate(callback: UpdateListener): Unsubscribe {
  updateListeners.add(callback);
  return () => updateListeners.delete(callback);
}

/** Pause heartbeats and polling (e.g. when the app is backgrounded). */
function pause(): void {
  if (heartbeatTimer) {
    clearInterval(heartbeatTimer);
    heartbeatTimer = null;
  }
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

/** Resume heartbeats and polling after a {@link pause}. */
function resume(): void {
  if (refCount > 0) {
    if (!heartbeatTimer) {
      _sendHeartbeat();
      heartbeatTimer = setInterval(_sendHeartbeat, HEARTBEAT_MS);
    }
    if (!pollTimer) {
      void _pollPresence();
      pollTimer = setInterval(_pollPresence, POLL_MS);
    }
  }
}

/**
 * Ref-counted presence tracker for DM online/offline indicators.
 * Sends heartbeats every 30s and polls tracked user status every 30s.
 */
export const PresenceTracker = { start, stop, setTrackedUsers, isOnline, onUpdate, pause, resume };
