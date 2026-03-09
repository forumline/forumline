/*
 * Reactive store primitive
 *
 * This file provides a minimal publish-subscribe state container used as the foundation for all app stores.
 *
 * It must:
 * - Hold a single state value that can be read synchronously via get()
 * - Accept new state via set(), supporting both direct values and updater functions
 * - Notify all subscribers synchronously whenever state changes
 * - Return an unsubscribe function from subscribe() for cleanup
 */
export type Subscriber<T> = (state: T) => void

export interface Store<T> {
  get: () => T
  set: (value: T | ((prev: T) => T)) => void
  subscribe: (fn: Subscriber<T>) => () => void
}

export function createStore<T>(initial: T): Store<T> {
  let state = initial
  const subs = new Set<Subscriber<T>>()

  return {
    get: () => state,
    set: (value) => {
      state = typeof value === 'function' ? (value as (prev: T) => T)(state) : value
      subs.forEach((fn) => fn(state))
    },
    subscribe: (fn) => {
      subs.add(fn)
      return () => { subs.delete(fn) }
    },
  }
}
