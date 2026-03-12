/*
 * Settings page (Van.js + VanX)
 *
 * This file provides the user's account settings, profile management, notification preferences,
 * privacy controls, and forum management.
 *
 * It must:
 * - Display user profile with avatar, display name, email, and editable status
 * - Provide toggles for notification preferences (push, email, DMs, mentions)
 * - Provide privacy controls (online status visibility, read receipts)
 * - Provide online status selector (online, away, appear offline)
 * - Provide change password via modal dialog
 * - Display the Forumline account section: show profile when signed in, or the auth form when not
 * - Provide a sign-out button for the connected Forumline account
 * - Display a list of all connected forums with their icons, names, and domains
 * - Allow removing forums from the user's list
 * - Allow muting/unmuting notifications per forum with optimistic UI updates
 * - Show an "Edit Site" button for forums the user owns on the hosted platform
 * - Open the site manager (file editor) for owned hosted forums
 * - Fetch forum membership data and notification mute states from the server
 * - Fetch the user's avatar from the identity API
 * - Detect which forums are hosted sites owned by the current user
 * - Reactively update the forum list when forums are added or removed
 */
import type { GoTrueAuthClient, ForumlineSession } from '../auth/gotrue-auth.js'
import type { ForumStore } from '../forums/forum-store.js'
import type { ForumlineStore } from '../shared/forumline-store.js'
import { createForumlineAuth } from '../auth/forumline-auth.js'
import { createSiteManager } from './site-manager.js'
import { tags, vanX } from '../shared/dom.js'
import { createButton, createCard, showToast } from '../shared/ui.js'

const { div, h2, p, span, label, select, option, img } = tags

interface SettingsPageOptions {
  forumlineSession: ForumlineSession | null
  forumStore: ForumStore
  forumlineStore: ForumlineStore
  auth: GoTrueAuthClient
  onClose: () => void
}

// ---- Toggle component ----

function createToggle(opts: { checked: boolean; onChange: (v: boolean) => void }): HTMLElement {
  const checkbox = tags.input({
    type: 'checkbox',
    checked: opts.checked,
    onchange: (e: Event) => {
      const val = (e.target as HTMLInputElement).checked
      opts.onChange(val)
    },
  }) as HTMLInputElement

  const el = label(
    { class: 'settings-toggle' },
    checkbox,
    span({ class: 'settings-toggle-track' }),
    span({ class: 'settings-toggle-knob' }),
  ) as HTMLElement

  return el
}

// ---- Row helpers ----

function settingsRow(labelText: string, right: HTMLElement | string): HTMLElement {
  return div(
    { class: 'settings-row' },
    span({ class: 'settings-row-label' }, labelText),
    typeof right === 'string'
      ? span({ class: 'settings-row-value' }, right)
      : right,
  ) as HTMLElement
}

function tappableRow(labelText: string, value: string, onClick: () => void): HTMLElement {
  return div(
    { class: 'settings-row tappable', onclick: onClick },
    span({ class: 'settings-row-label' }, labelText),
    div(
      { style: 'display:flex;align-items:center;gap:0.25rem;min-width:0' },
      span({ class: 'settings-row-value' }, value),
      span({ class: 'settings-row-chevron' }, '\u203A'),
    ),
  ) as HTMLElement
}

function settingsSection(title: string, rows: HTMLElement[], footer?: string): HTMLElement {
  return div(
    { class: 'settings-section' },
    title ? div({ class: 'settings-section-label' }, title) : null,
    div({ class: 'settings-group' }, ...rows),
    footer ? p({ class: 'settings-section-footer' }, footer) : null,
  ) as HTMLElement
}

// ---- Edit modal ----

function settingsInput(opts: { type?: string; value?: string; placeholder?: string; minLength?: number }): HTMLInputElement {
  return tags.input({
    class: 'settings-input',
    type: opts.type ?? 'text',
    value: opts.value ?? '',
    placeholder: opts.placeholder ?? '',
    ...(opts.minLength ? { minlength: opts.minLength } : {}),
  }) as HTMLInputElement
}

