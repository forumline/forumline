/**
 * @module identity
 *
 * User profile CRUD, identity search, and batch presence status.
 *
 * @example
 * ```ts
 * const profile = await Identity.getProfile();
 * await Identity.updateProfile({ display_name: 'New Name' });
 * ```
 */

import type { ForumlineProfile } from '@forumline/protocol';
import { ForumlineAPI } from './client.js';

/** The current user's full profile. */
export interface UserProfile {
  /** User ID (UUID). */
  id: string;
  /** Unique username. */
  username: string;
  /** Human-readable display name. */
  display_name: string;
  /** Avatar image URL, or `null` if using the default DiceBear avatar. */
  avatar_url: string | null;
  /** Optional bio/about text. */
  bio?: string;
}

export interface ProfileUpdateData {
  username?: string;
  display_name?: string;
  avatar_url?: string;
  bio?: string;
}

interface PresenceStatusMap {
  [userId: string]: boolean | { online: boolean };
}

/** Identity and profile API for the current authenticated user. */
export const Identity = {
  /** Fetch the current user's profile. */
  getProfile(): Promise<UserProfile> {
    return ForumlineAPI.apiFetch('/api/identity');
  },

  /**
   * Update the current user's profile. Only provided fields are changed.
   * @param data - Fields to update (username, display_name, avatar_url, bio).
   */
  updateProfile(data: ProfileUpdateData): Promise<UserProfile> {
    return ForumlineAPI.apiFetch('/api/identity', { method: 'PUT', body: JSON.stringify(data) });
  },

  /**
   * Permanently delete the current user's account.
   * This is irreversible — use with care.
   */
  deleteAccount(): Promise<void> {
    return ForumlineAPI.apiFetch('/api/identity', { method: 'DELETE' });
  },

  /**
   * Search for users across the Forumline network by username or display name.
   * @param q - Search query string.
   */
  searchProfiles(q: string): Promise<ForumlineProfile[]> {
    return ForumlineAPI.apiFetch('/api/identity/search?q=' + encodeURIComponent(q));
  },

  /** Send a presence heartbeat so the server marks this user as online. */
  heartbeat(): Promise<void> {
    return ForumlineAPI.apiFetch('/api/identity/heartbeat', { method: 'POST', silent: true });
  },

  /**
   * Batch-fetch online/offline status for up to 200 user IDs.
   * @param userIds - Array of user IDs to check. Capped at 200.
   * @returns Map of userId to online status.
   */
  batchPresenceStatus(userIds: string[]): Promise<PresenceStatusMap> {
    if (!userIds || !userIds.length) return Promise.resolve({});
    return ForumlineAPI.apiFetch('/api/identity/status', {
      method: 'POST',
      body: JSON.stringify({ user_ids: userIds.slice(0, 200) }),
      silent: true,
    });
  },
};
