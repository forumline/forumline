/**
 * Smart Data Cache with TTL, Stale-While-Revalidate, and Request Prioritization
 *
 * Features:
 * - In-memory caching with configurable TTL
 * - Stale-while-revalidate for instant UI with background refresh
 * - Request prioritization (active > preload)
 * - Request deduplication (multiple callers share one in-flight request)
 * - Cache invalidation via keys or patterns
 */

type Priority = 'high' | 'low'

interface CacheEntry<T> {
  data: T
  timestamp: number
  ttl: number // ms
}

interface PendingRequest<T> {
  promise: Promise<T>
  priority: Priority
  abortController?: AbortController
}

// Cache configuration by data type
export const CACHE_CONFIG = {
  // Static/rarely-changing data - long TTL
  categories: { ttl: 60 * 60 * 1000, staleTime: 5 * 60 * 1000 }, // 1hr cache, 5min stale
  channels: { ttl: 60 * 60 * 1000, staleTime: 5 * 60 * 1000 },
  voiceRooms: { ttl: 60 * 60 * 1000, staleTime: 5 * 60 * 1000 },

  // User data - medium TTL
  profiles: { ttl: 15 * 60 * 1000, staleTime: 2 * 60 * 1000 }, // 15min cache, 2min stale

  // Dynamic lists - short TTL
  threads: { ttl: 2 * 60 * 1000, staleTime: 30 * 1000 }, // 2min cache, 30s stale
  threadsByCategory: { ttl: 2 * 60 * 1000, staleTime: 30 * 1000 },
  posts: { ttl: 1 * 60 * 1000, staleTime: 15 * 1000 }, // 1min cache, 15s stale

  // Real-time data - very short TTL (rely on subscriptions mostly)
  chatMessages: { ttl: 30 * 1000, staleTime: 10 * 1000 },
  dmMessages: { ttl: 30 * 1000, staleTime: 10 * 1000 },
} as const

type CacheType = keyof typeof CACHE_CONFIG

class DataCache {
  private cache = new Map<string, CacheEntry<unknown>>()
  private pending = new Map<string, PendingRequest<unknown>>()
  private subscribers = new Map<string, Set<() => void>>()

  /**
   * Get cached data if fresh, or trigger fetch
   */
  get<T>(key: string): T | undefined {
    const entry = this.cache.get(key) as CacheEntry<T> | undefined
    if (!entry) return undefined

    const age = Date.now() - entry.timestamp
    if (age > entry.ttl) {
      // Expired, remove from cache
      this.cache.delete(key)
      return undefined
    }

    return entry.data
  }

  /**
   * Check if data is stale (past staleTime but not expired)
   */
  isStale(key: string, type: CacheType): boolean {
    const entry = this.cache.get(key)
    if (!entry) return true

    const config = CACHE_CONFIG[type]
    const age = Date.now() - entry.timestamp
    return age > config.staleTime
  }

  /**
   * Store data in cache
   */
  set<T>(key: string, data: T, type: CacheType): void {
    const config = CACHE_CONFIG[type]
    this.cache.set(key, {
      data,
      timestamp: Date.now(),
      ttl: config.ttl,
    })

    // Notify subscribers
    this.notifySubscribers(key)
  }

  /**
   * Fetch with deduplication and priority handling
   * - If already fetching with same/higher priority, return existing promise
   * - If fetching with lower priority, cancel and start new high-priority fetch
   */
  async fetch<T>(
    key: string,
    type: CacheType,
    fetcher: (signal?: AbortSignal) => Promise<T>,
    options: { priority?: Priority; forceRefresh?: boolean } = {}
  ): Promise<T> {
    const { priority = 'high', forceRefresh = false } = options

    // Check cache first (unless forcing refresh)
    if (!forceRefresh) {
      const cached = this.get<T>(key)
      if (cached !== undefined && !this.isStale(key, type)) {
        return cached
      }
    }

    // Check for pending request
    const pending = this.pending.get(key) as PendingRequest<T> | undefined
    if (pending) {
      // If current is high priority and we're low, reuse existing
      if (pending.priority === 'high' && priority === 'low') {
        return pending.promise
      }
      // If same priority, reuse existing
      if (pending.priority === priority) {
        return pending.promise
      }
      // If we're high and current is low, cancel and replace
      if (priority === 'high' && pending.priority === 'low') {
        pending.abortController?.abort()
      }
    }

    // Create new fetch
    const abortController = new AbortController()
    const promise = fetcher(abortController.signal)
      .then((data) => {
        this.set(key, data, type)
        this.pending.delete(key)
        return data
      })
      .catch((err) => {
        this.pending.delete(key)
        // Don't throw for aborted requests
        if (err?.name === 'AbortError') {
          // Return stale data if available
          const stale = this.get<T>(key)
          if (stale !== undefined) return stale
        }
        throw err
      })

    this.pending.set(key, { promise, priority, abortController })
    return promise
  }

