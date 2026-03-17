/**
 * @module identity
 *
 * User profile CRUD and search.
 *
 * @example
 * ```ts
 * const profile = await Identity.getProfile();
 * await Identity.updateProfile({ display_name: 'New Name' });
 * ```
 */

import { ForumlineAPI } from './client.js';
import type { paths } from './api.gen.js';

/** The current user's full profile (as returned by GET /api/identity). */
export type UserProfile = paths['/api/identity']['get']['responses']['200']['content']['application/json'];

export interface ProfileUpdateData {
  /**
   * Updates the user's display name.
   * NOTE: The API request field is named `username` but updates the display_name column — known naming
   * inconsistency in the Go handler (sets["display_name"] = body.Username). A Go handler fix is pending.
   */
  display_name?: string;
  status_message?: string;
  online_status?: 'online' | 'away' | 'offline';
  show_online_status?: boolean;
}

/** Identity and profile API for the current authenticated user. */
export const Identity = {
  /** Fetch the current user's full profile. */
  getProfile(): Promise<UserProfile> {
    return ForumlineAPI.apiFetch('/api/identity');
  },

  /**
   * Update the current user's profile. Only provided fields are changed.
   * @param data - Fields to update.
   */
  async updateProfile(data: ProfileUpdateData): Promise<void> {
    // The handler field for display_name is confusingly called "username" — map here until Go is fixed.
    const body: Record<string, unknown> = {};
    if (data.display_name !== undefined) body['username'] = data.display_name;
    if (data.status_message !== undefined) body['status_message'] = data.status_message;
    if (data.online_status !== undefined) body['online_status'] = data.online_status;
    if (data.show_online_status !== undefined) body['show_online_status'] = data.show_online_status;
    await ForumlineAPI.apiFetch('/api/identity', { method: 'PUT', body: JSON.stringify(body) });
  },

  /**
   * Permanently delete the current user's account.
   * This is irreversible — use with care.
   */
  deleteAccount(): Promise<void> {
    return ForumlineAPI.apiFetch('/api/identity', { method: 'DELETE' });
  },

  /**
   * Search for users by username or display name.
   * @param q - Search query string.
   */
  searchProfiles: ForumlineAPI.searchProfiles,

  /** Send a presence heartbeat so the server marks this user as online. */
  heartbeat: ForumlineAPI.presenceHeartbeat,

  /**
   * Batch-fetch online/offline status for up to 200 user IDs.
   * @param userIds - Array of user IDs to check.
   * @returns Map of userId → online status.
   */
  batchPresenceStatus: ForumlineAPI.getPresenceStatus,
};
