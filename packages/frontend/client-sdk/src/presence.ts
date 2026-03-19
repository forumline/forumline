/**
 * @module presence
 *
 * Reactive presence system for DM online/offline status.
 * Uses nanostores `onMount` for automatic lifecycle — heartbeats and polling
 * start when the first subscriber attaches and stop when the last unsubscribes.
 *
 * @example
 * ```ts
 * setTrackedUsers(['user-123', 'user-456']);
 * const unsub = $onlineUsers.subscribe((online) => {
 *   console.log('user-123 is', online['user-123'] ? 'online' : 'offline');
 * });
 * // later: unsub() when the DM view unmounts
 * ```
 */

import { map, onMount } from 'nanostores';
import { ForumlineAPI } from './client.js';

const HEARTBEAT_MS = 30000;
const POLL_MS = 30000;

type OnlineMap = Record<string, boolean>;

// ── Atoms ──────────────────────────────────────────────────────────────

/** Reactive map of user ID → online status. */
export const $onlineUsers = map<OnlineMap>({});

// ── Internal state ─────────────────────────────────────────────────────

let trackedUserIds: string[] = [];
let heartbeatTimer: ReturnType<typeof setInterval> | null = null;
let pollTimer: ReturnType<typeof setInterval> | null = null;
let mounted = false;

// ── Internals ──────────────────────────────────────────────────────────

function _sendHeartbeat(): void {
  ForumlineAPI.presenceHeartbeat().catch(() => {});
}

async function _pollPresence(): Promise<void> {
  if (!trackedUserIds.length) return;
  try {
    const result = await ForumlineAPI.getPresenceStatus(trackedUserIds);
    if (!result) return;
    const newOnline: OnlineMap = {};
    for (const uid of trackedUserIds) {
      const val = result[uid];
      const isOn =
        typeof val === 'boolean' ? val : (val && (val as { online: boolean }).online) || false;
      newOnline[uid] = isOn;
    }
    $onlineUsers.set(newOnline);
  } catch {
    // silently ignore polling errors
  }
}

function _startTimers(): void {
  _sendHeartbeat();
  void _pollPresence();
  heartbeatTimer = setInterval(_sendHeartbeat, HEARTBEAT_MS);
  pollTimer = setInterval(_pollPresence, POLL_MS);
}

function _stopTimers(): void {
  if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null; }
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
}

// ── Lifecycle: auto-start on first subscriber ──────────────────────────

onMount($onlineUsers, () => {
  mounted = true;
  _startTimers();
  return () => {
    mounted = false;
    _stopTimers();
    trackedUserIds = [];
    $onlineUsers.set({});
  };
});

// ── Actions ────────────────────────────────────────────────────────────

/**
 * Set which user IDs to poll for online status.
 * Triggers an immediate poll if tracking is active.
 */
export function setTrackedUsers(userIds: string[]): void {
  trackedUserIds = userIds || [];
  if (mounted && trackedUserIds.length) void _pollPresence();
}

/** Check if a specific user is currently online. */
export function isOnline(userId: string): boolean {
  return !!$onlineUsers.get()[userId];
}

/** Pause heartbeats and polling (e.g. when the app is backgrounded). */
export function pause(): void {
  _stopTimers();
}

/** Resume heartbeats and polling after a {@link pause}. */
export function resume(): void {
  if (mounted) {
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

// ── Backward-compatible namespace export ───────────────────────────────

/**
 * Ref-counted presence tracker for DM online/offline indicators.
 *
 * For reactive subscriptions, use `$onlineUsers.subscribe()` directly —
 * lifecycle is automatic (heartbeats start on first subscriber).
 *
 * @deprecated Prefer `$onlineUsers` atom and standalone functions.
 */
export const PresenceTracker = {
  /** @deprecated Subscribe to `$onlineUsers` instead — lifecycle is automatic. */
  start() { return $onlineUsers.subscribe(() => {}); },
  /** @deprecated Unsubscribe from `$onlineUsers` instead. */
  stop() { /* no-op — lifecycle managed by nanostores */ },
  setTrackedUsers,
  isOnline,
  /** @deprecated Subscribe to `$onlineUsers` instead. */
  onUpdate(callback: (onlineUsers: OnlineMap) => void) {
    return $onlineUsers.subscribe(callback);
  },
  pause,
  resume,
};