  /**
   * Stale-while-revalidate pattern
   * Returns cached data immediately (even if stale) and refreshes in background
   */
  async fetchWithSWR<T>(
    key: string,
    type: CacheType,
    fetcher: (signal?: AbortSignal) => Promise<T>,
    options: { priority?: Priority } = {}
  ): Promise<{ data: T | undefined; isStale: boolean; refresh: Promise<T> | null }> {
    const cached = this.get<T>(key)
    const stale = this.isStale(key, type)

    let refresh: Promise<T> | null = null
    if (stale || cached === undefined) {
      // Fetch in background (low priority if we have stale data)
      refresh = this.fetch(key, type, fetcher, {
        priority: cached !== undefined ? 'low' : options.priority || 'high',
        forceRefresh: true,
      })
    }

    return {
      data: cached,
      isStale: stale,
      refresh,
    }
  }

  /**
   * Preload data with low priority (won't interrupt active fetches)
   */
  preload<T>(
    key: string,
    type: CacheType,
    fetcher: (signal?: AbortSignal) => Promise<T>
  ): void {
    // Only preload if not already cached or pending
    if (this.get(key) !== undefined || this.pending.has(key)) {
      return
    }

    this.fetch(key, type, fetcher, { priority: 'low' }).catch(() => {
      // Silently ignore preload failures
    })
  }

  /**
   * Invalidate cache entries by key or pattern
   */
  invalidate(keyOrPattern: string | RegExp): void {
    if (typeof keyOrPattern === 'string') {
      this.cache.delete(keyOrPattern)
      this.notifySubscribers(keyOrPattern)
    } else {
      for (const key of this.cache.keys()) {
        if (keyOrPattern.test(key)) {
          this.cache.delete(key)
          this.notifySubscribers(key)
        }
      }
    }
  }

  /**
   * Subscribe to cache changes for a key
   */
  subscribe(key: string, callback: () => void): () => void {
    if (!this.subscribers.has(key)) {
      this.subscribers.set(key, new Set())
    }
    this.subscribers.get(key)!.add(callback)

    return () => {
      this.subscribers.get(key)?.delete(callback)
    }
  }

  private notifySubscribers(key: string): void {
    this.subscribers.get(key)?.forEach((cb) => cb())
  }

  /**
   * Clear all cache (useful for logout)
   */
  clear(): void {
    this.cache.clear()
    // Cancel all pending requests
    for (const pending of this.pending.values()) {
      pending.abortController?.abort()
    }
    this.pending.clear()
  }

  /**
   * Get cache stats for debugging
   */
  getStats(): { entries: number; pending: number; keys: string[] } {
    return {
      entries: this.cache.size,
      pending: this.pending.size,
      keys: Array.from(this.cache.keys()),
    }
  }
}

// Singleton instance
export const dataCache = new DataCache()

// Cache key generators for consistency
export const cacheKeys = {
  categories: () => 'categories',
  channels: () => 'channels',
  voiceRooms: () => 'voiceRooms',
  threads: (limit?: number) => `threads:${limit || 'all'}`,
  threadsByCategory: (categoryId: string) => `threads:category:${categoryId}`,
  thread: (threadId: string) => `thread:${threadId}`,
  posts: (threadId: string) => `posts:${threadId}`,
  profile: (userId: string) => `profile:${userId}`,
  profileByUsername: (username: string) => `profile:username:${username}`,
  chatMessages: (channelId: string) => `chat:${channelId}`,
  dmConversations: (userId: string) => `dm:conversations:${userId}`,
  dmMessages: (recipientId: string) => `dm:messages:${recipientId}`,
}
