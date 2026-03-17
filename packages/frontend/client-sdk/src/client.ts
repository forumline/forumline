/**
 * @module client
 *
 * HTTP client singleton for all Forumline API calls.
 * Must be configured with {@link ForumlineAPI.configure} before use.
 *
 * @example
 * ```ts
 * ForumlineAPI.configure({ baseUrl: '', accessToken: token, userId: uid });
 * const convos = await ForumlineAPI.getConversations();
 * ```
 */

import type {
  ForumlineDirectMessage,
  ForumlineDmConversation,
  ForumlineProfile,
  ForumNotification,
  UnreadCounts,
} from '@forumline/protocol';

/** Options for {@link ForumlineAPI.configure}. All fields are optional — only provided fields are updated. */
export interface ConfigureOptions {
  /** Base URL for API requests (e.g. `""` for same-origin, or `"https://app.forumline.net"`). */
  baseUrl?: string;
  /** Bearer token for authenticated requests. Set to `null` to clear. */
  accessToken?: string | null;
  /** Current user's ID. Set to `null` to clear. */
  userId?: string | null;
}

export interface ApiFetchOptions extends RequestInit {
  /** When `true`, suppresses console.error logging on failure. */
  silent?: boolean;
}

let _accessToken: string | null = null;
let _userId: string | null = null;
let _baseUrl = '';

/**
 * Set the API base URL, access token, and/or user ID.
 * Typically called once after authentication succeeds.
 */
function configure({ baseUrl, accessToken, userId }: ConfigureOptions): void {
  if (baseUrl !== undefined) _baseUrl = baseUrl;
  if (accessToken !== undefined) _accessToken = accessToken ?? null;
  if (userId !== undefined) _userId = userId ?? null;
}

/** Returns the current access token, or `null` if not authenticated. */
function getToken(): string | null {
  return _accessToken;
}

/** Returns the current user's ID, or `null` if not set. */
function getUserId(): string | null {
  return _userId;
}

/** Returns `true` if an access token is currently set. */
function isAuthenticated(): boolean {
  return !!_accessToken;
}

/**
 * Authenticated fetch wrapper. Automatically injects the Bearer token and
 * Content-Type header. Parses JSON responses.
 *
 * @typeParam T - Expected response body type.
 * @param path - API path relative to the configured base URL (e.g. `/api/conversations`).
 * @param options - Standard fetch options plus an optional `silent` flag.
 * @throws {Error} If not authenticated or the server returns a non-2xx status.
 */
