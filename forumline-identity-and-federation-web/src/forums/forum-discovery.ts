/*
 * Forum discovery component (Van.js)
 *
 * This file renders the forum directory with search, tag filtering, and peer-based recommendations.
 *
 * It must:
 * - Show a search input that queries the forum directory by name/description/domain
 * - Display clickable tag pills fetched from the server, filtering forums when tapped
 * - Show recommended forums based on the user's forum-mates (waggle dance algorithm)
 * - Display forums as cards with icon, name, description, member count, and tags
 * - Allow joining a forum directly from the discovery card
 * - Refresh the discovery list and recommendations after joining a forum
 * - Guard against stale search results with a fetch generation counter
 * - Reactively update when search/tag/filter state changes
 */
import type { ForumStore } from './forum-store.js'
import type { GoTrueAuthClient } from '../auth/gotrue-auth.js'
import { tags, html } from '../shared/dom.js'
import { reactive } from 'vanjs-ext'
import { createSpinner, showToast } from '../shared/ui.js'

const { div, h2, p, button: btn, input: inputTag, span } = tags

interface DiscoveryForum {
  id: string
  domain: string
  name: string
  icon_url: string | null
  api_base: string
  web_base: string
  capabilities: string[]
  description: string | null
  screenshot_url: string | null
  tags: string[]
  member_count: number
  shared_member_count?: number
}

interface ForumDiscoveryOptions {
  forumStore: ForumStore
  auth: GoTrueAuthClient
}

