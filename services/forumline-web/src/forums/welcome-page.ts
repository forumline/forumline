/*
 * Welcome / home page (Van.js)
 *
 * Structured as three zones:
 * 1. Status strip — compact connection indicator at top
 * 2. The Shelf — forums displayed as glossy icons on a wooden shelf
 * 3. The Bulletin Board — forum discovery (cork board with pinned flyers)
 */
import type { GoTrueAuthClient, ForumlineSession } from '../auth/gotrue-auth.js'
import type { ForumStore, ForumMembership } from './forum-store.js'
import type { ForumlineStore } from '../shared/forumline-store.js'
import { createMobileForumList } from './mobile-forum-list.js'
import { createForumDiscovery } from './forum-discovery.js'
import { createButton } from '../shared/ui.js'
import { tags, html } from '../shared/dom.js'

const { div, p, span, button: btn } = tags

interface WelcomePageOptions {
  forumlineSession: ForumlineSession | null
  forumStore: ForumStore
  forumlineStore: ForumlineStore
  auth: GoTrueAuthClient
  onGoToSettings: () => void
}

function totalUnread(counts: { notifications: number; chat_mentions: number; dms: number } | undefined): number {
  if (!counts) return 0
  return counts.notifications + counts.chat_mentions + counts.dms
}

export function createWelcomePage({ forumlineSession, forumStore, forumlineStore, auth, onGoToSettings }: WelcomePageOptions) {
  let forumListInstance: ReturnType<typeof createMobileForumList> | null = null
  let discoveryInstance: ReturnType<typeof createForumDiscovery> | null = null

  // Keep mobile-forum-list around for the add-forum modal only
  function ensureForumList() {
    if (!forumListInstance) {
      forumListInstance = createMobileForumList({ forumStore })
    }
    return forumListInstance
  }

  function ensureDiscovery() {
    if (!discoveryInstance) {
      discoveryInstance = createForumDiscovery({ forumStore, auth })
    }
    return discoveryInstance
  }

  // -- Shelf icon for a single forum --
  function createShelfIcon(forum: ForumMembership, unread: number): HTMLElement {
    const iconSize = 64

    const iconEl = forum.icon_url
      ? (() => {
          const src = forum.icon_url.startsWith('/') ? `${forum.web_base}${forum.icon_url}` : forum.icon_url
          return tags.img({
            src,
            alt: forum.name,
            class: 'shelf-icon__img',
            width: iconSize,
            height: iconSize,
            onerror: (e: Event) => { (e.target as HTMLImageElement).style.display = 'none' },
          })
        })()
      : div({ class: 'shelf-icon__fallback' }, forum.name[0].toUpperCase())

    const wrapper = btn({
      class: 'shelf-icon',
      onclick: () => forumStore.switchForum(forum.domain),
      title: forum.name,
    },
      div({ class: 'shelf-icon__icon-wrap' },
        iconEl,
        // Badge
        unread > 0
          ? div({ class: 'badge badge--red shelf-icon__badge' }, unread > 99 ? '99+' : String(unread))
          : document.createTextNode(''),
      ),
      span({ class: 'shelf-icon__label' }, forum.name),
    ) as HTMLElement

    return wrapper
  }

  const el = div({
    class: 'page-scroll',
    style: 'padding-left:1rem;padding-right:1rem',
  },

    // ── Zone 1: Status strip ──
    () => {
      const { isForumlineConnected } = forumlineStore.state.val
      const dotCls = `status-dot ${isForumlineConnected ? 'status-dot--connected' : 'status-dot--disconnected'}`

      if (isForumlineConnected) {
        const username = forumlineSession?.user?.user_metadata?.username || forumlineSession?.user?.email || 'user'
        return div({ class: 'status-strip' },
          div({ class: dotCls }),
          span({ class: 'text-xs text-muted' }, `@${username}`),
        )
      }

      return div({ class: 'status-strip status-strip--disconnected' },
        div({ class: dotCls }),
        span({ class: 'text-xs text-muted' }, 'Not connected'),
        createButton({
          text: 'Sign in',
          variant: 'primary',
          className: 'btn--small',
          onClick: onGoToSettings,
        }),
      )
    },

    // ── Zone 2: The Shelf ──
    () => {
      const { forums, unreadCounts } = forumStore.state.val

      if (forums.length === 0) {
        // Empty shelf — inviting first-forum state
        const list = ensureForumList()
        return div({ class: 'shelf' },
          div({ class: 'shelf__empty' },
            div({ class: 'welcome-icon' },
              html(`<svg class="icon-xl" style="color:#fff" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/></svg>`),
            ),
            p({ class: 'text-lg font-semibold text-white', style: 'margin-top:1rem' }, 'Your shelf is empty'),
            p({ class: 'text-sm text-muted', style: 'margin-top:0.25rem' }, 'Add your first forum to get started'),
            div({ style: 'margin-top:1rem;width:100%;max-width:20rem' }, list.el),
          ),
          div({ class: 'shelf__ledge' }),
        )
      }

      // Populated shelf — glossy icons in a grid
      const grid = div({ class: 'shelf__grid' }) as HTMLElement
      for (const forum of forums) {
        const unread = totalUnread(unreadCounts[forum.domain])
        grid.appendChild(createShelfIcon(forum, unread))
      }

      // Add button as a shelf icon
      grid.appendChild(btn({
        class: 'shelf-icon shelf-icon--add',
        onclick: () => { ensureForumList(); forumListInstance!.el.querySelector('.add-forum-btn')?.dispatchEvent(new MouseEvent('click')) },
      },
        div({ class: 'shelf-icon__icon-wrap shelf-icon__icon-wrap--add' },
          html(`<svg class="icon-lg" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"/></svg>`),
        ),
        span({ class: 'shelf-icon__label' }, 'Add Forum'),
      ) as HTMLElement)

      return div({ class: 'shelf' },
        grid,
        div({ class: 'shelf__ledge' }),
      )
    },

    // Hidden forum list (for modal functionality)
    (() => {
      const wrap = div({ style: 'display:none' })
      wrap.appendChild(ensureForumList().el)
      return wrap
    })(),

    // ── Zone 3: The Bulletin Board ──
    div({ class: 'mx-auto', style: 'max-width:36rem;width:100%;padding-bottom:2rem;margin-top:1.5rem' },
      ensureDiscovery().el,
    ),
  )

  return {
    el: el as HTMLElement,
    destroy() {
      forumListInstance?.destroy()
      discoveryInstance?.destroy()
    },
  }
}