async function apiFetch<T = unknown>(path: string, options: ApiFetchOptions = {}): Promise<T> {
  if (!_accessToken) throw new Error('Not authenticated');
  const { silent, ...fetchOpts } = options;
  try {
    const res = await fetch(`${_baseUrl}${path}`, {
      ...fetchOpts,
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${_accessToken}`,
        ...fetchOpts.headers,
      },
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || `API error: ${res.status}`);
    }
    const text = await res.text();
    return (text ? JSON.parse(text) : null) as T;
  } catch (err) {
    if (!silent) console.error('[ForumlineAPI]', (err as Error).message);
    throw err;
  }
}

export interface GetMessagesOpts {
  /** Cursor — return messages created before this message ID. */
  before?: string;
  /** Maximum number of messages to return. */
  limit?: number;
}

export interface ConversationUpdates {
  /** New display name for the conversation. */
  name?: string;
}

/** Fetch all DM conversations for the current user. */
function getConversations(): Promise<ForumlineDmConversation[]> {
  return apiFetch('/api/conversations');
}

/** Fetch a single conversation by ID, including member list. */
function getConversation(id: string): Promise<ForumlineDmConversation> {
  return apiFetch(`/api/conversations/${id}`);
}

/**
 * Fetch messages in a conversation with optional cursor-based pagination.
 * @param id - Conversation ID.
 * @param opts - Pagination options (`before` cursor, `limit`).
 */
function getMessages(id: string, opts: GetMessagesOpts = {}): Promise<ForumlineDirectMessage[]> {
  const params = new URLSearchParams();
  if (opts.before) params.set('before', opts.before);
  if (opts.limit) params.set('limit', String(opts.limit));
  const qs = params.toString();
  return apiFetch(`/api/conversations/${id}/messages${qs ? '?' + qs : ''}`);
}

/**
 * Send a text message in a conversation.
 * @param id - Conversation ID.
 * @param content - Message body text.
 */
function sendMessage(id: string, content: string): Promise<ForumlineDirectMessage> {
  return apiFetch(`/api/conversations/${id}/messages`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  });
}

/** Mark all messages in a conversation as read. */
function markRead(id: string): Promise<void> {
  return apiFetch(`/api/conversations/${id}/read`, { method: 'POST', silent: true });
}

/**
 * Get or create a 1:1 DM conversation with another user.
 * Returns the existing conversation if one already exists.
 * @param userId - The other user's ID.
 */
function getOrCreateDM(userId: string): Promise<ForumlineDmConversation> {
  return apiFetch('/api/conversations/dm', {
    method: 'POST',
    body: JSON.stringify({ userId }),
  });
}

/**
 * Create a new group conversation.
 * @param memberIds - Array of user IDs to include (besides the current user).
 * @param name - Optional display name for the group.
 */
function createGroupConversation(
  memberIds: string[],
  name?: string,
): Promise<ForumlineDmConversation> {
  return apiFetch('/api/conversations', {
    method: 'POST',
    body: JSON.stringify({ memberIds, name }),
  });
}

/**
 * Update a conversation's metadata (e.g. rename a group).
 * @param id - Conversation ID.
 * @param updates - Fields to update.
 */
function updateConversation(id: string, updates: ConversationUpdates): Promise<void> {
  return apiFetch(`/api/conversations/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(updates),
  });
}

/** Leave a group conversation. Cannot be undone. */
function leaveConversation(id: string): Promise<void> {
  return apiFetch(`/api/conversations/${id}/leave`, { method: 'POST' });
}

/** Search user profiles by username or display name. */
function searchProfiles(query: string): Promise<ForumlineProfile[]> {
  return apiFetch(`/api/profiles/search?q=${encodeURIComponent(query)}`);
}

/** Search identities across the Forumline network by username or display name. */
function searchIdentity(query: string): Promise<ForumlineProfile[]> {
  return apiFetch(`/api/identity/search?q=${encodeURIComponent(query)}`);
}

interface ActivityItem {
  id: string;
  type: string;
  [key: string]: unknown;
}

/** Fetch the current user's recent activity feed. */
function getActivity(): Promise<ActivityItem[]> {
  return apiFetch('/api/activity');
}

/** Fetch all notifications for the current user. */
function getNotifications(): Promise<ForumNotification[]> {
  return apiFetch('/api/notifications');
}

/** Fetch aggregated unread counts (notifications, chat mentions, DMs). */
function getUnreadCount(): Promise<UnreadCounts> {
  return apiFetch('/api/notifications/unread');
}

/** Mark a single notification as read by ID. */
function markNotificationRead(id: string): Promise<void> {
  return apiFetch('/api/notifications/read', {
    method: 'POST',
    body: JSON.stringify({ id }),
  });
}

/** Mark all notifications as read. */
function markAllNotificationsRead(): Promise<void> {
  return apiFetch('/api/notifications/read-all', { method: 'POST' });
}

/** Send a presence heartbeat so the server knows we're online. Called automatically by {@link PresenceTracker}. */
function presenceHeartbeat(): Promise<void> {
  return apiFetch('/api/presence/heartbeat', { method: 'POST', silent: true });
}

interface PresenceStatusMap {
  [userId: string]: boolean | { online: boolean };
}

/**
 * Batch-fetch online/offline status for a list of user IDs.
 * Returns an empty object if the list is empty.
 */
function getPresenceStatus(userIds: string[]): Promise<PresenceStatusMap> {
  if (!userIds.length) return Promise.resolve({});
  return apiFetch(`/api/presence/status?userIds=${userIds.join(',')}`);
}

/**
 * Singleton API client for the Forumline platform.
 * Call {@link ForumlineAPI.configure} first, then use any method.
 */
export const ForumlineAPI = {
  configure,
  getToken,
  getUserId,
  isAuthenticated,
  apiFetch,
  getConversations,
  getConversation,
  getMessages,
  sendMessage,
  markRead,
  getOrCreateDM,
  createGroupConversation,
  updateConversation,
  leaveConversation,
  searchProfiles,
  searchIdentity,
  getActivity,
  presenceHeartbeat,
  getPresenceStatus,
  getNotifications,
  getUnreadCount,
  markNotificationRead,
  markAllNotificationsRead,
};
