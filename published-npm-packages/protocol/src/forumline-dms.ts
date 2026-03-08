// ============================================================================
// Forumline Direct Messages — Cross-forum DM types (1:1 and group)
// ============================================================================

/** A direct message stored on Forumline Central Services */
export interface ForumlineDirectMessage {
  id: string
  conversation_id: string
  sender_id: string
  content: string
  created_at: string
}

/** A member of a conversation */
export interface ForumlineConversationMember {
  id: string
  username: string
  displayName: string
  avatarUrl: string | null
}

/** A forumline DM conversation summary (1:1 or group) */
export interface ForumlineDmConversation {
  id: string
  isGroup: boolean
  name: string | null
  members: ForumlineConversationMember[]
  lastMessage: string
  lastMessageTime: string
  unreadCount: number
}

/** A forumline user profile (search result) */
export interface ForumlineProfile {
  id: string
  username: string
  display_name: string
  avatar_url: string | null
}
