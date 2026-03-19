// @forumline/client-sdk — Forumline API client, auth, real-time, and state management.
// Zero DOM dependencies. Pure data/network layer.

export type { AuthCallback, AuthEvent, Session, SessionUser } from './auth.js';
export { ForumlineAuth } from './auth.js';
export type { CallState, CallStateListener, CallStateValue } from './calls.js';
export { CallManager } from './calls.js';
export type {
  ApiFetchOptions,
  ConfigureOptions,
  ConversationUpdates,
  GetMessagesOpts,
} from './client.js';
export { ForumlineAPI } from './client.js';
export { DmStore, $conversations, $dmUnreadCount, $dmInitialLoad, $dmLoadError, fetchConversations } from './dm-store.js';
export type {
  CallListener,
  CallSignal,
  DmEvent,
  DmListener,
  NotificationEvent,
  NotificationListener,
  StatusListener,
  StreamStatus,
  Unsubscribe,
} from './event-stream.js';
export { EventStream } from './event-stream.js';
export type {
  ForumRegistrationData,
  ForumSearchOptions,
  ForumSearchResult,
} from './forum-discovery.js';
export { ForumDiscoveryAPI, ForumRegistrationAPI } from './forum-discovery.js';
export type { ForumMembership, ForumUnreadCounts } from './forum-store.js';
export { ForumStore, $forums, $activeForum, $activePath, $unreadCounts } from './forum-store.js';
export type { ProfileUpdateData, UserProfile } from './identity.js';
export { Identity } from './identity.js';
export type { CallInfo, NativeBridgeHandlers } from './native-bridge.js';
export { NativeBridge } from './native-bridge.js';
export { PresenceTracker, $onlineUsers, setTrackedUsers, isOnline, pause as pausePresence, resume as resumePresence } from './presence.js';
export type { NotificationClickData } from './push.js';
export { PushNotifications } from './push.js';
