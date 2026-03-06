/**
 * ApiForumDataProvider — Go REST API implementation of ForumDataProvider.
 *
 * Replaces Supabase PostgREST calls with Go backend endpoints.
 */

import type { ForumDataProvider } from './data-provider'
import type {
  Category,
  ChatChannel,
  ChatMessageWithAuthor,
  Notification,
  PostWithAuthor,
  Profile,
  ThreadWithAuthor,
  VoiceRoom,
} from '../types'

export class ApiForumDataProvider implements ForumDataProvider {
  private getAccessToken: () => Promise<string | null>

  constructor(getAccessToken: () => Promise<string | null>) {
    this.getAccessToken = getAccessToken
  }

  private async headers(): Promise<HeadersInit> {
    const token = await this.getAccessToken()
    const h: Record<string, string> = { 'Content-Type': 'application/json' }
    if (token) h['Authorization'] = `Bearer ${token}`
    return h
  }

  private async get<T>(url: string, auth = false): Promise<T> {
    const h = auth ? await this.headers() : { 'Content-Type': 'application/json' }
    const res = await fetch(url, { headers: h })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new Error(body.error || `GET ${url} failed: ${res.status}`)
    }
    return res.json()
  }

  private async post<T>(url: string, body: unknown): Promise<T> {
    const res = await fetch(url, {
      method: 'POST',
      headers: await this.headers(),
      body: JSON.stringify(body),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.error || `POST ${url} failed: ${res.status}`)
    }
    return res.json()
  }

  private async put(url: string, body: unknown): Promise<void> {
    const res = await fetch(url, {
      method: 'PUT',
      headers: await this.headers(),
      body: JSON.stringify(body),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.error || `PUT ${url} failed: ${res.status}`)
    }
  }

  private async del(url: string): Promise<void> {
    const res = await fetch(url, {
      method: 'DELETE',
      headers: await this.headers(),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.error || `DELETE ${url} failed: ${res.status}`)
    }
  }

  // ========================================================================
  // Reads
  // ========================================================================

  async getCategories(): Promise<Category[]> {
    return this.get('/api/categories')
  }

  async getChannels(): Promise<ChatChannel[]> {
    return this.get('/api/channels')
  }

  async getVoiceRooms(): Promise<VoiceRoom[]> {
    return this.get('/api/voice-rooms')
  }

  async getThreads(limit = 20): Promise<ThreadWithAuthor[]> {
    return this.get(`/api/threads?limit=${limit}`)
  }

  async getThreadsByCategory(categorySlug: string): Promise<ThreadWithAuthor[]> {
    return this.get(`/api/categories/${encodeURIComponent(categorySlug)}/threads`)
  }

  async getThread(threadId: string): Promise<ThreadWithAuthor | null> {
    try {
      return await this.get(`/api/threads/${encodeURIComponent(threadId)}`)
    } catch {
      return null
    }
  }

  async getPosts(threadId: string): Promise<PostWithAuthor[]> {
    return this.get(`/api/threads/${encodeURIComponent(threadId)}/posts`)
  }

  async getCategory(slug: string): Promise<Category | null> {
    try {
      return await this.get(`/api/categories/${encodeURIComponent(slug)}`)
    } catch {
      return null
    }
  }

  async getProfile(userId: string): Promise<Profile | null> {
    try {
      return await this.get(`/api/profiles/${encodeURIComponent(userId)}`)
    } catch {
      return null
    }
  }

  async getProfileByUsername(username: string): Promise<Profile | null> {
    try {
      return await this.get(`/api/profiles/by-username/${encodeURIComponent(username)}`)
    } catch {
      return null
    }
  }

  async getChatMessages(channelSlug: string): Promise<ChatMessageWithAuthor[]> {
    return this.get(`/api/channels/${encodeURIComponent(channelSlug)}/messages`)
  }

  async getBookmarksWithMeta(_userId: string): Promise<Array<{
    id: string
    created_at: string
    thread: ThreadWithAuthor
  }>> {
    return this.get('/api/bookmarks', true)
  }

  async getBookmarks(_userId: string): Promise<ThreadWithAuthor[]> {
    const bookmarks = await this.getBookmarksWithMeta(_userId)
    return bookmarks.map(b => b.thread)
  }

  async isBookmarked(_userId: string, threadId: string): Promise<boolean> {
    const status = await this.get<{ bookmarked: boolean }>(`/api/bookmarks/${encodeURIComponent(threadId)}/status`, true)
    return status.bookmarked
  }

  async getUserThreads(userId: string): Promise<ThreadWithAuthor[]> {
    return this.get(`/api/users/${encodeURIComponent(userId)}/threads`)
  }

  async getUserPosts(userId: string): Promise<PostWithAuthor[]> {
    return this.get(`/api/users/${encodeURIComponent(userId)}/posts`)
  }

  async searchThreads(query: string): Promise<ThreadWithAuthor[]> {
    if (!query.trim()) return []
    return this.get(`/api/search/threads?q=${encodeURIComponent(query)}`)
  }

  async searchPosts(query: string): Promise<PostWithAuthor[]> {
    if (!query.trim()) return []
    return this.get(`/api/search/posts?q=${encodeURIComponent(query)}`)
  }

  async getAdminStats(): Promise<{ totalUsers: number; totalThreads: number; totalPosts: number }> {
    return this.get('/api/admin/stats', true)
  }

  async getAdminUsers(): Promise<Profile[]> {
    return this.get('/api/admin/users', true)
  }

  async getNotifications(_userId: string): Promise<Notification[]> {
    return this.get('/api/notifications', true)
  }

  // ========================================================================
  // Writes
  // ========================================================================

  async createThread(input: {
    category_id: string
    author_id: string
    title: string
    slug: string
    content?: string
    image_url?: string
  }): Promise<{ id: string } | null> {
    return this.post('/api/threads', input)
  }

  async updateThread(threadId: string, updates: {
    image_url?: string
    last_post_at?: string
    post_count?: number
    is_pinned?: boolean
    is_locked?: boolean
  }): Promise<void> {
    const res = await fetch(`/api/threads/${encodeURIComponent(threadId)}`, {
      method: 'PATCH',
      headers: await this.headers(),
      body: JSON.stringify(updates),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.error || 'updateThread failed')
    }
  }

  async createPost(input: {
    thread_id: string
    author_id: string
    content: string
    reply_to_id?: string
  }): Promise<{ id: string } | null> {
    return this.post('/api/posts', input)
  }

  async sendChatMessage(input: {
    channel_id: string
    author_id: string
    content: string
  }): Promise<void> {
    // The API endpoint uses channel slug, but we have channel_id here.
    // We need to look up the slug, or the API can accept channel_id.
    // For now, we pass channel_id and the frontend will need to send the slug.
    // Actually, looking at the existing sendChatMessage callers: Chat.tsx passes
    // channel_id (the UUID). But the Go endpoint expects slug in the URL.
    // We need to handle this. Let's accept that Chat.tsx passes channel_id
    // and we'll need the slug. Actually, the Go handler looks up by slug.
    // The caller in Chat.tsx has access to the channel object.
    // For the data provider interface, channel_id is passed.
    // We need a way to resolve channel_id -> slug. Let's cache channels.
    // Actually, simplest: add a POST /api/messages endpoint that accepts channel_id directly.
    // OR: change the interface. But the interface is fixed.
    // Let's use the channel cache approach - the channels are already fetched.
    // For now, let's just pass channel_id as slug and handle it on the Go side.
    // Actually the simplest fix: change the Go handler to also accept channel_id.
    // Let me just make a direct POST to a channel-id-based endpoint.
    await this.post(`/api/channels/_by-id/${encodeURIComponent(input.channel_id)}/messages`, {
      content: input.content,
    })
  }

  async addBookmark(_userId: string, threadId: string): Promise<void> {
    await this.post('/api/bookmarks', { thread_id: threadId })
  }

  async removeBookmark(_userId: string, threadId: string): Promise<void> {
    await this.del(`/api/bookmarks/${encodeURIComponent(threadId)}`)
  }

  async removeBookmarkById(bookmarkId: string): Promise<void> {
    await this.del(`/api/bookmarks/by-id/${encodeURIComponent(bookmarkId)}`)
  }

  async markNotificationRead(notificationId: string): Promise<void> {
    await this.post('/api/forumline/notifications/read', { id: notificationId })
  }

  async markAllNotificationsRead(_userId: string): Promise<void> {
    await this.post('/api/notifications/read-all', {})
  }

  async upsertProfile(userId: string, data: {
    username: string
    display_name?: string | null
    avatar_url?: string | null
  }): Promise<void> {
    await this.put(`/api/profiles/${encodeURIComponent(userId)}`, data)
  }

  async updateProfile(userId: string, updates: {
    display_name?: string | null
    bio?: string | null
    website?: string | null
    avatar_url?: string | null
  }): Promise<void> {
    await this.put(`/api/profiles/${encodeURIComponent(userId)}`, updates)
  }

  async setVoicePresence(_userId: string, roomSlug: string): Promise<void> {
    await this.put('/api/voice-presence', { room_slug: roomSlug })
  }

  async clearVoicePresence(_userId: string): Promise<void> {
    await this.del('/api/voice-presence')
  }
}