export function createForumDiscovery({ forumStore, auth }: ForumDiscoveryOptions) {
  const state = reactive({
    query: '',
    activeTag: null as string | null,
    tags: [] as string[],
    forums: [] as DiscoveryForum[],
    recommended: [] as DiscoveryForum[],
    loading: false,
    loadingRecommended: false,
    joiningDomain: null as string | null,
    // Incremented on every join to trigger reactive re-renders for isJoined checks
    joinGeneration: 0,
  })

  let searchTimeout: ReturnType<typeof setTimeout> | null = null
  // Guard against stale results — only apply results from the latest fetch
  let fetchGeneration = 0
  let recommendedGeneration = 0

  function getToken(): string | null {
    return auth.getSession()?.access_token ?? null
  }

  async function fetchTags() {
    try {
      const res = await fetch('/api/forums/tags')
      if (res.ok) {
        state.tags = await res.json()
      }
    } catch { /* non-critical */ }
  }

  async function fetchForums() {
    const gen = ++fetchGeneration
    state.loading = true
    try {
      const params = new URLSearchParams()
      if (state.query) params.set('q', state.query)
      if (state.activeTag) params.set('tag', state.activeTag)
      params.set('sort', 'popular')
      params.set('limit', '20')

      const res = await fetch(`/api/forums?${params}`)
      if (res.ok && gen === fetchGeneration) {
        state.forums = await res.json()
      }
    } catch { /* non-critical */ }
    if (gen === fetchGeneration) {
      state.loading = false
    }
  }

  async function fetchRecommended() {
    const token = getToken()
    if (!token) return

    const gen = ++recommendedGeneration
    state.loadingRecommended = true
    try {
      const res = await fetch('/api/forums/recommended', {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (res.ok && gen === recommendedGeneration) {
        state.recommended = await res.json()
      }
    } catch { /* non-critical */ }
    if (gen === recommendedGeneration) {
      state.loadingRecommended = false
    }
  }

  async function joinForum(forum: DiscoveryForum) {
    state.joiningDomain = forum.domain
    try {
      await forumStore.addForum(forum.domain)
      showToast(`Joined ${forum.name}`, 'success')
      // Trigger re-render so cards update "Joined" state
      state.joinGeneration++
      // Refresh recommendations (new forum-mates may change results)
      void fetchRecommended()
      // Refresh forum list (member counts changed)
      void fetchForums()
    } catch (err) {
      showToast(`Failed to join ${forum.name}`, 'error')
    }
    state.joiningDomain = null
  }

  function handleSearchInput(e: Event) {
    state.query = (e.target as HTMLInputElement).value
    if (searchTimeout) clearTimeout(searchTimeout)
    searchTimeout = setTimeout(() => void fetchForums(), 300)
  }

  function selectTag(tag: string | null) {
    state.activeTag = state.activeTag === tag ? null : tag
    // Clear any pending debounced search to prevent it from overwriting tag-filtered results
    if (searchTimeout) {
      clearTimeout(searchTimeout)
      searchTimeout = null
    }
    void fetchForums()
  }

  function isJoined(domain: string): boolean {
    // Reading joinGeneration ensures this is re-evaluated after a join
    void state.joinGeneration
    return forumStore.get().forums.some(f => f.domain === domain)
  }

  function createForumCard(forum: DiscoveryForum, showSharedCount = false): HTMLElement {
    const joined = isJoined(forum.domain)

    const iconEl = forum.icon_url
      ? tags.img({
          src: forum.icon_url.startsWith('/') ? `${forum.web_base}${forum.icon_url}` : forum.icon_url,
          alt: forum.name,
          class: 'discovery-card__icon',
          onerror: (e: Event) => { (e.target as HTMLImageElement).style.display = 'none' },
        })
      : div({ class: 'discovery-card__icon-fallback' }, forum.name[0]?.toUpperCase() ?? '?')

    const metaParts: HTMLElement[] = []
    if (forum.member_count > 0) {
      metaParts.push(span({ class: 'discovery-card__meta-item' },
        `${forum.member_count} member${forum.member_count !== 1 ? 's' : ''}`,
      ) as HTMLElement)
    }
    if (showSharedCount && forum.shared_member_count && forum.shared_member_count > 0) {
      metaParts.push(span({ class: 'discovery-card__meta-item discovery-card__meta-item--highlight' },
        `${forum.shared_member_count} in common`,
      ) as HTMLElement)
    }

    const tagEls = (forum.tags || []).slice(0, 3).map(t =>
      span({ class: 'discovery-card__tag' }, t) as HTMLElement,
    )

    const actionBtn = joined
      ? btn({ class: 'btn btn--ghost btn--sm', disabled: true }, 'Joined')
      : btn({
          class: 'btn btn--primary btn--sm',
          disabled: () => state.joiningDomain === forum.domain,
          onclick: () => void joinForum(forum),
        }, () => state.joiningDomain === forum.domain ? 'Joining...' : 'Join')

    const card = div({ class: 'discovery-card' },
      div({ class: 'discovery-card__header' },
        iconEl,
        div({ class: 'discovery-card__info' },
          div({ class: 'discovery-card__name' }, forum.name),
          p({ class: 'discovery-card__domain' }, forum.domain),
        ),
        actionBtn,
      ),
      forum.description
        ? p({ class: 'discovery-card__desc' }, forum.description)
        : document.createTextNode(''),
      (metaParts.length > 0 || tagEls.length > 0)
        ? div({ class: 'discovery-card__footer' },
            ...metaParts,
            ...tagEls,
          )
        : document.createTextNode(''),
    ) as HTMLElement

    return card
  }

  // Build the main element
  const el = div({ class: 'forum-discovery' },
    // Search
    div({ class: 'discovery-search' },
      html(`<svg class="discovery-search__icon" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>`),
      inputTag({
        type: 'search',
        class: 'discovery-search__input',
        placeholder: 'Search forums...',
        'aria-label': 'Search forums',
        oninput: handleSearchInput,
      }),
    ),

    // Tags
    () => {
      if (state.tags.length === 0) return document.createTextNode('')
      const container = div({ class: 'discovery-tags' }) as HTMLElement
      for (const tag of state.tags) {
        const isActive = state.activeTag === tag
        const pill = btn({
          class: `discovery-tag${isActive ? ' discovery-tag--active' : ''}`,
          'aria-pressed': isActive ? 'true' : 'false',
          onclick: () => selectTag(tag),
        }, tag) as HTMLElement
        container.appendChild(pill)
      }
      return container
    },

    // Recommended forums (waggle dance)
    () => {
      if (state.recommended.length === 0) return document.createTextNode('')
      const section = div({ class: 'discovery-section' },
        h2({ class: 'discovery-section__title' }, 'Recommended for you'),
        p({ class: 'discovery-section__subtitle' }, 'Popular with people in your forums'),
      ) as HTMLElement
      const list = div({ class: 'discovery-list', role: 'list' }) as HTMLElement
      for (const forum of state.recommended) {
        list.appendChild(createForumCard(forum, true))
      }
      section.appendChild(list)
      return section
    },

    // All forums
    () => {
      const section = div({ class: 'discovery-section' },
        h2({ class: 'discovery-section__title' },
          state.query || state.activeTag ? 'Search results' : 'Popular forums',
        ),
      ) as HTMLElement

      if (state.loading) {
        section.appendChild(div({ class: 'discovery-loading' }, createSpinner(true)) as HTMLElement)
        return section
      }

      if (state.forums.length === 0) {
        section.appendChild(p({ class: 'text-muted text-sm' },
          state.query ? 'No forums found' : 'No forums available yet',
        ) as HTMLElement)
        return section
      }

      const list = div({ class: 'discovery-list', role: 'list' }) as HTMLElement
      for (const forum of state.forums) {
        list.appendChild(createForumCard(forum))
      }
      section.appendChild(list)
      return section
    },
  ) as HTMLElement

  // Initial data fetch
  void fetchTags()
  void fetchForums()
  void fetchRecommended()

  return {
    el,
    destroy() {
      if (searchTimeout) clearTimeout(searchTimeout)
    },
  }
}
