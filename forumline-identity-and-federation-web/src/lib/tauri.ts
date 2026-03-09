/*
 * Tauri desktop app bridge
 *
 * This file provides platform detection and lazy-loaded access to Tauri native APIs.
 *
 * It must:
 * - Detect whether the app is running inside the Tauri desktop wrapper
 * - Lazy-load Tauri notification, autostart, and shell plugins only when needed
 * - Provide an openExternal helper that uses Tauri's shell.open on desktop or window.open on web
 * - Ensure Tauri imports are tree-shaken from the web build by using dynamic imports
 */
export function isTauri(): boolean {
  return '__TAURI_INTERNALS__' in window
}

// Lazy-load Tauri APIs only when running in desktop app.
// Dynamic imports ensure this code is tree-shaken from the web build.

export async function getTauriNotification() {
  const mod = await import('@tauri-apps/plugin-notification')
  return mod
}

export async function getTauriAutostart() {
  const mod = await import('@tauri-apps/plugin-autostart')
  return mod
}

export async function getTauriShell() {
  const mod = await import('@tauri-apps/plugin-shell')
  return mod
}

/** Open a URL in the default browser (Tauri) or a new tab (web). */
export async function openExternal(url: string) {
  if (isTauri()) {
    const { open } = await getTauriShell()
    await open(url)
  } else {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}
