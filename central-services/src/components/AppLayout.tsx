import { useState, useCallback, useEffect } from 'react'
import type { Session } from '@supabase/supabase-js'
import type { ForumNotification } from '@johnvondrashek/forumline-protocol'
import { ForumRail, ForumWebview, useForum, useHub, isTauri, getTauriNotification, useDeepLink } from '@johnvondrashek/forumline-react'
import type { DeepLinkTarget } from '@johnvondrashek/forumline-react'
import WelcomePage from './WelcomePage'
import DmPanel from './DmPanel'
import SettingsPage from './SettingsPage'

type AppView = 'forums' | 'settings'

interface AppLayoutProps {
  hubSession: Session | null
}

/** Persist auth state change to hub DB */
async function updateForumAuthState(accessToken: string, forumDomain: string, authed: boolean) {
  try {
    await fetch('/api/memberships', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify({ forum_domain: forumDomain, authed }),
    })
  } catch (err) {
    console.error('[Hub] Failed to persist forum auth state:', err)
  }
}

export default function AppLayout({ hubSession }: AppLayoutProps) {
  const { activeForum, setUnreadCounts, switchForum } = useForum()
  const { isHubConnected } = useHub()
  const [showDmPanel, setShowDmPanel] = useState(false)
  const [view, setView] = useState<AppView>('forums')
  const [authedForums, setAuthedForums] = useState<Set<string>>(new Set())
  const [deepLinkPath, setDeepLinkPath] = useState<string | null>(null)

  const dmUnreadCount = 0

  // Handle deep links (desktop only)
  const handleDeepLink = useCallback((target: DeepLinkTarget) => {
    switchForum(target.domain)
    setDeepLinkPath(target.path)
  }, [switchForum])
  useDeepLink(handleDeepLink)

  // Clear deep link path when forum changes (so it doesn't persist to next switch)
  useEffect(() => {
    return () => setDeepLinkPath(null)
  }, [activeForum?.domain])

  // Pre-populate authedForums from persisted membership data on mount
  useEffect(() => {
    if (!hubSession) return
    const fetchMemberships = async () => {
      try {
        const res = await fetch('/api/memberships', {
          headers: { Authorization: `Bearer ${hubSession.access_token}` },
        })
        if (!res.ok) return
        const memberships: { forum_domain: string; forum_authed_at: string | null }[] = await res.json()
        const authed = new Set(
          memberships.filter(m => m.forum_authed_at).map(m => m.forum_domain),
        )
        if (authed.size > 0) setAuthedForums(authed)
      } catch {
        // Non-critical — postMessage handshake will detect auth state anyway
      }
    }
    fetchMemberships()
  }, [hubSession])

  const authUrlForForum = activeForum && hubSession && !authedForums.has(activeForum.domain)
    ? `${activeForum.api_base}/auth?hub_token=${hubSession.access_token}`
    : null

  const handleForumAuthed = useCallback((domain: string) => {
    setAuthedForums(prev => new Set(prev).add(domain))
    if (hubSession) updateForumAuthState(hubSession.access_token, domain, true)
  }, [hubSession])

  const handleForumSignedOut = useCallback((domain: string) => {
    setAuthedForums(prev => {
      const next = new Set(prev)
      next.delete(domain)
      return next
    })
    if (hubSession) updateForumAuthState(hubSession.access_token, domain, false)
  }, [hubSession])

  const handleForumNotification = useCallback(async (_domain: string, notification: ForumNotification) => {
    // Only notify when window is not focused
    if (document.hasFocus()) return

    const { title, body } = notification

    if (isTauri()) {
      try {
        const { sendNotification, isPermissionGranted, requestPermission } = await getTauriNotification()
        let permitted = await isPermissionGranted()
        if (!permitted) {
          const result = await requestPermission()
          permitted = result === 'granted'
        }
        if (permitted) {
          sendNotification({ title, body })
        }
      } catch (err) {
        console.error('[Hub] Tauri notification error:', err)
      }
    } else if ('Notification' in window) {
      if (Notification.permission === 'default') {
        await Notification.requestPermission()
      }
      if (Notification.permission === 'granted') {
        new Notification(title, { body })
      }
    }
  }, [])

  return (
    <div className="flex h-screen">
      <ForumRail
        onDmClick={() => setShowDmPanel(prev => !prev)}
        dmUnreadCount={dmUnreadCount}
        onSettingsClick={() => {
          setView(view === 'settings' ? 'forums' : 'settings')
        }}
      />

      <div className="relative flex flex-1 overflow-hidden">
        {view === 'settings' ? (
          <SettingsPage hubSession={hubSession} onClose={() => setView('forums')} />
        ) : activeForum ? (
          <ForumWebview
            forum={activeForum}
            authUrl={authUrlForForum}
            onAuthed={handleForumAuthed}
            onSignedOut={handleForumSignedOut}
            onUnreadCounts={setUnreadCounts}
            onNotification={handleForumNotification}
            initialPath={deepLinkPath}
          />
        ) : (
          <WelcomePage hubSession={hubSession} isHubConnected={isHubConnected} />
        )}

        {showDmPanel && (
          <DmPanel onClose={() => setShowDmPanel(false)} />
        )}
      </div>
    </div>
  )
}