function showEditModal(opts: {
  title: string
  value: string
  placeholder: string
  onSave: (v: string) => void
}): HTMLElement {
  const inputEl = settingsInput({ value: opts.value, placeholder: opts.placeholder })

  function save() {
    opts.onSave(inputEl.value)
    overlay.remove()
  }

  const overlay = div(
    {
      class: 'settings-modal-overlay',
      onclick: (e: Event) => {
        if ((e.target as HTMLElement).classList.contains('settings-modal-overlay')) overlay.remove()
      },
    },
    div(
      { class: 'settings-modal' },
      div({ class: 'settings-modal-header' },
        div({ class: 'settings-modal-title' }, opts.title),
      ),
      div({ class: 'settings-modal-body' },
        inputEl,
      ),
      div(
        { class: 'settings-modal-footer' },
        tags.button({ onclick: () => overlay.remove() }, 'Cancel'),
        tags.button({ onclick: save }, 'Save'),
      ),
    ),
  ) as HTMLElement

  inputEl.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') save()
    if (e.key === 'Escape') overlay.remove()
  })

  document.body.appendChild(overlay)
  setTimeout(() => inputEl.focus(), 50)
  return overlay
}

// ---- Change password modal ----

function showChangePasswordModal(auth: GoTrueAuthClient): HTMLElement {
  const newPassInput = settingsInput({ type: 'password', placeholder: 'New password', minLength: 8 })
  const confirmInput = settingsInput({ type: 'password', placeholder: 'Confirm new password' })

  async function save() {
    if (newPassInput.value.length < 8) {
      showToast('Password must be at least 8 characters', 'error')
      return
    }
    if (newPassInput.value !== confirmInput.value) {
      showToast('Passwords do not match', 'error')
      return
    }
    const { error } = await auth.updateUser({ password: newPassInput.value })
    if (error) {
      showToast(`Failed to change password: ${error.message}`, 'error')
      return
    }
    showToast('Password changed successfully', 'success')
    overlay.remove()
  }

  const overlay = div(
    {
      class: 'settings-modal-overlay',
      onclick: (e: Event) => {
        if ((e.target as HTMLElement).classList.contains('settings-modal-overlay')) overlay.remove()
      },
    },
    div(
      { class: 'settings-modal' },
      div({ class: 'settings-modal-header' },
        div({ class: 'settings-modal-title' }, 'Change Password'),
      ),
      div({ class: 'settings-modal-body' },
        newPassInput,
        confirmInput,
      ),
      div(
        { class: 'settings-modal-footer' },
        tags.button({ onclick: () => overlay.remove() }, 'Cancel'),
        tags.button({ onclick: () => void save() }, 'Save'),
      ),
    ),
  ) as HTMLElement

  confirmInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') void save()
    if (e.key === 'Escape') overlay.remove()
  })

  document.body.appendChild(overlay)
  setTimeout(() => newPassInput.focus(), 50)
  return overlay
}

// ---- Main Settings Page ----

