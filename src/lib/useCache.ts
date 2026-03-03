/**
 * React hooks for the data cache
 *
 * Provides:
 * - useCachedData: Fetch with automatic caching and SWR
 * - usePreload: Preload data on hover/focus for instant navigation
 * - useCacheInvalidation: Invalidate cache on real-time events
 */

import { useEffect, useState, useCallback, useRef } from 'react'
import { dataCache, cacheKeys, CACHE_CONFIG } from './cache'

type CacheType = keyof typeof CACHE_CONFIG

interface UseCachedDataOptions {
  /** Skip fetching (useful for conditional fetches) */
  skip?: boolean
  /** Force refresh on mount */
  forceRefresh?: boolean
  /** Dependencies that trigger refetch when changed */
  deps?: unknown[]
}

interface UseCachedDataResult<T> {
  data: T | undefined
  loading: boolean
  error: Error | null
  isStale: boolean
  refetch: () => Promise<void>
}

/**
 * Hook for fetching data with automatic caching
 *
 * Features:
 * - Returns cached data instantly if available
 * - Refreshes stale data in background (SWR pattern)
 * - Deduplicates concurrent requests
 * - Handles loading and error states
 */
export function useCachedData<T>(
  key: string,
  type: CacheType,
  fetcher: () => Promise<T>,
  options: UseCachedDataOptions = {}
): UseCachedDataResult<T> {
  const { skip = false, forceRefresh = false, deps = [] } = options

  // Get initial cached value
  const [data, setData] = useState<T | undefined>(() => dataCache.get<T>(key))
  const [loading, setLoading] = useState(!data && !skip)
  const [error, setError] = useState<Error | null>(null)
  const [isStale, setIsStale] = useState(() => dataCache.isStale(key, type))

  const fetcherRef = useRef(fetcher)
  fetcherRef.current = fetcher

  const doFetch = useCallback(async (_force = false) => {
    if (skip) return

    setError(null)

    try {
      const result = await dataCache.fetchWithSWR(key, type, fetcherRef.current, {
        priority: 'high',
      })

      // Set cached data immediately
      if (result.data !== undefined) {
        setData(result.data)
        setIsStale(result.isStale)
        setLoading(false)
      }

      // Wait for refresh if we had stale/no data
      if (result.refresh) {
        if (result.data === undefined) {
          setLoading(true)
        }
        const fresh = await result.refresh
        setData(fresh)
        setIsStale(false)
        setLoading(false)
      }
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)))
      setLoading(false)
    }
  }, [key, type, skip])

  // Fetch on mount and when deps change
  useEffect(() => {
    doFetch(forceRefresh)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, skip, forceRefresh, ...deps])

  // Subscribe to cache changes (for real-time updates)
  useEffect(() => {
    return dataCache.subscribe(key, () => {
      const cached = dataCache.get<T>(key)
      if (cached !== undefined) {
        setData(cached)
        setIsStale(dataCache.isStale(key, type))
      }
    })
  }, [key, type])

  const refetch = useCallback(async () => {
    await dataCache.fetch(key, type, fetcherRef.current, {
      priority: 'high',
      forceRefresh: true,
    })
    setData(dataCache.get<T>(key))
    setIsStale(false)
  }, [key, type])

  return { data, loading, error, isStale, refetch }
}

/**
 * Hook for preloading data on user intent (hover, focus)
 *
 * Usage:
 * ```tsx
 * const preloadThread = usePreload()
 *
 * <Link
 *   to={`/t/${thread.id}`}
 *   onMouseEnter={() => preloadThread(
 *     cacheKeys.thread(thread.id),
 *     'threads',
 *     () => fetchThread(thread.id)
 *   )}
 * >
 * ```
 */
export function usePreload() {
  return useCallback(<T>(
    key: string,
    type: CacheType,
    fetcher: () => Promise<T>
  ) => {
    dataCache.preload(key, type, fetcher)
  }, [])
}

/**
 * Hook for invalidating cache on real-time events
 *
 * Usage:
 * ```tsx
 * const invalidate = useCacheInvalidation()
 *
 * useEffect(() => {
 *   const sub = supabase.channel('posts')
 *     .on('postgres_changes', { event: 'INSERT', ... }, () => {
 *       invalidate(cacheKeys.posts(threadId))
 *     })
 *     .subscribe()
 * }, [])
 * ```
 */
export function useCacheInvalidation() {
  return useCallback((keyOrPattern: string | RegExp) => {
    dataCache.invalidate(keyOrPattern)
  }, [])
}

/**
 * Clear all cache (call on logout)
 */
export function clearCache() {
  dataCache.clear()
}

// Re-export for convenience
export { cacheKeys, CACHE_CONFIG }
