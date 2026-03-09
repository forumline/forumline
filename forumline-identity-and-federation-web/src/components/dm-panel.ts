/*
 * Direct Messages panel
 *
 * This file is the top-level container for all DM functionality, managing navigation between DM sub-views.
 *
 * It must:
 * - Show the conversation list as the default view
 * - Navigate to a message thread when a conversation is selected
 * - Navigate to the new 1:1 message screen when the compose button is tapped
 * - Navigate to the new group conversation screen when the group button is tapped
 * - Provide back navigation from any sub-view to the conversation list
 * - Show a sign-in prompt with a link to settings when the user is not connected to Forumline
 * - Create or retrieve a 1:1 conversation when a user is selected from the new message search
 * - Display a header with contextual title (Messages, New Message, New Group) and action buttons
 * - Destroy child views on navigation to prevent memory leaks
 */
import type { ForumlineStore } from '../lib/index.js'
import { createButton } from './ui.js'
import { createDmConversationList } from './dm-conversation-list.js'
import { createDmMessageView } from './dm-message-view.js'
import { createDmNewMessage } from './dm-new-message.js'
import { createDmNewGroup } from './dm-new-group.js'

type DmView = 'list' | 'conversation' | 'new' | 'new-group'

interface DmPanelOptions {
  forumlineStore: ForumlineStore
  onClose: () => void
  onGoToSettings: () => void
}

export function createDmPanel({ forumlineStore, onClose, onGoToSettings }: DmPanelOptions) {
  let dmView: DmView = 'list'
  let selectedConversationId: string | null = null
  let currentChild: { el: HTMLElement; destroy: () => void } | null = null

  const el = document.createElement('div')
  el.className = 'flex flex-col w-full overflow-hidden'
  el.style.height = '100%'
  el.style.background = 'var(--color-bg)'

  function render() {
    currentChild?.destroy()
    currentChild = null
    el.innerHTML = ''

    const { isForumlineConnected } = forumlineStore.get()

    // Header
    const header = document.createElement('div')
    header.className = 'dm-header'

    const headerLeft = document.createElement('div')
    headerLeft.className = 'flex items-center gap-sm'

    if (dmView !== 'list') {
      const backBtn = document.createElement('button')
      backBtn.className = 'btn--icon'
      backBtn.innerHTML = `<svg class="icon-sm" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"/></svg>`
      backBtn.addEventListener('click', () => {
        dmView = 'list'
        selectedConversationId = null
        render()
      })
      headerLeft.appendChild(backBtn)
    }

    const title = document.createElement('h2')
    title.className = 'font-semibold text-white'
    title.textContent = dmView === 'new' ? 'New Message' : dmView === 'new-group' ? 'New Group' : 'Messages'
    headerLeft.appendChild(title)
    header.appendChild(headerLeft)

    const headerRight = document.createElement('div')
    headerRight.className = 'flex items-center gap-sm'
    if (dmView === 'list' && isForumlineConnected) {
      // New group button
      const groupBtn = document.createElement('button')
      groupBtn.className = 'btn--icon'
      groupBtn.title = 'New group'
      groupBtn.innerHTML = `<svg class="icon-sm" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"/></svg>`
      groupBtn.addEventListener('click', () => { dmView = 'new-group'; render() })
      headerRight.appendChild(groupBtn)

      // New message button
      const newBtn = document.createElement('button')
      newBtn.className = 'btn--icon'
      newBtn.title = 'New message'
      newBtn.innerHTML = `<svg class="icon-sm" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>`
      newBtn.addEventListener('click', () => { dmView = 'new'; render() })
      headerRight.appendChild(newBtn)
    }
    header.appendChild(headerRight)
    el.appendChild(header)

    // Content
    const content = document.createElement('div')
    content.className = 'flex-1 overflow-hidden'

    if (!isForumlineConnected) {
      const empty = document.createElement('div')
      empty.className = 'empty-state'
      const p = document.createElement('p')
      p.className = 'text-muted'
      p.textContent = 'Sign in to send direct messages across forums'
      empty.appendChild(p)
      empty.appendChild(createButton({
        text: 'Sign in',
        variant: 'primary',
        className: 'mt-lg',
        onClick: onGoToSettings,
      }))
      content.appendChild(empty)
    } else if (dmView === 'new') {
      const newMsg = createDmNewMessage({
        forumlineStore,
        onSelectUser: async (userId) => {
          const { forumlineClient } = forumlineStore.get()
          if (!forumlineClient) return
          try {
            const { id } = await forumlineClient.getOrCreateDM(userId)
            selectedConversationId = id
            dmView = 'conversation'
            render()
          } catch (err) {
            console.error('[Forumline:DM] Failed to get/create DM:', err)
          }
        },
      })
      currentChild = newMsg
      content.appendChild(newMsg.el)
    } else if (dmView === 'new-group') {
      const newGroup = createDmNewGroup({
        forumlineStore,
        onCreated: (conversationId) => {
          selectedConversationId = conversationId
          dmView = 'conversation'
          render()
        },
      })
      currentChild = newGroup
      content.appendChild(newGroup.el)
    } else if (dmView === 'conversation' && selectedConversationId) {
      const msgView = createDmMessageView({
        forumlineStore,
        conversationId: selectedConversationId,
      })
      currentChild = msgView
      content.appendChild(msgView.el)
    } else {
      const convList = createDmConversationList({
        forumlineStore,
        onSelectConversation: (conversationId) => {
          selectedConversationId = conversationId
          dmView = 'conversation'
          render()
        },
      })
      currentChild = convList
      content.appendChild(convList.el)
    }

    el.appendChild(content)
  }

  render()

  return {
    el,
    destroy() {
      currentChild?.destroy()
    },
  }
}