export function createSettingsPage({ forumlineSession, forumStore, forumlineStore: _forumlineStore, auth, onClose }: SettingsPageOptions) {
  interface OwnedForum {
    id: string
    domain: string
    name: string
    icon_url: string | null
    api_base: string
    web_base: string
    approved: boolean
    member_count: number
    last_seen_at: string | null
    consecutive_failures: number
    created_at: string
  }

  const settings = vanX.reactive({
    memberships: [] as { forum_domain: string; notifications_muted: boolean }[],
    avatarUrl: null as string | null,
    ownedSites: {} as Record<string, string>, // domain -> slug
    ownedForums: [] as OwnedForum[],
    displayName: (forumlineSession?.user?.user_metadata?.username as string) || '',
    statusMessage: '',
  })

  let siteManagerChild: { el: HTMLElement; destroy: () => void } | null = null

  const el = div({ class: 'settings-page' }) as HTMLElement
  const settingsWrapper = div() as HTMLElement

  // Brushed metal navbar
  const navbar = div(
    { class: 'settings-navbar' },
    tags.button({ class: 'settings-back-btn', onclick: onClose }, '\u2039 Back'),
    span({ class: 'settings-navbar-title' }, 'Settings'),
  ) as HTMLElement
  settingsWrapper.appendChild(navbar)

  const content = div({ class: 'settings-container' }) as HTMLElement
  settingsWrapper.appendChild(content)
  el.appendChild(settingsWrapper)

  // ---- Site manager ----

  function openSiteManager(forum: { name: string; domain: string }) {
    const slug = settings.ownedSites[forum.domain]
    if (!slug) return
    settingsWrapper.style.display = 'none'
    siteManagerChild?.destroy()
    siteManagerChild = createSiteManager({
      slug,
      forumName: forum.name,
      domain: forum.domain,
      auth,
      onClose: () => {
        siteManagerChild?.el.remove()
        siteManagerChild?.destroy()
        siteManagerChild = null
        settingsWrapper.style.display = ''
      },
    })
    el.appendChild(siteManagerChild.el)
  }

  // ===========================================================================
  // User Settings Sections (profile, notifications, privacy, account)
  // ===========================================================================

  if (forumlineSession) {
    const userId = forumlineSession.user.id
    const userEmail = forumlineSession.user.email || ''
    const avatarUrl = settings.avatarUrl ||
      `https://api.dicebear.com/9.x/avataaars/svg?seed=${encodeURIComponent(userId)}&size=256`

    // ---- Profile section ----
    const profileAvatarImg = img({
      class: 'settings-profile-avatar',
      src: avatarUrl,
      alt: 'Avatar',
      loading: 'lazy',
    }) as HTMLImageElement

    const profileNameEl = span({ class: 'settings-profile-name' }) as HTMLElement
    profileNameEl.textContent = settings.displayName || userEmail

    const profileSection = settingsSection('Profile', [
      div(
        {
          class: 'settings-profile-header-row',
          onclick: () => showEditModal({
            title: 'Edit Display Name',
            value: settings.displayName,
            placeholder: 'Enter display name',
            onSave: (v) => {
              settings.displayName = v
              profileNameEl.textContent = v || userEmail
              void updateProfile({ username: v })
            },
          }),
        },
        profileAvatarImg,
        div(
          { class: 'settings-profile-info' },
          profileNameEl,
          div({ class: 'settings-profile-subtitle' }, userEmail),
        ),
        span({ class: 'settings-row-chevron' }, '\u203A'),
      ) as HTMLElement,
      tappableRow('Display Name', settings.displayName || userEmail, () => {
        showEditModal({
          title: 'Edit Display Name',
          value: settings.displayName,
          placeholder: 'Enter display name',
          onSave: (v) => {
            settings.displayName = v
            profileNameEl.textContent = v || userEmail
            void updateProfile({ username: v })
          },
        })
      }),
      tappableRow('Status', settings.statusMessage || 'Set a status\u2026', () => {
        showEditModal({
          title: 'Edit Status',
          value: settings.statusMessage,
          placeholder: "What's on your mind?",
          onSave: (v) => {
            settings.statusMessage = v
            void updateProfile({ status_message: v })
          },
        })
      }),
      settingsRow('Email', userEmail),
    ])
    content.appendChild(profileSection)

    // ---- Online Status section ----
    const statusSelect = select(
      {
        class: 'settings-select',
        onchange: (e: Event) => {
          void updateProfile({ online_status: (e.target as HTMLSelectElement).value })
        },
      },
      option({ value: 'online', selected: true }, 'Online'),
      option({ value: 'away' }, 'Away'),
      option({ value: 'offline' }, 'Appear Offline'),
    ) as HTMLSelectElement

    const onlineStatusSection = settingsSection('Online Status', [
      settingsRow('Status', statusSelect),
      settingsRow('Show Online Status', createToggle({
        checked: true,
        onChange: (v) => void updateProfile({ show_online_status: v }),
      })),
    ], "When hidden, other users won\u2019t see when you\u2019re online.")
    content.appendChild(onlineStatusSection)

    // ---- Notifications section ----
    const notificationsSection = settingsSection('Notifications', [
      settingsRow('Push Notifications', createToggle({
        checked: true,
        onChange: (v) => {
          if (v && 'Notification' in window && Notification.permission !== 'granted') {
            void Notification.requestPermission()
          }
        },
      })),
      settingsRow('Email Notifications', createToggle({
        checked: true,
        onChange: () => {},
      })),
      settingsRow('Direct Messages', createToggle({
        checked: true,
        onChange: () => {},
      })),
      settingsRow('Mentions', createToggle({
        checked: true,
        onChange: () => {},
      })),
    ], 'Push notifications require browser permission.')
    content.appendChild(notificationsSection)

    // ---- Privacy section ----
    const privacySection = settingsSection('Privacy', [
      settingsRow('Read Receipts', createToggle({
        checked: true,
        onChange: () => {},
      })),
    ], "When off, you won\u2019t send or receive read receipts.")
    content.appendChild(privacySection)

    // ---- Account section ----
    const accountSection = settingsSection('Account', [
      tappableRow('Change Password', '', () => showChangePasswordModal(auth)),
    ])
    content.appendChild(accountSection)

    // ---- Sign Out ----
    const signOutSection = settingsSection('', [
      div(
        { class: 'settings-row tappable danger', onclick: () => void auth.signOut() },
        span({ class: 'settings-row-label' }, 'Sign Out'),
      ) as HTMLElement,
    ])
    content.appendChild(signOutSection)

    // ---- Delete Account ----
    const deleteSection = settingsSection('', [
      div(
        {
          class: 'settings-row tappable danger',
          onclick: () => {
            if (confirm('Are you sure you want to delete your account? This action cannot be undone.')) {
              void deleteAccount()
            }
          },
        },
        span({ class: 'settings-row-label' }, 'Delete Account'),
      ) as HTMLElement,
    ])
    content.appendChild(deleteSection)
  } else {
    // Not signed in — show auth form
    const accountCard = createCard()
    accountCard.appendChild(h2({ class: 'text-lg font-semibold text-white' }, 'Forumline') as HTMLElement)
    accountCard.appendChild(p({ class: 'text-sm text-muted mt-sm' }, 'Connect to the Forumline for cross-forum direct messages') as HTMLElement)
    const { el: authEl } = createForumlineAuth({ auth })
    accountCard.appendChild(authEl)
    content.appendChild(accountCard)
  }

  // ===========================================================================
  // Forums Management (preserved from existing settings page)
  // ===========================================================================

  const forumsCard = createCard()
  forumsCard.appendChild(h2({ class: 'text-lg font-semibold text-white' }, 'Forums') as HTMLElement)
  forumsCard.appendChild(p({ class: 'text-sm text-muted mt-sm' }, 'Manage your connected forums') as HTMLElement)

  function isMuted(domain: string): boolean {
    return settings.memberships.find(m => m.forum_domain === domain)?.notifications_muted ?? false
  }

  function buildMuteButton(domain: string): HTMLElement {
    const muted = isMuted(domain)
    const muteBtn = tags.button({
      class: 'btn--icon',
      title: muted ? 'Unmute notifications' : 'Mute notifications',
      onclick: () => void toggleMute(domain, !isMuted(domain)),
    }) as HTMLButtonElement
    // eslint-disable-next-line no-unsanitized/property -- hardcoded SVG icon strings, no user input
    muteBtn.innerHTML = muted
      ? `<svg class="icon-sm" style="color:var(--color-text-faint)" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2"/></svg>`
      : `<svg class="icon-sm" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072M18.364 5.636a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"/></svg>`
    return muteBtn
  }

  function buildForumRow(forum: { name: string; domain: string; icon_url?: string; web_base: string }) {
    const row = div({ class: 'settings-forum-row' }) as HTMLElement

    if (forum.icon_url) {
      const iconSrc = forum.icon_url.startsWith('/') ? `${forum.web_base}${forum.icon_url}` : forum.icon_url
      const iconImg = tags.img({ src: iconSrc, alt: forum.name, class: 'forum-card__icon', onerror: () => { iconImg.style.display = 'none' } }) as HTMLImageElement
      row.appendChild(iconImg)
    } else {
      row.appendChild(div({ class: 'forum-card__icon-fallback' }, forum.name[0].toUpperCase()) as HTMLElement)
    }

    const info = div({ class: 'flex-1' },
      p({ class: 'font-medium text-white' }, forum.name),
      p({ class: 'text-sm text-muted' }, forum.domain),
    ) as HTMLElement
    row.appendChild(info)

    if (settings.ownedSites[forum.domain]) {
      row.appendChild(createButton({ text: 'Edit Site', variant: 'ghost', className: 'text-sm', onClick: () => openSiteManager(forum) }))
    }

    row.appendChild(buildMuteButton(forum.domain))

    const removeBtn = createButton({
      text: 'Remove',
      variant: 'danger',
      className: 'text-sm',
      onClick: () => handleRemoveForum(forum.domain, removeBtn),
    })
    row.appendChild(removeBtn)

    return row
  }

  // Reactive forum list
  const reactiveForumList = div(
    () => {
      const { forums } = forumStore.state.val
      void settings.memberships
      void settings.ownedSites
      if (forums.length === 0) {
        return p({ class: 'text-sm text-faint mt-lg' }, 'No forums added yet. Go to Home and tap Add Forum to add one.') as HTMLElement
      }
      const container = div({ style: 'display:flex;flex-direction:column;gap:0.5rem;margin-top:1rem' }) as HTMLElement
      forums.forEach(f => container.appendChild(buildForumRow(f)))
      return container
    },
  ) as HTMLElement
  forumsCard.appendChild(reactiveForumList)
  content.appendChild(forumsCard)

  // Owned forums card
  function healthIndicator(failures: number): HTMLElement {
    let color: string, labelText: string
    if (failures === 0) {
      color = '#22c55e'; labelText = 'Healthy'
    } else if (failures < 3) {
      color = '#eab308'; labelText = 'Unreachable'
    } else {
      color = '#ef4444'; labelText = 'Offline (delisted)'
    }
    return span(
      { class: 'text-sm', style: `display:inline-flex;align-items:center;gap:0.375rem` },
      span({ style: `width:8px;height:8px;border-radius:50%;background:${color};display:inline-block` }) as HTMLElement,
      labelText,
    ) as HTMLElement
  }

  function buildOwnedForumRow(forum: OwnedForum) {
    const row = div({ class: 'settings-forum-row' }) as HTMLElement

    if (forum.icon_url) {
      const iconSrc = forum.icon_url.startsWith('/') ? `${forum.web_base}${forum.icon_url}` : forum.icon_url
      const iconImg = tags.img({ src: iconSrc, alt: forum.name, class: 'forum-card__icon', onerror: () => { iconImg.style.display = 'none' } }) as HTMLImageElement
      row.appendChild(iconImg)
    } else {
      row.appendChild(div({ class: 'forum-card__icon-fallback' }, forum.name[0].toUpperCase()) as HTMLElement)
    }

    const info = div({ class: 'flex-1' },
      p({ class: 'font-medium text-white' }, forum.name),
      div({ style: 'display:flex;align-items:center;gap:0.75rem' },
        p({ class: 'text-sm text-muted' }, forum.domain),
        healthIndicator(forum.consecutive_failures),
        !forum.approved
          ? span({ class: 'text-sm', style: 'color:#ef4444' }, 'Delisted')
          : span() as HTMLElement,
      ),
      p({ class: 'text-sm text-faint' }, `${forum.member_count} member${forum.member_count !== 1 ? 's' : ''}`),
    ) as HTMLElement
    row.appendChild(info)

    const deleteBtn = createButton({
      text: 'Delete',
      variant: 'danger',
      className: 'text-sm',
      onClick: () => void handleDeleteOwnedForum(forum, deleteBtn),
    })
    row.appendChild(deleteBtn)

    return row
  }

  const ownedForumsWrapper = div(
    () => {
      const forums = settings.ownedForums
      if (forums.length === 0) return div() as HTMLElement
      const card = createCard()
      card.appendChild(h2({ class: 'text-lg font-semibold text-white' }, 'Your Forums') as HTMLElement)
      card.appendChild(p({ class: 'text-sm text-muted mt-sm' }, 'Forums you have registered on the network') as HTMLElement)
      const container = div({ style: 'display:flex;flex-direction:column;gap:0.5rem;margin-top:1rem' }) as HTMLElement
      forums.forEach(f => container.appendChild(buildOwnedForumRow(f)))
      card.appendChild(container)
      return card
    },
  ) as HTMLElement
  content.appendChild(ownedForumsWrapper)

  // Version footer
  content.appendChild(div({ class: 'settings-version' }, 'Forumline v1.0') as HTMLElement)

  // ===========================================================================
  // API calls
  // ===========================================================================

  async function updateProfile(data: Record<string, unknown>) {
    try {
      const session = auth.getSession()
      if (!session) return
      await fetch('/api/identity', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${session.access_token}` },
        body: JSON.stringify(data),
      })
    } catch { /* non-critical */ }
  }

  async function deleteAccount() {
    try {
      const session = auth.getSession()
      if (!session) return
      const res = await fetch('/api/identity', {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${session.access_token}` },
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error((data as { error?: string }).error || 'Failed to delete account')
      }
      void auth.signOut()
    } catch (err) {
      showToast(`Failed to delete account: ${(err as Error).message}`, 'error')
    }
  }

  async function handleDeleteOwnedForum(forum: OwnedForum, btn: HTMLElement) {
    const confirmed = window.confirm(
      `Delete ${forum.name}?\n\nThis will remove the forum from Forumline and disconnect all ${forum.member_count} member${forum.member_count !== 1 ? 's' : ''}. This cannot be undone.`,
    )
    if (!confirmed) return

    const btnEl = btn as HTMLButtonElement
    btnEl.disabled = true
    btnEl.textContent = 'Deleting...'

    try {
      const session = auth.getSession()
      if (!session) throw new Error('No session')
      const res = await fetch('/api/forums', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${session.access_token}` },
        body: JSON.stringify({ forum_domain: forum.domain }),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error((data as { error?: string }).error || 'Failed to delete forum')
      }
      void fetchOwnedForums()
    } catch (err) {
      btnEl.disabled = false
      btnEl.textContent = 'Delete'
      alert(`Failed to delete forum: ${(err as Error).message}`)
    }
  }

  async function fetchOwnedForums() {
    try {
      const session = auth.getSession()
      if (!session) return
      const res = await fetch('/api/forums/owned', { headers: { Authorization: `Bearer ${session.access_token}` } })
      if (!res.ok) return
      vanX.replace(settings.ownedForums, await res.json())
    } catch { /* non-critical */ }
  }

  async function fetchMemberships() {
    try {
      const session = auth.getSession()
      if (!session) return
      const res = await fetch('/api/memberships', { headers: { Authorization: `Bearer ${session.access_token}` } })
      if (!res.ok) return
      vanX.replace(settings.memberships, await res.json())
    } catch { /* non-critical */ }
  }

  async function fetchOwnedSites() {
    const session = auth.getSession()
    if (!session) return
    try {
      const res = await fetch('https://hosted.forumline.net/api/platform/owned-sites', {
        headers: { 'X-Forumline-ID': session.user.id },
      })
      if (!res.ok) return
      const sites = await res.json() as { domain: string; slug: string }[]
      const newOwned: Record<string, string> = {}
      for (const s of sites) newOwned[s.domain] = s.slug
      vanX.replace(settings.ownedSites, newOwned)
    } catch { /* hosted platform unreachable — not critical */ }
  }

  async function fetchAvatar() {
    const userId = forumlineSession?.user?.id
    if (!userId) return
    try {
      const session = auth.getSession()
      if (!session) return
      const res = await fetch('/api/identity', { headers: { Authorization: `Bearer ${session.access_token}` } })
      if (!res.ok) return
      const data = await res.json() as { avatar_url?: string; username?: string; status_message?: string }
      if (data.avatar_url) settings.avatarUrl = data.avatar_url
      if (data.username) {
        settings.displayName = data.username
      }
      if (data.status_message) settings.statusMessage = data.status_message
    } catch { /* ignore */ }
  }

  async function toggleMute(forumDomain: string, muted: boolean) {
    vanX.replace(settings.memberships, settings.memberships.map(
      m => m.forum_domain === forumDomain ? { ...m, notifications_muted: muted } : m,
    ))
    try {
      const session = auth.getSession()
      if (!session) throw new Error('No session')
      const res = await fetch('/api/memberships', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${session.access_token}` },
        body: JSON.stringify({ forum_domain: forumDomain, muted }),
      })
      if (!res.ok) throw new Error('Failed to toggle mute')
    } catch {
      vanX.replace(settings.memberships, settings.memberships.map(
        m => m.forum_domain === forumDomain ? { ...m, notifications_muted: !muted } : m,
      ))
    }
  }

  function handleRemoveForum(domain: string, removeBtn: HTMLElement) {
    (removeBtn as HTMLButtonElement).disabled = true
    removeBtn.textContent = 'Removing...'
    forumStore.removeForum(domain)
  }

  // ---- Init ----
  void fetchMemberships()
  void fetchAvatar()
  void fetchOwnedSites()
  void fetchOwnedForums()

  return {
    el,
    destroy() { siteManagerChild?.destroy() },
  }
}
