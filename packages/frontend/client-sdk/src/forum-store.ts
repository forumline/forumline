/**
 * @module forum-store
 *
 * Reactive store for the user's forum memberships. Server is the source of truth;
 * localStorage serves as a fast cache for instant sidebar rendering on app load.
 *
 * @example
 * ```ts
 * forumStore.loadCache();
 * await forumStore.syncFromServer(accessToken);
 * $forums.subscribe((forums) => renderSidebar(forums));
 * ```
 */

import { map, atom } from 'nanostores';
import type { ForumCapability } from '@forumline/protocol';

/** A forum the user is a member of, with local UI state (unread counts, active state). */
export interface ForumMembership {
  /** Forum's canonical domain. */
  domain: string;
  /** Human-readable forum name. */
  name: string;
  /** Forum icon URL. */
  icon_url: string;
  /** Base URL for the forum's web UI (loaded in iframe). */
  web_base: string;
  /** Base URL for the forum's API endpoints. */
  api_base: string;
  /** Features this forum supports (threads, chat, voice, notifications). */
  capabilities: ForumCapability[];
  /** ISO timestamp of when the user joined. */
  added_at: string;
  /** Local unique ID for keying in UI lists. */
  id: string;
  /** Seed value for DiceBear avatar generation. */
  seed: string;
  /** Approximate member count. */
  members: number;
  /** Aggregated unread count (notifications + chat mentions + DMs). */
  unread: number;
  /** Thread count (reserved for future use). */
  threads: number;
  /** Always `true` for server-synced memberships. */
  isReal: boolean;
  /** Whether notifications from this forum are muted. */
  muted?: boolean;
}

/** Per-forum unread counts received from the forum's `/unread` endpoint. */
export interface ForumUnreadCounts {
  /** Unread notification count. */
  notifications?: number;
  /** Unread chat mention count. */
  chat_mentions?: number;
  /** Unread DM count. */
  dms?: number;
}

// ── Atoms ──────────────────────────────────────────────────────────────

/** Reactive list of forums the user has joined. */
export const $forums = atom<ForumMembership[]>([]);

/** The currently selected forum (shown in the main content area), or `null` for home. */
export const $activeForum = atom<ForumMembership | null>(null);

/** The current path within the active forum's iframe. */
export const $activePath = atom('');

/** Per-domain unread counts map. */
export const $unreadCounts = map<Record<string, ForumUnreadCounts>>({});

// ── Internal state (not reactive — no UI cares about the token) ────────

let _accessToken: string | null = null;

// ── Helpers ────────────────────────────────────────────────────────────

function _persistCache(): void {
  try {
    localStorage.setItem('forumline-memberships', JSON.stringify($forums.get()));
  } catch {}
}

function _toMembership(m: {
  forum_domain: string;
  forum_name: string;
  forum_icon_url?: string;
  web_base: string;
  api_base: string;
  capabilities?: ForumCapability[];
  joined_at: string;
  member_count?: number;
}): ForumMembership {
  return {
    domain: m.forum_domain,
    name: m.forum_name,
    icon_url: m.forum_icon_url || '',
    web_base: m.web_base,
    api_base: m.api_base,
    capabilities: m.capabilities || [],
    added_at: m.joined_at,
    id: 'real_' + m.forum_domain.replace(/[^a-z0-9]/g, '_'),
    seed: m.forum_domain,
    members: m.member_count || 0,
    unread: 0,
    threads: 0,
    isReal: true,
  };
}

// ── Actions ────────────────────────────────────────────────────────────

/**
 * Fetch the user's forum memberships from the server and update the store.
 * Optionally accepts an access token (useful on first call after login).
 */
async function syncFromServer(accessToken?: string): Promise<void> {
  if (accessToken) _accessToken = accessToken;
  if (!_accessToken) return;
  try {
    const res = await fetch('/api/memberships', {
      headers: { Authorization: `Bearer ${_accessToken}` },
    });
    if (!res.ok) {
      console.error('[ForumStore] membership sync failed:', res.status);
      return;
    }
    const memberships: Array<{
      forum_domain: string;
      forum_name: string;
      forum_icon_url?: string;
      web_base: string;
      api_base: string;
      capabilities?: ForumCapability[];
      joined_at: string;
      member_count?: number;
    }> = await res.json();
    $forums.set((memberships || []).map(_toMembership));
    _persistCache();
  } catch (e) {
    console.warn('Failed to sync memberships:', e);
  }
}

