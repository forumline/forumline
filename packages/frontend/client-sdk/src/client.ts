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

import createClient, { type Middleware } from 'openapi-fetch';
import type { paths } from './api.gen.js';

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

// Rebuild the client whenever config changes
let _client = createClient<paths>({ baseUrl: _baseUrl });

const authMiddleware: Middleware = {
  async onRequest({ request }) {
    if (_accessToken) {
      request.headers.set('Authorization', `Bearer ${_accessToken}`);
    }
    return request;
  },
};

_client.use(authMiddleware);

function rebuildClient(): void {
  _client = createClient<paths>({ baseUrl: _baseUrl });
  _client.use(authMiddleware);
}

/**
 * Set the API base URL, access token, and/or user ID.
 * Typically called once after authentication succeeds.
 */
function configure({ baseUrl, accessToken, userId }: ConfigureOptions): void {
  if (baseUrl !== undefined) {
    _baseUrl = baseUrl;
    rebuildClient();
  }
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
 * Low-level authenticated fetch for paths not yet in the OpenAPI spec.
 * @deprecated Prefer typed methods; this is for migration transitional use only.
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
      throw new Error((err as { error?: string }).error || `API error: ${res.status}`);
    }
    const text = await res.text();
    return (text ? JSON.parse(text) : null) as T;
  } catch (err) {
    if (!silent) console.error('[ForumlineAPI]', (err as Error).message);
    throw err;
  }
}

async function unwrap<T>(promise: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T> {
  const { data, error, response } = await promise;
  if (error || !data) {
    const msg = (error as { error?: string } | undefined)?.error ?? `API error: ${response.status}`;
    throw new Error(msg);
  }
  return data;
}

// --- Conversations ---

export type Conversation = paths['/api/conversations']['get']['responses']['200']['content']['application/json'][number];
export type DirectMessage = paths['/api/conversations/{conversationId}/messages']['get']['responses']['200']['content']['application/json'][number];

/** Fetch all DM conversations for the current user. */
function getConversations(): Promise<Conversation[]> {
  return unwrap(_client.GET('/api/conversations'));
}

/** Fetch a single conversation by ID, including member list. */
function getConversation(id: string): Promise<Conversation> {
  return unwrap(_client.GET('/api/conversations/{conversationId}', { params: { path: { conversationId: id } } }));
}

export interface GetMessagesOpts {
  /** Cursor — return messages created before this message ID. */
  before?: string;
  /** Maximum number of messages to return. */
  limit?: number;
}

/**
 * Fetch messages in a conversation with optional cursor-based pagination.
 * @param id - Conversation ID.
 * @param opts - Pagination options (`before` cursor, `limit`).
 */
function getMessages(id: string, opts: GetMessagesOpts = {}): Promise<DirectMessage[]> {
  return unwrap(
    _client.GET('/api/conversations/{conversationId}/messages', {
      params: { path: { conversationId: id }, query: { before: opts.before, limit: opts.limit } },
    }),
  );
}

/**
 * Send a text message in a conversation.
 * @param id - Conversation ID.
 * @param content - Message body text.
 */
function sendMessage(id: string, content: string): Promise<DirectMessage> {
  return unwrap(
    _client.POST('/api/conversations/{conversationId}/messages', {
      params: { path: { conversationId: id } },
      body: { content },
    }),
  );
}

/** Mark all messages in a conversation as read. */
function markRead(id: string): Promise<void> {
  return unwrap(
    _client.POST('/api/conversations/{conversationId}/read', { params: { path: { conversationId: id } } }),
  ).then(() => undefined);
}

/**
 * Get or create a 1:1 DM conversation with another user.
 * Returns the conversation ID.
 * @param userId - The other user's ID.
 */
function getOrCreateDM(userId: string): Promise<string> {
  return unwrap(_client.POST('/api/conversations/dm', { body: { userId } })).then((r) => r.id);
}

export interface ConversationUpdates {
  name?: string | null;
  addMembers?: string[];
  removeMembers?: string[];
}

/**
 * Create a new group conversation.
 * @param memberIds - Array of user IDs to include (besides the current user).
 * @param name - Optional display name for the group.
 */
function createGroupConversation(memberIds: string[], name?: string): Promise<Conversation> {
  return unwrap(_client.POST('/api/conversations', { body: { memberIds, name } }));
}

/**
 * Update a conversation's metadata (e.g. rename a group, add/remove members).
 * @param id - Conversation ID.
 * @param updates - Fields to update.
 */
function updateConversation(id: string, updates: ConversationUpdates): Promise<void> {
  return unwrap(
    _client.PATCH('/api/conversations/{conversationId}', {
      params: { path: { conversationId: id } },
      body: updates,
    }),
  ).then(() => undefined);
}

/** Leave a group conversation. */
function leaveConversation(id: string): Promise<void> {
  return unwrap(
    _client.DELETE('/api/conversations/{conversationId}/members/me', {
      params: { path: { conversationId: id } },
    }),
  ).then(() => undefined);
}

// --- Identity / Profiles ---

export type ProfileSearchResult = paths['/api/profiles/search']['get']['responses']['200']['content']['application/json'][number];

/** Search user profiles by username or display name. */
function searchProfiles(query: string): Promise<ProfileSearchResult[]> {
  return unwrap(_client.GET('/api/profiles/search', { params: { query: { q: query } } }));
}

// --- Activity ---

export type ActivityItem = paths['/api/activity']['get']['responses']['200']['content']['application/json'][number];

/** Fetch the current user's recent activity feed. */
function getActivity(): Promise<ActivityItem[]> {
  return unwrap(_client.GET('/api/activity'));
}

// --- Notifications ---

export type Notification = paths['/api/notifications']['get']['responses']['200']['content']['application/json'][number];

/** Fetch all notifications for the current user. */
function getNotifications(): Promise<Notification[]> {
  return unwrap(_client.GET('/api/notifications'));
}

/** Fetch the unread notification count. */
function getUnreadCount(): Promise<number> {
  return unwrap(_client.GET('/api/notifications/unread')).then((r) => r.count);
}

/** Mark a single notification as read by ID. */
function markNotificationRead(id: string): Promise<void> {
  return unwrap(_client.POST('/api/notifications/read', { body: { id } })).then(() => undefined);
}

/** Mark all notifications as read. */
function markAllNotificationsRead(): Promise<void> {
  return unwrap(_client.POST('/api/notifications/read-all')).then(() => undefined);
}

// --- Presence ---

/** Send a presence heartbeat so the server knows we're online. */
function presenceHeartbeat(): Promise<void> {
  return unwrap(_client.POST('/api/presence/heartbeat')).then(() => undefined);
}

/** Batch-fetch online/offline status for a list of user IDs. */
function getPresenceStatus(userIds: string[]): Promise<Record<string, boolean>> {
  if (!userIds.length) return Promise.resolve({});
  return unwrap(
    _client.GET('/api/presence/status', { params: { query: { userIds: userIds.join(',') } } }),
  ) as Promise<Record<string, boolean>>;
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
  /** @deprecated Use typed methods instead */
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
  getActivity,
  presenceHeartbeat,
  getPresenceStatus,
  getNotifications,
  getUnreadCount,
  markNotificationRead,
  markAllNotificationsRead,
};
