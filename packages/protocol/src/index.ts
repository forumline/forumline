// ============================================================================
// @forumline/protocol — Federation Protocol Types
//
// Zero-dependency TypeScript types that define the Forumline federation contract.
// This is the source of truth for how forums communicate with the Forumline app.
// ============================================================================

export type { ForumManifest, ForumCapability } from './manifest'
export type {
  ForumlineIdentity,
  ForumlineMembership,
  AuthResult,
  AuthSession,
  HubAuthorizeParams,
  HubTokenRequest,
  HubTokenResponse,
} from './identity'
export type {
  ForumNotification,
  ForumNotificationType,
  UnreadCounts,
  NotificationInput,
} from './notifications'
export type {
  ForumlineApiEndpoints,
  ForumlineAuthEndpoints,
} from './api'
export type {
  HubDirectMessage,
  HubDmConversation,
  HubProfile,
} from './hub-dms'
