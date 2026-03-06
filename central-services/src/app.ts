import { createClient, type SupabaseClient, type Session } from '@supabase/supabase-js'
import { createForumStore, createHubStore, type ForumStore, type HubStore } from '@johnvondrashek/forumline-core'
import { createResetPassword } from './components/reset-password.js'
import { createHubAuth } from './components/hub-auth.js'
import { createAppLayout } from './components/app-layout.js'

const hubSupabaseUrl = import.meta.env.VITE_HUB_SUPABASE_URL
const hubSupabaseAnonKey = import.meta.env.VITE_HUB_SUPABASE_ANON_KEY

// Hub Supabase client — custom storageKey avoids "Multiple GoTrueClient instances" warning
export const hubSupabase: SupabaseClient = createClient(hubSupabaseUrl, hubSupabaseAnonKey, {
  auth: { storageKey: 'forumline-hub-auth' },
})

export const forumStore: ForumStore = createForumStore()
export const hubStore: HubStore = createHubStore({
  hubSupabaseUrl,
  hubSupabaseAnonKey,
  hubUrl: '',
})

export function createApp(root: HTMLElement) {
  let currentDestroy: (() => void) | null = null

  function renderLoading() {
    cleanup()
    root.innerHTML = ''
    const screen = document.createElement('div')
    screen.className = 'loading-screen'
    const spinner = document.createElement('div')
    spinner.className = 'spinner'
    screen.appendChild(spinner)
    root.appendChild(screen)
  }

  function renderResetPassword() {
    cleanup()
    root.innerHTML = ''
    const { el, destroy } = createResetPassword({
      supabase: hubSupabase,
      onComplete() {
        passwordRecovery = false
        renderForSession(currentSession)
      },
    })
    currentDestroy = destroy
    root.appendChild(el)
  }

  function renderAuth() {
    cleanup()
    root.innerHTML = ''
    const page = document.createElement('div')
    page.className = 'auth-page'
    const { el } = createHubAuth({ supabase: hubSupabase })
    page.appendChild(el)
    root.appendChild(page)
  }

  function renderApp(session: Session) {
    cleanup()
    root.innerHTML = ''

    // Init hub store with the direct session
    hubStore.init({
      access_token: session.access_token,
      user_id: session.user.id,
    })

    const { el, destroy } = createAppLayout({
      hubSession: session,
      forumStore,
      hubStore,
      supabase: hubSupabase,
    })
    currentDestroy = destroy
    root.appendChild(el)
  }

  function renderForSession(session: Session | null) {
    if (passwordRecovery) {
      renderResetPassword()
      return
    }
    if (!session) {
      renderAuth()
      return
    }
    renderApp(session)
  }

  function cleanup() {
    if (currentDestroy) {
      currentDestroy()
      currentDestroy = null
    }
  }

  let currentSession: Session | null = null
  let passwordRecovery = false
  let hasRenderedApp = false

  renderLoading()

  const { data: { subscription } } = hubSupabase.auth.onAuthStateChange((event, session) => {
    currentSession = session
    if (event === 'PASSWORD_RECOVERY') {
      passwordRecovery = true
      renderForSession(session)
      hasRenderedApp = !!session
    } else if (event === 'TOKEN_REFRESHED') {
      // Token refresh doesn't need to re-create the app layout — the supabase
      // client handles token management internally. Re-rendering would destroy
      // the webview iframe and cause fetch failures in pending callbacks.
    } else if (event === 'INITIAL_SESSION' || event === 'SIGNED_IN') {
      // Supabase can fire SIGNED_IN before INITIAL_SESSION when a stored
      // session exists. Guard both so the app only renders once.
      if (!hasRenderedApp) {
        renderForSession(session)
        hasRenderedApp = !!session
      }
    } else {
      // SIGNED_OUT or other events
      hasRenderedApp = false
      renderForSession(session)
    }
  })

  // Return top-level cleanup
  return () => {
    subscription.unsubscribe()
    cleanup()
    hubStore.destroy()
  }
}