/** Load cached memberships from localStorage for instant UI rendering. */
function loadCache(): void {
  try {
    const c = localStorage.getItem('forumline-memberships');
    if (c) $forums.set(JSON.parse(c));
  } catch {}
}

/**
 * Fetch and validate a forum's manifest from its well-known URL.
 * @throws {Error} If the forum doesn't serve a valid manifest.
 */
async function fetchManifest(url: string): Promise<{ forumline_version: string; [key: string]: unknown }> {
  let n = url.trim();
  if (!/^https?:\/\//i.test(n)) n = 'https://' + n;
  const mu = n.includes('/.well-known/forumline-manifest.json')
    ? n
    : n.replace(/\/$/, '') + '/.well-known/forumline-manifest.json';
  const r = await fetch(mu);
  if (!r.ok) throw new Error('Forum returned HTTP ' + r.status + ': not a valid Forumline forum');
  const m = await r.json();
  if (m.forumline_version !== '1') throw new Error('Unsupported version: ' + m.forumline_version);
  return m;
}

/**
 * Join a forum by URL or domain. Calls the server-side join endpoint
 * which validates the forum's manifest internally (avoids CORS issues).
 * @throws {Error} If already joined, not authenticated, or the join fails.
 */
async function addForum(url: string): Promise<ForumMembership> {
  const domain = url
    .trim()
    .replace(/^https?:\/\//i, '')
    .replace(/\/.*$/, '');
  if (!domain) throw new Error('Please enter a forum URL or domain');
  if ($forums.get().some(f => f.domain === domain)) throw new Error('Already joined this forum');
  if (!_accessToken) throw new Error('You must be signed in to add a forum');

  const res = await fetch('/api/memberships/join', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + _accessToken },
    body: JSON.stringify({ forum_domain: domain }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || 'Failed to add forum (HTTP ' + res.status + ')');
  }
  const info: {
    domain: string;
    name: string;
    icon_url?: string;
    web_base: string;
    api_base: string;
    capabilities?: ForumCapability[];
    joined_at?: string;
    member_count?: number;
  } = await res.json();
  const mem: ForumMembership = {
    domain: info.domain,
    name: info.name,
    icon_url: info.icon_url || '',
    web_base: info.web_base,
    api_base: info.api_base,
    capabilities: info.capabilities || [],
    added_at: info.joined_at || new Date().toISOString(),
    id: 'real_' + info.domain.replace(/[^a-z0-9]/g, '_'),
    seed: info.domain,
    members: info.member_count || 1,
    unread: 0,
    threads: 0,
    isReal: true,
  };
  $forums.set([...$forums.get(), mem]);
  _persistCache();
  return mem;
}

/**
 * Join a forum by domain when you already have its info (e.g. from discovery results).
 * @throws {Error} If already joined or not authenticated.
 */
async function joinByDomain(
  domain: string,
  forumInfo?: Partial<ForumMembership>,
): Promise<ForumMembership> {
  if ($forums.get().some(f => f.domain === domain)) throw new Error('Already joined');
  if (!_accessToken) throw new Error('Not authenticated');
  const res = await fetch('/api/memberships/join', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + _accessToken },
    body: JSON.stringify({ forum_domain: domain }),
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error('Join failed: ' + (text || res.status));
  }
  const info = forumInfo || {};
  const mem: ForumMembership = {
    domain: domain,
    name: info.name || domain,
    icon_url: info.icon_url || '',
    web_base: info.web_base || 'https://' + domain,
    api_base: info.api_base || '',
    capabilities: info.capabilities || [],
    added_at: new Date().toISOString(),
    id: 'real_' + domain.replace(/[^a-z0-9]/g, '_'),
    seed: domain,
    members: info.members || 0,
    unread: 0,
    threads: 0,
    isReal: true,
  };
  $forums.set([...$forums.get(), mem]);
  _persistCache();
  return mem;
}

/**
 * Leave a forum. Removes the membership on the server and locally.
 * If this was the active forum, switches back to home.
 */
async function leaveForum(domain: string): Promise<void> {
  if (_accessToken) {
    try {
      await fetch('/api/memberships', {
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
          Authorization: 'Bearer ' + _accessToken,
        },
        body: JSON.stringify({ forum_domain: domain }),
      });
    } catch (e) {
      console.error('[ForumStore] leave request failed:', e);
    }
  }
  $forums.set($forums.get().filter(f => f.domain !== domain));
  const active = $activeForum.get();
  if (active && active.domain === domain) {
    $activeForum.set(null);
  }
  _persistCache();
}

