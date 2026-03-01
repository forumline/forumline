import { useState, useRef, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useAuth } from '../lib/auth'

interface DirectMessage {
  id: string
  conversationId: string
  senderId: string
  content: string
  timestamp: Date
}

interface Conversation {
  id: string
  recipientId: string
  recipientName: string
  recipientAvatar: string
  lastMessage: string
  lastMessageTime: Date
  unreadCount: number
}

// Demo conversations
const demoConversations: Conversation[] = [
  {
    id: 'sarah',
    recipientId: '2',
    recipientName: 'Sarah',
    recipientAvatar: 'S',
    lastMessage: "That sounds great! Let me know when you're ready.",
    lastMessageTime: new Date(Date.now() - 300000),
    unreadCount: 2,
  },
  {
    id: 'mike',
    recipientId: '3',
    recipientName: 'Mike',
    recipientAvatar: 'M',
    lastMessage: 'Thanks for the help with the voice rooms!',
    lastMessageTime: new Date(Date.now() - 3600000),
    unreadCount: 0,
  },
  {
    id: 'alex',
    recipientId: '4',
    recipientName: 'Alex',
    recipientAvatar: 'A',
    lastMessage: 'Did you see the new update?',
    lastMessageTime: new Date(Date.now() - 86400000),
    unreadCount: 0,
  },
  {
    id: 'emma',
    recipientId: '6',
    recipientName: 'Emma',
    recipientAvatar: 'E',
    lastMessage: "I'll send over the API docs tomorrow.",
    lastMessageTime: new Date(Date.now() - 172800000),
    unreadCount: 0,
  },
]

// Demo messages for each conversation
const demoMessages: Record<string, DirectMessage[]> = {
  sarah: [
    { id: '1', conversationId: 'sarah', senderId: '2', content: 'Hey! How are you doing?', timestamp: new Date(Date.now() - 600000) },
    { id: '2', conversationId: 'sarah', senderId: 'me', content: "I'm good! Working on the forum project.", timestamp: new Date(Date.now() - 540000) },
    { id: '3', conversationId: 'sarah', senderId: '2', content: "That's awesome! The voice rooms look really cool.", timestamp: new Date(Date.now() - 480000) },
    { id: '4', conversationId: 'sarah', senderId: 'me', content: 'Thanks! Want to test them out later?', timestamp: new Date(Date.now() - 420000) },
    { id: '5', conversationId: 'sarah', senderId: '2', content: "That sounds great! Let me know when you're ready.", timestamp: new Date(Date.now() - 300000) },
  ],
  mike: [
    { id: '1', conversationId: 'mike', senderId: 'me', content: 'Hey Mike, the moderation tools are ready!', timestamp: new Date(Date.now() - 7200000) },
    { id: '2', conversationId: 'mike', senderId: '3', content: 'Perfect timing! I was just about to ask.', timestamp: new Date(Date.now() - 7000000) },
    { id: '3', conversationId: 'mike', senderId: 'me', content: 'You can mute, kick, and ban users now.', timestamp: new Date(Date.now() - 6800000) },
    { id: '4', conversationId: 'mike', senderId: '3', content: 'Thanks for the help with the voice rooms!', timestamp: new Date(Date.now() - 3600000) },
  ],
  alex: [
    { id: '1', conversationId: 'alex', senderId: '4', content: 'The dark mode looks great!', timestamp: new Date(Date.now() - 172800000) },
    { id: '2', conversationId: 'alex', senderId: 'me', content: "Thanks! It's the default now.", timestamp: new Date(Date.now() - 172000000) },
    { id: '3', conversationId: 'alex', senderId: '4', content: 'Did you see the new update?', timestamp: new Date(Date.now() - 86400000) },
  ],
  emma: [
    { id: '1', conversationId: 'emma', senderId: 'me', content: 'Hey Emma! Any updates on the API?', timestamp: new Date(Date.now() - 259200000) },
    { id: '2', conversationId: 'emma', senderId: '6', content: "I'll send over the API docs tomorrow.", timestamp: new Date(Date.now() - 172800000) },
  ],
}

