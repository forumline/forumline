/**
 * @module dm-store
 *
 * Reactive conversation list with real-time SSE updates and polling fallback.
 * Uses nanostores `onMount` for automatic lifecycle — SSE subscription opens
 * when the first subscriber attaches and closes when the last unsubscribes.
 *
 * @example
 * ```ts
 * // Subscribe to trigger SSE connection (auto-cleanup on unsubscribe)
 * const unsub = $conversations.subscribe((convos) => renderList(convos));
 * // later: unsub() when the DM view unmounts
 * ```
 */

import { atom, onMount } from 'nanostores';
import { type Conversation, ForumlineAPI } from './client.js';
import { EventStream } from './event-stream.js';

// ── Atoms ──────────────────────────────────────────────────────────────

/** Reactive list of DM conversations (sorted by last message time). */
export const $conversations = atom<Conversation[]>([]);

/** Total unread count across all conversations. */
export const $dmUnreadCount = atom(0);

/** `true` while the initial fetch is in progress. */
export const $dmInitialLoad = atom(true);

/** `true` if the initial fetch failed. */
export const $dmLoadError = atom(false);

// ── Fetch logic ────────────────────────────────────────────────────────

export async function fetchConversations(): Promise<void> {
  if (!ForumlineAPI.isAuthenticated()) return;
  try {
    const data = await ForumlineAPI.getConversations();
    $conversations.set(data || []);
    $dmUnreadCount.set((data || []).reduce((sum, c) => sum + (c.unreadCount || 0), 0));
    $dmInitialLoad.set(false);
    $dmLoadError.set(false);
  } catch {
    if ($dmInitialLoad.get()) {
      $dmLoadError.set(true);
      $dmInitialLoad.set(false);
    }
  }
}

// ── Lifecycle: auto-start SSE + polling on first subscriber ────────────

onMount($conversations, () => {
  void fetchConversations();

  let sseDebounce: ReturnType<typeof setTimeout> | null = null;
  const sseUnsub = EventStream.subscribeDm(() => {
    if (sseDebounce) clearTimeout(sseDebounce);
    sseDebounce = setTimeout(fetchConversations, 200);
  });
  const pollInterval = setInterval(fetchConversations, 30000);

  return () => {
    sseUnsub();
    if (sseDebounce) clearTimeout(sseDebounce);
    clearInterval(pollInterval);
  };
});

// ── Backward-compatible namespace export ───────────────────────────────

/**
 * Reactive DM conversation store.
 *
 * For reactive subscriptions, use the atoms directly:
 * - `$conversations` — conversation list (subscribing auto-starts SSE)
 * - `$dmUnreadCount` — total unread badge count
 * - `$dmInitialLoad` / `$dmLoadError` — loading state
 *
 * @deprecated Prefer atoms directly. This object is kept for migration.
 */
export const DmStore = {
  getConversations(): Conversation[] { return $conversations.get(); },
  getUnreadCount(): number { return $dmUnreadCount.get(); },
  isInitialLoad(): boolean { return $dmInitialLoad.get(); },
  hasError(): boolean { return $dmLoadError.get(); },
  fetchConversations,
  /** @deprecated Subscribe to `$conversations` instead — lifecycle is automatic. */
  startUpdates() {
    return $conversations.subscribe(() => {});
  },
  /** @deprecated Subscribe to `$conversations` instead. */
  onChanged(fn: () => void) {
    return $conversations.subscribe(fn);
  },
};
