/*
 * Toast Notifications
 *
 * Gives users brief, non-intrusive feedback when actions succeed or fail (e.g., "Profile updated", "Failed to bookmark").
 *
 * It must:
 * - Display temporary popup messages that auto-dismiss after a few seconds
 * - Support success and error variants so users can distinguish positive outcomes from problems
 * - Stack multiple toasts when several actions happen in quick succession
 */

let toastId = 0

export function toast(message, type = 'info') {
  const container = document.getElementById('toast-container')
  if (!container) return

  const id = ++toastId
  const el = document.createElement('div')
  el.id = `toast-${id}`
  el.className = `toast toast-${type}`
  el.textContent = message
  container.appendChild(el)

  requestAnimationFrame(() => {
    el.classList.add('visible')
  })

  setTimeout(() => {
    el.classList.remove('visible')
    setTimeout(() => el.remove(), 300)
  }, 3000)
}

toast.success = (msg) => toast(msg, 'success')
toast.error = (msg) => toast(msg, 'error')
