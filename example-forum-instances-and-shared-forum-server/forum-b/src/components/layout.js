/*
 * Page Layout Shell
 *
 * Assembles the persistent page structure that wraps every forum view, providing consistent navigation and visual framing.
 *
 * It must:
 * - Render the header, sidebar, and main content area in a responsive three-column layout
 * - Provide both desktop and mobile sidebar variants for navigation on all screen sizes
 * - Re-render the header when the user's auth state changes (sign in/out)
 * - Load sidebar data (categories, channels, voice rooms) and update the sidebar once ready
 * - Add decorative floating star elements to maintain the gothic visual theme
 */

import { renderHeader, cleanupHeader } from './header.js'
import { renderSidebar, renderMobileSidebar, loadSidebarData, closeMobileSidebar } from './sidebar.js'
import { authStore } from '../lib/auth.js'

export function renderLayout(container) {
  container.innerHTML = `
    <div id="header-container"></div>
    <div class="layout-wrapper">
      <div id="sidebar-container"></div>
      <div class="layout-main">
        <div id="page-content"></div>
      </div>
    </div>
    <div id="mobile-sidebar-container"></div>
  `

  renderHeader(document.getElementById('header-container'))
  renderSidebar(document.getElementById('sidebar-container'))
  renderMobileSidebar(document.getElementById('mobile-sidebar-container'))

  loadSidebarData().then(() => {
    renderSidebar(document.getElementById('sidebar-container'))
    renderMobileSidebar(document.getElementById('mobile-sidebar-container'))
  })

  // Re-render header when auth changes
  const unsub = authStore.subscribe(() => {
    renderHeader(document.getElementById('header-container'))
  })

  // Add floating star decorations
  spawnFloatingStars()

  return () => {
    unsub()
    cleanupHeader()
  }
}

export function getPageContainer() {
  return document.getElementById('page-content')
}

function spawnFloatingStars() {
  const container = document.getElementById('floating-decor')
  if (!container) return
  const stars = ['*', '+', '.', '*']
  for (let i = 0; i < 12; i++) {
    const star = document.createElement('div')
    star.className = 'floating-star'
    star.textContent = stars[i % stars.length]
    star.style.left = Math.random() * 100 + '%'
    star.style.top = Math.random() * 100 + '%'
    star.style.animationDelay = (Math.random() * 6) + 's'
    star.style.animationDuration = (4 + Math.random() * 4) + 's'
    star.style.fontSize = (8 + Math.random() * 8) + 'px'
    container.appendChild(star)
  }
}