/**
 * Toggle notification mute for a forum.
 */
async function toggleMute(domain: string): Promise<void> {
  if (!_accessToken) return;
  const forum = $forums.get().find(f => f.domain === domain);
  const newMuted = forum ? !forum.muted : true;
  try {
    await fetch('/api/memberships', {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        Authorization: 'Bearer ' + _accessToken,
      },
      body: JSON.stringify({ forum_domain: domain, muted: newMuted }),
    });
    if (forum) {
      $forums.set($forums.get().map(f =>
        f.domain === domain ? { ...f, muted: newMuted } : f
      ));
    }
  } catch (e) {
    console.error('[ForumStore] mute request failed:', e);
  }
}

/**
 * Switch the active forum (loads its iframe in the main content area).
 */
function switchForum(domain: string, path?: string): void {
  const forum = $forums.get().find(f => f.domain === domain);
  if (!forum) return;
  $activeForum.set(forum);
  $activePath.set(path || '');
}

/** Deactivate the current forum and show the home view. */
function goHome(): void {
  $activeForum.set(null);
  $activePath.set('');
}

/**
 * Update a forum's unread counts (received from the forum's iframe via postMessage).
 * Recalculates the aggregated `unread` badge count.
 */
function setUnreadCounts(domain: string, counts: ForumUnreadCounts): void {
  $unreadCounts.setKey(domain, counts);
  const total = (counts.notifications || 0) + (counts.chat_mentions || 0) + (counts.dms || 0);
  $forums.set($forums.get().map(f =>
    f.domain === domain ? { ...f, unread: total } : f
  ));
}

/** Clear all state (memberships, active forum, cache). Called on sign-out. */
function clear(): void {
  _accessToken = null;
  $forums.set([]);
  $activeForum.set(null);
  $activePath.set('');
  $unreadCounts.set({});
  try {
    localStorage.removeItem('forumline-memberships');
  } catch {}
}

// ── Backward-compatible namespace export ───────────────────────────────

/**
 * Reactive forum membership store. Manages the list of forums the user
 * has joined, syncs with the server, and caches in localStorage.
 *
 * For reactive subscriptions, use the atoms directly:
 * - `$forums` — forum list
 * - `$activeForum` — currently selected forum
 * - `$activePath` — path within active forum
 *
 * For actions, use `ForumStore.syncFromServer()`, `.addForum()`, etc.
 */
/** Get the current access token (for cross-origin iframe auth handshake). */
export function getAccessToken(): string | null {
  return _accessToken;
}

interface ForumStoreAPI {
  subscribe(fn: (store: ForumStoreAPI) => void): () => void;
  readonly forums: ForumMembership[];
  readonly activeForum: ForumMembership | null;
  readonly activePath: string;
  getAccessToken: typeof getAccessToken;
  syncFromServer: typeof syncFromServer;
  loadCache: typeof loadCache;
  fetchManifest: typeof fetchManifest;
  addForum: typeof addForum;
  joinByDomain: typeof joinByDomain;
  leaveForum: typeof leaveForum;
  toggleMute: typeof toggleMute;
  switchForum: typeof switchForum;
  goHome: typeof goHome;
  setUnreadCounts: typeof setUnreadCounts;
  clear: typeof clear;
}

export const ForumStore: ForumStoreAPI = {
  /** @deprecated Use `$forums.subscribe()` instead. */
  subscribe(fn: (store: ForumStoreAPI) => void): () => void {
    const unsubs = [
      $forums.subscribe(() => fn(ForumStore)),
      $activeForum.subscribe(() => fn(ForumStore)),
      $activePath.subscribe(() => fn(ForumStore)),
    ];
    return () => unsubs.forEach(u => u());
  },

  get forums(): ForumMembership[] { return $forums.get(); },
  get activeForum(): ForumMembership | null { return $activeForum.get(); },
  get activePath(): string { return $activePath.get(); },

  getAccessToken,
  syncFromServer,
  loadCache,
  fetchManifest,
  addForum,
  joinByDomain,
  leaveForum,
  toggleMute,
  switchForum,
  goHome,
  setUnreadCounts,
  clear,
};