export default function DirectMessages() {
  const { recipientId } = useParams()
  useAuth() // Keep for future auth checks
  const [conversations, setConversations] = useState<Conversation[]>(demoConversations)
  const [messages, setMessages] = useState<DirectMessage[]>([])
  const [newMessage, setNewMessage] = useState('')
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const currentConversation = recipientId
    ? conversations.find(c => c.id === recipientId)
    : null

  useEffect(() => {
    if (recipientId && demoMessages[recipientId]) {
      setMessages(demoMessages[recipientId])
      // Mark as read
      setConversations(prev =>
        prev.map(c =>
          c.id === recipientId ? { ...c, unreadCount: 0 } : c
        )
      )
    } else {
      setMessages([])
    }
  }, [recipientId])

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const handleSend = (e: React.FormEvent) => {
    e.preventDefault()
    if (!newMessage.trim() || !recipientId) return

    const message: DirectMessage = {
      id: Date.now().toString(),
      conversationId: recipientId,
      senderId: 'me',
      content: newMessage.trim(),
      timestamp: new Date(),
    }

    setMessages(prev => [...prev, message])

    // Update last message in conversation
    setConversations(prev =>
      prev.map(c =>
        c.id === recipientId
          ? { ...c, lastMessage: newMessage.trim(), lastMessageTime: new Date() }
          : c
      )
    )

    setNewMessage('')
  }

  const formatTime = (date: Date) => {
    const now = new Date()
    const diff = now.getTime() - date.getTime()
    const minutes = Math.floor(diff / 60000)
    const hours = Math.floor(minutes / 60)
    const days = Math.floor(hours / 24)

    if (minutes < 1) return 'now'
    if (minutes < 60) return `${minutes}m`
    if (hours < 24) return `${hours}h`
    if (days < 7) return `${days}d`
    return date.toLocaleDateString()
  }

  const formatMessageTime = (date: Date) => {
    return date.toLocaleTimeString('en-US', {
      hour: 'numeric',
      minute: '2-digit',
    })
  }

  // Mobile: show conversation list or messages
  // Desktop: show both
  const showConversationList = !recipientId
  const showMessages = !!recipientId

  return (
    <div className="mx-auto max-w-6xl">
      <div className="flex h-[calc(100vh-7rem)] overflow-hidden rounded-xl border border-slate-700 bg-slate-800/50">
        {/* Conversation List */}
        <div className={`${showConversationList ? 'flex' : 'hidden'} w-full flex-col border-r border-slate-700 md:flex md:w-80`}>
          <div className="flex items-center justify-between border-b border-slate-700 px-4 py-3">
            <h2 className="font-semibold text-white">Messages</h2>
            <button className="rounded-lg p-1.5 text-slate-400 hover:bg-slate-700 hover:text-white">
              <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
            </button>
          </div>

          <div className="flex-1 overflow-y-auto">
            {conversations.length === 0 ? (
              <div className="p-4 text-center text-slate-400">
                No conversations yet
              </div>
            ) : (
              conversations.map(conversation => (
                <Link
                  key={conversation.id}
                  to={`/dm/${conversation.id}`}
                  className={`flex items-center gap-3 px-4 py-3 transition-colors hover:bg-slate-700/50 ${
                    recipientId === conversation.id ? 'bg-slate-700/50' : ''
                  }`}
                >
                  <div className="relative">
                    <div className="flex h-10 w-10 items-center justify-center rounded-full bg-indigo-500 text-sm font-medium text-white">
                      {conversation.recipientAvatar}
                    </div>
                    {conversation.unreadCount > 0 && (
                      <div className="absolute -right-1 -top-1 flex h-5 w-5 items-center justify-center rounded-full bg-indigo-500 text-xs font-medium text-white">
                        {conversation.unreadCount}
                      </div>
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center justify-between">
                      <span className={`font-medium ${conversation.unreadCount > 0 ? 'text-white' : 'text-slate-200'}`}>
                        {conversation.recipientName}
                      </span>
                      <span className="text-xs text-slate-500">
                        {formatTime(conversation.lastMessageTime)}
                      </span>
                    </div>
                    <p className={`truncate text-sm ${conversation.unreadCount > 0 ? 'font-medium text-slate-300' : 'text-slate-400'}`}>
                      {conversation.lastMessage}
                    </p>
                  </div>
                </Link>
              ))
            )}
          </div>
        </div>

        {/* Messages */}
        <div className={`${showMessages ? 'flex' : 'hidden'} flex-1 flex-col md:flex`}>
          {currentConversation ? (
            <>
              {/* Header */}
              <div className="flex items-center gap-3 border-b border-slate-700 px-4 py-3">
                <Link
                  to="/dm"
                  className="rounded-lg p-1.5 text-slate-400 hover:bg-slate-700 hover:text-white md:hidden"
                >
                  <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                  </svg>
                </Link>
                <div className="flex h-8 w-8 items-center justify-center rounded-full bg-indigo-500 text-sm font-medium text-white">
                  {currentConversation.recipientAvatar}
                </div>
                <div>
                  <h3 className="font-medium text-white">{currentConversation.recipientName}</h3>
                  <p className="text-xs text-slate-400">Online</p>
                </div>
              </div>

              {/* Messages */}
              <div className="flex-1 overflow-y-auto p-4">
                <div className="space-y-4">
                  {messages.map(message => {
                    const isMe = message.senderId === 'me'
                    return (
                      <div
                        key={message.id}
                        className={`flex ${isMe ? 'justify-end' : 'justify-start'}`}
                      >
                        <div className={`max-w-[75%] ${isMe ? 'order-2' : 'order-1'}`}>
                          <div
                            className={`rounded-2xl px-4 py-2 ${
                              isMe
                                ? 'bg-indigo-600 text-white'
                                : 'bg-slate-700 text-slate-200'
                            }`}
                          >
                            {message.content}
                          </div>
                          <div className={`mt-1 text-xs text-slate-500 ${isMe ? 'text-right' : 'text-left'}`}>
                            {formatMessageTime(message.timestamp)}
                          </div>
                        </div>
                      </div>
                    )
                  })}
                  <div ref={messagesEndRef} />
                </div>
              </div>

              {/* Input */}
              <div className="border-t border-slate-700 p-4">
                <form onSubmit={handleSend} className="flex gap-2">
                  <input
                    type="text"
                    value={newMessage}
                    onChange={e => setNewMessage(e.target.value)}
                    placeholder={`Message ${currentConversation.recipientName}...`}
                    className="flex-1 rounded-lg border border-slate-600 bg-slate-700 px-4 py-2 text-white placeholder-slate-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  />
                  <button
                    type="submit"
                    disabled={!newMessage.trim()}
                    className="rounded-lg bg-indigo-600 px-4 py-2 font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
                  >
                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                    </svg>
                  </button>
                </form>
                <p className="mt-2 text-center text-xs text-slate-500">
                  Demo mode - messages are stored locally
                </p>
              </div>
            </>
          ) : (
            <div className="hidden flex-1 items-center justify-center md:flex">
              <div className="text-center">
                <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-slate-700">
                  <svg className="h-8 w-8 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                  </svg>
                </div>
                <h3 className="font-medium text-white">Your Messages</h3>
                <p className="mt-1 text-sm text-slate-400">
                  Select a conversation to start chatting
                </p>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
