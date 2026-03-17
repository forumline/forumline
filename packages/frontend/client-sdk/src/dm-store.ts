/**
 * @module dm-store
 *
 * Reactive conversation list with real-time SSE updates and polling fallback.
 * Ref-counted — multiple UI components can call {@link DmStore.startUpdates}
 * and the SSE subscription stays alive until the last one unsubscribes.
 *
 * @example
 * ```ts
 * const stopUpdates = DmStore.startUpdates();
 * DmStore.onChanged(() => renderConversationList(DmStore.getConversations()));
 * // later: stopUpdates() when the DM view unmounts
 * ```
 */

import type { ForumlineDmConversation } from '@forumline/protocol';
import { ForumlineAPI } from './client.js';
import { EventStream } from './event-stream.js';

type ChangeListener = () => void;
type Unsubscribe = () => void;

let conversations: ForumlineDmConversation[] = [];
let initialLoad = true;
let loadError = false;
let unreadCount = 0;
let sseUnsub: Unsubscribe | null = null;
let sseDebounce: ReturnType<typeof setTimeout> | null = null;
let pollInterval: ReturnType<typeof setInterval> | null = null;
let refCount = 0;
const changeListeners = new Set<ChangeListener>();

/** Get the current list of DM conversations (sorted by last message time). */
function getConversations(): ForumlineDmConversation[] {
  return conversations;
}

/** Get the total number of unread messages across all conversations. */
function getUnreadCount(): number {
  return unreadCount;
}

/** Returns `true` if the initial fetch hasn't completed yet. */
function isInitialLoad(): boolean {
  return initialLoad;
}

/** Returns `true` if the initial fetch failed. */
function hasError(): boolean {
  return loadError;
}

function _notify(): void {
  for (const fn of changeListeners) fn();
}

/**
 * Register a callback that fires whenever the conversation list changes
 * (new message, read status, initial load, etc.).
 * @returns Unsubscribe function.
 */
function onChanged(fn: ChangeListener): Unsubscribe {
  changeListeners.add(fn);
  return () => changeListeners.delete(fn);
}

/** Force a fresh fetch of all conversations from the server. */
async function fetchConversations(): Promise<void> {
  if (!ForumlineAPI.isAuthenticated()) return;
  try {
    const data = await ForumlineAPI.getConversations();
    conversations = data || [];
    unreadCount = conversations.reduce((sum, c) => sum + (c.unreadCount || 0), 0);
    initialLoad = false;
    loadError = false;
    _notify();
  } catch {
    if (initialLoad) {
      loadError = true;
      initialLoad = false;
      _notify();
    }
  }
}

/**
 * Start real-time conversation updates (SSE + 30s polling fallback).
 * Ref-counted: safe to call from multiple components. The SSE connection
 * opens on the first call and closes when the last unsubscribe fires.
 *
 * @returns Unsubscribe function — call when the component unmounts.
 */
function startUpdates(): Unsubscribe {
  refCount++;
  if (refCount === 1) {
    void fetchConversations();
    sseUnsub = EventStream.subscribeDm(() => {
      if (sseDebounce) clearTimeout(sseDebounce);
      sseDebounce = setTimeout(fetchConversations, 200);
    });
    pollInterval = setInterval(fetchConversations, 30000);
  }
  return () => {
    refCount--;
    if (refCount === 0) {
      sseUnsub?.();
      sseUnsub = null;
      if (sseDebounce) {
        clearTimeout(sseDebounce);
        sseDebounce = null;
      }
      if (pollInterval) {
        clearInterval(pollInterval);
        pollInterval = null;
      }
    }
  };
}

/**
 * Reactive DM conversation store. Keeps the conversation list in sync
 * with the server via SSE events and periodic polling.
 */
export const DmStore = {
  getConversations,
  getUnreadCount,
  isInitialLoad,
  hasError,
  fetchConversations,
  startUpdates,
  onChanged,
};
